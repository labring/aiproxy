package mistral

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
)

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeMistral, &Adaptor{})
}

const baseURL = "https://api.mistral.ai/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Mistral API\nOpenAI-compatible endpoint\nSupports Gemini-compatible request conversion",
		Models: ModelList,
	}
}
