package textembeddingsinference

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

// text-embeddings-inference adaptor supports rerank and embeddings models deployed by https://github.com/huggingface/text-embeddings-inference
type Adaptor struct{}

// base url for text-embeddings-inference, fake
const baseURL = "https://api.text-embeddings.net"

func (a *Adaptor) GetBaseURL() string {
	return baseURL
}

func (a *Adaptor) GetModelList() []*model.ModelConfig {
	return ModelList
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
	switch meta.Mode {
	case mode.Rerank:
		return meta.Channel.BaseURL + "/rerank", nil
	case mode.Embeddings:
		return meta.Channel.BaseURL + "/v1/embeddings", nil
	default:
		return "", fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

// text-embeddings-inference api see https://huggingface.github.io/text-embeddings-inference/#/Text%20Embeddings%20Inference/rerank

func (a *Adaptor) SetupRequestHeader(meta *meta.Meta, _ *gin.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)
	return nil
}

func (a *Adaptor) ConvertRequest(meta *meta.Meta, req *http.Request) (string, http.Header, io.Reader, error) {
	switch meta.Mode {
	case mode.Rerank:
		return ConvertRerankRequest(meta, req)
	case mode.Embeddings:
		return openai.ConvertRequest(meta, req)
	default:
		return "", nil, nil, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoRequest(_ *meta.Meta, _ *gin.Context, req *http.Request) (*http.Response, error) {
	return utils.DoRequest(req)
}

func (a *Adaptor) DoResponse(meta *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *relaymodel.ErrorWithStatusCode) {
	switch meta.Mode {
	case mode.Rerank:
		return RerankHandler(meta, c, resp)
	case mode.Embeddings:
		return EmbeddingsHandler(meta, c, resp)
	default:
		return nil, openai.ErrorWrapperWithMessage(fmt.Sprintf("unsupported mode: %s", meta.Mode), "unsupported_mode", http.StatusBadRequest)
	}
}
