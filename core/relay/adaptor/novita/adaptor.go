package novita

import (
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

type Adaptor struct {
	passthrough.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeNovita, &Adaptor{})
}

const (
	baseURL = "https://api.novita.ai/v3/openai"
	// Novita serves Responses API at /openai/v1, same layout as PPIO.
	responsesBaseURL = "https://api.novita.ai/openai/v1"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

// GetRequestURL builds the upstream URL with Novita-specific path handling.
//
// When the channel's path_base_map config is populated (set automatically by
// EnsureNovitaChannels), the passthrough base implementation handles routing.
// Otherwise it falls back to deriving the Responses API base URL from the
// channel's BaseURL, preserving backward compatibility.
func (a *Adaptor) GetRequestURL(
	m *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	// If path_base_map is configured, let the passthrough base handle it.
	if _, ok := m.ChannelConfigs[model.ChannelConfigPathBaseMapKey]; ok {
		return a.Adaptor.GetRequestURL(m, store, c)
	}

	// Legacy fallback: derive Responses API base URL from channel BaseURL.
	switch m.Mode {
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		rb := responsesBase(m.Channel.BaseURL)

		return openai.ResponsesURL(rb, m.Mode, m.ResponseID)

	default:
		return a.Adaptor.GetRequestURL(m, store, c)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Novita AI API\nOpenAI-compatible endpoint\nSupports chat, embeddings, Responses API and passthrough forwarding",
		Models: ModelList,
		ConfigSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				model.ChannelConfigPathBaseMapKey: map[string]any{
					"type":  "keyValue",
					"title": "路径 URL 映射 (path_base_map)",
					"description": "将特定请求路径前缀路由到不同的上游 Base URL。" +
						"同步时自动填充，也可手动覆盖。" +
						"例：/v1/responses → https://api.novita.ai/openai/v1",
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
// Novita uses /openai/v1 for Responses instead of /v3/openai.
func responsesBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/openai/v1", 1); r != channelBaseURL {
		return r
	}

	return responsesBaseURL
}
