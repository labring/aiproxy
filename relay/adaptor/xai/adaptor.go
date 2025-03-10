package xai

import (
	"github.com/labring/aiproxy/model"
	"github.com/labring/aiproxy/relay/adaptor/openai"
)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.xai.com/v1"

func (a *Adaptor) GetBaseURL() string {
	return baseURL
}

func (a *Adaptor) GetModelList() []*model.ModelConfig {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return "xai"
}
