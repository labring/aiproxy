package model

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens"`

	WebSearchCount int64 `json:"web_search_count,omitempty"`

	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

func (u Usage) ToModelUsage() model.Usage {
	usage := model.Usage{
		InputTokens:    model.ZeroNullInt64(u.PromptTokens),
		OutputTokens:   model.ZeroNullInt64(u.CompletionTokens),
		TotalTokens:    model.ZeroNullInt64(u.TotalTokens),
		WebSearchCount: model.ZeroNullInt64(u.WebSearchCount),
	}
	if u.PromptTokensDetails != nil {
		usage.CachedTokens = model.ZeroNullInt64(u.PromptTokensDetails.CachedTokens)
		usage.CacheCreationTokens = model.ZeroNullInt64(u.PromptTokensDetails.CacheCreationTokens)
	}
	if u.CompletionTokensDetails != nil {
		usage.ReasoningTokens = model.ZeroNullInt64(u.CompletionTokensDetails.ReasoningTokens)
	}
	return usage
}

func (u *Usage) Add(other *Usage) {
	if other == nil {
		return
	}
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
	if other.PromptTokensDetails != nil {
		if u.PromptTokensDetails == nil {
			u.PromptTokensDetails = &PromptTokensDetails{}
		}
		u.PromptTokensDetails.Add(other.PromptTokensDetails)
	}
}

type PromptTokensDetails struct {
	CachedTokens        int64 `json:"cached_tokens"`
	AudioTokens         int64 `json:"audio_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens,omitempty"`
}

func (d *PromptTokensDetails) Add(other *PromptTokensDetails) {
	if other == nil {
		return
	}
	d.CachedTokens += other.CachedTokens
	d.AudioTokens += other.AudioTokens
	d.CacheCreationTokens += other.CacheCreationTokens
}

type CompletionTokensDetails struct {
	ReasoningTokens          int64 `json:"reasoning_tokens"`
	AudioTokens              int64 `json:"audio_tokens"`
	AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
}

type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

type OpenAIError struct {
	Code    any    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
	Param   string `json:"param,omitempty"`
}

func NewOpenAIError(statusCode int, err OpenAIError) adaptor.Error {
	return adaptor.NewError(statusCode, OpenAIErrorResponse{
		Error: err,
	})
}

func WrapperOpenAIError(err error, code any, statusCode int) adaptor.Error {
	return WrapperOpenAIErrorWithMessage(err.Error(), code, statusCode)
}

func WrapperOpenAIErrorWithMessage(message string, code any, statusCode int) adaptor.Error {
	return NewOpenAIError(statusCode, OpenAIError{
		Message: message,
		Type:    ErrorTypeAIPROXY,
		Code:    code,
	})
}
