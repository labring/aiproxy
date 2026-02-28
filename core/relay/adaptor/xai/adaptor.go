package xai

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.x.ai/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	return a.Adaptor.DoResponse(meta, store, c, resp)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Models: ModelList,
	}
}
