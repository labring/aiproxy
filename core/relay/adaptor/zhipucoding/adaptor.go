package zhipucoding

import (
	"net/http"
	"net/url"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/zhipu"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://open.bigmodel.cn"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Anthropic
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	switch meta.Mode {
	case mode.Anthropic:
		url, err := url.JoinPath(u, "/api/anthropic/v1/messages")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	default:
		meta.Channel.BaseURL += "/api/coding/paas/v4"
		defer func() {
			meta.Channel.BaseURL = u
		}()

		return a.Adaptor.GetRequestURL(meta, store, c)
	}
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.Anthropic:
		return anthropic.ConvertRequest(meta, req, func(node *ast.Node) error {
			if !node.Get("max_tokens").Exists() {
				_, err := node.Set("max_tokens", ast.NewNumber("4096"))
				return err
			}

			return nil
		})
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
	switch meta.Mode {
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			return anthropic.StreamHandler(meta, c, resp)
		}
		return anthropic.Handler(meta, c, resp)
	default:
		return a.Adaptor.DoResponse(meta, store, c, resp)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Models: zhipu.ModelList,
	}
}
