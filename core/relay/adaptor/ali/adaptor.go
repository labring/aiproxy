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
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

// https://help.aliyun.com/zh/dashscope/developer-reference/api-details

type Adaptor struct{}

func init() {
	registry.Register(model.ChannelTypeAli, &Adaptor{})
}

const baseURL = "https://dashscope.aliyuncs.com"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Embeddings ||
		m == mode.ImagesGenerations ||
		m == mode.ImagesEdits ||
		m == mode.Rerank ||
		m == mode.AudioSpeech ||
		m == mode.AudioTranscription ||
		m == mode.AudioTranslation ||
		m == mode.Anthropic ||
		m == mode.Gemini ||
		m == mode.AliVideo ||
		m == mode.AliVideoTasks ||
		m == mode.VideoGenerationsJobs ||
		m == mode.VideoGenerationsGetJobs ||
		m == mode.VideoGenerationsContent ||
		m == mode.Videos ||
		m == mode.VideosGet ||
		m == mode.VideosContent ||
		m == mode.VideosRemix ||
		m == mode.VideosEdits ||
		m == mode.VideosExtensions ||
		m == mode.Responses ||
		m == mode.ResponsesGet ||
		m == mode.ResponsesDelete ||
		m == mode.ResponsesCancel ||
		m == mode.ResponsesInputItems
}

