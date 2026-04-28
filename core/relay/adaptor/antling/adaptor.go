package antling

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeAntLing, &Adaptor{})
}

const baseURL = "https://api.tbox.cn/api"

const (
	chatCompletionsPath   = "/llm/v1/chat/completions"
	anthropicMessagesPath = "/anthropic/v1/messages"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	return m == mode.ChatCompletions ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	var endpointPath string

	switch meta.Mode {
	case mode.ChatCompletions, mode.Gemini:
		endpointPath = chatCompletionsPath
	case mode.Anthropic:
		endpointPath = anthropicMessagesPath
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}

	targetURL, err := url.JoinPath(meta.Channel.BaseURL, endpointPath)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	if meta.Mode == mode.Anthropic && c != nil {
		if beta := c.Query("beta"); beta != "" {
			parsedURL, err := url.Parse(targetURL)
			if err != nil {
				return adaptor.RequestURL{}, err
			}

			queryValues := parsedURL.Query()
			queryValues.Set("beta", beta)
			parsedURL.RawQuery = queryValues.Encode()
			targetURL = parsedURL.String()
		}
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    targetURL,
	}, nil
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) error {
	if meta.Mode != mode.Anthropic {
		return a.Adaptor.SetupRequestHeader(meta, store, c, req)
	}

	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)
	req.Header.Set(anthropic.AnthropicTokenHeader, meta.Channel.Key)

	anthropicVersion := c.Request.Header.Get("Anthropic-Version")
	if anthropicVersion == "" {
		anthropicVersion = anthropic.AnthropicVersion
	}

	req.Header.Set("Anthropic-Version", anthropicVersion)

	if rawBetas := c.Request.Header.Get(anthropic.AnthropicBeta); rawBetas != "" {
		req.Header.Set(
			anthropic.AnthropicBeta,
			anthropic.FixBetasStringWithModel(
				anthropic.ResolveModelName(meta.OriginModel, meta.ActualModel),
				rawBetas,
			),
		)
	}

	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	if meta.Mode == mode.Anthropic {
		return anthropic.ConvertRequest(meta, req)
	}

	return a.Adaptor.ConvertRequest(meta, store, req)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if meta.Mode == mode.Anthropic {
		if utils.IsStreamResponse(resp) {
			return anthropic.StreamHandler(meta, c, resp)
		}
		return anthropic.Handler(meta, c, resp)
	}

	return a.Adaptor.DoResponse(meta, store, c, resp)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Ant Ling (蚂蚁百灵) endpoint\nOpenAI-compatible chat endpoint: /api/llm/v1/chat/completions\nAnthropic-compatible endpoint: /api/anthropic/v1/messages\nSupports Gemini-compatible request conversion via OpenAI chat endpoint",
		Models: ModelList,
	}
}
