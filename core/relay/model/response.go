package model

import (
	"slices"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
)

var nullBytes = []byte("null")

type ResponseArguments string

func (a *ResponseArguments) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.ConfigDefault.Unmarshal(data, &s); err == nil {
		*a = ResponseArguments(s)
		return nil
	}

	if slices.Equal(data, nullBytes) {
		*a = ""
		return nil
	}

	*a = ResponseArguments(data)

	return nil
}

func (a ResponseArguments) MarshalJSON() ([]byte, error) {
	return sonic.ConfigDefault.Marshal(string(a))
}

func (a ResponseArguments) String() string {
	return string(a)
}

// InputItemType represents the type of an input item
type InputItemType = string

const (
	InputItemTypeMessage            InputItemType = "message"
	InputItemTypeReasoning          InputItemType = "reasoning"
	InputItemTypeFunctionCall       InputItemType = "function_call"
	InputItemTypeFunctionCallOutput InputItemType = "function_call_output"
)

// InputContentType represents the type of input content
type InputContentType = string

const (
	InputContentTypeInputText  InputContentType = "input_text"
	InputContentTypeOutputText InputContentType = "output_text"
)

// OutputContentType represents the type of output content
type OutputContentType = string

const (
	OutputContentTypeText       OutputContentType = "text"
	OutputContentTypeOutputText OutputContentType = "output_text"
)

// ResponseStatus represents the status of a response
type ResponseStatus = string

const (
	ResponseStatusInProgress ResponseStatus = "in_progress"
	ResponseStatusQueued     ResponseStatus = "queued"
	ResponseStatusCompleted  ResponseStatus = "completed"
	ResponseStatusFailed     ResponseStatus = "failed"
	ResponseStatusIncomplete ResponseStatus = "incomplete"
	ResponseStatusCancelled  ResponseStatus = "cancelled"
)

// ResponseStreamEventType represents the type of a response stream event
type ResponseStreamEventType = string

