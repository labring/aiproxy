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
		usage.ImageInputTokens = model.ZeroNullInt64(u.PromptTokensDetails.ImageTokens)
		usage.AudioInputTokens = model.ZeroNullInt64(u.PromptTokensDetails.AudioTokens)
		usage.VideoInputTokens = model.ZeroNullInt64(u.PromptTokensDetails.VideoTokens)
		usage.CachedTokens = model.ZeroNullInt64(u.PromptTokensDetails.CachedTokens)
		usage.CacheCreationTokens = model.ZeroNullInt64(
			u.PromptTokensDetails.CacheCreationTokenCount(),
		)
	}

	if u.CompletionTokensDetails != nil {
		usage.ImageOutputTokens = model.ZeroNullInt64(u.CompletionTokensDetails.ImageTokens)
		usage.AudioOutputTokens = model.ZeroNullInt64(u.CompletionTokensDetails.AudioTokens)
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

	if other.CompletionTokensDetails != nil {
		if u.CompletionTokensDetails == nil {
			u.CompletionTokensDetails = &CompletionTokensDetails{}
		}

		u.CompletionTokensDetails.Add(other.CompletionTokensDetails)
	}
}

func (u ChatUsage) ToClaudeUsage() ClaudeUsage {
	cu := ClaudeUsage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
	}

	if u.PromptTokensDetails != nil {
		cu.CacheCreationInputTokens = u.PromptTokensDetails.CacheCreationTokenCount()
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
		(u.PromptTokensDetails.CachedTokens > 0 ||
			u.PromptTokensDetails.CacheCreationTokenCount() > 0 ||
			u.PromptTokensDetails.ImageTokens > 0 ||
			u.PromptTokensDetails.VideoTokens > 0 ||
			u.PromptTokensDetails.AudioTokens > 0) {
		usage.InputTokensDetails = &ResponseUsageDetails{
			AudioTokens:      u.PromptTokensDetails.AudioTokens,
			CachedTokens:     u.PromptTokensDetails.CachedTokens,
			CacheWriteTokens: u.PromptTokensDetails.CacheCreationTokenCount(),
			ImageTokens:      u.PromptTokensDetails.ImageTokens,
			VideoTokens:      u.PromptTokensDetails.VideoTokens,
		}
	}

	if u.CompletionTokensDetails != nil &&
		(u.CompletionTokensDetails.ReasoningTokens > 0 ||
			u.CompletionTokensDetails.AudioTokens > 0 ||
			u.CompletionTokensDetails.ImageTokens > 0) {
		usage.OutputTokensDetails = &ResponseUsageDetails{
			AudioTokens:     u.CompletionTokensDetails.AudioTokens,
			ReasoningTokens: u.CompletionTokensDetails.ReasoningTokens,
			ImageTokens:     u.CompletionTokensDetails.ImageTokens,
		}
	}

	return usage
}

// ToGeminiUsage converts ChatUsage to GeminiUsageMetadata (Google Gemini format).
// OpenAI completion_tokens include reasoning_tokens. Gemini candidatesTokenCount
// does not include thoughtsTokenCount, and ToUsage adds them back together.
func (u ChatUsage) ToGeminiUsage() GeminiUsageMetadata {
	var reasoningTokens int64
	if u.CompletionTokensDetails != nil {
		reasoningTokens = u.CompletionTokensDetails.ReasoningTokens
	}

	usage := GeminiUsageMetadata{
		PromptTokenCount:     u.PromptTokens,
		CandidatesTokenCount: max(0, u.CompletionTokens-reasoningTokens),
		TotalTokenCount:      u.TotalTokens,
	}

	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		usage.CachedContentTokenCount = u.PromptTokensDetails.CachedTokens
	}

	if reasoningTokens > 0 {
		usage.ThoughtsTokenCount = reasoningTokens
	}

	return usage
}

type PromptTokensDetails struct {
	CachedTokens        int64 `json:"cached_tokens"`
	AudioTokens         int64 `json:"audio_tokens"`
	ImageTokens         int64 `json:"image_tokens,omitempty"`
	VideoTokens         int64 `json:"video_tokens,omitempty"`
	CacheCreationTokens int64 `json:"cache_creation_tokens,omitempty"`
	CacheWriteTokens    int64 `json:"cache_write_tokens,omitempty"`
}

func (d *PromptTokensDetails) CacheCreationTokenCount() int64 {
	if d == nil {
		return 0
	}

	return max(d.CacheCreationTokens, d.CacheWriteTokens)
}

func (d *PromptTokensDetails) Add(other *PromptTokensDetails) {
	if other == nil {
		return
	}

	d.CachedTokens += other.CachedTokens
	d.AudioTokens += other.AudioTokens
	d.ImageTokens += other.ImageTokens
	d.VideoTokens += other.VideoTokens
	d.CacheCreationTokens += other.CacheCreationTokens
	d.CacheWriteTokens += other.CacheWriteTokens
}

type CompletionTokensDetails struct {
	ReasoningTokens          int64 `json:"reasoning_tokens"`
	AudioTokens              int64 `json:"audio_tokens"`
	AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
	ImageTokens              int64 `json:"image_tokens"`
}

func (d *CompletionTokensDetails) Add(other *CompletionTokensDetails) {
	if other == nil {
		return
	}

	d.ReasoningTokens += other.ReasoningTokens
	d.AudioTokens += other.AudioTokens
	d.AcceptedPredictionTokens += other.AcceptedPredictionTokens
	d.RejectedPredictionTokens += other.RejectedPredictionTokens
	d.ImageTokens += other.ImageTokens
}

type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

func (e OpenAIErrorResponse) ErrorCode() any {
	return e.Error.Code
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
