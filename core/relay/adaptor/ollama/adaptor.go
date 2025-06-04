package ollama

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct{}

const baseURL = "http://localhost:11434"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	// https://github.com/ollama/ollama/blob/main/docs/api.md
	u := meta.Channel.BaseURL
	switch meta.Mode {
	case mode.Embeddings:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api/embed",
		}, nil
	case mode.ChatCompletions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api/chat",
		}, nil
	case mode.Completions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api/generate",
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

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
	_ adaptor.Store,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	if request == nil {
		return adaptor.ConvertResult{}, errors.New("request is nil")
	}
	switch meta.Mode {
	case mode.Embeddings:
		return ConvertEmbeddingRequest(meta, request)
	case mode.ChatCompletions, mode.Completions:
		return ConvertRequest(meta, request)
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
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.Embeddings:
		usage, err = EmbeddingHandler(meta, c, resp)
	case mode.ChatCompletions, mode.Completions:
		if utils.IsStreamResponse(resp) {
			usage, err = StreamHandler(meta, c, resp)
		} else {
			usage, err = Handler(meta, c, resp)
		}
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}
	return
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"Chat、Embeddings Support",
		},
		Models: ModelList,
	}
}
