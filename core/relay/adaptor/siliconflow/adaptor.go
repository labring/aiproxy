package siliconflow

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.siliconflow.cn/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Models: ModelList,
	}
}

//
//nolint:gocritic
func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	usage, err = a.Adaptor.DoResponse(meta, store, c, resp)
	if err != nil {
		return model.Usage{}, err
	}
	switch meta.Mode {
	case mode.AudioSpeech:
		size := c.Writer.Size()
		usage = model.Usage{
			OutputTokens: model.ZeroNullInt64(size),
			TotalTokens:  model.ZeroNullInt64(size),
		}
	}
	return usage, nil
}
