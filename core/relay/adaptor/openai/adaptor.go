package openai

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct{}

const baseURL = "https://api.openai.com/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Embeddings ||
		m == mode.Moderations ||
		m == mode.ImagesGenerations ||
		m == mode.ImagesEdits ||
		m == mode.AudioSpeech ||
		m == mode.AudioTranscription ||
		m == mode.AudioTranslation ||
		m == mode.Rerank ||
		m == mode.ParsePdf ||
		m == mode.VideoGenerationsJobs ||
		m == mode.VideoGenerationsGetJobs ||
		m == mode.VideoGenerationsContent ||
		m == mode.Anthropic ||
		m == mode.Gemini ||
		m == mode.Responses ||
		m == mode.ResponsesGet ||
		m == mode.ResponsesDelete ||
		m == mode.ResponsesCancel ||
		m == mode.ResponsesInputItems
}

//
//nolint:gocyclo
func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	switch meta.Mode {
	case mode.Responses:
		url, err := url.JoinPath(u, "/responses")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ResponsesGet:
		url, err := url.JoinPath(u, "/responses", meta.ResponseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	case mode.ResponsesDelete:
		url, err := url.JoinPath(u, "/responses", meta.ResponseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodDelete,
			URL:    url,
		}, nil
	case mode.ResponsesCancel:
		url, err := url.JoinPath(u, "/responses", meta.ResponseID, "cancel")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ResponsesInputItems:
		url, err := url.JoinPath(u, "/responses", meta.ResponseID, "input_items")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	case mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		url, err := url.JoinPath(u, "/chat/completions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Completions:
		url, err := url.JoinPath(u, "/completions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Embeddings:
		url, err := url.JoinPath(u, "/embeddings")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Moderations:
		url, err := url.JoinPath(u, "/moderations")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ImagesGenerations:
		url, err := url.JoinPath(u, "/images/generations")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ImagesEdits:
		url, err := url.JoinPath(u, "/images/edits")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.AudioSpeech:
		url, err := url.JoinPath(u, "/audio/speech")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.AudioTranscription:
		url, err := url.JoinPath(u, "/audio/transcriptions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.AudioTranslation:
		url, err := url.JoinPath(u, "/audio/translations")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Rerank:
		url, err := url.JoinPath(u, "/rerank")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.VideoGenerationsJobs:
		url, err := url.JoinPath(u, "/video/generations/jobs")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.VideoGenerationsGetJobs:
		url, err := url.JoinPath(u, "/video/generations/jobs", meta.JobID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	case mode.VideoGenerationsContent:
		url, err := url.JoinPath(u, "/video/generations", meta.GenerationID, "/content/video")
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

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)
	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return ConvertRequest(meta, store, req)
}

func ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	if req == nil {
		return adaptor.ConvertResult{}, errors.New("request is nil")
	}

	switch meta.Mode {
	case mode.Responses:
		return ConvertResponseRequest(meta, req)
	case mode.ResponsesGet, mode.ResponsesDelete, mode.ResponsesCancel, mode.ResponsesInputItems:
		// These endpoints don't need request conversion
		return adaptor.ConvertResult{}, nil
	case mode.Moderations:
		return ConvertModerationsRequest(meta, req)
	case mode.Embeddings:
		return ConvertEmbeddingsRequest(meta, req, false)
	case mode.Completions:
		return ConvertCompletionsRequest(meta, req)
	case mode.ChatCompletions:
		return ConvertChatCompletionsRequest(meta, req, false)
	case mode.Anthropic:
		return ConvertClaudeRequest(meta, req)
	case mode.ImagesGenerations:
		return ConvertImagesRequest(meta, req)
	case mode.ImagesEdits:
		return ConvertImagesEditsRequest(meta, req)
	case mode.AudioTranscription, mode.AudioTranslation:
		return ConvertSTTRequest(meta, req)
	case mode.AudioSpeech:
		return ConvertTTSRequest(meta, req, "")
	case mode.Rerank:
		return ConvertRerankRequest(meta, req)
	case mode.VideoGenerationsJobs:
		return ConvertVideoRequest(meta, req)
	case mode.VideoGenerationsGetJobs:
		return ConvertVideoGetJobsRequest(meta, req)
	case mode.VideoGenerationsContent:
		return ConvertVideoGetJobsContentRequest(meta, req)
	case mode.Gemini:
		return ConvertGeminiRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.Responses:
		if utils.IsStreamResponse(resp) {
			usage, err = ResponseStreamHandler(meta, store, c, resp)
		} else {
			usage, err = ResponseHandler(meta, store, c, resp)
		}
	case mode.ResponsesGet:
		usage, err = GetResponseHandler(meta, c, resp)
	case mode.ResponsesDelete:
		usage, err = DeleteResponseHandler(meta, c, resp)
	case mode.ResponsesCancel:
		usage, err = CancelResponseHandler(meta, c, resp)
	case mode.ResponsesInputItems:
		usage, err = GetInputItemsHandler(meta, c, resp)
	case mode.ImagesGenerations, mode.ImagesEdits:
		usage, err = ImagesHandler(meta, c, resp)
	case mode.AudioTranscription, mode.AudioTranslation:
		usage, err = STTHandler(meta, c, resp)
	case mode.AudioSpeech:
		usage, err = TTSHandler(meta, c, resp)
	case mode.Rerank:
		usage, err = RerankHandler(meta, c, resp)
	case mode.Moderations:
		usage, err = ModerationsHandler(meta, c, resp)
	case mode.Embeddings:
		usage, err = EmbeddingsHandler(meta, c, resp, nil)
	case mode.Completions, mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			usage, err = StreamHandler(meta, c, resp, nil)
		} else {
			usage, err = Handler(meta, c, resp, nil)
		}
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			usage, err = ClaudeStreamHandler(meta, c, resp)
		} else {
			usage, err = ClaudeHandler(meta, c, resp)
		}
	case mode.VideoGenerationsJobs:
		usage, err = VideoHandler(meta, store, c, resp)
	case mode.VideoGenerationsGetJobs:
		usage, err = VideoGetJobsHandler(meta, store, c, resp)
	case mode.VideoGenerationsContent:
		usage, err = VideoGetJobsContentHandler(meta, store, c, resp)
	case mode.Gemini:
		if utils.IsStreamResponse(resp) {
			usage, err = GeminiStreamHandler(meta, c, resp)
		} else {
			usage, err = GeminiHandler(meta, c, resp)
		}
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}

	return usage, err
}

const MetaResponseFormat = "response_format"

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequest(req, meta.RequestTimeout)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	return DoResponse(meta, store, c, resp)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "OpenAI compatibility\nAnthropic conversation",
		Models: ModelList,
	}
}
