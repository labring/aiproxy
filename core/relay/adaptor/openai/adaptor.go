package openai

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	configCache utils.ChannelConfigCache[Config]
}

func init() {
	registry.Register(model.ChannelTypeOpenAI, &Adaptor{})
}

const baseURL = "https://api.openai.com/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)
	if m == mode.Decisions {
		// Do not advertise a new OpenAI endpoint on every provider embedding this adaptor.
		return mt.Channel.Type == model.ChannelTypeOpenAI ||
			mt.Channel.Type == model.ChannelTypeAIProxyHZH ||
			mt.Channel.Type == model.ChannelTypeAIProxyUSW1
	}

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
		m == mode.Videos ||
		m == mode.VideosGet ||
		m == mode.VideosContent ||
		m == mode.VideosDelete ||
		m == mode.VideosRemix ||
		m == mode.VideosEdits ||
		m == mode.VideosExtensions ||
		m == mode.Anthropic ||
		m == mode.Gemini ||
		m == mode.Responses ||
		m == mode.ResponsesCompact ||
		m == mode.ResponsesGet ||
		m == mode.ResponsesDelete ||
		m == mode.ResponsesCancel ||
		m == mode.ResponsesInputItems ||
		m == mode.AlphaSearch
}

//nolint:gocyclo
func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	switch meta.Mode {
	case mode.Decisions:
		requestURL, err := url.JoinPath(u, "/decisions")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodPost, URL: requestURL}, nil
	case mode.AlphaSearch:
		url, err := url.JoinPath(u, "/alpha/search")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Responses:
		url, err := url.JoinPath(u, "/responses")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.ResponsesCompact:
		url, err := url.JoinPath(u, "/responses/compact")
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
		// Check if model requires Responses API
		if IsResponsesOnlyModelAny(&meta.ModelConfig, meta.OriginModel, meta.ActualModel) {
			url, err := url.JoinPath(u, "/responses")
			if err != nil {
				return adaptor.RequestURL{}, err
			}

			return adaptor.RequestURL{
				Method: http.MethodPost,
				URL:    url,
			}, nil
		}

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
		url, err := url.JoinPath(u, "/video/generations", meta.GenerationID, "content/video")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	case mode.Videos:
		url, err := url.JoinPath(u, "/videos")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.VideosGet:
		url, err := url.JoinPath(u, "/videos", meta.VideoID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	case mode.VideosContent:
		url, err := url.JoinPath(u, "/videos", meta.VideoID, "content")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    url,
		}, nil
	case mode.VideosDelete:
		url, err := url.JoinPath(u, "/videos", meta.VideoID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodDelete,
			URL:    url,
		}, nil
	case mode.VideosRemix:
		url, err := url.JoinPath(u, "/videos", meta.VideoID, "remix")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.VideosEdits:
		url, err := url.JoinPath(u, "/videos/edits")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.VideosExtensions:
		url, err := url.JoinPath(u, "/videos/extensions")
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
	c *gin.Context,
	req *http.Request,
) error {
	if c != nil && c.Request != nil {
		for _, headerName := range openAIRequestHeaderWhitelist {
			if value := c.Request.Header.Values(headerName); len(value) > 0 {
				req.Header[http.CanonicalHeaderKey(headerName)] = append([]string(nil), value...)
			}
		}
	}

	if meta.Mode == mode.ResponsesCompact || meta.Mode == mode.Decisions {
		req.Header.Set("Accept", "application/json")
	}

	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)

	return nil
}

