package zhipu

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeZhipu, &Adaptor{})
}

const baseURL = "https://open.bigmodel.cn/api/paas/v4"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	switch meta.Mode {
	case mode.Embeddings:
		return EmbeddingsHandler(c, resp)
	default:
		if resp.StatusCode != http.StatusOK {
			return adaptor.DoResponseResult{}, ErrorHandler(resp)
		}
		return openai.DoResponse(meta, store, c, resp)
	}
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Zhipu AI BigModel API\nOpenAI-compatible endpoint\nSupports Gemini-compatible request conversion",
		Models: ModelList,
	}
}
