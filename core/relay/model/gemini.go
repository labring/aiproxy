package model

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

// Gemini API request and response types
// https://ai.google.dev/api/generate-content

type GeminiChatRequest struct {
	Contents          []*GeminiChatContent        `json:"contents"`
	SystemInstruction *GeminiChatContent          `json:"systemInstruction,omitempty"`
	SafetySettings    []GeminiChatSafetySettings  `json:"safetySettings,omitempty"`
	GenerationConfig  *GeminiChatGenerationConfig `json:"generationConfig,omitempty"`
	Tools             []GeminiChatTools           `json:"tools,omitempty"`
	ToolConfig        *GeminiToolConfig           `json:"toolConfig,omitempty"`
}

type GeminiChatContent struct {
	Role  string        `json:"role,omitempty"`
	Parts []*GeminiPart `json:"parts"`
}

type GeminiPart struct {
	InlineData       *GeminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
	Text             string                  `json:"text,omitempty"`
	Thought          bool                    `json:"thought,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GeminiFunctionCall struct {
	Args map[string]any `json:"args"`
	Name string         `json:"name"`
}

type GeminiFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
	// vertexai gemini not support `id` filed
	ID string `json:"id,omitempty"`
}

type GeminiChatSafetySettings struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type GeminiChatTools struct {
	FunctionDeclarations any `json:"functionDeclarations,omitempty"`
}

type GeminiChatGenerationConfig struct {
	ResponseSchema     map[string]any        `json:"responseSchema,omitempty"`
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"topP,omitempty"`
	ResponseMimeType   string                `json:"responseMimeType,omitempty"`
	StopSequences      []string              `json:"stopSequences,omitempty"`
	TopK               float64               `json:"topK,omitempty"`
	MaxOutputTokens    *int                  `json:"maxOutputTokens,omitempty"`
	CandidateCount     int                   `json:"candidateCount,omitempty"`
	ResponseModalities []string              `json:"responseModalities,omitempty"`
	ThinkingConfig     *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

type GeminiThinkingConfig struct {
	ThinkingBudget  int  `json:"thinkingBudget,omitempty"`
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
}

type GeminiFunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type GeminiToolConfig struct {
	FunctionCallingConfig GeminiFunctionCallingConfig `json:"functionCallingConfig"`
}

type GeminiChatResponse struct {
	Candidates     []*GeminiChatCandidate    `json:"candidates"`
	PromptFeedback *GeminiChatPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsageMetadata      `json:"usageMetadata,omitempty"`
	ModelVersion   string                    `json:"modelVersion,omitempty"`
}

// GetWebSearchCount returns the total number of web search queries from all candidates
func (r *GeminiChatResponse) GetWebSearchCount() int64 {
	var count int64
	for _, candidate := range r.Candidates {
		if candidate.GroundingMetadata != nil {
			count += int64(len(candidate.GroundingMetadata.WebSearchQueries))
		}
	}
	return count
}

type GeminiUsageMetadata struct {
	PromptTokenCount            int64                `json:"promptTokenCount"`
	CandidatesTokenCount        int64                `json:"candidatesTokenCount"`
	TotalTokenCount             int64                `json:"totalTokenCount"`
	ThoughtsTokenCount          int64                `json:"thoughtsTokenCount,omitempty"`
	PromptTokensDetails         []GeminiTokensDetail `json:"promptTokensDetails"`
	CandidatesTokensDetails     []GeminiTokensDetail `json:"candidatesTokensDetails,omitempty"`
	CachedContentTokenCount     int64                `json:"cachedContentTokenCount,omitempty"`
	CacheTokensDetails          []GeminiTokensDetail `json:"cacheTokensDetails,omitempty"`
	ToolUsePromptTokenCount     int64                `json:"toolUsePromptTokenCount,omitempty"`
	ToolUsePromptTokensDetails  []GeminiTokensDetail `json:"toolUsePromptTokensDetails,omitempty"`
}

type GeminiTokensDetail struct {
	Modality   string `json:"modality"`
	TokenCount int64  `json:"tokenCount"`
}

type GeminiChatCandidate struct {
	FinishReason  string            `json:"finishReason,omitempty"`
	Content       GeminiChatContent `json:"content"`
	SafetyRatings []struct {
		Category    string `json:"category"`
		Probability string `json:"probability"`
	} `json:"safetyRatings,omitempty"`
	Index             int64                    `json:"index"`
	GroundingMetadata *GeminiGroundingMetadata `json:"groundingMetadata,omitempty"`
}

type GeminiGroundingMetadata struct {
	WebSearchQueries []string `json:"webSearchQueries,omitempty"`
}

type GeminiChatPromptFeedback struct {
	SafetyRatings []struct {
		Category    string `json:"category"`
		Probability string `json:"probability"`
	} `json:"safetyRatings,omitempty"`
}

// Gemini Modality constants
const (
	GeminiModalityText  = "TEXT"
	GeminiModalityImage = "IMAGE"
	GeminiModalityAudio = "AUDIO"
	GeminiModalityVideo = "VIDEO"
)

// GetImageInputTokens returns the number of image input tokens from PromptTokensDetails
func (u *GeminiUsageMetadata) GetImageInputTokens() int64 {
	for _, detail := range u.PromptTokensDetails {
		if detail.Modality == GeminiModalityImage {
			return detail.TokenCount
		}
	}
	return 0
}

// GetImageOutputTokens returns the number of image output tokens from CandidatesTokensDetails
func (u *GeminiUsageMetadata) GetImageOutputTokens() int64 {
	for _, detail := range u.CandidatesTokensDetails {
		if detail.Modality == GeminiModalityImage {
			return detail.TokenCount
		}
	}
	return 0
}

// ToUsage converts GeminiUsageMetadata to ChatUsage format
func (u *GeminiUsageMetadata) ToUsage() ChatUsage {
	chatUsage := ChatUsage{
		PromptTokens: u.PromptTokenCount,
		CompletionTokens: u.CandidatesTokenCount +
			u.ThoughtsTokenCount,
		TotalTokens: u.TotalTokenCount,
		PromptTokensDetails: &PromptTokensDetails{
			CachedTokens: u.CachedContentTokenCount,
		},
		CompletionTokensDetails: &CompletionTokensDetails{
			ReasoningTokens: u.ThoughtsTokenCount,
		},
	}

	return chatUsage
}

// ToModelUsage converts GeminiUsageMetadata to model.Usage format with image token support
func (u *GeminiUsageMetadata) ToModelUsage() model.Usage {
	// Input tokens should include both prompt tokens and tool use prompt tokens
	inputTokens := u.PromptTokenCount + u.ToolUsePromptTokenCount

	usage := model.Usage{
		InputTokens:       model.ZeroNullInt64(inputTokens),
		ImageInputTokens:  model.ZeroNullInt64(u.GetImageInputTokens()),
		OutputTokens:      model.ZeroNullInt64(u.CandidatesTokenCount + u.ThoughtsTokenCount),
		ImageOutputTokens: model.ZeroNullInt64(u.GetImageOutputTokens()),
		CachedTokens:      model.ZeroNullInt64(u.CachedContentTokenCount),
		ReasoningTokens:   model.ZeroNullInt64(u.ThoughtsTokenCount),
		TotalTokens:       model.ZeroNullInt64(u.TotalTokenCount),
	}

	return usage
}

// ToResponseUsage converts GeminiUsageMetadata to ResponseUsage (OpenAI Responses API format)
func (u *GeminiUsageMetadata) ToResponseUsage() ResponseUsage {
	usage := ResponseUsage{
		InputTokens:  u.PromptTokenCount,
		OutputTokens: u.CandidatesTokenCount,
		TotalTokens:  u.TotalTokenCount,
	}

	if u.CachedContentTokenCount > 0 {
		usage.InputTokensDetails = &ResponseUsageDetails{
			CachedTokens: u.CachedContentTokenCount,
		}
	}

	if u.ThoughtsTokenCount > 0 {
		usage.OutputTokensDetails = &ResponseUsageDetails{
			ReasoningTokens: u.ThoughtsTokenCount,
		}
	}

	return usage
}

// ToClaudeUsage converts GeminiUsageMetadata to ClaudeUsage (Anthropic Claude format)
func (u *GeminiUsageMetadata) ToClaudeUsage() ClaudeUsage {
	usage := ClaudeUsage{
		InputTokens:  u.PromptTokenCount,
		OutputTokens: u.CandidatesTokenCount,
	}

	if u.CachedContentTokenCount > 0 {
		usage.CacheReadInputTokens = u.CachedContentTokenCount
	}

	return usage
}

type GeminiError struct {
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
	Code    int    `json:"code,omitempty"`
}

type GeminiErrorResponse struct {
	Error GeminiError `json:"error,omitempty"`
}

func NewGeminiError(statusCode int, err GeminiError) adaptor.Error {
	return adaptor.NewError(statusCode, GeminiErrorResponse{
		Error: err,
	})
}

// Gemini Role constants
const (
	GeminiRoleModel = "model"
	GeminiRoleUser  = "user"
)

// Gemini Finish Reason constants
const (
	GeminiFinishReasonStop         = "STOP"
	GeminiFinishReasonMaxTokens    = "MAX_TOKENS"
	GeminiFinishReasonSafety       = "SAFETY"
	GeminiFinishReasonRecitation   = "RECITATION"
	GeminiFinishReasonOther        = "OTHER"
	GeminiFinishReasonToolCalls    = "TOOL_CALLS"
	GeminiFinishReasonFunctionCall = "FUNCTION_CALL"
)

// Gemini FunctionCallingConfig Mode constants
const (
	GeminiFunctionCallingModeAuto = "AUTO"
	GeminiFunctionCallingModeAny  = "ANY"
	GeminiFunctionCallingModeNone = "NONE"
)

// Gemini Safety Setting Category constants
const (
	GeminiSafetyCategoryHarassment       = "HARM_CATEGORY_HARASSMENT"
	GeminiSafetyCategoryHateSpeech       = "HARM_CATEGORY_HATE_SPEECH"
	GeminiSafetyCategorySexuallyExplicit = "HARM_CATEGORY_SEXUALLY_EXPLICIT"
	GeminiSafetyCategoryDangerousContent = "HARM_CATEGORY_DANGEROUS_CONTENT"
	GeminiSafetyCategoryCivicIntegrity   = "HARM_CATEGORY_CIVIC_INTEGRITY"
)

// Gemini Safety Setting Threshold constants
const (
	GeminiSafetyThresholdBlockNone           = "BLOCK_NONE"
	GeminiSafetyThresholdBlockLowAndAbove    = "BLOCK_LOW_AND_ABOVE"
	GeminiSafetyThresholdBlockMediumAndAbove = "BLOCK_MEDIUM_AND_ABOVE"
	GeminiSafetyThresholdBlockOnlyHigh       = "BLOCK_ONLY_HIGH"
)
