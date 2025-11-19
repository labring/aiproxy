package model

type ClaudeOpenAIRequest struct {
	ToolChoice  any                    `json:"tool_choice,omitempty"`
	Stop        any                    `json:"stop,omitempty"`
	Temperature *float64               `json:"temperature,omitempty"`
	TopP        *float64               `json:"top_p,omitempty"`
	Model       string                 `json:"model,omitempty"`
	Messages    []*ClaudeOpenaiMessage `json:"messages,omitempty"`
	Tools       []*ClaudeOpenaiTool    `json:"tools,omitempty"`
	Seed        float64                `json:"seed,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	TopK        int                    `json:"top_k,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
}

type ClaudeOpenaiMessage struct {
	Message
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"`
}

type ClaudeOpenaiTool struct {
	Tool
	Name            string              `json:"name,omitempty"`
	DisplayWidthPx  int                 `json:"display_width_px,omitempty"`
	DisplayHeightPx int                 `json:"display_height_px,omitempty"`
	DisplayNumber   int                 `json:"display_number,omitempty"`
	CacheControl    *ClaudeCacheControl `json:"cache_control,omitempty"`

	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool#tool-definition
	MaxUses        int                 `json:"max_uses,omitempty"`
	AllowedDomains []string            `json:"allowed_domains,omitempty"`
	BlockedDomains []string            `json:"blocked_domains,omitempty"`
	UserLocation   *ClaudeUserLocation `json:"user_location,omitempty"`
}

// https://docs.anthropic.com/claude/reference/messages_post

type ClaudeMetadata struct {
	UserID string `json:"user_id"`
}

type ClaudeImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type ClaudeContent struct {
	Type         string              `json:"type"`
	Text         string              `json:"text,omitempty"`
	Thinking     string              `json:"thinking,omitempty"`
	Source       *ClaudeImageSource  `json:"source,omitempty"`
	ID           string              `json:"id,omitempty"`
	Name         string              `json:"name,omitempty"`
	Input        any                 `json:"input,omitempty"`
	Content      any                 `json:"content,omitempty"`
	ToolUseID    string              `json:"tool_use_id,omitempty"`
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"`
	Signature    string              `json:"thought_signature,omitempty"`
}

type ClaudeAnyContentMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type ClaudeMessage struct {
	Role    string          `json:"role"`
	Content []ClaudeContent `json:"content"`
}

type ClaudeTool struct {
	InputSchema     *ClaudeInputSchema  `json:"input_schema,omitempty"`
	Name            string              `json:"name"`
	Description     string              `json:"description,omitempty"`
	Type            string              `json:"type,omitempty"`
	DisplayWidthPx  int                 `json:"display_width_px,omitempty"`
	DisplayHeightPx int                 `json:"display_height_px,omitempty"`
	DisplayNumber   int                 `json:"display_number,omitempty"`
	CacheControl    *ClaudeCacheControl `json:"cache_control,omitempty"`

	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool#tool-definition
	MaxUses        int                 `json:"max_uses,omitempty"`
	AllowedDomains []string            `json:"allowed_domains,omitempty"`
	BlockedDomains []string            `json:"blocked_domains,omitempty"`
	UserLocation   *ClaudeUserLocation `json:"user_location,omitempty"`
}

type ClaudeUserLocation struct {
	Type     string `json:"type,omitempty"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type ClaudeCacheControl struct {
	Type string `json:"type"`
	// "5m" | "1h"
	TTL string `json:"ttl,omitempty"`
}

func (cc *ClaudeCacheControl) ResetTTL() *ClaudeCacheControl {
	if cc == nil {
		return nil
	}

	cc.TTL = ""

	return cc
}

type ClaudeInputSchema struct {
	Properties any    `json:"properties,omitempty"`
	Required   any    `json:"required,omitempty"`
	Type       string `json:"type"`
}

type ClaudeThinkingType = string

const (
	ClaudeThinkingTypeEnabled  ClaudeThinkingType = "enabled"
	ClaudeThinkingTypeDisabled ClaudeThinkingType = "disabled"
)

type ClaudeThinking struct {
	Type ClaudeThinkingType `json:"type"`
	// when type is "disabled", this field must be 0
	BudgetTokens int `json:"budget_tokens,omitempty"`
}

type ClaudeRequest struct {
	ToolChoice    any             `json:"tool_choice,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	Model         string          `json:"model,omitempty"`
	System        []ClaudeContent `json:"system,omitempty"`
	Messages      []ClaudeMessage `json:"messages"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Tools         []ClaudeTool    `json:"tools,omitempty"`
	MaxTokens     int             `json:"max_tokens,omitempty"`
	TopK          int             `json:"top_k,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Thinking      *ClaudeThinking `json:"thinking,omitempty"`
}

type ClaudeAnyContentRequest struct {
	ToolChoice          any                       `json:"tool_choice,omitempty"`
	Temperature         *float64                  `json:"temperature,omitempty"`
	TopP                *float64                  `json:"top_p,omitempty"`
	Model               string                    `json:"model,omitempty"`
	System              []ClaudeContent           `json:"system,omitempty"`
	Messages            []ClaudeAnyContentMessage `json:"messages"`
	StopSequences       []string                  `json:"stop_sequences,omitempty"`
	Tools               []ClaudeTool              `json:"tools,omitempty"`
	MaxTokens           int                       `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                       `json:"max_completion_tokens,omitempty"`
	TopK                int                       `json:"top_k,omitempty"`
	Stream              bool                      `json:"stream,omitempty"`
	Thinking            *ClaudeThinking           `json:"thinking,omitempty"`
}

type ClaudeUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`

	CacheCreationInputTokens int64                `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64                `json:"cache_read_input_tokens"`
	CacheCreation            *ClaudeCacheCreation `json:"cache_creation,omitempty"`
	ServerToolUse            *ClaudeServerToolUse `json:"server_tool_use,omitempty"`
}

type ClaudeServerToolUse struct {
	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool
	WebSearchRequests int64 `json:"web_search_requests,omitempty"`
	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/code-execution-tool
	ExecutionTimeSeconds float64 `json:"execution_time_seconds,omitempty"`
}

func (u *ClaudeUsage) ToOpenAIUsage() ChatUsage {
	usage := ChatUsage{
		PromptTokens:     u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
		CompletionTokens: u.OutputTokens,
		PromptTokensDetails: &PromptTokensDetails{
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
type ClaudeCacheCreation struct {
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens,omitempty"`
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens,omitempty"`
}

type ClaudeResponse struct {
	StopReason   string          `json:"stop_reason,omitempty"`
	StopSequence *string         `json:"stop_sequence,omitempty"`
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Model        string          `json:"model"`
	Content      []ClaudeContent `json:"content"`
	Usage        ClaudeUsage     `json:"usage"`
}

type ClaudeDelta struct {
	StopReason   *string `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
	Type         string  `json:"type,omitempty"`
	Thinking     string  `json:"thinking,omitempty"`
	Signature    string  `json:"signature,omitempty"`
	Text         string  `json:"text,omitempty"`
	PartialJSON  string  `json:"partial_json,omitempty"`
}

type ClaudeStreamResponse struct {
	Message      *ClaudeResponse `json:"message,omitempty"`
	ContentBlock *ClaudeContent  `json:"content_block,omitempty"`
	Delta        *ClaudeDelta    `json:"delta,omitempty"`
	Usage        *ClaudeUsage    `json:"usage,omitempty"`
	Type         string          `json:"type"`
	Index        int             `json:"index"`
}