const (
	// Response lifecycle events
	EventResponseCreated    ResponseStreamEventType = "response.created"
	EventResponseInProgress ResponseStreamEventType = "response.in_progress"
	EventResponseCompleted  ResponseStreamEventType = "response.completed"
	EventResponseFailed     ResponseStreamEventType = "response.failed"
	EventResponseIncomplete ResponseStreamEventType = "response.incomplete"
	EventResponseQueued     ResponseStreamEventType = "response.queued"
	EventResponseDone       ResponseStreamEventType = "response.done" // Legacy/compatibility

	// Output item events
	EventOutputItemAdded ResponseStreamEventType = "response.output_item.added"
	EventOutputItemDone  ResponseStreamEventType = "response.output_item.done"

	// Content part events
	EventContentPartAdded ResponseStreamEventType = "response.content_part.added"
	EventContentPartDone  ResponseStreamEventType = "response.content_part.done"

	// Text output events
	EventOutputTextDelta ResponseStreamEventType = "response.output_text.delta"
	EventOutputTextDone  ResponseStreamEventType = "response.output_text.done"

	// Refusal events
	EventRefusalDelta ResponseStreamEventType = "response.refusal.delta"
	EventRefusalDone  ResponseStreamEventType = "response.refusal.done"

	// Function call events
	EventFunctionCallArgumentsDelta ResponseStreamEventType = "response.function_call_arguments.delta"
	EventFunctionCallArgumentsDone  ResponseStreamEventType = "response.function_call_arguments.done"

	// Reasoning events
	EventReasoningSummaryPartAdded ResponseStreamEventType = "response.reasoning_summary_part.added"
	EventReasoningSummaryPartDone  ResponseStreamEventType = "response.reasoning_summary_part.done"
	EventReasoningSummaryTextDelta ResponseStreamEventType = "response.reasoning_summary_text.delta"
	EventReasoningSummaryTextDone  ResponseStreamEventType = "response.reasoning_summary_text.done"
	EventReasoningTextDelta        ResponseStreamEventType = "response.reasoning_text.delta"
	EventReasoningTextDone         ResponseStreamEventType = "response.reasoning_text.done"

	// Tool call events
	EventFileSearchCallInProgress ResponseStreamEventType = "response.file_search_call.in_progress"
	EventFileSearchCallSearching  ResponseStreamEventType = "response.file_search_call.searching"
	EventFileSearchCallCompleted  ResponseStreamEventType = "response.file_search_call.completed"

	EventWebSearchCallInProgress ResponseStreamEventType = "response.web_search_call.in_progress"
	EventWebSearchCallSearching  ResponseStreamEventType = "response.web_search_call.searching"
	EventWebSearchCallCompleted  ResponseStreamEventType = "response.web_search_call.completed"

	EventCodeInterpreterCallInProgress   ResponseStreamEventType = "response.code_interpreter_call.in_progress"
	EventCodeInterpreterCallInterpreting ResponseStreamEventType = "response.code_interpreter_call.interpreting"
	EventCodeInterpreterCallCompleted    ResponseStreamEventType = "response.code_interpreter_call.completed"
	EventCodeInterpreterCallCodeDelta    ResponseStreamEventType = "response.code_interpreter_call_code.delta"
	EventCodeInterpreterCallCodeDone     ResponseStreamEventType = "response.code_interpreter_call_code.done"

	EventImageGenerationCallInProgress   ResponseStreamEventType = "response.image_generation_call.in_progress"
	EventImageGenerationCallGenerating   ResponseStreamEventType = "response.image_generation_call.generating"
	EventImageGenerationCallCompleted    ResponseStreamEventType = "response.image_generation_call.completed"
	EventImageGenerationCallPartialImage ResponseStreamEventType = "response.image_generation_call.partial_image"

	EventMCPCallInProgress      ResponseStreamEventType = "response.mcp_call.in_progress"
	EventMCPCallCompleted       ResponseStreamEventType = "response.mcp_call.completed"
	EventMCPCallFailed          ResponseStreamEventType = "response.mcp_call.failed"
	EventMCPCallArgumentsDelta  ResponseStreamEventType = "response.mcp_call_arguments.delta"
	EventMCPCallArgumentsDone   ResponseStreamEventType = "response.mcp_call_arguments.done"
	EventMCPListToolsInProgress ResponseStreamEventType = "response.mcp_list_tools.in_progress"
	EventMCPListToolsCompleted  ResponseStreamEventType = "response.mcp_list_tools.completed"
	EventMCPListToolsFailed     ResponseStreamEventType = "response.mcp_list_tools.failed"

	EventCustomToolCallInputDelta ResponseStreamEventType = "response.custom_tool_call_input.delta"
	EventCustomToolCallInputDone  ResponseStreamEventType = "response.custom_tool_call_input.done"

	// Annotation events
	EventOutputTextAnnotationAdded ResponseStreamEventType = "response.output_text.annotation.added"

	// Error event
	EventError ResponseStreamEventType = "error"

	// Compatibility event
	EventKeepAlive ResponseStreamEventType = "keepalive"
)

// ResponseError represents an error in a response
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ResponseTool represents a tool in the Responses API format (flattened structure)
type ResponseTool struct {
	Type         string `json:"type"`
	Name         string `json:"name,omitempty"`
	Execution    string `json:"execution,omitempty"`
	Description  string `json:"description,omitempty"`
	Parameters   any    `json:"parameters,omitempty"`
	Strict       *bool  `json:"strict,omitempty"`
	DeferLoading *bool  `json:"defer_loading,omitempty"`
}

// IncompleteDetails represents details about why a response is incomplete
type IncompleteDetails struct {
	Reason string `json:"reason"`
}

// SummaryPart represents a part of the reasoning summary in response
type SummaryPart struct {
	Type string `json:"type"` // Always "summary_text"
	Text string `json:"text"`
}

