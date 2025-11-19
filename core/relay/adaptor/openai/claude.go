package openai

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

// ConvertClaudeRequest converts Claude API request format to OpenAI format
func ConvertClaudeRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	openAIRequest, err := ConvertClaudeRequestModel(meta, req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Marshal the converted request
	jsonData, err := sonic.Marshal(openAIRequest)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonData))},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
}

func ConvertClaudeRequestModel(
	meta *meta.Meta,
	req *http.Request,
) (*relaymodel.GeneralOpenAIRequest, error) {
	// Parse Claude request
	var claudeRequest relaymodel.ClaudeAnyContentRequest

	err := common.UnmarshalRequestReusable(req, &claudeRequest)
	if err != nil {
		return nil, err
	}

	// Convert to OpenAI format
	openAIRequest := relaymodel.GeneralOpenAIRequest{
		Model:               meta.ActualModel,
		Stream:              claudeRequest.Stream,
		MaxTokens:           claudeRequest.MaxTokens,
		MaxCompletionTokens: claudeRequest.MaxCompletionTokens,
		Temperature:         claudeRequest.Temperature,
		TopP:                claudeRequest.TopP,
	}

	// Convert messages
	openAIRequest.Messages = convertClaudeMessagesToOpenAI(claudeRequest)

	// Convert tools
	if len(claudeRequest.Tools) > 0 {
		openAIRequest.Tools = convertClaudeToolsToOpenAI(claudeRequest.Tools)
		openAIRequest.ToolChoice = convertClaudeToolChoice(claudeRequest.ToolChoice)
	}

	// Convert stop sequences
	if len(claudeRequest.StopSequences) > 0 {
		openAIRequest.Stop = claudeRequest.StopSequences
	}

	// Set stream options if streaming
	if claudeRequest.Stream {
		openAIRequest.StreamOptions = &relaymodel.StreamOptions{
			IncludeUsage: true,
		}
	}

	return &openAIRequest, nil
}

// convertClaudeMessagesToOpenAI converts Claude message format to OpenAI format
func convertClaudeMessagesToOpenAI(
	claudeRequest relaymodel.ClaudeAnyContentRequest,
) []relaymodel.Message {
	messages := make([]relaymodel.Message, 0)

	// Add system messages
	if len(claudeRequest.System) > 0 {
		var systemContent strings.Builder
		for i, content := range claudeRequest.System {
			if i > 0 {
				systemContent.WriteString("\n")
			}

			if content.Type == "text" {
				systemContent.WriteString(content.Text)
			}
		}

		if systemContent.Len() > 0 {
			messages = append(messages, relaymodel.Message{
				Role:    "system",
				Content: systemContent.String(),
			})
		}
	}

	// Convert regular messages
	for _, msg := range claudeRequest.Messages {
		openAIMsg := relaymodel.Message{
			Role: msg.Role,
		}

		result := convertClaudeContent(msg.Content)
		messages = append(messages, result.Messages...)
		openAIMsg.ToolCalls = result.ToolCalls

		openAIMsg.Content = result.Content
		if openAIMsg.Content != nil {
			messages = append(messages, openAIMsg)
		}
	}

	return messages
}

type convertClaudeContentResult struct {
	Content   any
	ToolCalls []relaymodel.ToolCall
	Messages  []relaymodel.Message
}

