package ppio

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/passthrough"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func init() {
	registry.Register(model.ChannelTypePPIO, &Adaptor{})
}

var _ adaptor.Adaptor = (*Adaptor)(nil)

// Adaptor is the PPIO adaptor. It embeds passthrough.Adaptor for zero-copy
// request/response forwarding and only overrides URL routing.
type Adaptor struct {
	passthrough.Adaptor
}

// ConvertRequest overrides passthrough for WebSearch: strips the "model" field
// which PPIO's /v3/web-search endpoint does not accept.
func (a *Adaptor) ConvertRequest(
	m *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	if m.Mode == mode.WebSearch {
		return openai.ConvertWebSearchRequest(m, req)
	}

	return a.Adaptor.ConvertRequest(m, store, req)
}

const (
	baseURL = "https://api.ppinfra.com/v3/openai"
	// PPIO serves Responses API on a different path prefix than chat/completions.
	// chat/completions → /v3/openai/chat/completions
	// responses        → /openai/v1/responses
	responsesBaseURL = "https://api.ppinfra.com/openai/v1"
	// PPIO web-search lives at /v3/web-search (not under /v3/openai/).
	webSearchBaseURL = "https://api.ppinfra.com/v3"

	// PathPrefixResponses and PathPrefixWebSearch are the path_base_map keys
	// used to look up per-channel base URL overrides for each endpoint family.
	PathPrefixResponses = "/v1/responses"
	PathPrefixWebSearch = "/v1/web-search"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

// GetRequestURL builds the upstream URL with PPIO-specific path handling.
//
// Responses and web-search use different base URLs than chat/completions on PPIO.
// The base is resolved in priority order:
//  1. path_base_map in channel Configs (set automatically by EnsurePPIOChannels)
//  2. String-replacing the OpenAI path segment in the channel's BaseURL (legacy)
//  3. Hardcoded fallback constants
//
// For all other modes (chat, embeddings, etc.) the passthrough adaptor handles routing.
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
		pbm := passthrough.GetPathBaseMap(m.ChannelConfigs)
		rb := pbm[PathPrefixResponses]
		if rb == "" {
			rb = ResponsesBase(m.Channel.BaseURL)
		}

		return openai.ResponsesURL(rb, m.Mode, m.ResponseID)

	case mode.WebSearch:
		pbm := passthrough.GetPathBaseMap(m.ChannelConfigs)
		wb := pbm[PathPrefixWebSearch]
		if wb == "" {
			wb = WebSearchBase(m.Channel.BaseURL)
		}

		u, err := url.JoinPath(wb, "/web-search")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodPost, URL: u}, nil

	case mode.Anthropic, mode.Gemini:
		// Request already converted to OpenAI format upstream; route to /chat/completions.
		u, err := url.JoinPath(m.Channel.BaseURL, "/chat/completions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodPost, URL: u}, nil

	default:
		return a.Adaptor.GetRequestURL(m, store, c)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "PPIO API Key，从 https://ppinfra.com 控制台获取",
		Readme:  "PPIO 派欧云 API\nOpenAI 兼容接口，支持 DeepSeek、Qwen、MiniMax、GLM 等模型\n文档：https://ppinfra.com/docs/model/llm.md",
		Models:  ModelList,
		ConfigSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				model.ChannelConfigPathBaseMapKey: map[string]any{
					"type":  "keyValue",
					"title": "路径 URL 映射 (path_base_map)",
					"description": "将特定请求路径前缀路由到不同的上游 Base URL。" +
						"同步时自动填充，也可手动覆盖。" +
						"例：/v1/responses → https://api.ppinfra.com/openai/v1",
				},
				model.ChannelConfigAllowPassthroughUnknown: map[string]any{
					"type":        "boolean",
					"title":       "透传未知模型 (allow_passthrough_unknown)",
					"description": "开启后，此渠道可作为兜底路由，将未在模型配置中注册的模型直接转发到上游，计费为零。",
				},
			},
		},
	}
}

// ResponsesBase derives the Responses API base URL from the channel's BaseURL.
func ResponsesBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/openai/v1", 1); r != channelBaseURL {
		return r
	}

	return responsesBaseURL
}

// WebSearchBase derives the web-search base URL by replacing /v3/openai with /v3.
func WebSearchBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/v3", 1); r != channelBaseURL {
		return r
	}

	return webSearchBaseURL
}
