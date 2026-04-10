package novita

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
	// Novita web-search lives at /v3/web-search (not under /v3/openai/).
	webSearchBaseURL = "https://api.novita.ai/v3"

	// PathPrefixWebSearch is the path_base_map key for web-search base URL override.
	PathPrefixWebSearch = "/v1/web-search"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

// ConvertRequest overrides passthrough for WebSearch: strips the "model" field
// which Novita's /v3/web-search endpoint does not accept.
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

// DoResponse delegates to the passthrough adaptor for full zero-copy relay.
// For web-search the upstream may carry no usage, so we fall back to counting
// 1 per successful request.
func (a *Adaptor) DoResponse(
	m *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	result, err := a.Adaptor.DoResponse(m, store, c, resp)
	if err != nil {
		return result, err
	}

	if m.Mode == mode.WebSearch && result.Usage.WebSearchCount == 0 {
		result.Usage.WebSearchCount = 1
	}

	return result, nil
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
	// If path_base_map is configured, let the passthrough base handle it
	// (except for WebSearch which needs model-specific path routing).
	pbm := passthrough.GetPathBaseMap(m.ChannelConfigs)

	switch m.Mode {
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		if len(pbm) > 0 {
			return a.Adaptor.GetRequestURL(m, store, c)
		}

		// Legacy fallback: derive Responses API base URL from channel BaseURL.
		rb := responsesBase(m.Channel.BaseURL)

		return openai.ResponsesURL(rb, m.Mode, m.ResponseID)

	case mode.WebSearch:
		wb := pbm[PathPrefixWebSearch]
		if wb == "" {
			wb = WebSearchBase(m.Channel.BaseURL)
		}

		// Route by model name: tavily uses a different upstream path.
		pathSuffix := "/web-search"
		if m.ActualModel == ModelNovitaTavilySearch {
			pathSuffix = "/tavily/search"
		}

		u, err := url.JoinPath(wb, pathSuffix)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodPost, URL: u}, nil

	case mode.Anthropic, mode.Gemini:
		// Request already converted to OpenAI format by the relay controller;
		// route to /chat/completions instead of the client's original path
		// (e.g. /v1/messages for Anthropic, /v1/generateContent for Gemini).
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

// WebSearchBase derives the web-search base URL by replacing /v3/openai with /v3.
func WebSearchBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/v3", 1); r != channelBaseURL {
		return r
	}

	return webSearchBaseURL
}
