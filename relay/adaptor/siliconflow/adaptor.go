package siliconflow

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/model"
	"github.com/labring/aiproxy/relay/adaptor"
	"github.com/labring/aiproxy/relay/adaptor/openai"
	"github.com/labring/aiproxy/relay/meta"
	"github.com/labring/aiproxy/relay/mode"
	relaymodel "github.com/labring/aiproxy/relay/model"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.siliconflow.cn/v1"

func (a *Adaptor) GetBaseURL() string {
	return baseURL
}

func (a *Adaptor) GetModelList() []*model.ModelConfig {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return "siliconflow"
}

//nolint:gocritic
func (a *Adaptor) DoResponse(meta *meta.Meta, c *gin.Context, resp *http.Response) (*relaymodel.Usage, *relaymodel.ErrorWithStatusCode) {
	usage, err := a.Adaptor.DoResponse(meta, c, resp)
	if err != nil {
		return nil, err
	}
	switch meta.Mode {
	case mode.AudioSpeech:
		size := c.Writer.Size()
		usage = &relaymodel.Usage{
			CompletionTokens: int64(size),
			TotalTokens:      int64(size),
		}
	}
	return usage, nil
}
