package adaptors

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/ai360"
	"github.com/labring/aiproxy/core/relay/adaptor/ali"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/aws"
	"github.com/labring/aiproxy/core/relay/adaptor/azure"
	"github.com/labring/aiproxy/core/relay/adaptor/azure2"
	"github.com/labring/aiproxy/core/relay/adaptor/baichuan"
	"github.com/labring/aiproxy/core/relay/adaptor/baidu"
	"github.com/labring/aiproxy/core/relay/adaptor/baiduv2"
	"github.com/labring/aiproxy/core/relay/adaptor/cloudflare"
	"github.com/labring/aiproxy/core/relay/adaptor/cohere"
	"github.com/labring/aiproxy/core/relay/adaptor/coze"
	"github.com/labring/aiproxy/core/relay/adaptor/deepseek"
	"github.com/labring/aiproxy/core/relay/adaptor/doc2x"
	"github.com/labring/aiproxy/core/relay/adaptor/doubao"
	"github.com/labring/aiproxy/core/relay/adaptor/doubaoaudio"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/adaptor/geminiopenai"
	"github.com/labring/aiproxy/core/relay/adaptor/groq"
	"github.com/labring/aiproxy/core/relay/adaptor/jina"
	"github.com/labring/aiproxy/core/relay/adaptor/lingyiwanwu"
	"github.com/labring/aiproxy/core/relay/adaptor/minimax"
	"github.com/labring/aiproxy/core/relay/adaptor/mistral"
	"github.com/labring/aiproxy/core/relay/adaptor/moonshot"
	"github.com/labring/aiproxy/core/relay/adaptor/novita"
	"github.com/labring/aiproxy/core/relay/adaptor/ollama"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/openrouter"
	"github.com/labring/aiproxy/core/relay/adaptor/siliconflow"
	"github.com/labring/aiproxy/core/relay/adaptor/stepfun"
	"github.com/labring/aiproxy/core/relay/adaptor/tencent"
	textembeddingsinference "github.com/labring/aiproxy/core/relay/adaptor/text-embeddings-inference"
	"github.com/labring/aiproxy/core/relay/adaptor/vertexai"
	"github.com/labring/aiproxy/core/relay/adaptor/xai"
	"github.com/labring/aiproxy/core/relay/adaptor/xunfei"
	"github.com/labring/aiproxy/core/relay/adaptor/zhipu"
	log "github.com/sirupsen/logrus"
)

var ChannelAdaptor = map[model.ChannelType]adaptor.Adaptor{
	model.ChannelTypeOpenAI:                  &openai.Adaptor{},
	model.ChannelTypeAzure:                   &azure.Adaptor{},
	model.ChannelTypeAzure2:                  &azure2.Adaptor{},
	model.ChannelTypeGoogleGeminiOpenAI:      &geminiopenai.Adaptor{},
	model.ChannelTypeBaiduV2:                 &baiduv2.Adaptor{},
	model.ChannelTypeAnthropic:               &anthropic.Adaptor{},
	model.ChannelTypeBaidu:                   &baidu.Adaptor{},
	model.ChannelTypeZhipu:                   &zhipu.Adaptor{},
	model.ChannelTypeAli:                     &ali.Adaptor{},
	model.ChannelTypeXunfei:                  &xunfei.Adaptor{},
	model.ChannelTypeAI360:                   &ai360.Adaptor{},
	model.ChannelTypeOpenRouter:              &openrouter.Adaptor{},
	model.ChannelTypeTencent:                 &tencent.Adaptor{},
	model.ChannelTypeGoogleGemini:            &gemini.Adaptor{},
	model.ChannelTypeMoonshot:                &moonshot.Adaptor{},
	model.ChannelTypeBaichuan:                &baichuan.Adaptor{},
	model.ChannelTypeMinimax:                 &minimax.Adaptor{},
	model.ChannelTypeMistral:                 &mistral.Adaptor{},
	model.ChannelTypeGroq:                    &groq.Adaptor{},
	model.ChannelTypeOllama:                  &ollama.Adaptor{},
	model.ChannelTypeLingyiwanwu:             &lingyiwanwu.Adaptor{},
	model.ChannelTypeStepfun:                 &stepfun.Adaptor{},
	model.ChannelTypeAWS:                     &aws.Adaptor{},
	model.ChannelTypeCoze:                    &coze.Adaptor{},
	model.ChannelTypeCohere:                  &cohere.Adaptor{},
	model.ChannelTypeDeepseek:                &deepseek.Adaptor{},
	model.ChannelTypeCloudflare:              &cloudflare.Adaptor{},
	model.ChannelTypeDoubao:                  &doubao.Adaptor{},
	model.ChannelTypeNovita:                  &novita.Adaptor{},
	model.ChannelTypeVertexAI:                &vertexai.Adaptor{},
	model.ChannelTypeSiliconflow:             &siliconflow.Adaptor{},
	model.ChannelTypeDoubaoAudio:             &doubaoaudio.Adaptor{},
	model.ChannelTypeXAI:                     &xai.Adaptor{},
	model.ChannelTypeDoc2x:                   &doc2x.Adaptor{},
	model.ChannelTypeJina:                    &jina.Adaptor{},
	model.ChannelTypeTextEmbeddingsInference: &textembeddingsinference.Adaptor{},
}

func GetAdaptor(channelType model.ChannelType) (adaptor.Adaptor, bool) {
	a, ok := ChannelAdaptor[channelType]
	return a, ok
}

type AdaptorMeta struct {
	Name           string                  `json:"name"`
	KeyHelp        string                  `json:"keyHelp"`
	DefaultBaseURL string                  `json:"defaultBaseUrl"`
	Fetures        []string                `json:"fetures,omitempty"`
	Config         adaptor.ConfigTemplates `json:"config,omitempty"`
}

var ChannelMetas = map[model.ChannelType]AdaptorMeta{}

func init() {
	for i, a := range ChannelAdaptor {
		adaptorMeta := a.Metadata()
		meta := AdaptorMeta{
			Name:           i.String(),
			KeyHelp:        adaptorMeta.KeyHelp,
			DefaultBaseURL: a.DefaultBaseURL(),
			Fetures:        adaptorMeta.Features,
			Config:         adaptorMeta.Config,
		}
		for key, template := range meta.Config {
			if template.Name == "" {
				log.Fatalf("config template %s is invalid: name is empty", key)
			}
			if err := adaptor.ValidateConfigTemplate(template); err != nil {
				log.Fatalf("config template %s(%s) is invalid: %v", key, template.Name, err)
			}
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
