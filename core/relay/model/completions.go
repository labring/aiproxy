package model

import (
	"strings"
)

type ResponseFormat struct {
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
	Type       string      `json:"type,omitempty"`
}

type JSONSchema struct {
	Schema      map[string]any `json:"schema,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
	Description string         `json:"description,omitempty"`
	Name        string         `json:"name"`
}

type Audio struct {
	Voice  string `json:"voice,omitempty"`
	Format string `json:"format,omitempty"`
}

type OutputAudio struct {
	ID         string `json:"id,omitempty"`
	Data       string `json:"data,omitempty"`
	Transcript string `json:"transcript,omitempty"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
}

type StreamOptions struct {
	IncludeUsage       bool  `json:"include_usage,omitempty"`
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
}

type PromptCacheOptions struct {
	Mode string `json:"mode,omitempty"`
	TTL  string `json:"ttl,omitempty"`
}

type GeneralOpenAIRequest struct {
	Prompt               any                 `json:"prompt,omitempty"`
	Input                any                 `json:"input,omitempty"`
	Metadata             any                 `json:"metadata,omitempty"`
	Functions            any                 `json:"functions,omitempty"`
	LogitBias            any                 `json:"logit_bias,omitempty"`
	FunctionCall         any                 `json:"function_call,omitempty"`
	ToolChoice           any                 `json:"tool_choice,omitempty"`
	Stop                 any                 `json:"stop,omitempty"`
	TopLogprobs          *int                `json:"top_logprobs,omitempty"`
	PresencePenalty      *float64            `json:"presence_penalty,omitempty"`
	ResponseFormat       *ResponseFormat     `json:"response_format,omitempty"`
	Audio                *Audio              `json:"audio,omitempty"`
	FrequencyPenalty     *float64            `json:"frequency_penalty,omitempty"`
	Logprobs             *bool               `json:"logprobs,omitempty"`
	StreamOptions        *StreamOptions      `json:"stream_options,omitempty"`
	Temperature          *float64            `json:"temperature,omitempty"`
	TopP                 *float64            `json:"top_p,omitempty"`
	ServiceTier          string              `json:"service_tier,omitempty"`
	PromptCacheKey       string              `json:"prompt_cache_key,omitempty"`
	PromptCacheOptions   *PromptCacheOptions `json:"prompt_cache_options,omitempty"`
	PromptCacheRetention string              `json:"prompt_cache_retention,omitempty"`
	SafetyIdentifier     string              `json:"safety_identifier,omitempty"`
	Moderation           any                 `json:"moderation,omitempty"`
	Verbosity            string              `json:"verbosity,omitempty"`
	Model                string              `json:"model,omitempty"`
	User                 string              `json:"user,omitempty"`
	Size                 string              `json:"size,omitempty"`
	Messages             []Message           `json:"messages,omitempty"`
	Tools                []Tool              `json:"tools,omitempty"`
	Modalities           []string            `json:"modalities,omitempty"`
	Seed                 float64             `json:"seed,omitempty"`
	N                    int                 `json:"n,omitempty"`
	MaxTokens            int                 `json:"max_tokens,omitempty"`
	MaxCompletionTokens  int                 `json:"max_completion_tokens,omitempty"`
	TopK                 int                 `json:"top_k,omitempty"`
	NumCtx               int                 `json:"num_ctx,omitempty"`
	Stream               bool                `json:"stream,omitempty"`
	ParallelToolCalls    *bool               `json:"parallel_tool_calls,omitempty"`
	ReasoningEffort      *string             `json:"reasoning_effort,omitempty"`
	EnableThinking       *bool               `json:"enable_thinking,omitempty"`
	ThinkingBudget       *int                `json:"thinking_budget,omitempty"`
	// aiproxy control field
	Thinking *GeneralThinking `json:"thinking,omitempty"`
}