var openAIRequestHeaderWhitelist = []string{
	"Accept",
	"Accept-Language",
	"OpenAI-Beta",
	"User-Agent",
	"Originator",
	"conversation_id",
	"session_id",
	"session-id",
	"thread-id",
	"x-client-request-id",
	"version",
	"x-codex-beta-features",
	"x-codex-installation-id",
	"x-codex-turn-state",
	"x-codex-turn-metadata",
	"x-codex-window-id",
	"x-openai-internal-codex-responses-lite",
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
	case mode.Decisions:
		return ConvertDecisionsRequest(meta, req)
	case mode.AlphaSearch:
		return ConvertAlphaSearchRequest(meta, req)
	case mode.Responses:
		return ConvertResponseRequest(meta, req, patchOpenAIResponsesReasoningEffort(meta))
	case mode.ResponsesCompact:
		return ConvertResponseCompactRequest(meta, req)
	case mode.ResponsesGet, mode.ResponsesDelete, mode.ResponsesCancel, mode.ResponsesInputItems:
		// These endpoints don't need request conversion
		return adaptor.ConvertResult{}, nil
	case mode.Moderations:
		return ConvertModerationsRequest(meta, req)
	case mode.Embeddings:
		return ConvertEmbeddingsRequest(meta, req, false)
	case mode.Completions:
		return ConvertCompletionsRequest(meta, req, patchOpenAIReasoningEffort(meta))
	case mode.ChatCompletions:
		// Check if model requires Responses API conversion
		if IsResponsesOnlyModelAny(&meta.ModelConfig, meta.OriginModel, meta.ActualModel) {
			return ConvertChatCompletionToResponsesRequest(meta, req)
		}
		return ConvertChatCompletionsRequest(meta, req, false, patchOpenAIReasoningEffort(meta))
	case mode.Anthropic:
		// Check if model requires Responses API conversion
		if IsResponsesOnlyModelAny(&meta.ModelConfig, meta.OriginModel, meta.ActualModel) {
			return ConvertClaudeToResponsesRequest(meta, req)
		}
		return ConvertClaudeRequest(meta, req)
	case mode.ImagesGenerations:
		return ConvertImagesRequest(meta, req)
	case mode.ImagesEdits:
		return ConvertImagesEditsRequest(meta, req, true)
	case mode.AudioTranscription, mode.AudioTranslation:
		return ConvertSTTRequest(meta, req)
	case mode.AudioSpeech:
		return ConvertTTSRequest(meta, req, "")
	case mode.Rerank:
		return ConvertRerankRequest(meta, req)
	case mode.VideoGenerationsJobs:
		return ConvertVideoGenerationJobRequest(meta, req)
	case mode.VideoGenerationsGetJobs:
		return ConvertVideoGetJobsRequest(meta, req)
	case mode.VideoGenerationsContent:
		return ConvertVideoGetJobsContentRequest(meta, req)
	case mode.Videos:
		return ConvertVideosRequest(meta, req)
	case mode.VideosRemix:
		return ConvertVideosRemixRequest(meta, req)
	case mode.VideosEdits:
		return ConvertVideosEditRequest(meta, req)
	case mode.VideosExtensions:
		return ConvertVideosExtensionRequest(meta, req)
	case mode.VideosGet:
		return ConvertVideosGetRequest(meta, req)
	case mode.VideosContent:
		return ConvertVideosContentRequest(meta, req)
	case mode.VideosDelete:
		return ConvertVideoNoBodyRequest(meta, req)
	case mode.Gemini:
		// Check if model requires Responses API conversion
		if IsResponsesOnlyModelAny(&meta.ModelConfig, meta.OriginModel, meta.ActualModel) {
			return ConvertGeminiToResponsesRequest(meta, req)
		}
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
	options ...DoResponseOptions,
) (result adaptor.DoResponseResult, err adaptor.Error) {
	responseOptions := defaultConfig().doResponseOptions()
	if len(options) > 0 {
		responseOptions = options[0]
	}

	return doResponse(meta, store, c, resp, responseOptions)
}

