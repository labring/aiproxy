package model

import "github.com/labring/aiproxy/core/relay/adaptor"

// Gemini API request and response types
// https://ai.google.dev/api/generate-content

type GeminiChatRequest struct {
	Contents          []*GeminiChatContent        `json:"contents"`
	SystemInstruction *GeminiChatContent          `json:"system_instruction,omitempty"`
	SafetySettings    []GeminiChatSafetySettings  `json:"safety_settings,omitempty"`
	GenerationConfig  *GeminiChatGenerationConfig `json:"generation_config,omitempty"`
	Tools             []GeminiChatTools           `json:"tools,omitempty"`
	ToolConfig        *GeminiToolConfig           `json:"tool_config,omitempty"`
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
	Name     string `json:"name"`
	Response struct {
		Name    string         `json:"name"`
		Content map[string]any `json:"content"`
	} `json:"response"`
}

type GeminiChatSafetySettings struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type GeminiChatTools struct {
	FunctionDeclarations any `json:"function_declarations,omitempty"`
}

type GeminiChatGenerationConfig struct {
	ResponseSchema     any                   `json:"responseSchema,omitempty"`
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"topP,omitempty"`
	ResponseMimeType   string                `json:"responseMimeType,omitempty"`
	StopSequences      []string              `json:"stopSequences,omitempty"`
	TopK               float64               `json:"topK,omitempty"`
	MaxOutputTokens    *int                  `json:"maxOutputTokens,omitempty"`
	CandidateCount     int                   `json:"candidateCount,omitempty"`
	ResponseModalities []string              `json:"responseModalities,omitempty"`
	ThinkingConfig     *GeminiThinkingConfig `json:"thinking_config,omitempty"`
}

type GeminiThinkingConfig struct {
	ThinkingBudget  int  `json:"thinking_budget,omitempty"`
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
}

type GeminiFunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"`
	AllowedFunctionNames []string `json:"allowed_function_names,omitempty"`
}

type GeminiToolConfig struct {
	FunctionCallingConfig GeminiFunctionCallingConfig `json:"function_calling_config"`
}

type GeminiChatResponse struct {
	Candidates     []*GeminiChatCandidate    `json:"candidates"`
	PromptFeedback *GeminiChatPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsageMetadata      `json:"usageMetadata,omitempty"`
	ModelVersion   string                    `json:"modelVersion,omitempty"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount        int64                      `json:"promptTokenCount"`
	CandidatesTokenCount    int64                      `json:"candidatesTokenCount"`
	TotalTokenCount         int64                      `json:"totalTokenCount"`
	ThoughtsTokenCount      int64                      `json:"thoughtsTokenCount,omitempty"`
	PromptTokensDetails     []GeminiPromptTokensDetail `json:"promptTokensDetails"`
	CachedContentTokenCount int64                      `json:"cachedContentTokenCount,omitempty"`
	CacheTokensDetails      []GeminiCacheTokensDetail  `json:"cacheTokensDetails,omitempty"`
}

type GeminiPromptTokensDetail struct {
	Modality   string `json:"modality"`
	TokenCount int64  `json:"tokenCount"`
}

type GeminiCacheTokensDetail struct {
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
	Index int64 `json:"index"`
}

type GeminiChatPromptFeedback struct {
	SafetyRatings []struct {
		Category    string `json:"category"`
		Probability string `json:"probability"`
	} `json:"safetyRatings,omitempty"`
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