// OutputTokenLimit returns the requested output token cap. OpenAI-format clients
// may send either max_tokens (legacy) or max_completion_tokens (newer models);
// the former takes precedence when both are set, matching the Responses
// conversion. Zero means unset.
func (r GeneralOpenAIRequest) OutputTokenLimit() int {
	if r.MaxTokens > 0 {
		return r.MaxTokens
	}

	if r.MaxCompletionTokens > 0 {
		return r.MaxCompletionTokens
	}

	return 0
}

func (r GeneralOpenAIRequest) ParseInput() []string {
	if r.Input == nil {
		return nil
	}

	var input []string
	switch v := r.Input.(type) {
	case string:
		input = []string{v}
	case []any:
		input = make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				input = append(input, str)
			}
		}
	}

	return input
}

type GeneralThinking = ClaudeThinking

type GeneralOpenAIThinkingRequest struct {
	Thinking *GeneralThinking `json:"thinking,omitempty"`
}

type ChatCompletionsStreamResponseChoice struct {
	FinishReason FinishReason    `json:"finish_reason,omitempty"`
	Delta        Message         `json:"delta"`
	Logprobs     *ChoiceLogprobs `json:"logprobs,omitempty"`
	Index        int             `json:"index"`
	Text         string          `json:"text,omitempty"`
}

type ChatCompletionsStreamResponse struct {
	Usage       *ChatUsage                             `json:"usage,omitempty"`
	Moderation  any                                    `json:"moderation,omitempty"`
	ServiceTier *string                                `json:"service_tier,omitempty"`
	Obfuscation string                                 `json:"obfuscation,omitempty"`
	ID          string                                 `json:"id"`
	Object      string                                 `json:"object"`
	Model       string                                 `json:"model"`
	Choices     []*ChatCompletionsStreamResponseChoice `json:"choices"`
	Created     int64                                  `json:"created"`
}

type TopLogprob struct {
	Token   string  `json:"token"`
	Bytes   []int   `json:"bytes"`
	Logprob float64 `json:"logprob"`
}

type ChatCompletionTokenLogprob struct {
	Token       string       `json:"token"`
	Bytes       []int        `json:"bytes"`
	Logprob     float64      `json:"logprob"`
	TopLogprobs []TopLogprob `json:"top_logprobs"`
}

type ChoiceLogprobs struct {
	Content []ChatCompletionTokenLogprob `json:"content,omitempty"`
	Refusal []ChatCompletionTokenLogprob `json:"refusal,omitempty"`
}

type TextResponseChoice struct {
	FinishReason FinishReason    `json:"finish_reason"`
	Message      Message         `json:"message"`
	Logprobs     *ChoiceLogprobs `json:"logprobs,omitempty"`
	Index        int             `json:"index"`
	Text         string          `json:"text,omitempty"`
}

type TextResponse struct {
	ID          string                `json:"id"`
	Model       string                `json:"model,omitempty"`
	Object      string                `json:"object"`
	Choices     []*TextResponseChoice `json:"choices"`
	Usage       ChatUsage             `json:"usage"`
	Moderation  any                   `json:"moderation,omitempty"`
	ServiceTier *string               `json:"service_tier,omitempty"`
	Created     int64                 `json:"created"`
}

