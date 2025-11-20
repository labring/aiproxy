package openai

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

// ConvertGeminiRequest converts a Gemini native request to OpenAI format
func ConvertGeminiRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	// Parse Gemini request
	geminiReq, err := utils.UnmarshalGeminiChatRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Convert to OpenAI format
	openaiReq := relaymodel.GeneralOpenAIRequest{
		Model: meta.ActualModel,
	}

	// Check if this is a streaming request by checking the URL path
	// URL format: /v1beta/models/{model}:streamGenerateContent
	if utils.IsGeminiStreamRequest(req.URL.Path) {
		openaiReq.Stream = true
	}

	// Convert system instruction to system message
	// Pre-allocate messages slice with estimated capacity
	estimatedCap := len(geminiReq.Contents)
	if geminiReq.SystemInstruction != nil && len(geminiReq.SystemInstruction.Parts) > 0 {
		estimatedCap++
	}

	messages := make([]relaymodel.Message, 0, estimatedCap)

	if systemMsgs := convertGeminiSystemToOpenAI(geminiReq); len(systemMsgs) > 0 {
		messages = append(messages, systemMsgs...)
	}

	// Track pending tool calls to match responses
	var pendingTools []relaymodel.ToolCall

	// Convert contents to messages
	for _, content := range geminiReq.Contents {
		msgs := convertGeminiContentToOpenAI(content, &pendingTools)
		messages = append(messages, msgs...)
	}

	openaiReq.Messages = messages

	// Convert generation config
	convertGeminiGenerationConfigToOpenAI(geminiReq, &openaiReq)

	// Convert tools
	openaiReq.Tools = convertGeminiToolsToOpenAI(geminiReq)

	// Convert tool config
	openaiReq.ToolChoice = convertGeminiToolConfigToOpenAI(geminiReq)

	// Marshal to JSON
	data, err := sonic.Marshal(openaiReq)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(data))},
		},
		Body: bytes.NewReader(data),
	}, nil
}

// ConvertOpenAIToGeminiResponse converts OpenAI response back to Gemini format
func ConvertOpenAIToGeminiResponse(
	meta *meta.Meta,
	openaiResp *relaymodel.TextResponse,
) *relaymodel.GeminiChatResponse {
	geminiResp := &relaymodel.GeminiChatResponse{
		ModelVersion: meta.ActualModel,
	}

	if openaiResp.Usage.TotalTokens > 0 {
		geminiResp.UsageMetadata = &relaymodel.GeminiUsageMetadata{
			PromptTokenCount:     openaiResp.Usage.PromptTokens,
			CandidatesTokenCount: openaiResp.Usage.CompletionTokens,
			TotalTokenCount:      openaiResp.Usage.TotalTokens,
		}
	}

	for _, choice := range openaiResp.Choices {
		candidate := &relaymodel.GeminiChatCandidate{
			Index: int64(choice.Index),
			Content: relaymodel.GeminiChatContent{
				Role:  "model",
				Parts: []*relaymodel.GeminiPart{},
			},
		}

		// Convert finish reason
		switch choice.FinishReason {
		case relaymodel.FinishReasonStop:
			candidate.FinishReason = "STOP"
		case relaymodel.FinishReasonLength:
			candidate.FinishReason = "MAX_TOKENS"
		case relaymodel.FinishReasonToolCalls:
			candidate.FinishReason = "STOP"
		default:
			candidate.FinishReason = "STOP"
		}

		// Convert content
		if choice.Message.Content != nil {
			switch content := choice.Message.Content.(type) {
			case string:
				if content != "" {
					candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
						Text: content,
					})
				}
			case []relaymodel.MessageContent:
				for _, part := range content {
					if part.Type == relaymodel.ContentTypeText {
						candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
							Text: part.Text,
						})
					}
				}
			}
		}

		// Convert tool calls
		for _, toolCall := range choice.Message.ToolCalls {
			var args map[string]any

			_ = sonic.UnmarshalString(toolCall.Function.Arguments, &args)

			candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
				FunctionCall: &relaymodel.GeminiFunctionCall{
					Name: toolCall.Function.Name,
					Args: args,
				},
			})
		}

		geminiResp.Candidates = append(geminiResp.Candidates, candidate)
	}

	return geminiResp
}

