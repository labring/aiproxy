package anthropic

import (
	"bufio"
	"bytes"
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

// ConvertGeminiRequest converts a Gemini native request to Claude format
func ConvertGeminiRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	// Parse Gemini request
	geminiReq, err := utils.UnmarshalGeminiChatRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Convert to Claude format
	claudeReq := relaymodel.ClaudeRequest{
		Model:     meta.ActualModel,
		MaxTokens: 4096,
		Messages:  []relaymodel.ClaudeMessage{},
		System:    []relaymodel.ClaudeContent{},
	}

	// Check if this is a streaming request by checking the URL path
	// URL format: /v1beta/models/{model}:streamGenerateContent
	if utils.IsGeminiStreamRequest(req.URL.Path) {
		claudeReq.Stream = true
	}

	// Convert system instruction
	if geminiReq.SystemInstruction != nil && len(geminiReq.SystemInstruction.Parts) > 0 {
		for _, part := range geminiReq.SystemInstruction.Parts {
			if part.Text != "" {
				claudeReq.System = append(claudeReq.System, relaymodel.ClaudeContent{
					Type: "text",
					Text: part.Text,
				})
			}
		}
	}

	// Convert contents to messages
	for _, content := range geminiReq.Contents {
		msg := relaymodel.ClaudeMessage{}

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
		for _, part := range content.Parts {
			switch {
			case part.FunctionCall != nil:
				// Handle function call - convert to tool use
				msg.Content = append(msg.Content, relaymodel.ClaudeContent{
					Type:  "tool_use",
					ID:    "toolu_" + part.FunctionCall.Name,
					Name:  part.FunctionCall.Name,
					Input: part.FunctionCall.Args,
				})
			case part.FunctionResponse != nil:
				// Handle function response - convert to tool result
				msg.Role = "user"
				content, _ := sonic.MarshalString(part.FunctionResponse.Response.Content)
				msg.Content = append(msg.Content, relaymodel.ClaudeContent{
					Type:      "tool_result",
					ToolUseID: "toolu_" + part.FunctionResponse.Name,
					Content:   content,
				})
			case part.Text != "":
				if part.Thought {
					// Handle thinking content
					msg.Content = append(msg.Content, relaymodel.ClaudeContent{
						Type:     "thinking",
						Thinking: part.Text,
					})
				} else {
					// Handle text content
					msg.Content = append(msg.Content, relaymodel.ClaudeContent{
						Type: "text",
						Text: part.Text,
					})
				}
			case part.InlineData != nil:
				// Handle image
				imageData := part.InlineData.Data
				// If not base64, assume it's a URL (shouldn't happen in gemini native)
				if strings.HasPrefix(imageData, "http") {
					// Download and convert to base64
					// For now, just skip
					continue
				}

				msg.Content = append(msg.Content, relaymodel.ClaudeContent{
					Type: "image",
					Source: &relaymodel.ClaudeImageSource{
						Type:      "base64",
						MediaType: part.InlineData.MimeType,
						Data:      imageData,
					},
				})
			}
		}

		claudeReq.Messages = append(claudeReq.Messages, msg)
	}

	// Convert generation config
	if geminiReq.GenerationConfig != nil {
		if geminiReq.GenerationConfig.Temperature != nil {
			claudeReq.Temperature = geminiReq.GenerationConfig.Temperature
		}

		if geminiReq.GenerationConfig.TopP != nil {
			claudeReq.TopP = geminiReq.GenerationConfig.TopP
		}

		if geminiReq.GenerationConfig.MaxOutputTokens != nil {
			claudeReq.MaxTokens = *geminiReq.GenerationConfig.MaxOutputTokens
		}
	}

	// Convert tools
	if len(geminiReq.Tools) > 0 {
		var tools []relaymodel.ClaudeTool
		for _, geminiTool := range geminiReq.Tools {
			if fnDecls, ok := geminiTool.FunctionDeclarations.([]any); ok {
				for _, fnDecl := range fnDecls {
					if fn, ok := fnDecl.(map[string]any); ok {
						var inputSchema *relaymodel.ClaudeInputSchema
						if params, ok := fn["parameters"].(map[string]any); ok {
							inputSchema = &relaymodel.ClaudeInputSchema{
								Type:       "object",
								Properties: params,
							}
						}

						name, _ := fn["name"].(string)
						description, _ := fn["description"].(string)
						tools = append(tools, relaymodel.ClaudeTool{
							Name:        name,
							Description: description,
							InputSchema: inputSchema,
						})
					}
				}
			}
		}

		if len(tools) > 0 {
			claudeReq.Tools = tools
		}
	}

	// Convert tool config
	if geminiReq.ToolConfig != nil {
		switch geminiReq.ToolConfig.FunctionCallingConfig.Mode {
		case "AUTO":
			claudeReq.ToolChoice = map[string]any{"type": "auto"}
		case "ANY":
			if len(geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames) > 0 {
				claudeReq.ToolChoice = map[string]any{
					"type": "tool",
					"name": geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames[0],
				}
			} else {
				claudeReq.ToolChoice = map[string]any{"type": "any"}
			}
		}
	}

	// Marshal to JSON
	data, err := sonic.Marshal(claudeReq)
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

// ConvertClaudeToGeminiResponse converts Claude response to Gemini format
func ConvertClaudeToGeminiResponse(
	meta *meta.Meta,
	claudeResp *relaymodel.ClaudeResponse,
) *relaymodel.GeminiChatResponse {
	geminiResp := &relaymodel.GeminiChatResponse{
		ModelVersion: meta.ActualModel,
		Candidates:   []*relaymodel.GeminiChatCandidate{},
	}

	// Convert usage
	if claudeResp.Usage.InputTokens > 0 || claudeResp.Usage.OutputTokens > 0 {
		geminiResp.UsageMetadata = &relaymodel.GeminiUsageMetadata{
			PromptTokenCount:     claudeResp.Usage.InputTokens,
			CandidatesTokenCount: claudeResp.Usage.OutputTokens,
			TotalTokenCount:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		}
	}

	// Create candidate
	candidate := &relaymodel.GeminiChatCandidate{
		Index: 0,
		Content: relaymodel.GeminiChatContent{
			Role:  "model",
			Parts: []*relaymodel.GeminiPart{},
		},
	}

	// Convert stop reason
	switch claudeResp.StopReason {
	case "end_turn":
		candidate.FinishReason = "STOP"
	case "max_tokens":
		candidate.FinishReason = "MAX_TOKENS"
	case "tool_use":
		candidate.FinishReason = "STOP"
	default:
		candidate.FinishReason = "STOP"
	}

	// Convert content
	for _, content := range claudeResp.Content {
		switch content.Type {
		case "text":
			if content.Text != "" {
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text: content.Text,
				})
			}
		case "thinking":
			if content.Thinking != "" {
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text:    content.Thinking,
					Thought: true,
				})
			}
		case "tool_use":
			if inputMap, ok := content.Input.(map[string]any); ok {
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					FunctionCall: &relaymodel.GeminiFunctionCall{
						Name: content.Name,
						Args: inputMap,
					},
				})
			}
		}
	}

	geminiResp.Candidates = append(geminiResp.Candidates, candidate)

	return geminiResp
}

