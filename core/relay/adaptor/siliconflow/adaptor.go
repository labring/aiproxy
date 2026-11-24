package siliconflow

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
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
	registry.Register(model.ChannelTypeSiliconflow, &Adaptor{})
}

const baseURL = "https://api.siliconflow.cn/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "SiliconFlow API\nOpenAI-compatible chat, embeddings, image, audio, rerank, and video endpoints\nChat supports audio/video understanding request conversion",
		Models: ModelList,
	}
}

func (a *Adaptor) SupportMode(meta *meta.Meta) bool {
	m := adaptor.ModeFromMeta(meta)

	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Embeddings ||
		m == mode.ImagesGenerations ||
		m == mode.AudioSpeech ||
		m == mode.AudioTranscription ||
		m == mode.Rerank ||
		m == mode.VideoGenerationsJobs ||
		m == mode.VideoGenerationsGetJobs ||
		m == mode.VideoGenerationsContent ||
		m == mode.Videos ||
		m == mode.VideosGet ||
		m == mode.VideosContent ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	var path string

	switch meta.Mode {
	case mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		path = "/chat/completions"
	case mode.Completions:
		path = "/completions"
	case mode.Embeddings:
		path = "/embeddings"
	case mode.ImagesGenerations:
		path = "/images/generations"
	case mode.AudioSpeech:
		path = "/audio/speech"
	case mode.AudioTranscription:
		path = "/audio/transcriptions"
	case mode.Rerank:
		path = "/rerank"
	case mode.VideoGenerationsJobs:
		path = "/video/submit"
	case mode.Videos:
		path = "/video/submit"
	case mode.VideoGenerationsGetJobs:
		path = "/video/status"
	case mode.VideoGenerationsContent:
		path = "/video/status"
	case mode.VideosGet:
		path = "/video/status"
	case mode.VideosContent:
		path = "/video/status"
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}

	requestURL, err := url.JoinPath(u, path)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    requestURL,
	}, nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		return openai.ConvertChatCompletionsRequest(
			meta,
			req,
			false,
			patchChatMultimodalContent,
		)
	case mode.Embeddings:
		if isVLEmbeddingModel(meta) {
			return openai.ConvertEmbeddingsRequest(meta, req, false, patchVLEmbeddingsInput)
		}

		return a.Adaptor.ConvertRequest(meta, store, req)
	case mode.ImagesGenerations:
		return ConvertImageRequest(meta, req)
	case mode.VideoGenerationsJobs:
		return ConvertVideoGenerationJobRequest(meta, req)
	case mode.Videos:
		return ConvertVideosRequest(meta, req)
	case mode.VideoGenerationsGetJobs:
		return ConvertVideoStatusRequest(meta, req)
	case mode.VideoGenerationsContent:
		return ConvertVideoContentStatusRequest(meta, req)
	case mode.VideosGet:
		return ConvertVideosStatusRequest(meta, req)
	case mode.VideosContent:
		return ConvertVideosStatusRequest(meta, req)
	case mode.Anthropic:
		// SiliconFlow documents top_k, unlike the OpenAI API the conversion
		// targets.
		return openai.ConvertClaudeRequest(
			meta,
			req,
			patchSiliconFlowMultimodalContent,
			openai.ForwardClaudeTopK(req),
		)
	case mode.Gemini:
		return openai.ConvertGeminiRequest(
			meta,
			req,
			patchSiliconFlowMultimodalContent,
			openai.ForwardGeminiTopK(req),
		)
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
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			return openai.StreamHandler(meta, c, resp, nil)
		}

		return openai.Handler(meta, c, resp, nil)
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			return openai.ClaudeStreamHandler(meta, c, resp)
		}

		return openai.ClaudeHandler(meta, c, resp)
	case mode.Gemini:
		if utils.IsStreamResponse(resp) {
			return openai.GeminiStreamHandler(meta, c, resp)
		}

		return openai.GeminiHandler(meta, c, resp)
	case mode.AudioSpeech:
		if resp.StatusCode != http.StatusOK {
			return adaptor.DoResponseResult{}, ErrorHandler(resp)
		}

		result, err := a.Adaptor.DoResponse(meta, store, c, resp)
		if err != nil {
			return adaptor.DoResponseResult{}, err
		}

		size := c.Writer.Size()
		result.Usage = model.Usage{
			OutputTokens: model.ZeroNullInt64(size),
			TotalTokens:  model.ZeroNullInt64(size),
		}

		return result, nil
	case mode.Rerank:
		if resp.StatusCode != http.StatusOK {
			return adaptor.DoResponseResult{}, ErrorHandler(resp)
		}

		return a.Adaptor.DoResponse(meta, store, c, resp)
	case mode.ImagesGenerations:
		return ImageHandler(meta, c, resp)
	case mode.VideoGenerationsJobs:
		return VideoGenerationJobSubmitHandler(meta, store, c, resp)
	case mode.Videos:
		return VideosSubmitHandler(meta, store, c, resp)
	case mode.VideoGenerationsGetJobs:
		return VideoGenerationJobStatusHandler(meta, store, c, resp)
	case mode.VideoGenerationsContent:
		return VideoGenerationJobContentHandler(meta, c, resp)
	case mode.VideosGet:
		return VideosStatusHandler(meta, store, c, resp)
	case mode.VideosContent:
		return VideosContentHandler(meta, c, resp)
	default:
		if !adaptor.IsSuccessfulResponseStatus(meta.Mode, resp.StatusCode) {
			return adaptor.DoResponseResult{}, ErrorHandler(resp)
		}
		return a.Adaptor.DoResponse(meta, store, c, resp)
	}
}
