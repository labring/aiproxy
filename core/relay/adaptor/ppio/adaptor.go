package ppio

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func init() {
	registry.Register(model.ChannelTypePPIO, &Adaptor{})
}

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

const (
	baseURL = "https://api.ppinfra.com/v3/openai"
	// PPIO serves Responses API on a different path prefix than chat/completions.
	// chat/completions → /v3/openai/chat/completions
	// responses        → /openai/v1/responses
	responsesBaseURL = "https://api.ppinfra.com/openai/v1"
	// PPIO web-search lives at /v3/web-search (not under /v3/openai/).
	webSearchBaseURL = "https://api.ppinfra.com/v3"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

// responsesBase derives the Responses API base URL from the channel's BaseURL.
// PPIO uses /openai/v1 for Responses instead of /v3/openai.
func responsesBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/openai/v1", 1); r != channelBaseURL {
		return r
	}

	return responsesBaseURL
}

// webSearchBase derives the web-search base URL by stripping /openai from the channel's BaseURL.
func webSearchBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/v3", 1); r != channelBaseURL {
		return r
	}

	return webSearchBaseURL
}

// GetRequestURL overrides the OpenAI adaptor to route all Responses API traffic
// (including responses-only models) to PPIO's /openai/v1 prefix instead of /v3/openai.
func (a *Adaptor) GetRequestURL(
	m *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	switch m.Mode {
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		rb := responsesBase(m.Channel.BaseURL)

		return openai.ResponsesURL(rb, m.Mode, m.ResponseID)

	case mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		// Responses-only models are internally converted to the Responses API;
		// rewrite the base URL so they hit /openai/v1/responses, not /v3/openai/responses.
		if openai.IsResponsesOnlyModel(&m.ModelConfig, m.ActualModel) {
			rb := responsesBase(m.Channel.BaseURL)

			return openai.ResponsesURL(rb, mode.Responses, m.ResponseID)
		}

		return a.Adaptor.GetRequestURL(m, store, c)

	case mode.WebSearch:
		// PPIO web-search is at /v3/web-search, not /v3/openai/web-search.
		wb := webSearchBase(m.Channel.BaseURL)

		u, err := url.JoinPath(wb, "/web-search")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u,
		}, nil

	default:
		return a.Adaptor.GetRequestURL(m, store, c)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "PPIO API Key，从 https://ppinfra.com 控制台获取",
		Readme:  "PPIO 派欧云 API\nOpenAI 兼容接口，支持 DeepSeek、Qwen、MiniMax、GLM 等模型\n文档：https://ppinfra.com/docs/model/llm.md",
		Models:  ModelList,
	}
}
