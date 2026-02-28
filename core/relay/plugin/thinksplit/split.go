package thinksplit

import (
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/noop"
	"github.com/labring/aiproxy/core/relay/plugin/thinksplit/splitter"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ plugin.Plugin = (*ThinkPlugin)(nil)

// ThinkPlugin implements the think content splitting functionality
type ThinkPlugin struct {
	noop.Noop
}

// NewThinkPlugin creates a new think plugin instance
func NewThinkPlugin() plugin.Plugin {
	return &ThinkPlugin{}
}

// getConfig retrieves the plugin configuration
func (p *ThinkPlugin) getConfig(meta *meta.Meta) (*Config, error) {
	pluginConfig := &Config{}
	if err := meta.ModelConfig.LoadPluginConfig("think-split", pluginConfig); err != nil {
		return nil, err
	}

	return pluginConfig, nil
}

// DoResponse handles the response processing to split think content
func (p *ThinkPlugin) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
) (adaptor.DoResponseResult, adaptor.Error) {
	// Only process chat completions
	if meta.Mode != mode.ChatCompletions {
		return do.DoResponse(meta, store, c, resp)
	}

	// Check if think splitting is enabled
	pluginConfig, err := p.getConfig(meta)
	if err != nil || !pluginConfig.Enable {
		return do.DoResponse(meta, store, c, resp)
	}

	return p.handleResponse(meta, store, c, resp, do)
}

// handleResponse processes streaming responses
func (p *ThinkPlugin) handleResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
) (adaptor.DoResponseResult, adaptor.Error) {
	// Create a custom response writer
	rw := &thinkResponseWriter{
		ResponseWriter: c.Writer,
	}

	c.Writer = rw
	defer func() {
		c.Writer = rw.ResponseWriter
	}()

	return do.DoResponse(meta, store, c, resp)
}

// thinkResponseWriter wraps the response writer for streaming responses
type thinkResponseWriter struct {
	gin.ResponseWriter
	thinkSplitter *splitter.Splitter
	isStream      bool
	done          bool
}

func (rw *thinkResponseWriter) getThinkSplitter() *splitter.Splitter {
	if rw.thinkSplitter == nil {
		rw.thinkSplitter = splitter.NewThinkSplitter()
	}
	return rw.thinkSplitter
}

// ignore WriteHeaderNow
func (rw *thinkResponseWriter) WriteHeaderNow() {}

func (rw *thinkResponseWriter) Write(b []byte) (int, error) {
	if rw.done {
		return rw.ResponseWriter.Write(b)
	}
	// For streaming responses, process each chunk
	node, err := sonic.Get(b)
	if err != nil || !node.Valid() {
		return rw.ResponseWriter.Write(b)
	}

	// Process the chunk
	respMap, err := node.Map()
	if err != nil {
		return rw.ResponseWriter.Write(b)
	}

	// Check if this is a streaming response chunk
	if rw.isStream || utils.IsStreamResponseWithHeader(rw.Header()) {
		rw.isStream = true

		rw.done = StreamSplitThink(respMap, rw.getThinkSplitter())

		jsonData, err := sonic.Marshal(respMap)
		if err != nil {
			return rw.ResponseWriter.Write(b)
		}

		return rw.ResponseWriter.Write(jsonData)
	}

	rw.done = true
	SplitThink(respMap, rw.getThinkSplitter())

	jsonData, err := sonic.Marshal(respMap)
	if err != nil {
		return rw.ResponseWriter.Write(b)
	}

	if rw.ResponseWriter.Header().Get("Content-Length") != "" {
		rw.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(jsonData)))
	}

	return rw.ResponseWriter.Write(jsonData)
}

func (rw *thinkResponseWriter) WriteString(s string) (int, error) {
	return rw.Write(conv.StringToBytes(s))
}

// renderCallback maybe reuse data, so don't modify data
func StreamSplitThink(data map[string]any, thinkSplitter *splitter.Splitter) (done bool) {
	choices, ok := data["choices"].([]any)
	// only support one choice
	if !ok || len(choices) != 1 {
		return false
	}

	choice := choices[0]

	choiceMap, ok := choice.(map[string]any)
	if !ok {
		return false
	}

	delta, ok := choiceMap["delta"].(map[string]any)
	if !ok {
		return false
	}

	content, ok := delta["content"].(string)
	if !ok {
		return false
	}

	if _, ok := delta["reasoning_content"].(string); ok {
		return true
	}

	think, remaining := thinkSplitter.Process(conv.StringToBytes(content))
	if len(think) == 0 && len(remaining) == 0 {
		delta["content"] = ""
		delete(delta, "reasoning_content")
		return false
	}

	if len(think) != 0 && len(remaining) != 0 {
		delta["content"] = conv.BytesToString(remaining)
		delta["reasoning_content"] = conv.BytesToString(think)
		return false
	}

	if len(think) > 0 {
		delta["content"] = ""
		delta["reasoning_content"] = conv.BytesToString(think)
		return false
	}

	if len(remaining) > 0 {
		delta["content"] = conv.BytesToString(remaining)
		delete(delta, "reasoning_content")
		return true
	}

	return false
}

func SplitThink(data map[string]any, thinkSplitter *splitter.Splitter) {
	choices, ok := data["choices"].([]any)
	if !ok {
		return
	}

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]any)
		if !ok {
			continue
		}

		message, ok := choiceMap["message"].(map[string]any)
		if !ok {
			continue
		}

		content, ok := message["content"].(string)
		if !ok {
			continue
		}

		if _, ok := message["reasoning_content"].(string); ok {
			continue
		}

		think, remaining := thinkSplitter.Process(conv.StringToBytes(content))
		message["reasoning_content"] = conv.BytesToString(think)
		message["content"] = conv.BytesToString(remaining)
	}
}
