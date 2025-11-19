package ali

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

// https://help.aliyun.com/zh/dashscope/developer-reference/api-details

type Adaptor struct{}

const baseURL = "https://dashscope.aliyuncs.com"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Embeddings ||
		m == mode.ImagesGenerations ||
		m == mode.Rerank ||
		m == mode.AudioSpeech ||
		m == mode.AudioTranscription ||
		m == mode.AudioTranslation ||
		m == mode.Anthropic
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	switch meta.Mode {
	case mode.ImagesGenerations:
		url, err := url.JoinPath(u, "/api/v1/services/aigc/text2image/image-synthesis")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ChatCompletions:
		url, err := url.JoinPath(u, "/compatible-mode/v1/chat/completions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Completions:
		url, err := url.JoinPath(u, "/compatible-mode/v1/completions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Embeddings:
		url, err := url.JoinPath(u, "/compatible-mode/v1/embeddings")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.AudioSpeech, mode.AudioTranscription:
		url, err := url.JoinPath(u, "/api-ws/v1/inference")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Rerank:
		url, err := url.JoinPath(u, "/api/v1/services/rerank/text-rerank/text-rerank")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Anthropic:
		url, err := url.JoinPath(u, "/api/v2/apps/claude-code-proxy/v1/messages")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)

	// req.Header.Set("X-Dashscope-Plugin", meta.Channel.Config.Plugin)
	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.ImagesGenerations:
		return ConvertImageRequest(meta, req)
	case mode.Rerank:
		return ConvertRerankRequest(meta, req)
	case mode.ChatCompletions:
		return ConvertChatCompletionsRequest(meta, store, req)
	case mode.Completions:
		return ConvertCompletionsRequest(meta, store, req)
	case mode.Embeddings:
		return openai.ConvertRequest(meta, store, req)
	case mode.AudioSpeech:
		return ConvertTTSRequest(meta, req)
	case mode.AudioTranscription:
		return ConvertSTTRequest(meta, req)
	case mode.Anthropic:
		return anthropic.ConvertRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	switch meta.Mode {
	case mode.AudioSpeech:
		return TTSDoRequest(meta, req)
	case mode.AudioTranscription:
		return STTDoRequest(meta, req)
	default:
		return utils.DoRequest(req, meta.RequestTimeout)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	switch meta.Mode {
	case mode.ImagesGenerations:
		return ImageHandler(meta, c, resp)
	case mode.Embeddings:
		return EmbeddingsHandler(meta, store, c, resp)
	case mode.Completions, mode.ChatCompletions:
		return ChatHandler(meta, store, c, resp)
	case mode.Rerank:
		return RerankHandler(meta, c, resp)
	case mode.AudioSpeech:
		return TTSDoResponse(meta, c, resp)
	case mode.AudioTranscription:
		return STTDoResponse(meta, c, resp)
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			return anthropic.StreamHandler(meta, c, resp)
		} else {
			return anthropic.Handler(meta, c, resp)
		}
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"OpenAI compatibility",
			"Network search metering support",
			"Rerank support: https://help.aliyun.com/zh/model-studio/text-rerank-api",
			"STT support: https://help.aliyun.com/zh/model-studio/sambert-speech-synthesis/",
			"Anthropic support: /api/v2/apps/claude-code-proxy",
		},
		Models: ModelList,
	}
}
