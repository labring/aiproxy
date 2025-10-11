package streamlake

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
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

func isKatClaude(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "kat")
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, store adaptor.Store) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	switch {
	case meta.Mode == mode.Anthropic && isKatClaude(meta.OriginModel):
		url, err := url.JoinPath(u, meta.ActualModel, "/claude-code-proxy/v1/messages")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	default:
		return a.Adaptor.GetRequestURL(meta, store)
	}
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch {
	case meta.Mode == mode.Anthropic && isKatClaude(meta.OriginModel):
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
) (usage model.Usage, err adaptor.Error) {
	switch {
	case meta.Mode == mode.Anthropic && isKatClaude(meta.OriginModel):
		if utils.IsStreamResponse(resp) {
			usage, err = anthropic.StreamHandler(meta, c, resp)
		} else {
			usage, err = anthropic.Handler(meta, c, resp)
		}
	default:
		usage, err = a.Adaptor.DoResponse(meta, store, c, resp)
	}

	return usage, err
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Models: ModelList,
	}
}
