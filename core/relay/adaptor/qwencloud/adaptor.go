package qwencloud

import (
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
	registry.Register(model.ChannelTypeQwenCloud, &Adaptor{})
}

func (a *Adaptor) DefaultBaseURL() string {
	return "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	return m == mode.ChatCompletions ||
		m == mode.Embeddings ||
		m == mode.Responses ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "Get an API key at https://home.qwencloud.com/api-keys",
		Readme: "QwenCloud OpenAI-compatible API\n" +
			"Supports chat completions, embeddings, Responses, and Anthropic/Gemini request conversion\n" +
			"The default endpoint is for international API keys\n" +
			"For Qwen AI Platform in mainland China, use https://dashscope.aliyuncs.com/compatible-mode/v1 with a China API key\n" +
			"Documentation: https://docs.qwencloud.com/developer-guides/getting-started/introduction\n" +
			"Models: https://www.qwencloud.com/models\n" +
			"Pricing: https://docs.qwencloud.com/developer-guides/getting-started/pricing",
		ConfigSchema: openai.ConfigSchema(),
		Models:       ModelList,
	}
}