//nolint:gocyclo
func doResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	options DoResponseOptions,
) (result adaptor.DoResponseResult, err adaptor.Error) {
	switch meta.Mode {
	case mode.Decisions:
		result, err = DecisionsHandler(meta, c, resp)
	case mode.AlphaSearch:
		result, err = AlphaSearchHandler(meta, c, resp)
	case mode.Responses:
		if utils.IsStreamResponse(resp) {
			result, err = responseStreamHandler(
				meta,
				store,
				c,
				resp,
				options.ResponsesFirstEventTimeout,
			)
		} else {
			result, err = ResponseHandler(meta, store, c, resp)
		}
	case mode.ResponsesCompact:
		result, err = CompactResponseHandler(meta, c, resp)
	case mode.ResponsesGet:
		result, err = GetResponseHandler(meta, c, resp)
	case mode.ResponsesDelete:
		result, err = DeleteResponseHandler(meta, c, resp)
	case mode.ResponsesCancel:
		result, err = CancelResponseHandler(meta, c, resp)
	case mode.ResponsesInputItems:
		result, err = GetInputItemsHandler(meta, c, resp)
	case mode.ImagesGenerations, mode.ImagesEdits:
		if utils.IsStreamResponse(resp) {
			result, err = ImagesStreamHandler(meta, c, resp)
		} else {
			result, err = ImagesHandler(meta, c, resp)
		}
	case mode.AudioTranscription, mode.AudioTranslation:
		result, err = STTHandler(meta, c, resp)
	case mode.AudioSpeech:
		result, err = TTSHandler(meta, c, resp)
	case mode.Rerank:
		result, err = RerankHandler(meta, c, resp)
	case mode.Moderations:
		result, err = ModerationsHandler(meta, c, resp)
	case mode.Embeddings:
		result, err = EmbeddingsHandler(meta, c, resp, nil)
	case mode.Completions, mode.ChatCompletions:
		var (
			streamPreHandler  PreHandler
			handlerPreHandler PreHandler
		)

		if meta.Mode == mode.ChatCompletions {
			var configErr error

			streamPreHandler, handlerPreHandler, configErr = getChatCompletionResponsePreHandlers(
				meta,
			)
			if configErr != nil {
				return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
					configErr,
					"load_channel_config_failed",
					http.StatusInternalServerError,
				)
			}
		}

		// Check if model required Responses API conversion
		if IsResponsesOnlyModelAny(&meta.ModelConfig, meta.OriginModel, meta.ActualModel) {
			// Convert Responses API response back to ChatCompletion format
			if utils.IsStreamResponse(resp) {
				result, err = ConvertResponsesToChatCompletionStreamResponse(meta, c, resp)
			} else {
				result, err = ConvertResponsesToChatCompletionResponse(meta, c, resp)
			}
		} else {
			if utils.IsStreamResponse(resp) {
				result, err = StreamHandler(meta, c, resp, streamPreHandler)
			} else {
				result, err = Handler(meta, c, resp, handlerPreHandler)
			}
		}
	case mode.Anthropic:
		// Check if model required Responses API conversion
		if IsResponsesOnlyModelAny(&meta.ModelConfig, meta.OriginModel, meta.ActualModel) {
			// Convert Responses API response back to Claude format
			if utils.IsStreamResponse(resp) {
				result, err = ConvertResponsesToClaudeStreamResponse(meta, c, resp)
			} else {
				result, err = ConvertResponsesToClaudeResponse(meta, c, resp)
			}
		} else {
			if utils.IsStreamResponse(resp) {
				result, err = ClaudeStreamHandler(meta, c, resp)
			} else {
				result, err = ClaudeHandler(meta, c, resp)
			}
		}
	case mode.VideoGenerationsJobs:
		result, err = VideoHandler(meta, store, c, resp)
	case mode.VideoGenerationsGetJobs:
		result, err = VideoGetJobsHandler(meta, store, c, resp)
	case mode.VideoGenerationsContent:
		result, err = VideoGetJobsContentHandler(meta, store, c, resp)
	case mode.Videos:
		result, err = VideosHandler(meta, store, c, resp)
	case mode.VideosRemix:
		result, err = VideosRemixHandler(meta, store, c, resp)
	case mode.VideosEdits:
		result, err = VideosEditHandler(meta, store, c, resp)
	case mode.VideosExtensions:
		result, err = VideosExtensionHandler(meta, store, c, resp)
	case mode.VideosGet:
		result, err = VideosGetHandler(meta, c, resp)
	case mode.VideosContent:
		result, err = VideosContentHandler(meta, c, resp)
	case mode.VideosDelete:
		result, err = VideoDeleteHandler(meta, c, resp)
	case mode.Gemini:
		// Check if model required Responses API conversion
		if IsResponsesOnlyModelAny(&meta.ModelConfig, meta.OriginModel, meta.ActualModel) {
			// Convert Responses API response back to Gemini format
			if utils.IsStreamResponse(resp) {
				result, err = ConvertResponsesToGeminiStreamResponse(meta, c, resp)
			} else {
				result, err = ConvertResponsesToGeminiResponse(meta, c, resp)
			}
		} else {
			if utils.IsStreamResponse(resp) {
				result, err = GeminiStreamHandler(meta, c, resp)
			} else {
				result, err = GeminiHandler(meta, c, resp)
			}
		}
	default:
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}

	return result, err
}

const MetaResponseFormat = "response_format"

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequestWithMeta(req, meta)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (result adaptor.DoResponseResult, err adaptor.Error) {
	options := defaultConfig().doResponseOptions()
	if meta.Mode == mode.Responses && utils.IsStreamResponse(resp) {
		var configErr error

		cfg, configErr := a.loadConfig(meta)
		if configErr != nil {
			return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
				configErr,
				"load_channel_config_failed",
				http.StatusInternalServerError,
			)
		}

		options = cfg.doResponseOptions()
	}

	return DoResponse(meta, store, c, resp, options)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme:       "OpenAI native API\nSupports chat, completions, embeddings, moderations, image, audio, rerank, PDF parsing, video generation, Responses API, and Decisions API\nAlso supports Anthropic-compatible and Gemini-compatible request conversion on top of the OpenAI endpoint\nChannel config `responses_first_event_timeout` sets the maximum seconds to wait for the first effective Responses stream event\nChannel config `map_reasoning_to_reasoning_content` rewrites upstream `reasoning` fields to `reasoning_content` in chat completion responses",
		ConfigSchema: ConfigSchema(),
		Models:       ModelList,
	}
}
