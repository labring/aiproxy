package openai

import (
	"bufio"
	"bytes"
	"net/http"
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

	if geminiReq.SystemInstruction != nil && len(geminiReq.SystemInstruction.Parts) > 0 {
		systemText := ""
		for _, part := range geminiReq.SystemInstruction.Parts {
			if part.Text != "" {
				systemText += part.Text
			}
		}

		if systemText != "" {
			messages = append(messages, relaymodel.Message{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	// Convert contents to messages
	for _, content := range geminiReq.Contents {
		msg := relaymodel.Message{}

		// Map role
		switch content.Role {
		case "model":
			msg.Role = "assistant"
		case "user":
			msg.Role = "user"
		default:
			msg.Role = content.Role
		}

		// Convert parts
		if len(content.Parts) == 1 && content.Parts[0].Text != "" &&
			content.Parts[0].InlineData == nil {
			// Simple text message
			msg.Content = content.Parts[0].Text
		} else {
			// Complex message with multiple parts
			var contentParts []relaymodel.MessageContent
			for _, part := range content.Parts {
				switch {
				case part.FunctionCall != nil:
					// Handle function call
					args, _ := sonic.MarshalString(part.FunctionCall.Args)
					toolCall := relaymodel.ToolCall{
						ID:   CallID(),
						Type: "function",
						Function: relaymodel.Function{
							Name:      part.FunctionCall.Name,
							Arguments: args,
						},
					}
					msg.ToolCalls = append(msg.ToolCalls, toolCall)
				case part.FunctionResponse != nil:
					// Handle function response
					msg.Role = "tool"
					responseContent, _ := sonic.MarshalString(part.FunctionResponse.Response.Content)
					msg.Content = responseContent

					msg.Name = &part.FunctionResponse.Name
					if len(geminiReq.Contents) > 0 {
						// Try to find the corresponding tool call ID
						msg.ToolCallID = "call_" + part.FunctionResponse.Name
					}
				case part.Text != "":
					contentParts = append(contentParts, relaymodel.MessageContent{
						Type: relaymodel.ContentTypeText,
						Text: part.Text,
					})
				case part.InlineData != nil:
					// Handle image
					imageURL := part.InlineData.Data
					if !strings.HasPrefix(imageURL, "http") && !strings.HasPrefix(imageURL, "data:") {
						// Base64 data
						imageURL = "data:" + part.InlineData.MimeType + ";base64," + part.InlineData.Data
					}

					contentParts = append(contentParts, relaymodel.MessageContent{
						Type: relaymodel.ContentTypeImageURL,
						ImageURL: &relaymodel.ImageURL{
							URL: imageURL,
						},
					})
				}
			}

			if len(contentParts) > 0 {
				msg.Content = contentParts
			}
		}

		messages = append(messages, msg)
	}

	openaiReq.Messages = messages

	// Convert generation config
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
				if schema, ok := geminiReq.GenerationConfig.ResponseSchema.(map[string]any); ok {
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

	// Convert tools
	if len(geminiReq.Tools) > 0 {
		var tools []relaymodel.Tool
		for _, geminiTool := range geminiReq.Tools {
			if fnDecls, ok := geminiTool.FunctionDeclarations.([]any); ok {
				for _, fnDecl := range fnDecls {
					if fn, ok := fnDecl.(map[string]any); ok {
						name, _ := fn["name"].(string)
						description, _ := fn["description"].(string)
						function := relaymodel.Function{
							Name:        name,
							Description: description,
							Parameters:  fn["parameters"],
						}
						tools = append(tools, relaymodel.Tool{
							Type:     "function",
							Function: function,
						})
					}
				}
			}
		}

		if len(tools) > 0 {
			openaiReq.Tools = tools
		}
	}

	// Convert tool config
	if geminiReq.ToolConfig != nil {
		switch geminiReq.ToolConfig.FunctionCallingConfig.Mode {
		case "AUTO":
			openaiReq.ToolChoice = "auto"
		case "NONE":
			openaiReq.ToolChoice = "none"
		case "ANY":
			if len(geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames) > 0 {
				openaiReq.ToolChoice = map[string]any{
					"type": "function",
					"function": map[string]any{
						"name": geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames[0],
					},
				}
			} else {
				openaiReq.ToolChoice = "required"
			}
		}
	}

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
		geminiResp := convertOpenAIStreamToGemini(meta, &openaiResp)
		if geminiResp != nil {
			_ = render.GeminiObjectData(c, geminiResp)

			if geminiResp.UsageMetadata != nil {
				usage = geminiResp.UsageMetadata.ToUsage().ToModelUsage()
			}
		}
	}

	return usage, nil
}

func convertOpenAIStreamToGemini(
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

	for _, choice := range openaiResp.Choices {
		candidate := &relaymodel.GeminiChatCandidate{
			Index: int64(choice.Index),
			Content: relaymodel.GeminiChatContent{
				Role:  "model",
				Parts: []*relaymodel.GeminiPart{},
			},
		}

		if choice.FinishReason != "" {
			switch choice.FinishReason {
			case relaymodel.FinishReasonStop:
				candidate.FinishReason = "STOP"
			case relaymodel.FinishReasonLength:
				candidate.FinishReason = "MAX_TOKENS"
			default:
				candidate.FinishReason = "STOP"
			}
		}

		// Convert delta content
		if choice.Delta.Content != nil {
			if content, ok := choice.Delta.Content.(string); ok && content != "" {
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text: content,
				})
			}
		}

		// Convert tool calls
		for _, toolCall := range choice.Delta.ToolCalls {
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
