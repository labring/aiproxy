package openai

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
)

// chatCompletionStreamState manages state for ChatCompletion stream conversion
type chatCompletionStreamState struct {
	messageID         string
	meta              *meta.Meta
	c                 *gin.Context
	currentToolCall   *relaymodel.ToolCall
	currentToolCallID string
	toolCallArgs      string
}

// handleResponseCreated handles response.created event for ChatCompletion
func (s *chatCompletionStreamState) handleResponseCreated(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Response == nil {
		return nil
	}

	s.messageID = event.Response.ID

	return &relaymodel.ChatCompletionsStreamResponse{
		ID:      s.messageID,
		Object:  "chat.completion.chunk",
		Created: event.Response.CreatedAt,
		Model:   event.Response.Model,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					Role: "assistant",
				},
			},
		},
	}
}

// handleOutputTextDelta handles response.output_text.delta event for ChatCompletion
func (s *chatCompletionStreamState) handleOutputTextDelta(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Delta == "" {
		return nil
	}

	return &relaymodel.ChatCompletionsStreamResponse{
		ID:      s.messageID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.meta.ActualModel,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					Content: event.Delta,
				},
			},
		},
	}
}

// handleOutputItemAdded handles response.output_item.added event for ChatCompletion
func (s *chatCompletionStreamState) handleOutputItemAdded(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Item == nil {
		return nil
	}

	// Track function calls
	if event.Item.Type == "function_call" {
		s.currentToolCallID = event.Item.ID
		s.currentToolCall = &relaymodel.ToolCall{
			ID:   event.Item.CallID,
			Type: "function",
			Function: relaymodel.Function{
				Name:      event.Item.Name,
				Arguments: "",
			},
		}
		s.toolCallArgs = ""

		// Send tool call start
		return &relaymodel.ChatCompletionsStreamResponse{
			ID:      s.messageID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   s.meta.ActualModel,
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: relaymodel.Message{
						ToolCalls: []relaymodel.ToolCall{
							{
								Index: 0,
								ID:    event.Item.CallID,
								Type:  "function",
								Function: relaymodel.Function{
									Name:      event.Item.Name,
									Arguments: "",
								},
							},
						},
					},
				},
			},
		}
	}

	if event.Item.Type == "message" {
		return &relaymodel.ChatCompletionsStreamResponse{
			ID:      s.messageID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   s.meta.ActualModel,
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: relaymodel.Message{
						Role: "assistant",
					},
				},
			},
		}
	}

	return nil
}

// handleFunctionCallArgumentsDelta handles response.function_call_arguments.delta event for ChatCompletion
func (s *chatCompletionStreamState) handleFunctionCallArgumentsDelta(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Delta == "" || s.currentToolCall == nil {
		return nil
	}

	// Accumulate arguments
	s.toolCallArgs += event.Delta

	// Send delta
	return &relaymodel.ChatCompletionsStreamResponse{
		ID:      s.messageID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.meta.ActualModel,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					ToolCalls: []relaymodel.ToolCall{
						{
							Index: 0,
							Function: relaymodel.Function{
								Arguments: event.Delta,
							},
						},
					},
				},
			},
		},
	}
}

// handleOutputItemDone handles response.output_item.done event for ChatCompletion
func (s *chatCompletionStreamState) handleOutputItemDone(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Item == nil {
		return nil
	}

	// Handle function call completion
	if event.Item.Type == "function_call" && s.currentToolCall != nil &&
		event.Item.ID == s.currentToolCallID {
		// Update with final arguments
		if s.toolCallArgs != "" {
			s.currentToolCall.Function.Arguments = s.toolCallArgs
		}

		// Reset state
		s.currentToolCall = nil
		s.currentToolCallID = ""
		s.toolCallArgs = ""

		// No need to send another chunk - arguments already streamed
		return nil
	}

	// Handle message content
	if len(event.Item.Content) > 0 {
		for _, content := range event.Item.Content {
			if (content.Type == "text" || content.Type == "output_text") && content.Text != "" {
				return &relaymodel.ChatCompletionsStreamResponse{
					ID:      s.messageID,
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   s.meta.ActualModel,
					Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
						{
							Index: 0,
							Delta: relaymodel.Message{
								Content: content.Text,
							},
						},
					},
				}
			}
		}
	}

	return nil
}

// handleResponseCompleted handles response.completed/done event for ChatCompletion
func (s *chatCompletionStreamState) handleResponseCompleted(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Response == nil || event.Response.Usage == nil {
		return nil
	}

	chatUsage := event.Response.Usage.ToChatUsage()

	return &relaymodel.ChatCompletionsStreamResponse{
		ID:      s.messageID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.meta.ActualModel,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: relaymodel.FinishReasonStop,
			},
		},
		Usage: &chatUsage,
	}
}

// claudeStreamState manages state for Claude stream conversion
type claudeStreamState struct {
	messageID            string
	sentMessageStart     bool
	contentIndex         int
	currentContentType   string
	currentToolUseID     string
	currentToolUseName   string
	currentToolUseCallID string
	toolUseInput         string
	meta                 *meta.Meta
	c                    *gin.Context
}