// ResponseReasoning represents reasoning information
type ResponseReasoning struct {
	Effort  *string `json:"effort"`
	Summary any     `json:"summary,omitempty"` // string ("detailed", "auto", "concise") or []SummaryPart in response
}

// ResponseTextFormat represents text format configuration
type ResponseTextFormat struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
	Description string         `json:"description,omitempty"`
}

// ResponseText represents text configuration
type ResponseText struct {
	Format    ResponseTextFormat `json:"format"`
	Verbosity string             `json:"verbosity,omitempty"`
}

// OutputContent represents content in an output item
type OutputContent struct {
	Type        string                       `json:"type"`
	Text        string                       `json:"text,omitempty"`
	Refusal     string                       `json:"refusal,omitempty"`
	Annotations []any                        `json:"annotations,omitempty"`
	Logprobs    []ChatCompletionTokenLogprob `json:"logprobs,omitempty"`
}

type ResponseStreamOptions struct {
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
}

// OutputItem represents an output item in a response
type OutputItem struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Status    ResponseStatus    `json:"status,omitempty"`
	Role      string            `json:"role,omitempty"`
	Content   []OutputContent   `json:"content,omitempty"`
	Arguments ResponseArguments `json:"arguments,omitempty"` // For function_call type
	CallID    string            `json:"call_id,omitempty"`   // For function_call type
	Name      string            `json:"name,omitempty"`      // For function_call type
	Summary   any               `json:"summary,omitempty"`   // For reasoning type: []SummaryPart or string
}

// InputContent represents content in an input item
type InputContent struct {
	Type                  string `json:"type"`
	Text                  string `json:"text,omitempty"`
	Refusal               string `json:"refusal,omitempty"`
	PromptCacheBreakpoint any    `json:"prompt_cache_breakpoint,omitempty"`
	// Fields for input_image type
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Detail   string `json:"detail,omitempty"`
	// Fields for function_call type
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	// Fields for function_result type
	CallID string `json:"call_id,omitempty"`
	Output string `json:"output,omitempty"`
}

// InputItem represents an input item
type InputItem struct {
	ID      string         `json:"id,omitempty"`
	Type    string         `json:"type"`
	Role    string         `json:"role,omitempty"`
	Content []InputContent `json:"content,omitempty"`
	// Fields for function_call type
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	// Fields for function_result type
	CallID string `json:"call_id,omitempty"`
	Output string `json:"output,omitempty"`
}

// ResponseUsageDetails represents detailed token usage information
type ResponseUsageDetails struct {
	AudioTokens      int64 `json:"audio_tokens,omitempty"`
	CachedTokens     int64 `json:"cached_tokens,omitempty"`
	CacheWriteTokens int64 `json:"cache_write_tokens,omitempty"`
	ReasoningTokens  int64 `json:"reasoning_tokens,omitempty"`
	ImageTokens      int64 `json:"image_tokens,omitempty"`
	VideoTokens      int64 `json:"video_tokens,omitempty"`
}

