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
		Readme: "Gemini support",
		Models: ModelList,
	}
}

//

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.AudioSpeech:
		_, err = a.Adaptor.DoResponse(meta, store, c, resp)
		if err != nil {
			return model.Usage{}, err
		}

		size := c.Writer.Size()
		usage = model.Usage{
			OutputTokens: model.ZeroNullInt64(size),
			TotalTokens:  model.ZeroNullInt64(size),
		}
	case mode.Rerank:
		if resp.StatusCode != http.StatusOK {
			return model.Usage{}, ErrorHandler(resp)
		}

		usage, err = a.Adaptor.DoResponse(meta, store, c, resp)
		if err != nil {
			return model.Usage{}, err
		}
	default:
		usage, err = a.Adaptor.DoResponse(meta, store, c, resp)
		if err != nil {
			return model.Usage{}, err
		}
	}

	return usage, nil
}