// handleResponseCreated handles response.created event for Claude
func (s *claudeStreamState) handleResponseCreated(event *relaymodel.ResponseStreamEvent) {
	if event.Response == nil {
		return
	}

	s.messageID = event.Response.ID
	s.sentMessageStart = true

	// Send message_start
	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type: "message_start",
		Message: &relaymodel.ClaudeResponse{
			ID:      s.messageID,
			Type:    "message",
			Role:    "assistant",
			Model:   event.Response.Model,
			Content: []relaymodel.ClaudeContent{},
		},
	})
}

// handleOutputItemAdded handles response.output_item.added event for Claude
func (s *claudeStreamState) handleOutputItemAdded(event *relaymodel.ResponseStreamEvent) {
	if event.Item == nil || !s.sentMessageStart {
		return
	}

	// Track if this is a reasoning item
	switch event.Item.Type {
	case "reasoning":
		s.currentContentType = "thinking"
		// Send content_block_start for thinking
		_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
			Type:  "content_block_start",
			Index: s.contentIndex,
			ContentBlock: &relaymodel.ClaudeContent{
				Type:     "thinking",
				Thinking: "",
			},
		})
	case "function_call":
		s.currentContentType = "tool_use"
		s.currentToolUseID = event.Item.ID
		s.currentToolUseName = event.Item.Name
		s.currentToolUseCallID = event.Item.CallID
		s.toolUseInput = ""
		// Send content_block_start for tool_use
		_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
			Type:  "content_block_start",
			Index: s.contentIndex,
			ContentBlock: &relaymodel.ClaudeContent{
				Type:  "tool_use",
				ID:    event.Item.CallID,
				Name:  event.Item.Name,
				Input: map[string]any{},
			},
		})
	}
}

// handleFunctionCallArgumentsDelta handles response.function_call_arguments.delta event for Claude
func (s *claudeStreamState) handleFunctionCallArgumentsDelta(
	event *relaymodel.ResponseStreamEvent,
) {
	if event.Delta == "" || !s.sentMessageStart || s.currentContentType != "tool_use" {
		return
	}

	// Accumulate input
	s.toolUseInput += event.Delta

	// Send input_json_delta
	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type:  "content_block_delta",
		Index: s.contentIndex,
		Delta: &relaymodel.ClaudeDelta{
			Type:        "input_json_delta",
			PartialJSON: event.Delta,
		},
	})
}

// handleContentPartAdded handles response.content_part.added event for Claude
func (s *claudeStreamState) handleContentPartAdded(event *relaymodel.ResponseStreamEvent) {
	if event.Part == nil || !s.sentMessageStart {
		return
	}

	if event.Part.Type == "output_text" && s.currentContentType != "thinking" {
		s.currentContentType = "text"
		// Send content_block_start for new text content
		_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
			Type:  "content_block_start",
			Index: s.contentIndex,
			ContentBlock: &relaymodel.ClaudeContent{
				Type: "text",
				Text: "",
			},
		})
	}
}

// handleReasoningTextDelta handles response.reasoning_text.delta event for Claude
func (s *claudeStreamState) handleReasoningTextDelta(event *relaymodel.ResponseStreamEvent) {
	if event.Delta == "" || !s.sentMessageStart {
		return
	}

	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type:  "content_block_delta",
		Index: s.contentIndex,
		Delta: &relaymodel.ClaudeDelta{
			Type:     "thinking_delta",
			Thinking: event.Delta,
		},
	})
}

// handleOutputTextDelta handles response.output_text.delta event for Claude
func (s *claudeStreamState) handleOutputTextDelta(event *relaymodel.ResponseStreamEvent) {
	if event.Delta == "" || !s.sentMessageStart {
		return
	}

	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type:  "content_block_delta",
		Index: s.contentIndex,
		Delta: &relaymodel.ClaudeDelta{
			Type: "text_delta",
			Text: event.Delta,
		},
	})
}

// handleOutputItemDone handles response.output_item.done event for Claude
func (s *claudeStreamState) handleOutputItemDone(event *relaymodel.ResponseStreamEvent) {
	if event.Item == nil || !s.sentMessageStart {
		return
	}

	// For tool_use blocks, parse and finalize input
	if event.Item.Type == "function_call" && s.currentContentType == "tool_use" {
		if s.toolUseInput != "" {
			var input map[string]any

			_ = sonic.Unmarshal([]byte(s.toolUseInput), &input)
		}
		// Reset tool use state
		s.currentToolUseID = ""
		s.currentToolUseName = ""
		s.currentToolUseCallID = ""
		s.toolUseInput = ""
	}

	// Send content_block_stop for any type
	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type:  "content_block_stop",
		Index: s.contentIndex,
	})
	s.contentIndex++
	s.currentContentType = ""
}

