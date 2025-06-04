package textembeddingsinference

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

// text-embeddings-inference adaptor supports rerank and embeddings models deployed by
// https://github.com/huggingface/text-embeddings-inference
type Adaptor struct{}

// base url for text-embeddings-inference, fake
const baseURL = "https://api.text-embeddings.net"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"https://github.com/huggingface/text-embeddings-inference",
			"Embeddings、Rerank Support",
		},
		Models: ModelList,
	}
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	switch meta.Mode {
	case mode.Rerank:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    meta.Channel.BaseURL + "/rerank",
		}, nil
	case mode.Embeddings:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    meta.Channel.BaseURL + "/v1/embeddings",
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

// text-embeddings-inference api see
// https://huggingface.github.io/text-embeddings-inference/#/Text%20Embeddings%20Inference/rerank

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)
	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.Rerank:
		return ConvertRerankRequest(meta, req)
	case mode.Embeddings:
		return openai.ConvertRequest(meta, store, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoRequest(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequest(req)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	switch meta.Mode {
	case mode.Rerank:
		return RerankHandler(meta, c, resp)
	case mode.Embeddings:
		return EmbeddingsHandler(meta, store, c, resp)
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}
}