func convertClaudeContent(content any) convertClaudeContentResult {
	result := convertClaudeContentResult{}
	switch content := content.(type) {
	case string:
		result.Content = content
	case []any:
		rawBytes, _ := sonic.Marshal(content)

		var contentArray []relaymodel.ClaudeContent

		_ = sonic.Unmarshal(rawBytes, &contentArray)

		var parts []relaymodel.MessageContent
		for _, content := range contentArray {
			switch content.Type {
			case "text":
				text := strings.TrimSpace(content.Text)
				if text == "" {
					continue
				}

				parts = append(parts, relaymodel.MessageContent{
					Type: relaymodel.ContentTypeText,
					Text: text,
				})
			case "thinking":
				text := strings.TrimSpace(content.Thinking)
				if text == "" {
					continue
				}

				parts = append(parts, relaymodel.MessageContent{
					Type: relaymodel.ContentTypeText,
					Text: text,
				})
			case "image":
				if content.Source != nil {
					imageURL := relaymodel.ImageURL{}
					switch content.Source.Type {
					case "url":
						imageURL.URL = content.Source.URL
					case "base64":
						imageURL.URL = fmt.Sprintf("data:%s;base64,%s",
							content.Source.MediaType, content.Source.Data)
					}

					parts = append(parts, relaymodel.MessageContent{
						Type:     relaymodel.ContentTypeImageURL,
						ImageURL: &imageURL,
					})
				}
			case "tool_use":
				// Handle tool calls
				args, _ := sonic.MarshalString(content.Input)
				toolCall := relaymodel.ToolCall{
					ID:   content.ID,
					Type: "function",
					Function: relaymodel.Function{
						Name:      content.Name,
						Arguments: args,
					},
				}
				// Preserve Gemini thought signature if present (OpenAI format)
				if content.ThoughtSignature != "" {
					toolCall.ExtraContent = &relaymodel.ExtraContent{
						Google: &relaymodel.GoogleExtraContent{
							ThoughtSignature: content.ThoughtSignature,
						},
					}
				}

				result.ToolCalls = append(result.ToolCalls, toolCall)
			case "tool_result":
				// Create a separate tool message for each tool_result
				var newContent any
				switch v := content.Content.(type) {
				case string:
					newContent = v
				case []any:
					result := convertClaudeContent(v)
					newContent = result.Content
				}

				toolMsg := relaymodel.Message{
					Role:       "tool",
					Content:    newContent,
					ToolCallID: content.ToolUseID,
				}

				result.Messages = append(result.Messages, toolMsg)

				continue
			default:
				continue
			}
		}

		if len(parts) > 0 {
			result.Content = parts
		}
	}

	return result
}

// convertClaudeToolsToOpenAI converts Claude tools to OpenAI format
func convertClaudeToolsToOpenAI(claudeTools []relaymodel.ClaudeTool) []relaymodel.Tool {
	openAITools := make([]relaymodel.Tool, 0, len(claudeTools))

	for _, tool := range claudeTools {
		openAITool := relaymodel.Tool{
			Type: "function",
			Function: relaymodel.Function{
				Name:        tool.Name,
				Description: tool.Description,
			},
		}

		// Convert input schema
		if tool.InputSchema != nil {
			openAITool.Function.Parameters = map[string]any{
				"type":       tool.InputSchema.Type,
				"properties": tool.InputSchema.Properties,
				"required":   tool.InputSchema.Required,
			}
		}

		openAITools = append(openAITools, openAITool)
	}

	return openAITools
}

// convertClaudeToolChoice converts Claude tool choice to OpenAI format
func convertClaudeToolChoice(toolChoice any) any {
	if toolChoice == nil {
		return "auto"
	}

	switch v := toolChoice.(type) {
	case string:
		if v == "any" {
			return "required"
		}
		return v
	case map[string]any:
		if toolType, ok := v["type"].(string); ok {
			switch toolType {
			case "tool":
				if name, ok := v["name"].(string); ok {
					return map[string]any{
						"type": "function",
						"function": map[string]any{
							"name": name,
						},
					}
				}
			case "any":
				return "required"
			case "auto":
				return "auto"
			}
		}
	}

	return "auto"
}

