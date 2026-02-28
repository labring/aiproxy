package openai

import (
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
		openAIRequest.Tools = ConvertClaudeToolsToOpenAI(claudeRequest.Tools)
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

			if content.Type == relaymodel.ClaudeContentTypeText {
				systemContent.WriteString(content.Text)
			}
		}

		if systemContent.Len() > 0 {
			messages = append(messages, relaymodel.Message{
				Role:    relaymodel.RoleSystem,
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
		// Include the message if it has content OR tool calls
		// This is important for function calling flow where assistant may only have tool calls
		if openAIMsg.Content != nil || len(openAIMsg.ToolCalls) > 0 {
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
			case relaymodel.ClaudeContentTypeText:
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
			case relaymodel.ClaudeContentTypeImage:
				if content.Source != nil {
					imageURL := relaymodel.ImageURL{}
					switch content.Source.Type {
					case relaymodel.ClaudeImageSourceTypeURL:
						imageURL.URL = content.Source.URL
					case relaymodel.ClaudeImageSourceTypeBase64:
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
					Type: relaymodel.ToolChoiceTypeFunction,
					Function: relaymodel.Function{
						Name:      content.Name,
						Arguments: args,
					},
				}
				// Preserve Gemini thought signature if present (OpenAI format)
				if content.Signature != "" {
					toolCall.ExtraContent = &relaymodel.ExtraContent{
						Google: &relaymodel.GoogleExtraContent{
							ThoughtSignature: content.Signature,
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
					Role:       relaymodel.RoleTool,
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

// ConvertClaudeToolsToOpenAI converts Claude tools to OpenAI format
func ConvertClaudeToolsToOpenAI(claudeTools []relaymodel.ClaudeTool) []relaymodel.Tool {
	openAITools := make([]relaymodel.Tool, 0, len(claudeTools))

	for _, tool := range claudeTools {
		openAITool := relaymodel.Tool{
			Type: relaymodel.ToolChoiceTypeFunction,
			Function: relaymodel.Function{
				Name:        tool.Name,
				Description: tool.Description,
			},
		}

		// Convert input schema
		if tool.InputSchema != nil {
			params := map[string]any{
				"type":       tool.InputSchema.Type,
				"properties": tool.InputSchema.Properties,
			}

			// Only add required field if it's non-empty
			// Some OpenAI-compatible APIs reject null or empty required arrays
			if tool.InputSchema.Required != nil {
				// Check if required is a non-empty array
				if reqArray, ok := tool.InputSchema.Required.([]string); ok && len(reqArray) > 0 {
					params["required"] = tool.InputSchema.Required
				} else if reqAnyArray, ok := tool.InputSchema.Required.([]any); ok && len(reqAnyArray) > 0 {
					params["required"] = tool.InputSchema.Required
				}
			}

			openAITool.Function.Parameters = params
		}

		openAITools = append(openAITools, openAITool)
	}

	return openAITools
}

// convertClaudeToolChoice converts Claude tool choice to OpenAI format
func convertClaudeToolChoice(toolChoice any) any {
	if toolChoice == nil {
		return relaymodel.ToolChoiceAuto
	}

	switch v := toolChoice.(type) {
	case string:
		if v == relaymodel.ToolChoiceAny {
			return relaymodel.ToolChoiceRequired
		}
		return v
	case map[string]any:
		if toolType, ok := v["type"].(string); ok {
			switch toolType {
			case relaymodel.RoleTool:
				if name, ok := v["name"].(string); ok {
					return map[string]any{
						"type": relaymodel.ToolChoiceTypeFunction,
						"function": map[string]any{
							"name": name,
						},
					}
				}
			case relaymodel.ToolChoiceAny:
				return relaymodel.ToolChoiceRequired
			case relaymodel.ToolChoiceAuto:
				return relaymodel.ToolChoiceAuto
			}
		}
	}

	return relaymodel.ToolChoiceAuto
}

// ClaudeStreamHandler handles OpenAI streaming responses and converts them to Claude format
func ClaudeStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ClaudeErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

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
				Type:  relaymodel.ClaudeStreamTypeContentBlockStop,
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
				Type: relaymodel.ClaudeStreamTypeMessageStart,
				Message: &relaymodel.ClaudeResponse{
					ID:      messageID,
					Type:    relaymodel.ClaudeTypeMessage,
					Role:    relaymodel.RoleAssistant,
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
			_ = render.ClaudeObjectData(
				c,
				relaymodel.ClaudeStreamResponse{Type: relaymodel.ClaudeStreamTypePing},
			)
		}

		// Process each choice
		for _, choice := range openAIResponse.Choices {
			// Handle reasoning/thinking content
			if choice.Delta.ReasoningContent != "" {
				// If we're not in a thinking block, start one
				if currentContentType != relaymodel.ClaudeContentTypeThinking {
					closeCurrentBlock()

					currentContentIndex++
					currentContentType = relaymodel.ClaudeContentTypeThinking

					_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
						Type:  relaymodel.ClaudeStreamTypeContentBlockStart,
						Index: currentContentIndex,
						ContentBlock: &relaymodel.ClaudeContent{
							Type:     relaymodel.ClaudeContentTypeThinking,
							Thinking: "",
						},
					})
				}

				thinkingText.WriteString(choice.Delta.ReasoningContent)

				_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
					Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
					Index: currentContentIndex,
					Delta: &relaymodel.ClaudeDelta{
						Type:     relaymodel.ClaudeDeltaTypeThinkingDelta,
						Thinking: choice.Delta.ReasoningContent,
					},
				})
			}

			// Handle text content
			if content, ok := choice.Delta.Content.(string); ok && content != "" {
				// If we're not in a text block, start one
				if currentContentType != relaymodel.ClaudeContentTypeText {
					closeCurrentBlock()

					currentContentIndex++
					currentContentType = relaymodel.ClaudeContentTypeText

					_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
						Type:  relaymodel.ClaudeStreamTypeContentBlockStart,
						Index: currentContentIndex,
						ContentBlock: &relaymodel.ClaudeContent{
							Type: relaymodel.ClaudeContentTypeText,
							Text: "",
						},
					})
				}

				contentText.WriteString(content)

				_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
					Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
					Index: currentContentIndex,
					Delta: &relaymodel.ClaudeDelta{
						Type: relaymodel.ClaudeDeltaTypeTextDelta,
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
						currentContentType = relaymodel.ClaudeContentTypeToolUse

						toolCallsBuffer[idx] = &relaymodel.ClaudeContent{
							Type:  relaymodel.ClaudeContentTypeToolUse,
							ID:    toolCall.ID,
							Name:  toolCall.Function.Name,
							Input: make(map[string]any),
						}

						// Send content_block_start for tool use
						_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
							Type:         relaymodel.ClaudeStreamTypeContentBlockStart,
							Index:        currentContentIndex,
							ContentBlock: toolCallsBuffer[idx],
						})
					}

					// Send tool arguments delta
					if toolCall.Function.Arguments != "" {
						_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
							Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
							Index: currentContentIndex,
							Delta: &relaymodel.ClaudeDelta{
								Type:        relaymodel.ClaudeDeltaTypeInputJSONDelta,
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
		stopReason = relaymodel.ClaudeStopReasonEndTurn
	}

	// Send message_delta with final usage
	_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
		Type: relaymodel.ClaudeStreamTypeMessageDelta,
		Delta: &relaymodel.ClaudeDelta{
			StopReason: &stopReason,
		},
		Usage: &claudeUsage,
	})

	// Send message_stop
	_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
		Type: relaymodel.ClaudeStreamTypeMessageStop,
	})

	return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, nil
}