// ResponseUsage represents usage information for a response
type ResponseUsage struct {
	InputTokens         int64                 `json:"input_tokens"`
	OutputTokens        int64                 `json:"output_tokens"`
	TotalTokens         int64                 `json:"total_tokens"`
	InputTokensDetails  *ResponseUsageDetails `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *ResponseUsageDetails `json:"output_tokens_details,omitempty"`
}

type ResponseToolUsageWebSearch struct {
	NumRequests int64 `json:"num_requests,omitempty"`
}

type ResponseToolUsageTokensDetails struct {
	ImageTokens int64 `json:"image_tokens,omitempty"`
	TextTokens  int64 `json:"text_tokens,omitempty"`
}

type ResponseToolUsageImageGen struct {
	InputTokens         int64                           `json:"input_tokens,omitempty"`
	InputTokensDetails  *ResponseToolUsageTokensDetails `json:"input_tokens_details,omitempty"`
	OutputTokens        int64                           `json:"output_tokens,omitempty"`
	OutputTokensDetails *ResponseToolUsageTokensDetails `json:"output_tokens_details,omitempty"`
	TotalTokens         int64                           `json:"total_tokens,omitempty"`
}

type ResponseToolUsage struct {
	ImageGen  *ResponseToolUsageImageGen  `json:"image_gen,omitempty"`
	WebSearch *ResponseToolUsageWebSearch `json:"web_search,omitempty"`
}

// Response represents an OpenAI response object
type Response struct {
	ID                   string             `json:"id"`
	Object               string             `json:"object"`
	CreatedAt            int64              `json:"created_at"`
	Status               ResponseStatus     `json:"status"`
	Background           *bool              `json:"background,omitempty"`
	Error                *ResponseError     `json:"error"`
	IncompleteDetails    *IncompleteDetails `json:"incomplete_details"`
	Instructions         *string            `json:"instructions"`
	MaxOutputTokens      *int               `json:"max_output_tokens"`
	Model                string             `json:"model"`
	Output               []OutputItem       `json:"output"`
	ParallelToolCalls    bool               `json:"parallel_tool_calls"`
	PreviousResponseID   *string            `json:"previous_response_id"`
	PromptCacheRetention *string            `json:"prompt_cache_retention,omitempty"`
	Reasoning            ResponseReasoning  `json:"reasoning"`
	Store                bool               `json:"store"`
	Temperature          float64            `json:"temperature"`
	Text                 ResponseText       `json:"text"`
	ToolChoice           any                `json:"tool_choice"`
	Tools                []ResponseTool     `json:"tools"`
	ToolUsage            *ResponseToolUsage `json:"tool_usage,omitempty"`
	TopP                 float64            `json:"top_p"`
	Truncation           string             `json:"truncation"`
	Usage                *ResponseUsage     `json:"usage"`
	ServiceTier          *string            `json:"service_tier,omitempty"`
	User                 *string            `json:"user"`
	Metadata             map[string]any     `json:"metadata"`
	Moderation           any                `json:"moderation,omitempty"`
}

// CreateResponseRequest represents a request to create a response
type CreateResponseRequest struct {
	Model                string                 `json:"model"`
	Input                any                    `json:"input"`
	Background           *bool                  `json:"background,omitempty"`
	Conversation         any                    `json:"conversation,omitempty"` // string or object
	Include              []string               `json:"include,omitempty"`
	Instructions         *string                `json:"instructions,omitempty"`
	MaxOutputTokens      *int                   `json:"max_output_tokens,omitempty"`
	MaxToolCalls         *int                   `json:"max_tool_calls,omitempty"`
	Metadata             map[string]any         `json:"metadata,omitempty"`
	ParallelToolCalls    *bool                  `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID   *string                `json:"previous_response_id,omitempty"`
	PromptCacheKey       *string                `json:"prompt_cache_key,omitempty"`
	PromptCacheOptions   *PromptCacheOptions    `json:"prompt_cache_options,omitempty"`
	PromptCacheRetention *string                `json:"prompt_cache_retention,omitempty"`
	Reasoning            *ResponseReasoning     `json:"reasoning,omitempty"`
	SafetyIdentifier     *string                `json:"safety_identifier,omitempty"`
	Moderation           any                    `json:"moderation,omitempty"`
	ServiceTier          *string                `json:"service_tier,omitempty"`
	Store                *bool                  `json:"store,omitempty"`
	Stream               bool                   `json:"stream,omitempty"`
	StreamOptions        *ResponseStreamOptions `json:"stream_options,omitempty"`
	Temperature          *float64               `json:"temperature,omitempty"`
	Text                 *ResponseText          `json:"text,omitempty"`
	ToolChoice           any                    `json:"tool_choice,omitempty"`
	Tools                []ResponseTool         `json:"tools,omitempty"`
	TopLogprobs          *int                   `json:"top_logprobs,omitempty"`
	TopP                 *float64               `json:"top_p,omitempty"`
	Truncation           *string                `json:"truncation,omitempty"`
	User                 *string                `json:"user,omitempty"` // Deprecated, use prompt_cache_key
}

