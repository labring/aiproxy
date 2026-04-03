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

// GetRequestURL builds the upstream URL with PPIO-specific path handling.
//
// When the channel's path_base_map config is populated (set automatically by
// EnsurePPIOChannels), the passthrough base implementation handles routing.
// Otherwise it falls back to deriving the Responses API and web-search base
// URLs from the channel's BaseURL, preserving backward compatibility with
// channels created before path_base_map was introduced.
func (a *Adaptor) GetRequestURL(
	m *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	// If path_base_map is configured, let the passthrough base handle it.
	if _, ok := m.ChannelConfigs[model.ChannelConfigPathBaseMapKey]; ok {
		return a.Adaptor.GetRequestURL(m, store, c)
	}

	// Legacy fallback: derive the Responses and web-search base URLs by
	// string-replacing the OpenAI path segment in the channel's BaseURL.
	// This supports existing channels that were created before the path_base_map
	// config was introduced.
	switch m.Mode {
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		rb := responsesBase(m.Channel.BaseURL)

		return openai.ResponsesURL(rb, m.Mode, m.ResponseID)

	case mode.WebSearch:
		wb := webSearchBase(m.Channel.BaseURL)

		u, err := url.JoinPath(wb, "/web-search")
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

// responsesBase derives the Responses API base URL from the channel's BaseURL.
func responsesBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/openai/v1", 1); r != channelBaseURL {
		return r
	}

	return responsesBaseURL
}

// webSearchBase derives the web-search base URL by replacing /v3/openai with /v3.
func webSearchBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/v3", 1); r != channelBaseURL {
		return r
	}

	return webSearchBaseURL
}