// ClaudeHandler handles OpenAI non-streaming responses and converts them to Claude format
func ClaudeHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ClaudeErrorHandler(resp)
	}

	defer resp.Body.Close()

	// Read OpenAI response
	body, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Parse OpenAI response
	var openAIResponse relaymodel.TextResponse

	err = sonic.Unmarshal(body, &openAIResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert to Claude response
	claudeResponse := relaymodel.ClaudeResponse{
		ID:           "msg_" + common.ShortUUID(),
		Type:         relaymodel.ClaudeTypeMessage,
		Role:         relaymodel.RoleAssistant,
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
				Type: relaymodel.ClaudeContentTypeText,
				Text: content,
			})
		}

		// Handle reasoning content (for o1 models)
		if choice.Message.ReasoningContent != "" {
			claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
				Type:     relaymodel.ClaudeContentTypeThinking,
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
				Type:  relaymodel.ClaudeContentTypeToolUse,
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
			Type: relaymodel.ClaudeContentTypeText,
			Text: "",
		})
	}

	// Convert usage
	claudeResponse.Usage = openAIResponse.Usage.ToClaudeUsage()

	// Add web search usage if available
	if openAIResponse.Usage.WebSearchCount > 0 {
		claudeResponse.Usage.ServerToolUse = &relaymodel.ClaudeServerToolUse{
			WebSearchRequests: openAIResponse.Usage.WebSearchCount,
		}
	}

	// Marshal Claude response
	claudeResponseData, err := sonic.Marshal(claudeResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Write response
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(claudeResponseData)))
	_, _ = c.Writer.Write(claudeResponseData)

	return adaptor.DoResponseResult{Usage: claudeResponse.Usage.ToOpenAIUsage().ToModelUsage()}, nil
}

