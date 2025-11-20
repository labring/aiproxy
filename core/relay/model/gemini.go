package model

import "github.com/labring/aiproxy/core/relay/adaptor"

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