func isAliVideoMode(m mode.Mode) bool {
	return m == mode.VideoGenerationsJobs ||
		m == mode.AliVideo ||
		m == mode.AliVideoTasks ||
		m == mode.VideoGenerationsGetJobs ||
		m == mode.VideoGenerationsContent ||
		m == mode.Videos ||
		m == mode.VideosGet ||
		m == mode.VideosContent ||
		m == mode.VideosRemix ||
		m == mode.VideosEdits ||
		m == mode.VideosExtensions
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	if isAliVideoMode(meta.Mode) {
		return getAliVideoRequestURL(u, meta, store)
	}

	switch meta.Mode {
	case mode.ImagesGenerations:
		return getAliImageRequestURL(u, meta)
	case mode.ImagesEdits:
		return getAliImageRequestURL(u, meta)
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
		path := "/compatible-mode/v1/embeddings"
		if isMultimodalEmbeddingModel(meta) {
			path = "/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding"
		}

		url, err := url.JoinPath(u, path)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.AudioTranscription:
		return sttRequestURL(meta)
	case mode.AudioSpeech:
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
		url, err := url.JoinPath(u, "/apps/anthropic/v1/messages")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Gemini:
		url, err := url.JoinPath(u, "/compatible-mode/v1/chat/completions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Responses:
		url, err := url.JoinPath(u, "/compatible-mode/v1/responses")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ResponsesGet:
		url, err := url.JoinPath(u, "/compatible-mode/v1/responses", meta.ResponseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	case mode.ResponsesDelete:
		url, err := url.JoinPath(u, "/compatible-mode/v1/responses", meta.ResponseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodDelete,
			URL:    url,
		}, nil
	case mode.ResponsesCancel:
		url, err := url.JoinPath(u, "/compatible-mode/v1/responses", meta.ResponseID, "cancel")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ResponsesInputItems:
		url, err := url.JoinPath(u, "/compatible-mode/v1/responses", meta.ResponseID, "input_items")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func getAliImageRequestURL(baseURL string, meta *meta.Meta) (adaptor.RequestURL, error) {
	path := "/api/v1/services/aigc/text2image/image-synthesis"
	if isAliMultimodalImageModel(meta) {
		path = "/api/v1/services/aigc/multimodal-generation/generation"
	} else if isQwenMTImageModel(meta) {
		path = "/api/v1/services/aigc/image2image/image-synthesis"
	}

	targetURL, err := url.JoinPath(baseURL, path)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    targetURL,
	}, nil
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)

	if meta.Mode == mode.AliVideo {
		req.Header.Set("X-Dashscope-Async", "enable")
	}

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
	case mode.ImagesEdits:
		return ConvertAliImageEditRequest(meta, req)
	case mode.Rerank:
		return ConvertRerankRequest(meta, req)
	case mode.ChatCompletions:
		return ConvertChatCompletionsRequest(meta, store, req)
	case mode.Completions:
		return ConvertCompletionsRequest(meta, store, req)
	case mode.Embeddings:
		if isMultimodalEmbeddingModel(meta) {
			return ConvertMultimodalEmbeddingsRequest(meta, req)
		}

		return openai.ConvertRequest(meta, store, req)
	case mode.AudioSpeech:
		return ConvertTTSRequest(meta, req)
	case mode.AudioTranscription:
		return convertSTTRequest(meta, req)
	case mode.AliVideo:
		return ConvertAliNativeVideoRequest(meta, req)
	case mode.AliVideoTasks:
		return adaptor.ConvertResult{}, nil
	case mode.VideoGenerationsJobs:
		return ConvertAliVideoGenerationJobRequest(meta, req)
	case mode.Videos:
		return ConvertAliVideosRequest(meta, req)
	case mode.VideosRemix:
		return ConvertAliVideosRemixRequest(meta, req)
	case mode.VideosEdits:
		return ConvertAliVideosEditRequest(meta, req)
	case mode.VideosExtensions:
		return ConvertAliVideosExtensionRequest(meta, req)
	case mode.VideoGenerationsGetJobs:
		return ConvertAliVideoGenerationGetJobsRequest(meta, req)
	case mode.VideoGenerationsContent:
		return ConvertAliVideoGenerationContentRequest(meta, req)
	case mode.VideosGet:
		return ConvertAliVideosGetRequest(meta, req)
	case mode.VideosContent:
		return ConvertAliVideosContentRequest(meta, req)
	case mode.VideosDelete:
		return adaptor.ConvertResult{}, nil
	case mode.Anthropic:
		return anthropic.ConvertRequest(meta, req)
	case mode.Gemini:
		return ConvertGeminiRequest(meta, req)
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		return openai.ConvertRequest(meta, store, req)
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
		return sttDoRequest(meta, req)
	default:
		return utils.DoRequestWithMeta(req, meta)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	switch meta.Mode {
	case mode.AliVideo:
		return AliNativeVideoHandler(meta, store, c, resp)
	case mode.AliVideoTasks:
		return AliNativeVideoTaskHandler(meta, store, c, resp)
	case mode.VideoGenerationsJobs:
		return AliVideoHandler(meta, store, c, resp)
	case mode.VideoGenerationsGetJobs:
		return AliVideoGetJobsHandler(meta, store, c, resp)
	case mode.VideoGenerationsContent:
		return AliVideoContentHandler(meta, c, resp)
	case mode.Videos:
		return AliVideosHandler(meta, store, c, resp)
	case mode.VideosRemix:
		return AliVideosRemixHandler(meta, store, c, resp)
	case mode.VideosEdits:
		return AliVideosEditHandler(meta, store, c, resp)
	case mode.VideosExtensions:
		return AliVideosExtensionHandler(meta, store, c, resp)
	case mode.VideosGet:
		return AliVideoGetHandler(meta, store, c, resp)
	case mode.VideosContent:
		return AliVideosContentHandler(meta, c, resp)
	case mode.ImagesGenerations, mode.ImagesEdits:
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
		return sttResponse(meta, c, resp)
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
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		if meta.Mode != mode.Responses || !utils.IsStreamResponse(resp) {
			return openai.DoResponse(meta, store, c, resp)
		}

		options, err := openai.LoadDoResponseOptions(meta)
		if err != nil {
			return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
				err,
				"load_channel_config_failed",
				http.StatusInternalServerError,
			)
		}

		return openai.DoResponse(meta, store, c, resp, options)
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
		Readme:       "OpenAI compatibility\nNative Responses API support\nNetwork search metering support\nImage generation/edit support: https://help.aliyun.com/zh/model-studio/qwen-image-api and https://help.aliyun.com/zh/model-studio/qwen-image-edit-api\nVideo generation support: DashScope /api/v1/services/aigc/video-generation/video-synthesis\nRerank support: https://help.aliyun.com/zh/model-studio/text-rerank-api\nSTT: Whisper-compatible uploads adapted to DashScope realtime or Qwen3-ASR chat audio; enable FFMPEG_ENABLED for browser audio. See docs/stt.md.\nAnthropic support: /api/v2/apps/claude-code-proxy\nGemini support",
		ConfigSchema: sttConfigSchema(),
		Models:       ModelList,
	}
}
