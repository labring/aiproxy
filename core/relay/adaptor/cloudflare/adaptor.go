package cloudflare

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

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
	registry.Register(model.ChannelTypeCloudflare, &Adaptor{})
}

const baseURL = "https://api.cloudflare.com/client/v4/accounts/"

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

func cloudflareAIBaseURL(baseURL string) (string, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.Fragment != "" ||
		(parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return "", errors.New("invalid Cloudflare base URL")
	}
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) != 4 || pathParts[0] != "client" || pathParts[1] != "v4" ||
		pathParts[2] != "accounts" || pathParts[3] == "" {
		return "", errors.New("cloudflare base URL must include /client/v4/accounts/{account_id}")
	}

	return url.JoinPath(parsedURL.String(), "ai")
}

func cloudflareAIGatewayAccountID(baseURL string) string {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) != 4 || pathParts[0] != "v1" || pathParts[1] == "" ||
		pathParts[2] == "" || pathParts[3] != "workers-ai" {
		return ""
	}
	return pathParts[1]
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
		var err error
		urlPrefix, err = cloudflareAIBaseURL(u)
		if err != nil {
			return adaptor.RequestURL{}, err
		}
	}

	switch meta.Mode {
	case mode.ChatCompletions, mode.Gemini:
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
		Readme:       "Cloudflare Workers AI\nDefault base URL uses the account-scoped REST API\nAlso supports AI Gateway Workers AI endpoints ending with `/workers-ai`\nChat and embeddings use OpenAI-compatible paths; other modes use `/run/{model}`",
		ConfigSchema: openai.ConfigSchema(),
		Models:       ModelList,
	}
}
