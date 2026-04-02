package ppio

import (
	"fmt"
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

// GetRequestURL overrides the OpenAI adaptor to route Responses API requests
// to PPIO's dedicated path prefix (/openai/v1/responses instead of
// /v3/openai/responses). This applies both to explicit Responses modes AND
// to responses-only models accessed via ChatCompletions/Anthropic/Gemini modes,
// where the OpenAI adaptor would otherwise use the wrong path prefix.
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
		// Compute the rewritten base only for Responses modes.
		rb := responsesBase(m.Channel.BaseURL)

		return responsesURL(rb, m.Mode, m.ResponseID)

	case mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		// Responses-only models (e.g. gpt-5.4-pro, gpt-5.3-codex) called via
		// ChatCompletions are internally converted to the Responses API. PPIO
		// serves Responses at /openai/v1/responses, not /v3/openai/responses,
		// so we must rewrite the base URL here instead of delegating to OpenAI.
		if openai.IsResponsesOnlyModel(&m.ModelConfig, m.ActualModel) {
			rb := responsesBase(m.Channel.BaseURL)

			return responsesURL(rb, mode.Responses, m.ResponseID)
		}

		return a.Adaptor.GetRequestURL(m, store, c)

	default:
		return a.Adaptor.GetRequestURL(m, store, c)
	}
}

func responsesURL(base string, m mode.Mode, responseID string) (adaptor.RequestURL, error) {
	switch m {
	case mode.Responses:
		u, err := url.JoinPath(base, "/responses")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodPost, URL: u}, nil

	case mode.ResponsesGet:
		u, err := url.JoinPath(base, "/responses", responseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodGet, URL: u}, nil

	case mode.ResponsesDelete:
		u, err := url.JoinPath(base, "/responses", responseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodDelete, URL: u}, nil

	case mode.ResponsesCancel:
		u, err := url.JoinPath(base, "/responses", responseID, "cancel")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodPost, URL: u}, nil

	case mode.ResponsesInputItems:
		u, err := url.JoinPath(base, "/responses", responseID, "input_items")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodGet, URL: u}, nil

	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported responses mode: %s", m)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "PPIO API Key，从 https://ppinfra.com 控制台获取",
		Readme:  "PPIO 派欧云 API\nOpenAI 兼容接口，支持 DeepSeek、Qwen、MiniMax、GLM 等模型\n文档：https://ppinfra.com/docs/model/llm.md",
		Models:  ModelList,
	}
}
