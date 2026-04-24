package zhipucoding

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/adaptor/zhipu"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeZhipuCoding, &Adaptor{})
}

const baseURL = "https://open.bigmodel.cn"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	originalBaseURL := meta.Channel.BaseURL

	switch meta.Mode {
	case mode.Anthropic:
		u, err := url.JoinPath(originalBaseURL, "/api/anthropic/v1/messages")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u,
		}, nil
	case mode.ChatCompletions, mode.Completions, mode.Gemini:
		u, err := url.JoinPath(originalBaseURL, "/api/coding/paas/v4")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		originalMode := meta.Mode
		if meta.Mode == mode.Gemini {
			meta.Mode = mode.ChatCompletions
		}

		meta.Channel.BaseURL = u
		defer func() {
			meta.Mode = originalMode
			meta.Channel.BaseURL = originalBaseURL
		}()

		return a.Adaptor.GetRequestURL(meta, store, c)
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
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
	case mode.ChatCompletions:
		return openai.ConvertChatCompletionsRequest(meta, req, false)
	case mode.Completions:
		return openai.ConvertCompletionsRequest(meta, req)
	case mode.Gemini:
		return openai.ConvertGeminiRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, zhipu.ErrorHandler(resp)
	}

	switch meta.Mode {
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			return anthropic.StreamHandler(meta, c, resp)
		}
		return anthropic.Handler(meta, c, resp)
	case mode.Gemini:
		if utils.IsStreamResponse(resp) {
			return openai.GeminiStreamHandler(meta, c, resp)
		}
		return openai.GeminiHandler(meta, c, resp)
	case mode.ChatCompletions, mode.Completions:
		return a.Adaptor.DoResponse(meta, store, c, resp)
	default:
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Zhipu Coding endpoint\nChat and completions are routed to `/api/coding/paas/v4`\nAnthropic-compatible requests are routed to `/api/anthropic/v1/messages`\nGemini-compatible requests are converted to chat completions",
		Models: zhipu.ModelList,
	}
}
