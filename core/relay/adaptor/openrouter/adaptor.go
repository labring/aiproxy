package openrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeOpenRouter, &Adaptor{})
}

const baseURL = "https://openrouter.ai/api/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			return openai.StreamHandler(
				meta,
				c,
				resp,
				openai.StreamReasoningToReasoningContentPreHandler,
			)
		}

		return openai.Handler(meta, c, resp, openai.ReasoningToReasoningContentPreHandler)
	default:
		return openai.DoResponse(meta, store, c, resp)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "OpenRouter OpenAI-compatible endpoint\nThe upstream `reasoning` field is normalized to `reasoning_content`\nAlso supports Gemini-compatible request conversion",
		Models: openai.ModelList,
	}
}
