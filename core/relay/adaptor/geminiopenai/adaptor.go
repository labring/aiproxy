package geminiopenai

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
)

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeGoogleGeminiOpenAI, &Adaptor{})
}

const baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "https://ai.google.dev/gemini-api/docs/openai\nGoogle Gemini OpenAI-compatible endpoint",
		Models: gemini.ModelList,
	}
}
