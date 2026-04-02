package novita

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

type Adaptor struct {
	openai.Adaptor
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

// responsesBase derives the Responses API base URL from the channel's BaseURL.
// Novita uses /openai/v1 for Responses instead of /v3/openai.
func responsesBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/openai/v1", 1); r != channelBaseURL {
		return r
	}

	return responsesBaseURL
}

// GetRequestURL overrides the OpenAI adaptor to route all Responses API traffic
// (including responses-only models) to Novita's /openai/v1 prefix instead of /v3/openai.
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

		fallthrough

	default:
		return a.Adaptor.GetRequestURL(m, store, c)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Novita AI API\nOpenAI-compatible endpoint\nSupports Gemini-compatible request conversion",
		Models: ModelList,
	}
}