// GeminiStreamHandler handles streaming responses and converts them to Gemini format
func GeminiStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	buf := utils.GetScannerBuffer()
	defer utils.PutScannerBuffer(buf)

	scanner.Buffer(*buf, cap(*buf))

	usage := model.Usage{}
	streamState := NewGeminiStreamState()

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var openaiResp relaymodel.ChatCompletionsStreamResponse
		if err := sonic.Unmarshal(data, &openaiResp); err != nil {
			continue
		}

		// Convert to Gemini stream format
		geminiResp := streamState.ConvertOpenAIStreamToGemini(meta, &openaiResp)
		if geminiResp != nil {
			_ = render.GeminiObjectData(c, geminiResp)

			if geminiResp.UsageMetadata != nil {
				usage = geminiResp.UsageMetadata.ToUsage().ToModelUsage()
			}
		}
	}

	return usage, nil
}

type GeminiStreamState struct {
	ToolCallBuffer map[string]*ToolCallState
}

type ToolCallState struct {
	Name      string
	Arguments string
}

func NewGeminiStreamState() *GeminiStreamState {
	return &GeminiStreamState{
		ToolCallBuffer: make(map[string]*ToolCallState),
	}
}

func (s *GeminiStreamState) ConvertOpenAIStreamToGemini(
	meta *meta.Meta,
	openaiResp *relaymodel.ChatCompletionsStreamResponse,
) *relaymodel.GeminiChatResponse {
	if len(openaiResp.Choices) == 0 {
		return nil
	}

	geminiResp := &relaymodel.GeminiChatResponse{
		ModelVersion: meta.ActualModel,
		Candidates:   []*relaymodel.GeminiChatCandidate{},
	}

	if openaiResp.Usage != nil {
		geminiResp.UsageMetadata = &relaymodel.GeminiUsageMetadata{
			PromptTokenCount:     openaiResp.Usage.PromptTokens,
			CandidatesTokenCount: openaiResp.Usage.CompletionTokens,
			TotalTokenCount:      openaiResp.Usage.TotalTokens,
		}
	}

	hasContent := geminiResp.UsageMetadata != nil

	for _, choice := range openaiResp.Choices {
		candidate := &relaymodel.GeminiChatCandidate{
			Index: int64(choice.Index),
			Content: relaymodel.GeminiChatContent{
				Role:  "model",
				Parts: []*relaymodel.GeminiPart{},
			},
		}

		// Convert delta content
		if choice.Delta.Content != nil {
			if content, ok := choice.Delta.Content.(string); ok && content != "" {
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text: content,
				})
				hasContent = true
			}
		}

		// Buffer tool calls
		for _, toolCall := range choice.Delta.ToolCalls {
			key := fmt.Sprintf("%d-%d", choice.Index, toolCall.Index)

			state, ok := s.ToolCallBuffer[key]
			if !ok {
				state = &ToolCallState{}
				s.ToolCallBuffer[key] = state
			}

			if toolCall.Function.Name != "" {
				state.Name = toolCall.Function.Name
			}

			if toolCall.Function.Arguments != "" {
				state.Arguments += toolCall.Function.Arguments
			}
		}

		// Check if we need to flush tool calls (on finish)
		if choice.FinishReason != "" {
			switch choice.FinishReason {
			case relaymodel.FinishReasonStop:
				candidate.FinishReason = "STOP"
			case relaymodel.FinishReasonLength:
				candidate.FinishReason = "MAX_TOKENS"
			case relaymodel.FinishReasonToolCalls:
				candidate.FinishReason = "STOP"
			default:
				candidate.FinishReason = "STOP"
			}

			// Flush buffered tool calls for this choice
			prefix := fmt.Sprintf("%d-", choice.Index)

			// Collect matching items to sort them
			type toolCallItem struct {
				Index int
				Key   string
				State *ToolCallState
			}

			var items []toolCallItem

			for key, state := range s.ToolCallBuffer {
				if strings.HasPrefix(key, prefix) {
					parts := strings.Split(key, "-")
					if len(parts) == 2 {
						idx, _ := strconv.Atoi(parts[1])
						items = append(items, toolCallItem{
							Index: idx,
							Key:   key,
							State: state,
						})
					}
				}
			}

			// Sort by index
			sort.Slice(items, func(i, j int) bool {
				return items[i].Index < items[j].Index
			})

			for _, item := range items {
				var args map[string]any

				_ = sonic.UnmarshalString(item.State.Arguments, &args)

				candidate.Content.Parts = append(
					candidate.Content.Parts,
					&relaymodel.GeminiPart{
						FunctionCall: &relaymodel.GeminiFunctionCall{
							Name: item.State.Name,
							Args: args,
						},
					},
				)
				hasContent = true
				// Remove from buffer
				delete(s.ToolCallBuffer, item.Key)
			}
		}

		if hasContent || candidate.FinishReason != "" {
			geminiResp.Candidates = append(geminiResp.Candidates, candidate)
		}
	}

	if !hasContent && len(geminiResp.Candidates) == 0 {
		return nil
	}

	return geminiResp
}

