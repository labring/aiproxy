package ppio

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
)

func init() {
	registry.Register(model.ChannelTypePPIO, &Adaptor{})
}

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.ppinfra.com/v3/openai"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "PPIO API Key，从 https://ppinfra.com 控制台获取",
		Readme:  "PPIO 派欧云 API\nOpenAI 兼容接口，支持 DeepSeek、Qwen、MiniMax、GLM 等模型\n文档：https://ppinfra.com/docs/model/llm.md",
		Models:  ModelList,
	}
}