// ClaudeStreamHandler handles OpenAI streaming responses and converts them to Claude format
func ClaudeStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ClaudeErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner := bufio.NewScanner(resp.Body)

	buf := utils.GetScannerBuffer()
	defer utils.PutScannerBuffer(buf)

	scanner.Buffer(*buf, cap(*buf))

	// Initialize Claude response tracking
	var (
		messageID           = "msg_" + common.ShortUUID()
		contentText         strings.Builder
		thinkingText        strings.Builder
		usage               relaymodel.ChatUsage
		stopReason          string
		currentContentIndex = -1
		currentContentType  = ""
		sentMessageStart    = false
		toolCallsBuffer     = make(map[int]*relaymodel.ClaudeContent)
	)

	// Helper function to close current content block
	closeCurrentBlock := func() {
		if currentContentIndex >= 0 {
			_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
				Type:  "content_block_stop",
				Index: currentContentIndex,
			})
		}
	}

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		// Parse OpenAI response
		var openAIResponse relaymodel.ChatCompletionsStreamResponse

		err := sonic.Unmarshal(data, &openAIResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		// Handle usage update
		if openAIResponse.Usage != nil {
			usage = *openAIResponse.Usage
		}

		// Send message_start event (only once)
		if !sentMessageStart {
			sentMessageStart = true

			// Include initial usage if available
			messageStartResp := relaymodel.ClaudeStreamResponse{
				Type: "message_start",
				Message: &relaymodel.ClaudeResponse{
					ID:      messageID,
					Type:    "message",
					Role:    "assistant",
					Model:   meta.ActualModel,
					Content: []relaymodel.ClaudeContent{},
				},
			}

			// Add initial usage if available
			if openAIResponse.Usage != nil {
				claudeUsage := openAIResponse.Usage.ToClaudeUsage()
				messageStartResp.Message.Usage = claudeUsage
			}

			_ = render.ClaudeObjectData(c, messageStartResp)

			// Send ping event
			_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{Type: "ping"})
		}

		// Process each choice
		for _, choice := range openAIResponse.Choices {
			// Handle reasoning/thinking content
			if choice.Delta.ReasoningContent != "" {
				// If we're not in a thinking block, start one
				if currentContentType != "thinking" {
					closeCurrentBlock()

					currentContentIndex++
					currentContentType = "thinking"

					_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
						Type:  "content_block_start",
						Index: currentContentIndex,
						ContentBlock: &relaymodel.ClaudeContent{
							Type:     "thinking",
							Thinking: "",
						},
					})
				}

				thinkingText.WriteString(choice.Delta.ReasoningContent)

				_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
					Type:  "content_block_delta",
					Index: currentContentIndex,
					Delta: &relaymodel.ClaudeDelta{
						Type:     "thinking_delta",
						Thinking: choice.Delta.ReasoningContent,
					},
				})
			}

			// Handle text content
			if content, ok := choice.Delta.Content.(string); ok && content != "" {
				// If we're not in a text block, start one
				if currentContentType != "text" {
					closeCurrentBlock()

					currentContentIndex++
					currentContentType = "text"

					_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
						Type:  "content_block_start",
						Index: currentContentIndex,
						ContentBlock: &relaymodel.ClaudeContent{
							Type: "text",
							Text: "",
						},
					})
				}

				contentText.WriteString(content)

				_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
					Type:  "content_block_delta",
					Index: currentContentIndex,
					Delta: &relaymodel.ClaudeDelta{
						Type: "text_delta",
						Text: content,
					},
				})
			}

			// Handle tool calls
			if len(choice.Delta.ToolCalls) > 0 {
				for _, toolCall := range choice.Delta.ToolCalls {
					idx := toolCall.Index

					// Initialize tool call if new
					if _, exists := toolCallsBuffer[idx]; !exists {
						// Close current block if needed
						closeCurrentBlock()

						currentContentIndex++
						currentContentType = "tool_use"

						toolCallsBuffer[idx] = &relaymodel.ClaudeContent{
							Type:  "tool_use",
							ID:    toolCall.ID,
							Name:  toolCall.Function.Name,
							Input: make(map[string]any),
						}

						// Send content_block_start for tool use
						_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
							Type:         "content_block_start",
							Index:        currentContentIndex,
							ContentBlock: toolCallsBuffer[idx],
						})
					}

					// Send tool arguments delta
					if toolCall.Function.Arguments != "" {
						_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
							Type:  "content_block_delta",
							Index: currentContentIndex,
							Delta: &relaymodel.ClaudeDelta{
								Type:        "input_json_delta",
								PartialJSON: toolCall.Function.Arguments,
							},
						})
					}
				}
			}

			// Handle finish reason
			if choice.FinishReason != "" {
				stopReason = *convertFinishReasonToClaude(choice.FinishReason)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	// Close the last open content block
	closeCurrentBlock()

	// Calculate final usage if not provided
	if usage.TotalTokens == 0 && (contentText.Len() > 0 || thinkingText.Len() > 0) {
		totalText := contentText.String()
		if thinkingText.Len() > 0 {
			totalText = thinkingText.String() + "\n" + totalText
		}

		usage = ResponseText2Usage(
			totalText,
			meta.ActualModel,
			int64(meta.RequestUsage.InputTokens),
		)
	}

	claudeUsage := usage.ToClaudeUsage()

	if stopReason == "" {
		stopReason = claudeStopReasonEndTurn
	}

	// Send message_delta with final usage
	_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
		Type: "message_delta",
		Delta: &relaymodel.ClaudeDelta{
			StopReason: &stopReason,
		},
		Usage: &claudeUsage,
	})

	// Send message_stop
	_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
		Type: "message_stop",
	})

	return usage.ToModelUsage(), nil
}

