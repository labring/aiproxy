package qianfan

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct {
	openai.Adaptor
	configCache utils.ChannelConfigCache[Config]
}

func init() {
	registry.Register(model.ChannelTypeQianfan, &Adaptor{})
}

const baseURL = "https://qianfan.baidubce.com/v2"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Anthropic ||
		m == mode.Gemini ||
		m == mode.Embeddings ||
		m == mode.Responses ||
		m == mode.ResponsesGet ||
		m == mode.ResponsesDelete ||
		m == mode.ResponsesInputItems
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	if meta.Mode == mode.ResponsesCancel {
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}

	return a.Adaptor.GetRequestURL(meta, store, c)
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	if meta.Mode == mode.ResponsesCancel {
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}

	return a.Adaptor.ConvertRequest(meta, store, req)
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) error {
	if err := a.Adaptor.SetupRequestHeader(meta, store, c, req); err != nil {
		return err
	}

	cfg, err := a.loadConfig(meta)
	if err != nil {
		return err
	}

	if appID := strings.TrimSpace(cfg.AppID); appID != "" {
		req.Header.Set("Appid", appID)
	}

	return nil
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if !adaptor.IsSuccessfulResponseStatus(meta.Mode, resp.StatusCode) {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	return a.Adaptor.DoResponse(meta, store, c, resp)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme:       "Baidu Qianfan OpenAI-compatible endpoint\nSupports chat, completions, embeddings, Responses API, Anthropic-compatible request conversion, and Gemini-compatible request conversion\nKey format example: `bce-v3/aaa/bbb`\nChannel config `appid` sets the upstream `appid` request header.",
		KeyHelp:      "bce-v3/aaa/bbb",
		ConfigSchema: configSchema(),
	}
}
