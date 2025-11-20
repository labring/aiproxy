package doubao

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

func GetRequestURL(meta *meta.Meta) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL
	switch meta.Mode {
	case mode.ChatCompletions, mode.Anthropic:
		if strings.HasPrefix(meta.ActualModel, "bot-") {
			url, err := url.JoinPath(u, "/api/v3/bots/chat/completions")
			if err != nil {
				return adaptor.RequestURL{}, err
			}

			return adaptor.RequestURL{
				Method: http.MethodPost,
				URL:    url,
			}, nil
		}

		url, err := url.JoinPath(u, "/api/v3/chat/completions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Embeddings:
		if strings.Contains(meta.ActualModel, "vision") {
			url, err := url.JoinPath(u, "/api/v3/embeddings/multimodal")
			if err != nil {
				return adaptor.RequestURL{}, err
			}

			return adaptor.RequestURL{
				Method: http.MethodPost,
				URL:    url,
			}, nil
		}

		url, err := url.JoinPath(u, "/api/v3/embeddings")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported relay mode %d for doubao", meta.Mode)
	}
}

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://ark.cn-beijing.volces.com"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Anthropic ||
		m == mode.Embeddings
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Bot support\nNetwork search metering support",
		Models: ModelList,
	}
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	return GetRequestURL(meta)
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.Embeddings:
		if strings.Contains(meta.ActualModel, "vision") {
			return openai.ConvertEmbeddingsRequest(meta, req, false, patchEmbeddingsVisionInput)
		}
		return openai.ConvertEmbeddingsRequest(meta, req, true)
	case mode.ChatCompletions:
		return ConvertChatCompletionsRequest(meta, req)
	default:
		return openai.ConvertRequest(meta, store, req)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		websearchCount := int64(0)
		if utils.IsStreamResponse(resp) {
			usage, err = openai.StreamHandler(meta, c, resp, newHandlerPreHandler(&websearchCount))
		} else {
			usage, err = openai.Handler(meta, c, resp, newHandlerPreHandler(&websearchCount))
		}

		usage.WebSearchCount += model.ZeroNullInt64(websearchCount)
	case mode.Embeddings:
		usage, err = openai.EmbeddingsHandler(
			meta,
			c,
			resp,
			embeddingPreHandler,
		)
	default:
		return openai.DoResponse(meta, store, c, resp)
	}

	return usage, err
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}
