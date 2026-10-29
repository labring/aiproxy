package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"gorm.io/gorm"
)

type ChannelModelArchitecture struct {
	Modality         string   `json:"modality,omitempty"`
	InputModalities  []string `json:"input_modalities,omitempty"`
	OutputModalities []string `json:"output_modalities,omitempty"`
	Tokenizer        string   `json:"tokenizer,omitempty"`
	InstructType     string   `json:"instruct_type,omitempty"`
}

type ChannelModelPricing struct {
	Prompt            string `json:"prompt,omitempty"`
	Completion        string `json:"completion,omitempty"`
	Request           string `json:"request,omitempty"`
	Image             string `json:"image,omitempty"`
	ImageOutput       string `json:"image_output,omitempty"`
	ImageToken        string `json:"image_token,omitempty"`
	Audio             string `json:"audio,omitempty"`
	AudioOutput       string `json:"audio_output,omitempty"`
	Discount          string `json:"discount,omitempty"`
	InputAudioCache   string `json:"input_audio_cache,omitempty"`
	InputCacheRead    string `json:"input_cache_read,omitempty"`
	InputCacheWrite   string `json:"input_cache_write,omitempty"`
	InputCacheWrite1H string `json:"input_cache_write_1h,omitempty"`
	InternalReasoning string `json:"internal_reasoning,omitempty"`
	WebSearch         string `json:"web_search,omitempty"`
}

type ChannelModelTopProvider struct {
	ContextLength       *int64 `json:"context_length,omitempty"`
	MaxCompletionTokens *int64 `json:"max_completion_tokens,omitempty"`
	IsModerated         *bool  `json:"is_moderated,omitempty"`
}

type ChannelModelRequestLimits struct {
	PromptTokens     *float64 `json:"prompt_tokens,omitempty"`
	CompletionTokens *float64 `json:"completion_tokens,omitempty"`
}

type ChannelModelDefaultParameters struct {
	Temperature       *float64 `json:"temperature,omitempty"`
	TopP              *float64 `json:"top_p,omitempty"`
	TopK              *int64   `json:"top_k,omitempty"`
	FrequencyPenalty  *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty   *float64 `json:"presence_penalty,omitempty"`
	RepetitionPenalty *float64 `json:"repetition_penalty,omitempty"`
}

type ChannelModel struct {
	ID              string                         `json:"id"                             validate:"required"`
	CanonicalSlug   string                         `json:"canonical_slug,omitempty"`
	HuggingFaceID   string                         `json:"hugging_face_id,omitempty"`
	Name            string                         `json:"name,omitempty"`
	Created         *int64                         `json:"created,omitempty"`
	Description     string                         `json:"description,omitempty"`
	ContextLength   *int64                         `json:"context_length,omitempty"`
	Architecture    *ChannelModelArchitecture      `json:"architecture,omitempty"`
	Pricing         *ChannelModelPricing           `json:"pricing,omitempty"`
	TopProvider     *ChannelModelTopProvider       `json:"top_provider,omitempty"`
	RequestLimits   *ChannelModelRequestLimits     `json:"per_request_limits,omitempty"`
	SupportedParams []string                       `json:"supported_parameters,omitempty"`
	DefaultParams   *ChannelModelDefaultParameters `json:"default_parameters,omitempty"`
	ExpirationDate  string                         `json:"expiration_date,omitempty"`
}

// GetChannelModels godoc
//
//	@Summary		Get a channel's model catalog
//	@Description	Reads the complete model catalog visible to a saved global channel through its provider or OpenAI-compatible AIProxy gateway. Results include models absent from channel.model_mapping and are not paginated.
//	@Tags			channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Channel ID"
//	@Success		200	{object}	middleware.APIResponse{data=[]ChannelModel}
//	@Failure		400	{object}	middleware.APIResponse
//	@Failure		401	{object}	middleware.APIResponse
//	@Failure		404	{object}	middleware.APIResponse
//	@Failure		422	{object}	middleware.APIResponse
//	@Failure		500	{object}	middleware.APIResponse
//	@Failure		502	{object}	middleware.APIResponse
//	@Failure		504	{object}	middleware.APIResponse
//	@Router			/api/channel/{id}/models [get]
func GetChannelModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid channel id")
		return
	}

	channel, err := model.GetChannelByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.ErrorResponse(c, http.StatusNotFound, "channel not found")
			return
		}

		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to load channel")

		return
	}

	if modelcatalog.IsUnsupportedType(channel.Type) {
		middleware.ErrorResponse(c, http.StatusUnprocessableEntity, modelcatalog.UnsupportedMessage)
		return
	}

	channelAdaptor, ok := adaptors.GetAdaptor(channel.Type)
	if !ok {
		middleware.ErrorResponse(c, http.StatusUnprocessableEntity, modelcatalog.UnsupportedMessage)
		return
	}

	lister, ok := channelAdaptor.(adaptor.ModelCatalogLister)
	if !ok {
		middleware.ErrorResponse(c, http.StatusUnprocessableEntity, modelcatalog.UnsupportedMessage)
		return
	}

	models, err := lister.ListModels(c.Request.Context(), channel, channelAdaptor.DefaultBaseURL())
	if err != nil {
		status := http.StatusBadGateway

		message := "failed to retrieve provider model catalog"
		if errors.Is(err, modelcatalog.ErrInvalidConfiguration) {
			status = http.StatusBadRequest
			message = "invalid channel provider configuration"
		} else if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
			message = "provider model catalog request timed out"
		}

		middleware.ErrorResponse(c, status, message)

		return
	}

	if models == nil {
		models = make([]adaptor.ModelCatalogItem, 0)
	}

	middleware.SuccessResponse(c, models)
}