// ClaudeHandler handles OpenAI non-streaming responses and converts them to Claude format
func ClaudeHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ClaudeErrorHandler(resp)
	}

	defer resp.Body.Close()

	// Read OpenAI response
	body, err := common.GetResponseBody(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Parse OpenAI response
	var openAIResponse relaymodel.TextResponse

	err = sonic.Unmarshal(body, &openAIResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert to Claude response
	claudeResponse := relaymodel.ClaudeResponse{
		ID:           "msg_" + common.ShortUUID(),
		Type:         "message",
		Role:         "assistant",
		Model:        meta.ActualModel,
		Content:      []relaymodel.ClaudeContent{},
		StopReason:   "",
		StopSequence: nil,
	}

	// Process each choice (typically only one)
	for _, choice := range openAIResponse.Choices {
		// Handle text content
		if content, ok := choice.Message.Content.(string); ok {
			claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
				Type: "text",
				Text: content,
			})
		}

		// Handle reasoning content (for o1 models)
		if choice.Message.ReasoningContent != "" {
			claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
				Type:     "thinking",
				Thinking: choice.Message.ReasoningContent,
			})
		}

		// Handle tool calls
		for _, toolCall := range choice.Message.ToolCalls {
			var input map[string]any
			if toolCall.Function.Arguments != "" {
				_ = sonic.UnmarshalString(toolCall.Function.Arguments, &input)
			}

			claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
				Type:  "tool_use",
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: input,
			})
		}

		// Set stop reason
		claudeResponse.StopReason = *convertFinishReasonToClaude(choice.FinishReason)
	}

	// If no content was added, ensure at least an empty text block
	if len(claudeResponse.Content) == 0 {
		claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
			Type: "text",
			Text: "",
		})
	}

	// Convert usage
	claudeResponse.Usage = relaymodel.ClaudeUsage{
		InputTokens:  openAIResponse.Usage.PromptTokens,
		OutputTokens: openAIResponse.Usage.CompletionTokens,
	}

	// Add cache information if available
	if openAIResponse.Usage.PromptTokensDetails != nil {
		claudeResponse.Usage.CacheReadInputTokens = openAIResponse.Usage.PromptTokensDetails.CachedTokens
		claudeResponse.Usage.CacheCreationInputTokens = openAIResponse.Usage.PromptTokensDetails.CacheCreationTokens
	}

	// Add web search usage if available
	if openAIResponse.Usage.WebSearchCount > 0 {
		claudeResponse.Usage.ServerToolUse = &relaymodel.ClaudeServerToolUse{
			WebSearchRequests: openAIResponse.Usage.WebSearchCount,
		}
	}

	// Marshal Claude response
	claudeResponseData, err := sonic.Marshal(claudeResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Write response
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(claudeResponseData)))
	_, _ = c.Writer.Write(claudeResponseData)

	return claudeResponse.Usage.ToOpenAIUsage().ToModelUsage(), nil
}

const (
	claudeStopReasonEndTurn      = "end_turn"
	claudeStopReasonMaxTokens    = "max_tokens"
	claudeStopReasonToolUse      = "tool_use"
	claudeStopReasonStopSequence = "stop_sequence"
)

// convertFinishReasonToClaude converts OpenAI finish reason to Claude stop reason
func convertFinishReasonToClaude(finishReason string) *string {
	switch finishReason {
	case relaymodel.FinishReasonStop:
		v := claudeStopReasonEndTurn
		return &v
	case relaymodel.FinishReasonLength:
		v := claudeStopReasonMaxTokens
		return &v
	case relaymodel.FinishReasonToolCalls:
		v := claudeStopReasonToolUse
		return &v
	case relaymodel.FinishReasonContentFilter:
		v := claudeStopReasonStopSequence
		return &v
	case "":
		v := claudeStopReasonEndTurn
		return &v
	default:
		return &finishReason
	}
}

// ClaudeErrorHandler converts OpenAI errors to Claude error format
func ClaudeErrorHandler(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	respBody, err := common.GetResponseBody(resp)
	if err != nil {
		return relaymodel.WrapperAnthropicError(
			err,
			"read_response_failed",
			http.StatusInternalServerError,
		)
	}

	// Try to parse as OpenAI error
	statusCode, openAIError := GetErrorWithBody(resp.StatusCode, respBody)

	// Convert to Claude error
	claudeError := relaymodel.AnthropicError{
		Type:    convertOpenAIErrorTypeToClaude(openAIError.Type),
		Message: openAIError.Message,
	}

	return relaymodel.NewAnthropicError(statusCode, claudeError)
}

// convertOpenAIErrorTypeToClaude converts OpenAI error type to Claude error type
func convertOpenAIErrorTypeToClaude(openAIType string) string {
	switch openAIType {
	case "invalid_request_error":
		return "invalid_request_error"
	case "authentication_error":
		return "authentication_error"
	case "permission_error":
		return "permission_error"
	case "not_found_error":
		return "not_found_error"
	case "request_too_large":
		return "request_too_large"
	case "rate_limit_error":
		return "rate_limit_error"
	case "api_error":
		return "api_error"
	case "overloaded_error":
		return "overloaded_error"
	default:
		return openAIType
	}
}
