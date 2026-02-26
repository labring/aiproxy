package streamlake

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://wanqing.streamlakeapi.com/api/gateway/v1/endpoints"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Anthropic
}

func supportClaudeCodeProxy(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "kat") &&
		strings.Contains(strings.ToLower(modelName), "coder")
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	switch {
	case meta.Mode == mode.Anthropic && supportClaudeCodeProxy(meta.OriginModel):
		url, err := url.JoinPath(u, meta.ActualModel, "/claude-code-proxy/v1/messages")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	default:
		return a.Adaptor.GetRequestURL(meta, store, c)
	}
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch {
	case meta.Mode == mode.Anthropic && supportClaudeCodeProxy(meta.OriginModel):
		return anthropic.ConvertRequest(meta, req)
	default:
		return a.Adaptor.ConvertRequest(meta, store, req)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	var result adaptor.DoResponseResult
	var err adaptor.Error

	switch {
	case meta.Mode == mode.Anthropic && supportClaudeCodeProxy(meta.OriginModel):
		if utils.IsStreamResponse(resp) {
			result, err = anthropic.StreamHandler(meta, c, resp)
		} else {
			result, err = anthropic.Handler(meta, c, resp)
		}
	default:
		result, err = a.Adaptor.DoResponse(meta, store, c, resp)
	}

	// Handle rate limit error: convert 400 with specific message to 429
	if err != nil && strings.Contains(err.Error(), "Request rate increased too quickl") {
		err = relaymodel.WrapperError(
			meta.Mode,
			http.StatusTooManyRequests,
			err,
			relaymodel.WithType("rate_limit_error"),
		)
	}

	return result, err
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Models: ModelList,
	}
}
