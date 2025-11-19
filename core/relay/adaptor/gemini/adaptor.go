package gemini

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
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
		(meta.Mode != mode.Gemini && utils.IsGeminiStreamRequest(c.Request.URL.Path)) {
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
		// For native Gemini mode, pass through the request body but set proper headers
		body, err := common.GetRequestBodyReusable(req)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}

		return adaptor.ConvertResult{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"Content-Length": {strconv.Itoa(len(body))},
			},
			Body: bytes.NewReader(body),
		}, nil
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
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.Embeddings:
		usage, err = EmbeddingHandler(meta, c, resp)
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			usage, err = StreamHandler(meta, c, resp)
		} else {
			usage, err = Handler(meta, c, resp)
		}
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			usage, err = ClaudeStreamHandler(meta, c, resp)
		} else {
			usage, err = ClaudeHandler(meta, c, resp)
		}
	case mode.Gemini:
		// For Gemini mode (native format), pass through the response as-is
		if utils.IsStreamResponse(resp) {
			usage, err = NativeStreamHandler(meta, c, resp)
		} else {
			usage, err = NativeHandler(meta, c, resp)
		}
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}

	return usage, err
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"https://ai.google.dev",
			"Chat、Embeddings、Image generation Support",
		},
		Models: ModelList,
	}
}
