package ppio

import (
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.ppio.com/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "PPIO API Key，从 https://ppio.com 控制台获取",
		Readme:  "PPIO 派欧云 API\nOpenAI 兼容接口，支持 DeepSeek、Qwen、MiniMax、GLM 等模型\n文档：https://ppio.com/docs/model/llm.md",
		Models:  ModelList,
	}
}
