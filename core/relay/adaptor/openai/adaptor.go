package openai

import (
	"errors"
	"fmt"
	"net/http"

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

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	switch meta.Mode {
	case mode.ChatCompletions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/chat/completions",
		}, nil
	case mode.Completions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/completions",
		}, nil
	case mode.Embeddings:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/embeddings",
		}, nil
	case mode.Moderations:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/moderations",
		}, nil
	case mode.ImagesGenerations:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/images/generations",
		}, nil
	case mode.ImagesEdits:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/images/edits",
		}, nil
	case mode.AudioSpeech:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/audio/speech",
		}, nil
	case mode.AudioTranscription:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/audio/transcriptions",
		}, nil
	case mode.AudioTranslation:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/audio/translations",
		}, nil
	case mode.Rerank:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/rerank",
		}, nil
	case mode.VideoGenerationsJobs:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/video/generations/jobs",
		}, nil
	case mode.VideoGenerationsGetJobs:
		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    fmt.Sprintf("%s/video/generations/jobs/%s", u, meta.JobID),
		}, nil
	case mode.VideoGenerationsContent:
		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    fmt.Sprintf("%s/video/generations/%s/content/video", u, meta.GenerationID),
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
	case mode.Moderations:
		return ConvertModerationsRequest(meta, req)
	case mode.Embeddings:
		return ConvertEmbeddingsRequest(meta, req, nil, false)
	case mode.Completions:
		return ConvertCompletionsRequest(meta, req, nil)
	case mode.ChatCompletions:
		return ConvertChatCompletionsRequest(meta, req, nil, false)
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
	case mode.Embeddings, mode.Completions:
		fallthrough
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			usage, err = StreamHandler(meta, c, resp, nil)
		} else {
			usage, err = Handler(meta, c, resp, nil)
		}
	case mode.VideoGenerationsJobs:
		usage, err = VideoHandler(meta, store, c, resp)
	case mode.VideoGenerationsGetJobs:
		usage, err = VideoGetJobsHandler(meta, store, c, resp)
	case mode.VideoGenerationsContent:
		usage, err = VideoGetJobsContentHandler(meta, store, c, resp)
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
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequest(req)
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
		Features: []string{
			"OpenAI compatibility",
		},
		Models: ModelList,
	}
}
