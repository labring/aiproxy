package model

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

type ChatUsage struct {
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens"`

	WebSearchCount int64 `json:"web_search_count,omitempty"`

	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

func (u ChatUsage) ToModelUsage() model.Usage {
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

func (u *ChatUsage) Add(other *ChatUsage) {
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

func (u ChatUsage) ToClaudeUsage() ClaudeUsage {
	cu := ClaudeUsage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
	}

	if u.PromptTokensDetails != nil {
		cu.CacheCreationInputTokens = u.PromptTokensDetails.CacheCreationTokens
		cu.CacheReadInputTokens = u.PromptTokensDetails.CachedTokens
	}

	return cu
}

// ToResponseUsage converts ChatUsage to ResponseUsage (OpenAI Responses API format)
func (u ChatUsage) ToResponseUsage() ResponseUsage {
	usage := ResponseUsage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
		TotalTokens:  u.TotalTokens,
	}

	if u.PromptTokensDetails != nil &&
		(u.PromptTokensDetails.CachedTokens > 0 || u.PromptTokensDetails.CacheCreationTokens > 0) {
		usage.InputTokensDetails = &ResponseUsageDetails{
			CachedTokens: u.PromptTokensDetails.CachedTokens,
		}
	}

	if u.CompletionTokensDetails != nil && u.CompletionTokensDetails.ReasoningTokens > 0 {
		usage.OutputTokensDetails = &ResponseUsageDetails{
			ReasoningTokens: u.CompletionTokensDetails.ReasoningTokens,
		}
	}

	return usage
}

// ToGeminiUsage converts ChatUsage to GeminiUsageMetadata (Google Gemini format)
func (u ChatUsage) ToGeminiUsage() GeminiUsageMetadata {
	usage := GeminiUsageMetadata{
		PromptTokenCount:     u.PromptTokens,
		CandidatesTokenCount: u.CompletionTokens,
		TotalTokenCount:      u.TotalTokens,
	}

	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		usage.CachedContentTokenCount = u.PromptTokensDetails.CachedTokens
	}

	if u.CompletionTokensDetails != nil && u.CompletionTokensDetails.ReasoningTokens > 0 {
		usage.ThoughtsTokenCount = u.CompletionTokensDetails.ReasoningTokens
	}

	return usage
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
	ImageTokens              int64 `json:"image_tokens"`
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

func WrapperOpenAIError(err error, code any, statusCode int, _type ...string) adaptor.Error {
	return WrapperOpenAIErrorWithMessage(err.Error(), code, statusCode, _type...)
}

func WrapperOpenAIErrorWithMessage(
	message string,
	code any,
	statusCode int,
	_type ...string,
) adaptor.Error {
	errType := ErrorTypeAIPROXY
	if len(_type) > 0 {
		errType = _type[0]
	}

	return NewOpenAIError(statusCode, OpenAIError{
		Message: message,
		Type:    errType,
		Code:    code,
	})
}