// convertFinishReasonToClaude converts OpenAI finish reason to Claude stop reason
func convertFinishReasonToClaude(finishReason string) *string {
	switch finishReason {
	case relaymodel.FinishReasonStop:
		v := relaymodel.ClaudeStopReasonEndTurn
		return &v
	case relaymodel.FinishReasonLength:
		v := relaymodel.ClaudeStopReasonMaxTokens
		return &v
	case relaymodel.FinishReasonToolCalls:
		v := relaymodel.ClaudeStopReasonToolUse
		return &v
	case relaymodel.FinishReasonContentFilter:
		v := relaymodel.ClaudeStopReasonStopSequence
		return &v
	case "":
		v := relaymodel.ClaudeStopReasonEndTurn
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

// ConvertClaudeToResponsesRequest converts a Claude request to Responses API format
func ConvertClaudeToResponsesRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	// First convert Claude to OpenAI format
	openAIRequest, err := ConvertClaudeRequestModel(meta, req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Create Responses API request
	responsesReq := relaymodel.CreateResponseRequest{
		Model:  meta.ActualModel,
		Input:  ConvertMessagesToInputItems(openAIRequest.Messages),
		Stream: openAIRequest.Stream,
	}

	// Map fields from OpenAI request
	if openAIRequest.Temperature != nil {
		responsesReq.Temperature = openAIRequest.Temperature
	}

	if openAIRequest.TopP != nil {
		responsesReq.TopP = openAIRequest.TopP
	}

	if openAIRequest.MaxTokens > 0 {
		responsesReq.MaxOutputTokens = &openAIRequest.MaxTokens
	} else if openAIRequest.MaxCompletionTokens > 0 {
		responsesReq.MaxOutputTokens = &openAIRequest.MaxCompletionTokens
	}

	// Map tools
	if len(openAIRequest.Tools) > 0 {
		responsesReq.Tools = ConvertToolsToResponseTools(openAIRequest.Tools)
	}

	if openAIRequest.ToolChoice != nil {
		responsesReq.ToolChoice = openAIRequest.ToolChoice
	}

	// Force non-store mode
	storeValue := false
	responsesReq.Store = &storeValue

	// Marshal to JSON
	jsonData, err := sonic.Marshal(responsesReq)
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

// ConvertResponsesToClaudeResponse converts Responses API response to Claude format
func ConvertResponsesToClaudeResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var responsesResp relaymodel.Response

	err = sonic.Unmarshal(responseBody, &responsesResp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert to Claude format
	claudeResp := relaymodel.ClaudeResponse{
		ID:      responsesResp.ID,
		Type:    relaymodel.ClaudeTypeMessage,
		Role:    relaymodel.RoleAssistant,
		Model:   responsesResp.Model,
		Content: []relaymodel.ClaudeContent{},
	}

	// Convert output items to Claude content
	for _, outputItem := range responsesResp.Output {
		// Handle different output types
		switch outputItem.Type {
		case "reasoning":
			// Convert reasoning to thinking content
			for _, content := range outputItem.Content {
				if (content.Type == relaymodel.ClaudeContentTypeText || content.Type == "output_text") &&
					content.Text != "" {
					claudeResp.Content = append(claudeResp.Content, relaymodel.ClaudeContent{
						Type:     relaymodel.ClaudeContentTypeThinking,
						Thinking: content.Text,
					})
				}
			}
		default:
			// Handle regular message content
			for _, content := range outputItem.Content {
				if (content.Type == relaymodel.ClaudeContentTypeText || content.Type == "output_text") &&
					content.Text != "" {
					claudeResp.Content = append(claudeResp.Content, relaymodel.ClaudeContent{
						Type: relaymodel.ClaudeContentTypeText,
						Text: content.Text,
					})
				}
			}
		}
	}

	// Set stop reason based on status
	switch responsesResp.Status {
	case relaymodel.ResponseStatusCompleted:
		claudeResp.StopReason = relaymodel.ClaudeStopReasonEndTurn
	case relaymodel.ResponseStatusIncomplete:
		claudeResp.StopReason = relaymodel.ClaudeStopReasonMaxTokens
	default:
		claudeResp.StopReason = relaymodel.ClaudeStopReasonEndTurn
	}

	// Convert usage
	if responsesResp.Usage != nil {
		claudeResp.Usage = responsesResp.Usage.ToClaudeUsage()
	}

	// Marshal and return
	claudeRespData, err := sonic.Marshal(claudeResp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(claudeRespData)))
	_, _ = c.Writer.Write(claudeRespData)

	if responsesResp.Usage != nil {
		return adaptor.DoResponseResult{Usage: responsesResp.Usage.ToModelUsage()}, nil
	}

	return adaptor.DoResponseResult{}, nil
}

// ConvertResponsesToClaudeStreamResponse converts Responses API stream to Claude stream
func ConvertResponsesToClaudeStreamResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

	var usage model.Usage

	state := &claudeStreamState{
		meta: meta,
		c:    c,
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

		// Parse the stream event
		var event relaymodel.ResponseStreamEvent

		err := sonic.Unmarshal(data, &event)
		if err != nil {
			log.Error("error unmarshalling response stream: " + err.Error())
			continue
		}

		// Handle events
		switch event.Type {
		case relaymodel.EventResponseCreated:
			state.handleResponseCreated(&event)
		case relaymodel.EventOutputItemAdded:
			state.handleOutputItemAdded(&event)
		case relaymodel.EventContentPartAdded:
			state.handleContentPartAdded(&event)
		case relaymodel.EventReasoningTextDelta:
			state.handleReasoningTextDelta(&event)
		case relaymodel.EventOutputTextDelta:
			state.handleOutputTextDelta(&event)
		case relaymodel.EventFunctionCallArgumentsDelta:
			state.handleFunctionCallArgumentsDelta(&event)
		case relaymodel.EventOutputItemDone:
			state.handleOutputItemDone(&event)
		case relaymodel.EventResponseCompleted, relaymodel.EventResponseDone:
			if event.Response != nil && event.Response.Usage != nil {
				usage = event.Response.Usage.ToModelUsage()
			}

			state.handleResponseCompleted(&event)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading response stream: " + err.Error())
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
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
		Type: relaymodel.ClaudeStreamTypeMessageStart,
		Message: &relaymodel.ClaudeResponse{
			ID:      s.messageID,
			Type:    relaymodel.ClaudeTypeMessage,
			Role:    relaymodel.RoleAssistant,
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
		s.currentContentType = relaymodel.ClaudeContentTypeThinking
		// Send content_block_start for thinking
		_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
			Type:  relaymodel.ClaudeStreamTypeContentBlockStart,
			Index: s.contentIndex,
			ContentBlock: &relaymodel.ClaudeContent{
				Type:     relaymodel.ClaudeContentTypeThinking,
				Thinking: "",
			},
		})
	case relaymodel.InputItemTypeFunctionCall:
		s.currentContentType = relaymodel.ClaudeContentTypeToolUse
		s.currentToolUseID = event.Item.ID
		s.currentToolUseName = event.Item.Name
		s.currentToolUseCallID = event.Item.CallID
		s.toolUseInput = ""
		// Send content_block_start for tool_use
		_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
			Type:  relaymodel.ClaudeStreamTypeContentBlockStart,
			Index: s.contentIndex,
			ContentBlock: &relaymodel.ClaudeContent{
				Type:  relaymodel.ClaudeContentTypeToolUse,
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
	if event.Delta == "" || !s.sentMessageStart ||
		s.currentContentType != relaymodel.ClaudeContentTypeToolUse {
		return
	}

	// Accumulate input
	s.toolUseInput += event.Delta

	// Send input_json_delta
	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
		Index: s.contentIndex,
		Delta: &relaymodel.ClaudeDelta{
			Type:        relaymodel.ClaudeDeltaTypeInputJSONDelta,
			PartialJSON: event.Delta,
		},
	})
}

// handleContentPartAdded handles response.content_part.added event for Claude
func (s *claudeStreamState) handleContentPartAdded(event *relaymodel.ResponseStreamEvent) {
	if event.Part == nil || !s.sentMessageStart {
		return
	}

	if event.Part.Type == "output_text" &&
		s.currentContentType != relaymodel.ClaudeContentTypeThinking {
		s.currentContentType = relaymodel.ClaudeContentTypeText
		// Send content_block_start for new text content
		_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
			Type:  relaymodel.ClaudeStreamTypeContentBlockStart,
			Index: s.contentIndex,
			ContentBlock: &relaymodel.ClaudeContent{
				Type: relaymodel.ClaudeContentTypeText,
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
		Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
		Index: s.contentIndex,
		Delta: &relaymodel.ClaudeDelta{
			Type:     relaymodel.ClaudeDeltaTypeThinkingDelta,
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
		Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
		Index: s.contentIndex,
		Delta: &relaymodel.ClaudeDelta{
			Type: relaymodel.ClaudeDeltaTypeTextDelta,
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
	if event.Item.Type == relaymodel.InputItemTypeFunctionCall &&
		s.currentContentType == relaymodel.ClaudeContentTypeToolUse {
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
		Type:  relaymodel.ClaudeStreamTypeContentBlockStop,
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
	stopReason := relaymodel.ClaudeStopReasonEndTurn
	claudeUsage := event.Response.Usage.ToClaudeUsage()
	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type: relaymodel.ClaudeStreamTypeMessageDelta,
		Delta: &relaymodel.ClaudeDelta{
			StopReason: &stopReason,
		},
		Usage: &claudeUsage,
	})

	// Send message_stop
	_ = render.ClaudeObjectData(s.c, relaymodel.ClaudeStreamResponse{
		Type: relaymodel.ClaudeStreamTypeMessageStop,
	})
}
