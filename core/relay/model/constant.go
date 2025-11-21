package model

// Common Role constants (used across different API formats)
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

const (
	ContentTypeText       = "text"
	ContentTypeImageURL   = "image_url"
	ContentTypeInputAudio = "input_audio"
)

const (
	ChatCompletionChunkObject = "chat.completion.chunk"
	ChatCompletionObject      = "chat.completion"
	VideoGenerationJobObject  = "video.generation.job"
	VideoGenerationObject     = "video.generation"
)

type FinishReason = string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonContentFilter FinishReason = "content_filter"
	FinishReasonToolCalls     FinishReason = "tool_calls"
	FinishReasonFunctionCall  FinishReason = "function_call"
)

// Tool Choice constants (used in OpenAI API)
const (
	ToolChoiceAuto     = "auto"
	ToolChoiceNone     = "none"
	ToolChoiceRequired = "required"
	ToolChoiceAny      = "any"
)

// Tool Choice Type constants
const (
	ToolChoiceTypeFunction = "function"
	ToolChoiceTypeTool     = "tool"
)