// GeminiHandler handles non-streaming responses and converts them to Gemini format
func GeminiHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var claudeResp relaymodel.ClaudeResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return model.Usage{}, relaymodel.WrapperAnthropicError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	geminiResp := ConvertClaudeToGeminiResponse(meta, &claudeResp)

	jsonResponse, err := sonic.Marshal(geminiResp)
	if err != nil {
		return claudeResp.Usage.ToOpenAIUsage().ToModelUsage(), relaymodel.WrapperAnthropicError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))

	_, _ = c.Writer.Write(jsonResponse)

	return claudeResp.Usage.ToOpenAIUsage().ToModelUsage(), nil
}

// GeminiStreamHandler handles streaming responses and converts them to Gemini format
func GeminiStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner := bufio.NewScanner(resp.Body)

	buf := utils.GetScannerBuffer()
	defer utils.PutScannerBuffer(buf)

	scanner.Buffer(*buf, cap(*buf))

	usage := model.Usage{}

	var (
		currentText     strings.Builder
		currentThinking strings.Builder
	)

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var claudeResp relaymodel.ClaudeStreamResponse
		if err := sonic.Unmarshal(data, &claudeResp); err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		// Convert to Gemini stream format
		geminiResp := ConvertClaudeStreamToGemini(meta, &claudeResp, &currentText, &currentThinking)
		if geminiResp != nil {
			_ = render.GeminiObjectData(c, geminiResp)

			if geminiResp.UsageMetadata != nil {
				usage = geminiResp.UsageMetadata.ToUsage().ToModelUsage()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	return usage, nil
}

func ConvertClaudeStreamToGemini(
	meta *meta.Meta,
	claudeResp *relaymodel.ClaudeStreamResponse,
	currentText *strings.Builder,
	currentThinking *strings.Builder,
) *relaymodel.GeminiChatResponse {
	geminiResp := &relaymodel.GeminiChatResponse{
		ModelVersion: meta.ActualModel,
		Candidates:   []*relaymodel.GeminiChatCandidate{},
	}

	candidate := &relaymodel.GeminiChatCandidate{
		Index: 0,
		Content: relaymodel.GeminiChatContent{
			Role:  "model",
			Parts: []*relaymodel.GeminiPart{},
		},
	}

	switch claudeResp.Type {
	case "message_start":
		if claudeResp.Message != nil && claudeResp.Message.Usage.InputTokens > 0 {
			geminiResp.UsageMetadata = &relaymodel.GeminiUsageMetadata{
				PromptTokenCount: claudeResp.Message.Usage.InputTokens,
			}
		}

		return geminiResp

	case "content_block_delta":
		if claudeResp.Delta != nil {
			if claudeResp.Delta.Type == "text_delta" && claudeResp.Delta.Text != "" {
				currentText.WriteString(claudeResp.Delta.Text)
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text: claudeResp.Delta.Text,
				})
			} else if claudeResp.Delta.Type == "thinking_delta" && claudeResp.Delta.Thinking != "" {
				currentThinking.WriteString(claudeResp.Delta.Thinking)
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text:    claudeResp.Delta.Thinking,
					Thought: true,
				})
			}
		}

	case "content_block_start":
		if claudeResp.ContentBlock != nil {
			if claudeResp.ContentBlock.Type == "tool_use" {
				if inputMap, ok := claudeResp.ContentBlock.Input.(map[string]any); ok {
					candidate.Content.Parts = append(
						candidate.Content.Parts,
						&relaymodel.GeminiPart{
							FunctionCall: &relaymodel.GeminiFunctionCall{
								Name: claudeResp.ContentBlock.Name,
								Args: inputMap,
							},
						},
					)
				}
			}
		}

	case "message_delta":
		if claudeResp.Delta != nil && claudeResp.Delta.StopReason != nil {
			switch *claudeResp.Delta.StopReason {
			case "end_turn":
				candidate.FinishReason = "STOP"
			case "max_tokens":
				candidate.FinishReason = "MAX_TOKENS"
			case "tool_use":
				candidate.FinishReason = "STOP"
			default:
				candidate.FinishReason = "STOP"
			}
		}

		if claudeResp.Usage != nil {
			geminiResp.UsageMetadata = &relaymodel.GeminiUsageMetadata{
				PromptTokenCount:     claudeResp.Usage.InputTokens,
				CandidatesTokenCount: claudeResp.Usage.OutputTokens,
				TotalTokenCount:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
			}
		}

	case "message_stop":
		return nil
	}

	if len(candidate.Content.Parts) > 0 {
		geminiResp.Candidates = append(geminiResp.Candidates, candidate)
		return geminiResp
	}

	return nil
}