type Message struct {
	Content          any          `json:"content,omitempty"`
	Audio            *OutputAudio `json:"audio,omitempty"`
	ReasoningContent string       `json:"reasoning_content,omitempty"`
	Refusal          string       `json:"refusal,omitempty"`
	Annotations      []any        `json:"annotations,omitempty"`
	Signature        string       `json:"signature,omitempty"`
	Name             *string      `json:"name,omitempty"`
	Role             string       `json:"role,omitempty"`
	ToolCallID       string       `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall   `json:"tool_calls,omitempty"`
}

func (m *Message) IsStringContent() bool {
	_, ok := m.Content.(string)
	return ok
}

func (m *Message) ToStringContentMessage() {
	if m.IsStringContent() {
		return
	}

	m.Content = m.StringContent()
}

func (m *Message) StringContent() string {
	if m.ReasoningContent != "" {
		return m.ReasoningContent
	}

	content, ok := m.Content.(string)
	if ok {
		return content
	}

	contentList, ok := m.Content.([]any)
	if !ok {
		return ""
	}

	var strBuilder strings.Builder
	for _, contentItem := range contentList {
		contentMap, ok := contentItem.(map[string]any)
		if !ok {
			continue
		}

		if contentMap["type"] == ContentTypeText {
			if subStr, ok := contentMap["text"].(string); ok {
				strBuilder.WriteString(subStr)
				strBuilder.WriteString("\n")
			}
		}
	}

	return strBuilder.String()
}

func (m *Message) ParseContent() []MessageContent {
	var contentList []MessageContent

	switch content := m.Content.(type) {
	case string:
		contentList = append(contentList, MessageContent{
			Type: ContentTypeText,
			Text: content,
		})

		return contentList
	case []any:
		for _, contentItem := range content {
			contentMap, ok := contentItem.(map[string]any)
			if !ok {
				continue
			}

			switch contentMap["type"] {
			case ContentTypeText:
				if subStr, ok := contentMap["text"].(string); ok {
					contentList = append(contentList, MessageContent{
						Type: ContentTypeText,
						Text: subStr,
					})
				}
			case ContentTypeImageURL:
				if subObj, ok := contentMap["image_url"].(map[string]any); ok {
					url, ok := subObj["url"].(string)
					if !ok {
						continue
					}

					contentList = append(contentList, MessageContent{
						Type: ContentTypeImageURL,
						ImageURL: &ImageURL{
							URL: url,
						},
					})
				}
			case ContentTypeInputAudio:
				if subObj, ok := contentMap["input_audio"].(map[string]any); ok {
					data, _ := subObj["data"].(string)
					format, _ := subObj["format"].(string)
					url, _ := subObj["url"].(string)

					contentList = append(contentList, MessageContent{
						Type: ContentTypeInputAudio,
						InputAudio: &InputAudio{
							Data:   data,
							Format: format,
							URL:    url,
						},
					})
				}
			case ContentTypeVideoURL:
				if subObj, ok := contentMap["video_url"].(map[string]any); ok {
					url, ok := subObj["url"].(string)
					if !ok {
						continue
					}

					contentList = append(contentList, MessageContent{
						Type: ContentTypeVideoURL,
						VideoURL: &VideoURL{
							URL: url,
						},
					})
				}
			}
		}

		return contentList
	case []MessageContent:
		for _, contentItem := range content {
			switch contentItem.Type {
			case ContentTypeText:
				contentList = append(contentList, MessageContent{
					Type: ContentTypeText,
					Text: contentItem.Text,
				})
			case ContentTypeImageURL:
				imageURL := contentItem.ImageURL
				if imageURL == nil {
					continue
				}

				contentList = append(contentList, MessageContent{
					Type: ContentTypeImageURL,
					ImageURL: &ImageURL{
						URL: imageURL.URL,
					},
				})
			case ContentTypeInputAudio:
				if contentItem.InputAudio == nil {
					continue
				}

				contentList = append(contentList, MessageContent{
					Type:       ContentTypeInputAudio,
					InputAudio: contentItem.InputAudio,
				})
			case ContentTypeVideoURL:
				if contentItem.VideoURL == nil {
					continue
				}

				contentList = append(contentList, MessageContent{
					Type:     ContentTypeVideoURL,
					VideoURL: contentItem.VideoURL,
				})
			}
		}

		return contentList
	default:
		return nil
	}
}

type ImageURL struct {
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type InputAudio struct {
	Data   string `json:"data,omitempty"`
	Format string `json:"format,omitempty"`
	URL    string `json:"url,omitempty"`
}

type VideoURL struct {
	URL string `json:"url,omitempty"`
}

type MessageContent struct {
	ImageURL              *ImageURL   `json:"image_url,omitempty"`
	InputAudio            *InputAudio `json:"input_audio,omitempty"`
	VideoURL              *VideoURL   `json:"video_url,omitempty"`
	PromptCacheBreakpoint any         `json:"prompt_cache_breakpoint,omitempty"`
	Type                  string      `json:"type,omitempty"`
	Text                  string      `json:"text,omitempty"`
}