// GeminiHandler handles non-streaming responses and converts them to Gemini format
func GeminiHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	var openaiResp relaymodel.TextResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	geminiResp := ConvertOpenAIToGeminiResponse(meta, &openaiResp)

	jsonResponse, err := sonic.Marshal(geminiResp)
	if err != nil {
		return openaiResp.Usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return openaiResp.Usage.ToModelUsage(), nil
}

func convertGeminiSystemToOpenAI(geminiReq *relaymodel.GeminiChatRequest) []relaymodel.Message {
	if geminiReq.SystemInstruction == nil || len(geminiReq.SystemInstruction.Parts) == 0 {
		return nil
	}

	systemText := ""
	for _, part := range geminiReq.SystemInstruction.Parts {
		if part.Text != "" {
			systemText += part.Text
		}
	}

	if systemText != "" {
		return []relaymodel.Message{{
			Role:    "system",
			Content: systemText,
		}}
	}

	return nil
}

func convertGeminiToolsToOpenAI(geminiReq *relaymodel.GeminiChatRequest) []relaymodel.Tool {
	if len(geminiReq.Tools) == 0 {
		return nil
	}

	var tools []relaymodel.Tool
	for _, geminiTool := range geminiReq.Tools {
		if fnDecls, ok := geminiTool.FunctionDeclarations.([]any); ok {
			for _, fnDecl := range fnDecls {
				if fn, ok := fnDecl.(map[string]any); ok {
					name, _ := fn["name"].(string)
					description, _ := fn["description"].(string)

					parameters := fn["parameters"]
					if parameters == nil {
						parameters = fn["parametersJsonSchema"]
					}

					function := relaymodel.Function{
						Name:        name,
						Description: description,
						Parameters:  parameters,
					}
					tools = append(tools, relaymodel.Tool{
						Type:     "function",
						Function: function,
					})
				}
			}
		}
	}

	return tools
}

func convertGeminiToolConfigToOpenAI(geminiReq *relaymodel.GeminiChatRequest) any {
	if geminiReq.ToolConfig == nil {
		return nil
	}

	switch geminiReq.ToolConfig.FunctionCallingConfig.Mode {
	case "AUTO":
		return "auto"
	case "NONE":
		return "none"
	case "ANY":
		if len(geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames) > 0 {
			return map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames[0],
				},
			}
		}

		return "required"
	}

	return nil
}

func convertGeminiGenerationConfigToOpenAI(
	geminiReq *relaymodel.GeminiChatRequest,
	openaiReq *relaymodel.GeneralOpenAIRequest,
) {
	if geminiReq.GenerationConfig != nil {
		openaiReq.Temperature = geminiReq.GenerationConfig.Temperature

		openaiReq.TopP = geminiReq.GenerationConfig.TopP
		if geminiReq.GenerationConfig.MaxOutputTokens != nil {
			openaiReq.MaxTokens = *geminiReq.GenerationConfig.MaxOutputTokens
		}

		// Handle response format
		if geminiReq.GenerationConfig.ResponseMimeType != "" {
			switch geminiReq.GenerationConfig.ResponseMimeType {
			case "application/json":
				openaiReq.ResponseFormat = &relaymodel.ResponseFormat{Type: "json_object"}
			case "text/plain":
				openaiReq.ResponseFormat = &relaymodel.ResponseFormat{Type: "text"}
			}

			if geminiReq.GenerationConfig.ResponseSchema != nil {
				schema := geminiReq.GenerationConfig.ResponseSchema
				openaiReq.ResponseFormat = &relaymodel.ResponseFormat{
					Type: "json_schema",
					JSONSchema: &relaymodel.JSONSchema{
						Name:   "response",
						Schema: schema,
					},
				}
			}
		}
	}
}