// handleResponseCompleted handles response.completed/done event for Claude
func (s *claudeStreamState) handleResponseCompleted(event *relaymodel.ResponseStreamEvent) {
	if event.Response == nil || event.Response.Usage == nil {
		return
	}

	// Send message_delta with stop reason
	stopReason := "end_turn"
	claudeUsage := event.Response.Usage.ToClaudeUsage()
	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type: "message_delta",
		Delta: &relaymodel.ClaudeDelta{
			StopReason: &stopReason,
		},
		Usage: &claudeUsage,
	})

	// Send message_stop
	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type: "message_stop",
	})
}

// geminiStreamState manages state for Gemini stream conversion
type geminiStreamState struct {
	meta              *meta.Meta
	c                 *gin.Context
	functionCallNames map[string]string // item_id -> function name
}

// handleOutputItemAdded handles response.output_item.added event for Gemini
func (s *geminiStreamState) handleOutputItemAdded(event *relaymodel.ResponseStreamEvent) {
	if event.Item == nil {
		return
	}

	// Track function call names for later use in done event
	if event.Item.Type == "function_call" && event.Item.Name != "" {
		if s.functionCallNames == nil {
			s.functionCallNames = make(map[string]string)
		}

		s.functionCallNames[event.Item.ID] = event.Item.Name
	}
}

// handleFunctionCallArgumentsDone handles response.function_call_arguments.done event for Gemini
func (s *geminiStreamState) handleFunctionCallArgumentsDone(event *relaymodel.ResponseStreamEvent) {
	if event.Arguments == "" || event.ItemID == "" {
		return
	}

	// Get function name from tracked state
	functionName := s.functionCallNames[event.ItemID]
	if functionName == "" {
		return
	}

	// Parse arguments
	var args map[string]any
	if err := sonic.UnmarshalString(event.Arguments, &args); err != nil {
		return
	}

	// Send complete function call
	geminiResp := relaymodel.GeminiChatResponse{
		ModelVersion: s.meta.ActualModel,
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Index: 0,
				Content: relaymodel.GeminiChatContent{
					Role: "model",
					Parts: []*relaymodel.GeminiPart{
						{
							FunctionCall: &relaymodel.GeminiFunctionCall{
								Name: functionName,
								Args: args,
							},
						},
					},
				},
			},
		},
	}
	_ = render.GeminiObjectData(s.c, &geminiResp)

	// Clean up tracked name
	delete(s.functionCallNames, event.ItemID)
}

// handleOutputTextDelta handles response.output_text.delta event for Gemini
func (s *geminiStreamState) handleOutputTextDelta(event *relaymodel.ResponseStreamEvent) {
	if event.Delta == "" {
		return
	}

	geminiResp := relaymodel.GeminiChatResponse{
		ModelVersion: s.meta.ActualModel,
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Index: 0,
				Content: relaymodel.GeminiChatContent{
					Role: "model",
					Parts: []*relaymodel.GeminiPart{
						{
							Text: event.Delta,
						},
					},
				},
			},
		},
	}
	_ = render.GeminiObjectData(s.c, &geminiResp)
}

// handleOutputItemDone handles response.output_item.done event for Gemini
func (s *geminiStreamState) handleOutputItemDone(event *relaymodel.ResponseStreamEvent) {
	if event.Item == nil {
		return
	}

	// Only handle reasoning items here, function_call is handled by handleFunctionCallArgumentsDone
	if event.Item.Type == "reasoning" {
		geminiResp := relaymodel.GeminiChatResponse{
			ModelVersion: s.meta.ActualModel,
			Candidates: []*relaymodel.GeminiChatCandidate{
				{
					Index: 0,
					Content: relaymodel.GeminiChatContent{
						Role:  "model",
						Parts: []*relaymodel.GeminiPart{},
					},
				},
			},
		}

		// Convert reasoning to thought parts
		for _, content := range event.Item.Content {
			if (content.Type == "text" || content.Type == "output_text") && content.Text != "" {
				geminiResp.Candidates[0].Content.Parts = append(
					geminiResp.Candidates[0].Content.Parts,
					&relaymodel.GeminiPart{
						Text:    content.Text,
						Thought: true,
					},
				)
			}
		}

		// Only send if there are parts
		if len(geminiResp.Candidates[0].Content.Parts) > 0 {
			_ = render.GeminiObjectData(s.c, &geminiResp)
		}
	}
}

// handleResponseCompleted handles response.completed/done event for Gemini
func (s *geminiStreamState) handleResponseCompleted(event *relaymodel.ResponseStreamEvent) {
	if event.Response == nil {
		return
	}

	// Send final chunk with usage and finish reason
	geminiResp := relaymodel.GeminiChatResponse{
		ModelVersion: s.meta.ActualModel,
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Index:        0,
				FinishReason: "STOP",
			},
		},
	}

	if event.Response.Usage != nil {
		geminiUsage := event.Response.Usage.ToGeminiUsage()
		geminiResp.UsageMetadata = &geminiUsage
	}

	_ = render.GeminiObjectData(s.c, &geminiResp)
}
