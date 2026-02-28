package gemini

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct{}

const baseURL = "https://generativelanguage.googleapis.com"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Anthropic ||
		m == mode.Embeddings ||
		m == mode.Gemini
}

var v1ModelMap = map[string]struct{}{}

func getRequestURL(meta *meta.Meta, action string) adaptor.RequestURL {
	u := meta.Channel.BaseURL
	if u == "" {
		u = baseURL
	}

	version := "v1beta"
	if _, ok := v1ModelMap[meta.ActualModel]; ok {
		version = "v1"
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s/%s/models/%s:%s", u, version, meta.ActualModel, action),
	}
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	var action string
	switch meta.Mode {
	case mode.Embeddings:
		action = "batchEmbedContents"
	default:
		action = "generateContent"
	}

	if meta.GetBool("stream") ||
		(meta.Mode == mode.Gemini && utils.IsGeminiStreamRequest(c.Request.URL.Path)) {
		action = "streamGenerateContent?alt=sse"
	}

	return getRequestURL(meta, action), nil
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("X-Goog-Api-Key", meta.Channel.Key)
	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.Embeddings:
		return ConvertEmbeddingRequest(meta, req)
	case mode.ChatCompletions:
		return ConvertRequest(meta, req)
	case mode.Anthropic:
		return ConvertClaudeRequest(meta, req)
	case mode.Gemini:
		return NativeConvertRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequest(req, meta.RequestTimeout)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	switch meta.Mode {
	case mode.Embeddings:
		return EmbeddingHandler(meta, c, resp)
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			return StreamHandler(meta, c, resp)
		}
		return Handler(meta, c, resp)
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			return ClaudeStreamHandler(meta, c, resp)
		}
		return ClaudeHandler(meta, c, resp)
	case mode.Gemini:
		// For Gemini mode (native format), pass through the response as-is
		if utils.IsStreamResponse(resp) {
			return NativeStreamHandler(meta, c, resp)
		}
		return NativeHandler(meta, c, resp)
	default:
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "https://ai.google.dev\nChat、Embeddings、Image generation Support",
		Models: ModelList,
	}
}