func convertGeminiContentToOpenAI(
	content *relaymodel.GeminiChatContent,
	pendingTools *[]relaymodel.ToolCall,
) []relaymodel.Message {
	var messages []relaymodel.Message

	// Map role
	role := content.Role
	switch role {
	case "model":
		role = "assistant"
	case "user":
		role = "user"
	}

	// Current message builder
	currentMsg := relaymodel.Message{
		Role: role,
	}

	var currentContentParts []relaymodel.MessageContent

	hasContent := false

	// Convert parts
	for _, part := range content.Parts {
		switch {
		case part.FunctionCall != nil:
			// Handle function call (Assistant)
			if part.FunctionCall.Name == "" {
				continue
			}

			args, _ := sonic.MarshalString(part.FunctionCall.Args)
			toolCall := relaymodel.ToolCall{
				ID:   CallID(),
				Type: "function",
				Function: relaymodel.Function{
					Name:      part.FunctionCall.Name,
					Arguments: args,
				},
			}
			currentMsg.ToolCalls = append(currentMsg.ToolCalls, toolCall)
			hasContent = true

			// Track this call
			*pendingTools = append(*pendingTools, toolCall)

		case part.FunctionResponse != nil:
			// Handle function response
			// Flush current message if it has content
			if hasContent {
				if len(currentContentParts) > 0 {
					currentMsg.Content = currentContentParts
				}

				messages = append(messages, currentMsg)
				// Reset
				currentMsg = relaymodel.Message{Role: role}
				currentContentParts = nil
				hasContent = false
			}

			// Create Tool Message
			name := part.FunctionResponse.Name

			var id string

			// Try to find in pendingTools by name to ensure we match the generated ID
			foundIdx := -1
			if pendingTools != nil {
				for i, tool := range *pendingTools {
					if tool.Function.Name == name {
						id = tool.ID
						foundIdx = i
						break
					}
				}
			}

			if foundIdx != -1 {
				// Remove found tool from pending
				*pendingTools = append((*pendingTools)[:foundIdx], (*pendingTools)[foundIdx+1:]...)
			} else {
				// If not found, use provided ID or fallback
				// OpenAI requires tool_call_id to be <= 40 characters
				if part.FunctionResponse.ID != "" && len(part.FunctionResponse.ID) <= 40 {
					id = part.FunctionResponse.ID
				} else {
					// Fallback to generated ID if not provided or too long
					// We use CallID() which generates a short ID (e.g. "call_" + uuid)
					// to ensure it meets length requirements
					id = CallID()
				}

				// Inject synthetic Assistant message with ToolCall to satisfy OpenAI protocol
				// This handles cases where the client omits the model's function call message
				syntheticCall := relaymodel.ToolCall{
					ID:   id,
					Type: "function",
					Function: relaymodel.Function{
						Name:      name,
						Arguments: "{}", // Assume empty args as we can't reconstruct them
					},
				}

				syntheticMsg := relaymodel.Message{
					Role:      "assistant",
					ToolCalls: []relaymodel.ToolCall{syntheticCall},
				}

				messages = append(messages, syntheticMsg)
			}

			responseContent, _ := sonic.MarshalString(part.FunctionResponse.Response)

			toolMsg := relaymodel.Message{
				Role:       "tool",
				Content:    responseContent,
				ToolCallID: id,
				Name:       &name,
			}
			messages = append(messages, toolMsg)

		case part.Text != "":
			currentContentParts = append(currentContentParts, relaymodel.MessageContent{
				Type: relaymodel.ContentTypeText,
				Text: part.Text,
			})
			hasContent = true

		case part.InlineData != nil:
			// Handle image
			imageURL := part.InlineData.Data
			if !strings.HasPrefix(imageURL, "http") && !strings.HasPrefix(imageURL, "data:") {
				// Base64 data
				imageURL = "data:" + part.InlineData.MimeType + ";base64," + part.InlineData.Data
			}

			currentContentParts = append(currentContentParts, relaymodel.MessageContent{
				Type: relaymodel.ContentTypeImageURL,
				ImageURL: &relaymodel.ImageURL{
					URL: imageURL,
				},
			})
			hasContent = true
		}
	}

	if hasContent {
		if len(currentContentParts) > 0 {
			if len(currentContentParts) == 1 &&
				currentContentParts[0].Type == relaymodel.ContentTypeText &&
				len(currentMsg.ToolCalls) == 0 {
				// Simple text message
				currentMsg.Content = currentContentParts[0].Text
			} else {
				currentMsg.Content = currentContentParts
			}
		}

		messages = append(messages, currentMsg)
	}

	return messages
}