// InputItemList represents a list of input items
type InputItemList struct {
	Object  string      `json:"object"`
	Data    []InputItem `json:"data"`
	FirstID string      `json:"first_id"`
	LastID  string      `json:"last_id"`
	HasMore bool        `json:"has_more"`
}

// ResponseStreamEvent represents a server-sent event for response streaming
type ResponseStreamEvent struct {
	Type           string                       `json:"type"`
	Response       *Response                    `json:"response,omitempty"`
	Error          *OpenAIError                 `json:"error,omitempty"`
	OutputIndex    *int                         `json:"output_index,omitempty"`
	Item           *OutputItem                  `json:"item,omitempty"`
	ItemID         string                       `json:"item_id,omitempty"`
	ContentIndex   *int                         `json:"content_index,omitempty"`
	Part           *OutputContent               `json:"part,omitempty"`  // For content_part events
	Delta          string                       `json:"delta,omitempty"` // For text.delta, function_call_arguments.delta
	Logprobs       []ChatCompletionTokenLogprob `json:"logprobs,omitempty"`
	Obfuscation    string                       `json:"obfuscation,omitempty"`
	Text           string                       `json:"text,omitempty"`      // For text content
	Arguments      ResponseArguments            `json:"arguments,omitempty"` // For function_call_arguments.done
	SequenceNumber int                          `json:"sequence_number,omitempty"`
}

func (r *Response) ToolUsageWebSearchCallCount() int64 {
	if r == nil || r.ToolUsage == nil || r.ToolUsage.WebSearch == nil {
		return 0
	}

	return r.ToolUsage.WebSearch.NumRequests
}

func (r *Response) ToModelUsage() model.Usage {
	if r == nil {
		return model.Usage{}
	}

	var usage model.Usage
	if r.Usage != nil {
		usage = r.Usage.ToModelUsage()
	}

	if count := r.ToolUsageWebSearchCallCount(); count > 0 {
		usage.WebSearchCount = model.ZeroNullInt64(count)
	}

	if r.ToolUsage != nil && r.ToolUsage.ImageGen != nil {
		imageUsage := r.ToolUsage.ImageGen
		usage.InputTokens += model.ZeroNullInt64(imageUsage.InputTokens)
		usage.OutputTokens += model.ZeroNullInt64(imageUsage.OutputTokens)
		usage.TotalTokens += model.ZeroNullInt64(imageUsage.TotalTokens)

		if imageUsage.InputTokensDetails != nil {
			usage.ImageInputTokens += model.ZeroNullInt64(
				imageUsage.InputTokensDetails.ImageTokens,
			)
		}

		if imageUsage.OutputTokensDetails != nil {
			usage.ImageOutputTokens += model.ZeroNullInt64(
				imageUsage.OutputTokensDetails.ImageTokens,
			)
		}
	}

	return usage
}

func (u *ResponseUsage) ToModelUsage() model.Usage {
	usage := model.Usage{
		InputTokens:  model.ZeroNullInt64(u.InputTokens),
		OutputTokens: model.ZeroNullInt64(u.OutputTokens),
		TotalTokens:  model.ZeroNullInt64(u.TotalTokens),
	}

	if u.InputTokensDetails != nil {
		usage.ImageInputTokens = model.ZeroNullInt64(u.InputTokensDetails.ImageTokens)
		usage.AudioInputTokens = model.ZeroNullInt64(u.InputTokensDetails.AudioTokens)
		usage.VideoInputTokens = model.ZeroNullInt64(u.InputTokensDetails.VideoTokens)
		usage.CachedTokens = model.ZeroNullInt64(u.InputTokensDetails.CachedTokens)
		usage.CacheCreationTokens = model.ZeroNullInt64(u.InputTokensDetails.CacheWriteTokens)
	}

	if u.OutputTokensDetails != nil {
		usage.ImageOutputTokens = model.ZeroNullInt64(u.OutputTokensDetails.ImageTokens)
		usage.AudioOutputTokens = model.ZeroNullInt64(u.OutputTokensDetails.AudioTokens)
		usage.ReasoningTokens = model.ZeroNullInt64(u.OutputTokensDetails.ReasoningTokens)
	}

	return usage
}

