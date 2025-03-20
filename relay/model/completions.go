package model

import "strings"

type ResponseFormat struct {
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
	Type       string      `json:"type,omitempty"`
}

type JSONSchema struct {
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Strict      *bool                  `json:"strict,omitempty"`
	Description string                 `json:"description,omitempty"`
	Name        string                 `json:"name"`
}

type Audio struct {
	Voice  string `json:"voice,omitempty"`
	Format string `json:"format,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type GeneralOpenAIRequest struct {
	Prediction          any             `json:"prediction,omitempty"`
	Prompt              any             `json:"prompt,omitempty"`
	Input               any             `json:"input,omitempty"`
	Metadata            any             `json:"metadata,omitempty"`
	Functions           any             `json:"functions,omitempty"`
	LogitBias           any             `json:"logit_bias,omitempty"`
	FunctionCall        any             `json:"function_call,omitempty"`
	ToolChoice          any             `json:"tool_choice,omitempty"`
	Stop                any             `json:"stop,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	TopLogprobs         *int            `json:"top_logprobs,omitempty"`
	Style               *string         `json:"style,omitempty"`
	Quality             *string         `json:"quality,omitempty"`
	Audio               *Audio          `json:"audio,omitempty"`
	PresencePenalty     *float64        `json:"presence_penalty,omitempty"`
	ResponseFormat      *ResponseFormat `json:"response_format,omitempty"`
	Store               *bool           `json:"store,omitempty"`
	ServiceTier         *string         `json:"service_tier,omitempty"`
	FrequencyPenalty    *float64        `json:"frequency_penalty,omitempty"`
	Logprobs            *bool           `json:"logprobs,omitempty"`
	StreamOptions       *StreamOptions  `json:"stream_options,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	ParallelToolCalls   *bool           `json:"parallel_tool_calls,omitempty"`
	EncodingFormat      string          `json:"encoding_format,omitempty"`
	Model               string          `json:"model,omitempty"`
	Instruction         string          `json:"instruction,omitempty"`
	User                string          `json:"user,omitempty"`
	Size                string          `json:"size,omitempty"`
	Modalities          []string        `json:"modalities,omitempty"`
	Messages            []*Message      `json:"messages,omitempty"`
	Tools               []*Tool         `json:"tools,omitempty"`
	N                   int             `json:"n,omitempty"`
	Dimensions          int             `json:"dimensions,omitempty"`
	Seed                float64         `json:"seed,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	TopK                int             `json:"top_k,omitempty"`
	NumCtx              int             `json:"num_ctx,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
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

type ChatCompletionsStreamResponseChoice struct {
	FinishReason *string `json:"finish_reason,omitempty"`
	Delta        Message `json:"delta"`
	Index        int     `json:"index"`
	Text         string  `json:"text,omitempty"`
}

type ChatCompletionsStreamResponse struct {
	Usage   *Usage                                 `json:"usage,omitempty"`
	ID      string                                 `json:"id"`
	Object  string                                 `json:"object"`
	Model   string                                 `json:"model"`
	Choices []*ChatCompletionsStreamResponseChoice `json:"choices"`
	Created int64                                  `json:"created"`
}

type TextResponseChoice struct {
	FinishReason string  `json:"finish_reason"`
	Message      Message `json:"message"`
	Index        int     `json:"index"`
	Text         string  `json:"text,omitempty"`
}

type TextResponse struct {
	ID      string                `json:"id"`
	Model   string                `json:"model,omitempty"`
	Object  string                `json:"object"`
	Choices []*TextResponseChoice `json:"choices"`
	Usage   `json:"usage"`
	Created int64 `json:"created"`
}

type Message struct {
	Content          any     `json:"content,omitempty"`
	ReasoningContent string  `json:"reasoning_content,omitempty"`
	Name             *string `json:"name,omitempty"`
	Role             string  `json:"role,omitempty"`
	ToolCallID       string  `json:"tool_call_id,omitempty"`
	ToolCalls        []*Tool `json:"tool_calls,omitempty"`
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
	content, ok := m.Content.(string)
	if ok {
		contentList = append(contentList, MessageContent{
			Type: ContentTypeText,
			Text: content,
		})
		return contentList
	}
	anyList, ok := m.Content.([]any)
	if ok {
		for _, contentItem := range anyList {
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
					contentList = append(contentList, MessageContent{
						Type: ContentTypeImageURL,
						ImageURL: &ImageURL{
							URL: subObj["url"].(string),
						},
					})
				}
			}
		}
		return contentList
	}
	return nil
}

type ImageURL struct {
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type MessageContent struct {
	ImageURL *ImageURL `json:"image_url,omitempty"`
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
}
