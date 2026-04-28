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

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	switch m {
	case mode.ChatCompletions,
		mode.Completions,
		mode.Anthropic,
		mode.Gemini,
		mode.Embeddings:
		return true
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesInputItems:
		return a.supportResponsesModel(mt)
	default:
		return false
	}
}

var builtinResponsesModels = map[string]struct{}{
	"deepseek-v3.2":                  {},
	"deepseek-v3.2-think":            {},
	"deepseek-v3.1-250821":           {},
	"deepseek-v3.1-think-250821":     {},
	"deepseek-v3":                    {},
	"deepseek-r1":                    {},
	"deepseek-r1-250528":             {},
	"kimi-k2-instruct":               {},
	"qwen3-coder-480b-a35b-instruct": {},
	"qwen3-coder-30b-a3b-instruct":   {},
	"qwen3-235b-a22b":                {},
	"qwen3-235b-a22b-thinking-2507":  {},
	"qwen3-235b-a22b-instruct-2507":  {},
	"qwen3-30b-a3b":                  {},
	"qwen3-30b-a3b-instruct-2507":    {},
	"qwen3-30b-a3b-thinking-2507":    {},
	"qwen3-32b":                      {},
	"qwen3-14b":                      {},
	"qwen3-8b":                       {},
	"qwen3-4b":                       {},
	"qwen3-1.7b":                     {},
	"qwen3-0.6b":                     {},
}

func (a *Adaptor) supportResponsesModel(mt *meta.Meta) bool {
	if qianfanModelMatches(mt, isBuiltinResponsesModel) {
		return true
	}

	if mt == nil {
		return false
	}

	cfg, err := a.loadConfig(mt)
	if err != nil {
		return false
	}

	return qianfanModelMatches(mt, func(modelName string) bool {
		return containsModel(cfg.ResponseModels, modelName)
	})
}

func qianfanModelMatches(mt *meta.Meta, match func(string) bool) bool {
	if mt == nil {
		return false
	}

	return utils.FirstMatchingModelName(mt.OriginModel, mt.ActualModel, match) != ""
}

func isBuiltinResponsesModel(modelName string) bool {
	_, ok := builtinResponsesModels[normalizeModelName(modelName)]
	return ok
}

func containsModel(models []string, modelName string) bool {
	target := normalizeModelName(modelName)
	if target == "" {
		return false
	}

	for _, m := range models {
		if normalizeModelName(m) == target {
			return true
		}
	}

	return false
}

func normalizeModelName(modelName string) string {
	return strings.ToLower(strings.TrimSpace(modelName))
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
