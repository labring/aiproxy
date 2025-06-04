package model

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
