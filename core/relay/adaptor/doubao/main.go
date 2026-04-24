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
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

func init() {
	registry.Register(model.ChannelTypeDoubao, &Adaptor{})
}

func featureModel(meta *meta.Meta) string {
	if meta == nil {
		return ""
	}

	if modelName := utils.FirstMatchingModelName(
		meta.OriginModel,
		meta.ActualModel,
		func(modelName string) bool {
			modelName = strings.ToLower(modelName)
			return strings.HasPrefix(modelName, "bot-") || strings.Contains(modelName, "vision")
		},
	); modelName != "" {
		return modelName
	}

	return utils.PreferredModelName(meta.OriginModel, meta.ActualModel)
}

func GetRequestURL(meta *meta.Meta) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	modelName := strings.ToLower(featureModel(meta))
	switch meta.Mode {
	case mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		if strings.HasPrefix(modelName, "bot-") {
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
		if strings.Contains(modelName, "vision") {
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
	case mode.Responses:
		url, err := url.JoinPath(u, "/api/v3/responses")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ResponsesGet:
		url, err := url.JoinPath(u, "/api/v3/responses", meta.ResponseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	case mode.ResponsesDelete:
		url, err := url.JoinPath(u, "/api/v3/responses", meta.ResponseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodDelete,
			URL:    url,
		}, nil
	case mode.ResponsesCancel:
		url, err := url.JoinPath(u, "/api/v3/responses", meta.ResponseID, "cancel")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ResponsesInputItems:
		url, err := url.JoinPath(u, "/api/v3/responses", meta.ResponseID, "input_items")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
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
		m == mode.Gemini ||
		m == mode.Embeddings ||
		m == mode.Responses ||
		m == mode.ResponsesGet ||
		m == mode.ResponsesDelete ||
		m == mode.ResponsesCancel ||
		m == mode.ResponsesInputItems
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Doubao / Volcano Engine endpoint\nSupports bot-style models, native Responses API, Gemini-compatible request conversion, and network search metering fields",
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
	reasoningHook := func(openAIReq *relaymodel.GeneralOpenAIRequest) error {
		reasoning := utils.ParseOpenAIReasoning(openAIReq)
		utils.ApplyReasoningToDoubaoRequest(openAIReq, reasoning)
		return nil
	}

	switch meta.Mode {
	case mode.Embeddings:
		if strings.Contains(strings.ToLower(featureModel(meta)), "vision") {
			return openai.ConvertEmbeddingsRequest(meta, req, false, patchEmbeddingsVisionInput)
		}
		return openai.ConvertEmbeddingsRequest(meta, req, true)
	case mode.ChatCompletions:
		return ConvertChatCompletionsRequest(meta, req)
	case mode.Anthropic:
		return openai.ConvertClaudeRequest(meta, req, reasoningHook)
	case mode.Gemini:
		return openai.ConvertGeminiRequest(meta, req, reasoningHook)
	default:
		return openai.ConvertRequest(meta, store, req)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		websearchCount := int64(0)

		var (
			result adaptor.DoResponseResult
			err    adaptor.Error
		)

		if utils.IsStreamResponse(resp) {
			result, err = openai.StreamHandler(meta, c, resp, newHandlerPreHandler(&websearchCount))
		} else {
			result, err = openai.Handler(meta, c, resp, newHandlerPreHandler(&websearchCount))
		}

		result.Usage.WebSearchCount += model.ZeroNullInt64(websearchCount)

		return result, err
	case mode.Embeddings:
		return openai.EmbeddingsHandler(
			meta,
			c,
			resp,
			embeddingPreHandler,
		)
	case mode.Gemini:
		if utils.IsStreamResponse(resp) {
			return openai.GeminiStreamHandler(meta, c, resp)
		}
		return openai.GeminiHandler(meta, c, resp)
	default:
		return openai.DoResponse(meta, store, c, resp)
	}
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}
