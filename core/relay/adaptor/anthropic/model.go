package anthropic

import (
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type OpenAIRequest struct {
	ToolChoice  any              `json:"tool_choice,omitempty"`
	Stop        any              `json:"stop,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	TopP        *float64         `json:"top_p,omitempty"`
	Model       string           `json:"model,omitempty"`
	Messages    []*OpenaiMessage `json:"messages,omitempty"`
	Tools       []*OpenaiTool    `json:"tools,omitempty"`
	Seed        float64          `json:"seed,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	TopK        int              `json:"top_k,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type OpenaiMessage struct {
	relaymodel.Message
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type OpenaiTool struct {
	relaymodel.Tool
	Name            string        `json:"name,omitempty"`
	DisplayWidthPx  int           `json:"display_width_px,omitempty"`
	DisplayHeightPx int           `json:"display_height_px,omitempty"`
	DisplayNumber   int           `json:"display_number,omitempty"`
	CacheControl    *CacheControl `json:"cache_control,omitempty"`

	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool#tool-definition
	MaxUses        int           `json:"max_uses,omitempty"`
	AllowedDomains []string      `json:"allowed_domains,omitempty"`
	BlockedDomains []string      `json:"blocked_domains,omitempty"`
	UserLocation   *UserLocation `json:"user_location,omitempty"`
}

// https://docs.anthropic.com/claude/reference/messages_post

type Metadata struct {
	UserID string `json:"user_id"`
}

type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type Content struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	Thinking     string        `json:"thinking,omitempty"`
	Source       *ImageSource  `json:"source,omitempty"`
	ID           string        `json:"id,omitempty"`
	Name         string        `json:"name,omitempty"`
	Input        any           `json:"input,omitempty"`
	Content      any           `json:"content,omitempty"`
	ToolUseID    string        `json:"tool_use_id,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type Message struct {
	Role    string     `json:"role"`
	Content []*Content `json:"content"`
}

type Tool struct {
	InputSchema     *InputSchema  `json:"input_schema,omitempty"`
	Name            string        `json:"name"`
	Description     string        `json:"description,omitempty"`
	Type            string        `json:"type,omitempty"`
	DisplayWidthPx  int           `json:"display_width_px,omitempty"`
	DisplayHeightPx int           `json:"display_height_px,omitempty"`
	DisplayNumber   int           `json:"display_number,omitempty"`
	CacheControl    *CacheControl `json:"cache_control,omitempty"`

	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool#tool-definition
	MaxUses        int           `json:"max_uses,omitempty"`
	AllowedDomains []string      `json:"allowed_domains,omitempty"`
	BlockedDomains []string      `json:"blocked_domains,omitempty"`
	UserLocation   *UserLocation `json:"user_location,omitempty"`
}

type UserLocation struct {
	Type     string `json:"type,omitempty"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type CacheControl struct {
	Type string `json:"type"`
	// "5m" | "1h"
	TTL string `json:"ttl,omitempty"`
}

func (cc *CacheControl) ResetTTL() *CacheControl {
	if cc == nil {
		return nil
	}
	cc.TTL = ""
	return cc
}

type InputSchema struct {
	Properties any    `json:"properties,omitempty"`
	Required   any    `json:"required,omitempty"`
	Type       string `json:"type"`
}

type Thinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type Request struct {
	ToolChoice    any       `json:"tool_choice,omitempty"`
	Temperature   *float64  `json:"temperature,omitempty"`
	TopP          *float64  `json:"top_p,omitempty"`
	Model         string    `json:"model,omitempty"`
	System        []Content `json:"system,omitempty"`
	Messages      []Message `json:"messages"`
	StopSequences []string  `json:"stop_sequences,omitempty"`
	Tools         []Tool    `json:"tools,omitempty"`
	MaxTokens     int       `json:"max_tokens,omitempty"`
	TopK          int       `json:"top_k,omitempty"`
	Stream        bool      `json:"stream,omitempty"`
	Thinking      *Thinking `json:"thinking,omitempty"`
}

type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`

	CacheCreationInputTokens int64          `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64          `json:"cache_read_input_tokens"`
	CacheCreation            *CacheCreation `json:"cache_creation,omitempty"`
	ServerToolUse            *ServerToolUse `json:"server_tool_use,omitempty"`
}

type ServerToolUse struct {
	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool
	WebSearchRequests int64 `json:"web_search_requests,omitempty"`
	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/code-execution-tool
	ExecutionTimeSeconds float64 `json:"execution_time_seconds,omitempty"`
}

func (u *Usage) ToOpenAIUsage() relaymodel.Usage {
	usage := relaymodel.Usage{
		PromptTokens:     u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
		CompletionTokens: u.OutputTokens,
		PromptTokensDetails: &relaymodel.PromptTokensDetails{
			CachedTokens:        u.CacheReadInputTokens,
			CacheCreationTokens: u.CacheCreationInputTokens,
		},
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	if u.ServerToolUse != nil {
		usage.WebSearchCount = u.ServerToolUse.WebSearchRequests
	}
	return usage
}

// https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching#1-hour-cache-duration-beta
type CacheCreation struct {
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens,omitempty"`
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens,omitempty"`
}

type Response struct {
	StopReason   string    `json:"stop_reason,omitempty"`
	StopSequence *string   `json:"stop_sequence"`
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Role         string    `json:"role"`
	Model        string    `json:"model"`
	Content      []Content `json:"content"`
	Usage        Usage     `json:"usage"`
}

type Delta struct {
	StopReason   *string `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
	Type         string  `json:"type"`
	Thinking     string  `json:"thinking,omitempty"`
	Text         string  `json:"text"`
	PartialJSON  string  `json:"partial_json,omitempty"`
}

type StreamResponse struct {
	Message      *Response `json:"message"`
	ContentBlock *Content  `json:"content_block"`
	Delta        *Delta    `json:"delta"`
	Usage        *Usage    `json:"usage"`
	Type         string    `json:"type"`
	Index        int       `json:"index"`
}
