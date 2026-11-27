package tokendance

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
	registry.Register(model.ChannelTypeTokenDance, &Adaptor{})
}

func (a *Adaptor) DefaultBaseURL() string {
	return "https://tokendance.space/gateway/v1"
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	return m == mode.ChatCompletions ||
		m == mode.Responses ||
		m == mode.Embeddings ||
		m == mode.ImagesGenerations ||
		m == mode.ImagesEdits ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "sk-xxx",
		Readme: "TokenDance OpenAI-compatible API\n" +
			"Supports chat completions, Responses, embeddings, image generation and editing\n" +
			"Anthropic/Gemini requests are converted to OpenAI-compatible requests\n" +
			"Configure model IDs with the corresponding OpenAI protocol in supported_protocols\n" +
			"API keys: https://tokendance.space/keys\n" +
			"Model catalog: https://tokendance.space/gateway/v1/models\n" +
			"Documentation: https://tokendance.space/docs/quickstart",
		ConfigSchema: openai.ConfigSchema(),
	}
}
