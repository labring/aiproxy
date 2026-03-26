package adaptors

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	_ "github.com/labring/aiproxy/core/relay/adaptor/ai360"
	_ "github.com/labring/aiproxy/core/relay/adaptor/ali"
	_ "github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	_ "github.com/labring/aiproxy/core/relay/adaptor/aws"
	_ "github.com/labring/aiproxy/core/relay/adaptor/azure"
	_ "github.com/labring/aiproxy/core/relay/adaptor/azure2"
	_ "github.com/labring/aiproxy/core/relay/adaptor/baichuan"
	_ "github.com/labring/aiproxy/core/relay/adaptor/baidu"
	_ "github.com/labring/aiproxy/core/relay/adaptor/baiduv2"
	_ "github.com/labring/aiproxy/core/relay/adaptor/cloudflare"
	_ "github.com/labring/aiproxy/core/relay/adaptor/cohere"
	_ "github.com/labring/aiproxy/core/relay/adaptor/coze"
	_ "github.com/labring/aiproxy/core/relay/adaptor/deepseek"
	_ "github.com/labring/aiproxy/core/relay/adaptor/doc2x"
	_ "github.com/labring/aiproxy/core/relay/adaptor/doubao"
	_ "github.com/labring/aiproxy/core/relay/adaptor/doubaoaudio"
	_ "github.com/labring/aiproxy/core/relay/adaptor/fake"
	_ "github.com/labring/aiproxy/core/relay/adaptor/gemini"
	_ "github.com/labring/aiproxy/core/relay/adaptor/geminiopenai"
	_ "github.com/labring/aiproxy/core/relay/adaptor/groq"
	_ "github.com/labring/aiproxy/core/relay/adaptor/jina"
	_ "github.com/labring/aiproxy/core/relay/adaptor/lingyiwanwu"
	_ "github.com/labring/aiproxy/core/relay/adaptor/minimax"
	_ "github.com/labring/aiproxy/core/relay/adaptor/mistral"
	_ "github.com/labring/aiproxy/core/relay/adaptor/moonshot"
	_ "github.com/labring/aiproxy/core/relay/adaptor/novita"
	_ "github.com/labring/aiproxy/core/relay/adaptor/ollama"
	_ "github.com/labring/aiproxy/core/relay/adaptor/openai"
	_ "github.com/labring/aiproxy/core/relay/adaptor/openrouter"
	_ "github.com/labring/aiproxy/core/relay/adaptor/qianfan"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	_ "github.com/labring/aiproxy/core/relay/adaptor/sangforaicp"
	_ "github.com/labring/aiproxy/core/relay/adaptor/siliconflow"
	_ "github.com/labring/aiproxy/core/relay/adaptor/stepfun"
	_ "github.com/labring/aiproxy/core/relay/adaptor/streamlake"
	_ "github.com/labring/aiproxy/core/relay/adaptor/tencent"
	_ "github.com/labring/aiproxy/core/relay/adaptor/text-embeddings-inference"
	_ "github.com/labring/aiproxy/core/relay/adaptor/vertexai"
	_ "github.com/labring/aiproxy/core/relay/adaptor/xai"
	_ "github.com/labring/aiproxy/core/relay/adaptor/xunfei"
	_ "github.com/labring/aiproxy/core/relay/adaptor/zhipu"
	_ "github.com/labring/aiproxy/core/relay/adaptor/zhipucoding"
)

var ChannelAdaptor = registry.Snapshot()

func GetAdaptor(channelType model.ChannelType) (adaptor.Adaptor, bool) {
	return registry.Get(channelType)
}

type AdaptorMeta struct {
	Name           string         `json:"name"`
	KeyHelp        string         `json:"keyHelp"`
	DefaultBaseURL string         `json:"defaultBaseUrl"`
	Readme         string         `json:"readme"`
	ConfigSchema   map[string]any `json:"configSchema,omitempty"`
}

var ChannelMetas = map[model.ChannelType]AdaptorMeta{}

func init() {
	ChannelAdaptor = registry.Snapshot()
	for i, a := range ChannelAdaptor {
		adaptorMeta := a.Metadata()

		meta := AdaptorMeta{
			Name:           i.String(),
			KeyHelp:        adaptorMeta.KeyHelp,
			DefaultBaseURL: a.DefaultBaseURL(),
			Readme:         adaptorMeta.Readme,
			ConfigSchema:   adaptorMeta.ConfigSchema,
		}

		ChannelMetas[i] = meta
	}
}

var defaultKeyValidator adaptor.KeyValidator = (*KeyValidatorNoop)(nil)

type KeyValidatorNoop struct{}

func (a *KeyValidatorNoop) ValidateKey(_ string) error {
	return nil
}

func GetKeyValidator(a adaptor.Adaptor) adaptor.KeyValidator {
	if keyValidator, ok := a.(adaptor.KeyValidator); ok {
		return keyValidator
	}
	return defaultKeyValidator
}
