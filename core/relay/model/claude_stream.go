package model

// Anthropic Messages API streaming spec requires certain fields to always be present,
// even when empty/null. The shared types (ClaudeResponse, ClaudeContent, ClaudeDelta)
// use `omitempty` for request parsing, which causes Go to omit zero-value fields.
// These dedicated output types ensure spec compliance for streaming conversion paths.
//
// See: https://docs.anthropic.com/en/api/messages-streaming

// ClaudeStreamStartMessage is used for message_start events.
// Unlike ClaudeResponse, stop_reason and stop_sequence are never omitted (serialized as null).
type ClaudeStreamStartMessage struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Model        string          `json:"model"`
	Content      []ClaudeContent `json:"content"`
	StopReason   *string         `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        ClaudeUsage     `json:"usage"`
}

// ClaudeStreamMessageDelta is used for message_delta event's delta field.
// stop_sequence must always be present (null when not applicable).
type ClaudeStreamMessageDelta struct {
	Type         string  `json:"type,omitempty"`
	StopReason   *string `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
}

// ClaudeTextContentBlock is used for content_block_start (text).
// The text field must always be present as "" per Anthropic spec.
type ClaudeTextContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ClaudeThinkingContentBlock is used for content_block_start (thinking).
// The thinking field must always be present as "" per Anthropic spec.
type ClaudeThinkingContentBlock struct {
	Type     string `json:"type"`
	Thinking string `json:"thinking"`
}

// Named wrapper types for Claude streaming events.
// These replace anonymous structs used at multiple call sites, ensuring
// the dedicated output-only types above are used instead of the shared
// request/response types (which have omitempty on required fields).

// ClaudeStreamMessageStartEvent wraps ClaudeStreamStartMessage for message_start events.
type ClaudeStreamMessageStartEvent struct {
	Type    string                    `json:"type"`
	Message *ClaudeStreamStartMessage `json:"message"`
}

// ClaudeStreamTextBlockStartEvent wraps ClaudeTextContentBlock for content_block_start (text).
type ClaudeStreamTextBlockStartEvent struct {
	Type         string                  `json:"type"`
	Index        int                     `json:"index"`
	ContentBlock *ClaudeTextContentBlock `json:"content_block"`
}

// ClaudeStreamThinkingBlockStartEvent wraps ClaudeThinkingContentBlock for content_block_start (thinking).
type ClaudeStreamThinkingBlockStartEvent struct {
	Type         string                      `json:"type"`
	Index        int                         `json:"index"`
	ContentBlock *ClaudeThinkingContentBlock `json:"content_block"`
}

// ClaudeStreamMessageDeltaEvent wraps ClaudeStreamMessageDelta for message_delta events.
type ClaudeStreamMessageDeltaEvent struct {
	Type  string                    `json:"type"`
	Delta *ClaudeStreamMessageDelta `json:"delta"`
	Usage *ClaudeUsage              `json:"usage,omitempty"`
}