// ToChatUsage converts ResponseUsage to ChatUsage (OpenAI Chat Completions format)
func (u *ResponseUsage) ToChatUsage() ChatUsage {
	usage := ChatUsage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
	}

	if u.InputTokensDetails != nil &&
		(u.InputTokensDetails.CachedTokens > 0 ||
			u.InputTokensDetails.CacheWriteTokens > 0 ||
			u.InputTokensDetails.AudioTokens > 0 ||
			u.InputTokensDetails.ImageTokens > 0 ||
			u.InputTokensDetails.VideoTokens > 0) {
		usage.PromptTokensDetails = &PromptTokensDetails{
			AudioTokens:      u.InputTokensDetails.AudioTokens,
			CachedTokens:     u.InputTokensDetails.CachedTokens,
			CacheWriteTokens: u.InputTokensDetails.CacheWriteTokens,
			ImageTokens:      u.InputTokensDetails.ImageTokens,
			VideoTokens:      u.InputTokensDetails.VideoTokens,
		}
	}

	if u.OutputTokensDetails != nil &&
		(u.OutputTokensDetails.ReasoningTokens > 0 ||
			u.OutputTokensDetails.AudioTokens > 0 ||
			u.OutputTokensDetails.ImageTokens > 0) {
		usage.CompletionTokensDetails = &CompletionTokensDetails{
			AudioTokens:     u.OutputTokensDetails.AudioTokens,
			ReasoningTokens: u.OutputTokensDetails.ReasoningTokens,
			ImageTokens:     u.OutputTokensDetails.ImageTokens,
		}
	}

	return usage
}

// ToClaudeUsage converts ResponseUsage to ClaudeUsage (Anthropic Claude format)
func (u *ResponseUsage) ToClaudeUsage() ClaudeUsage {
	usage := ClaudeUsage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
	}

	if u.InputTokensDetails != nil && u.InputTokensDetails.CachedTokens > 0 {
		usage.CacheReadInputTokens = u.InputTokensDetails.CachedTokens
	}

	if u.InputTokensDetails != nil && u.InputTokensDetails.CacheWriteTokens > 0 {
		usage.CacheCreationInputTokens = u.InputTokensDetails.CacheWriteTokens
	}

	return usage
}

// ToGeminiUsage converts ResponseUsage to GeminiUsageMetadata (Google Gemini format).
// OpenAI output_tokens include reasoning_tokens. Gemini candidatesTokenCount
// does not include thoughtsTokenCount, and ToUsage adds them back together.
func (u *ResponseUsage) ToGeminiUsage() GeminiUsageMetadata {
	var reasoningTokens int64
	if u.OutputTokensDetails != nil {
		reasoningTokens = u.OutputTokensDetails.ReasoningTokens
	}

	usage := GeminiUsageMetadata{
		PromptTokenCount:     u.InputTokens,
		CandidatesTokenCount: max(0, u.OutputTokens-reasoningTokens),
		TotalTokenCount:      u.TotalTokens,
	}

	if u.InputTokensDetails != nil && u.InputTokensDetails.CachedTokens > 0 {
		usage.CachedContentTokenCount = u.InputTokensDetails.CachedTokens
	}

	if reasoningTokens > 0 {
		usage.ThoughtsTokenCount = reasoningTokens
	}

	return usage
}
