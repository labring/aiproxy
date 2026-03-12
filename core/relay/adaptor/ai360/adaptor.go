package ai360

import (
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://ai.360.cn/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "360 AI open platform\nOpenAI-compatible chat endpoint\nSupports Gemini-compatible request conversion",
		Models: ModelList,
	}
}
