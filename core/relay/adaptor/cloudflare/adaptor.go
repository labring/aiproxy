package cloudflare

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.cloudflare.com"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

// WorkerAI cannot be used across accounts with AIGateWay
// https://developers.cloudflare.com/ai-gateway/providers/workersai/#openai-compatible-endpoints
// https://gateway.ai.cloudflare.com/v1/{account_id}/{gateway_id}/workers-ai
func isAIGateWay(baseURL string) bool {
	return strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") &&
		strings.HasSuffix(baseURL, "/workers-ai")
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL
	isAIGateWay := isAIGateWay(u)

	var urlPrefix string
	if isAIGateWay {
		urlPrefix = u
	} else {
		urlPrefix = fmt.Sprintf("%s/client/v4/accounts/%s/ai", u, meta.Channel.Key)
	}

	switch meta.Mode {
	case mode.ChatCompletions:
		url, err := url.JoinPath(urlPrefix, "/v1/chat/completions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Embeddings:
		url, err := url.JoinPath(urlPrefix, "/v1/embeddings")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	default:
		if isAIGateWay {
			url, err := url.JoinPath(urlPrefix, meta.ActualModel)
			if err != nil {
				return adaptor.RequestURL{}, err
			}

			return adaptor.RequestURL{
				Method: http.MethodPost,
				URL:    url,
			}, nil
		}

		url, err := url.JoinPath(urlPrefix, "/run", meta.ActualModel)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Models: ModelList,
	}
}
