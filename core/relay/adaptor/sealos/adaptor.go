package sealos

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
)

type Adaptor struct {
	openai.Adaptor
	baseURL string
}

func init() {
	registry.Register(model.ChannelTypeAIProxyHZH, &Adaptor{
		baseURL: "https://aiproxy.hzh.sealos.run/v1",
	})
	registry.Register(model.ChannelTypeAIProxyUSW1, &Adaptor{
		baseURL: "https://aiproxy.usw-1.sealos.io/v1",
	})
}

func (a *Adaptor) DefaultBaseURL() string {
	return a.baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "Use a Sealos AI Proxy API key",
		Readme: "Sealos AIProxy OpenAI-compatible API\n" +
			"Configure model IDs available to your API key; GET /v1/models lists accessible models\n" +
			"Supports Anthropic/Gemini request conversion\n" +
			"Endpoint: " + a.DefaultBaseURL(),
		ConfigSchema: openai.ConfigSchema(),
	}
}
