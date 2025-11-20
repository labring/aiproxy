// Package openai provides adapters for converting between OpenAI API formats
// and the Responses API format for gpt-5 models.
package openai

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

// cleanToolParameters removes null or empty required field from tool parameters
// Responses API requires the 'required' field to be either:
// - A non-empty array of strings
// - Completely absent from the schema
// It cannot be null or an empty array
func cleanToolParameters(parameters any) any {
	if params, ok := parameters.(map[string]any); ok {
		if required, hasRequired := params["required"]; hasRequired {
			// Remove if null or empty array
			if required == nil {
				delete(params, "required")
			} else if reqArray, ok := required.([]any); ok && len(reqArray) == 0 {
				delete(params, "required")
			}
		}

		return params
	}

	return parameters
}

// convertToolsToResponseTools converts OpenAI Tool format to Responses API format
func convertToolsToResponseTools(tools []relaymodel.Tool) []relaymodel.ResponseTool {
	responseTools := make([]relaymodel.ResponseTool, 0, len(tools))

	for _, tool := range tools {
		responseTool := relaymodel.ResponseTool{
			Type:        tool.Type,
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  cleanToolParameters(tool.Function.Parameters),
		}
		responseTools = append(responseTools, responseTool)
	}

	return responseTools
}

// convertMessagesToInputItems converts Message array to InputItem array for Responses API
func convertMessagesToInputItems(messages []relaymodel.Message) []relaymodel.InputItem {
	inputItems := make([]relaymodel.InputItem, 0, len(messages))

	for _, msg := range messages {
		// Handle tool responses (function results from tool role)
		if msg.Role == "tool" && msg.ToolCallID != "" {
			// Extract the actual content from the tool message
			var output string
			switch content := msg.Content.(type) {
			case string:
				output = content
			default:
				// Try to marshal non-string content
				if data, err := sonic.MarshalString(content); err == nil {
					output = data
				}
			}

			// Create separate InputItem for function call output
			inputItems = append(inputItems, relaymodel.InputItem{
				Type:   relaymodel.InputItemTypeFunctionCallOutput,
				CallID: msg.ToolCallID,
				Output: output,
			})

			continue
		}

		// Handle tool calls (function calls from assistant)
		if len(msg.ToolCalls) > 0 {
			// Create separate InputItems for each function call
			for _, toolCall := range msg.ToolCalls {
				inputItems = append(inputItems, relaymodel.InputItem{
					Type:      relaymodel.InputItemTypeFunctionCall,
					CallID:    toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				})
			}
			// If there's also text content in the message, add it as a separate message item
			var textContent string
			if content, ok := msg.Content.(string); ok {
				textContent = content
			}

			if textContent != "" {
				inputItems = append(inputItems, relaymodel.InputItem{
					Type: relaymodel.InputItemTypeMessage,
					Role: msg.Role,
					Content: []relaymodel.InputContent{
						{
							Type: relaymodel.InputContentTypeOutputText,
							Text: textContent,
						},
					},
				})
			}

			continue
		}

		// Handle regular messages
		role := msg.Role
		// Tool role without ToolCallID is treated as user role
		if role == "tool" {
			role = "user"
		}

		inputItem := relaymodel.InputItem{
			Type:    relaymodel.InputItemTypeMessage,
			Role:    role,
			Content: make([]relaymodel.InputContent, 0),
		}

		// Determine content type based on role
		// assistant uses 'output_text', others use 'input_text'
		contentType := relaymodel.InputContentTypeInputText
		if role == "assistant" {
			contentType = relaymodel.InputContentTypeOutputText
		}

		// Handle regular text content
		switch content := msg.Content.(type) {
		case string:
			// Simple string content
			if content != "" {
				inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
					Type: contentType,
					Text: content,
				})
			}
		case []relaymodel.MessageContent:
			// Array of MessageContent (from Claude conversion)
			for _, part := range content {
				if part.Type == relaymodel.ContentTypeText && part.Text != "" {
					inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
						Type: contentType,
						Text: part.Text,
					})
				}
			}
		case []any:
			// Array of content parts (multimodal)
			for _, part := range content {
				if partMap, ok := part.(map[string]any); ok {
					if partType, ok := partMap["type"].(string); ok && partType == "text" {
						if text, ok := partMap["text"].(string); ok {
							inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
								Type: contentType,
								Text: text,
							})
						}
					}
				}
			}
		}

		// Only append the message if it has content
		if len(inputItem.Content) > 0 {
			inputItems = append(inputItems, inputItem)
		}
	}

	return inputItems
}

// ConvertChatCompletionToResponsesRequest converts a ChatCompletion request to Responses API format
func ConvertChatCompletionToResponsesRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	// Parse ChatCompletion request
	var chatReq relaymodel.GeneralOpenAIRequest

	err := common.UnmarshalRequestReusable(req, &chatReq)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Create Responses API request
	responsesReq := relaymodel.CreateResponseRequest{
		Model:  meta.ActualModel,
		Input:  convertMessagesToInputItems(chatReq.Messages),
		Stream: chatReq.Stream,
	}

	// Map common fields
	if chatReq.Temperature != nil {
		responsesReq.Temperature = chatReq.Temperature
	}

	if chatReq.TopP != nil {
		responsesReq.TopP = chatReq.TopP
	}

	if chatReq.MaxTokens > 0 {
		responsesReq.MaxOutputTokens = &chatReq.MaxTokens
	} else if chatReq.MaxCompletionTokens > 0 {
		responsesReq.MaxOutputTokens = &chatReq.MaxCompletionTokens
	}

	// Map tools
	if len(chatReq.Tools) > 0 {
		responsesReq.Tools = convertToolsToResponseTools(chatReq.Tools)
	}

	if chatReq.ToolChoice != nil {
		responsesReq.ToolChoice = chatReq.ToolChoice
	}

	// Map user
	if chatReq.User != "" {
		responsesReq.User = &chatReq.User
	}

	// Map metadata
	if chatReq.Metadata != nil {
		if metadata, ok := chatReq.Metadata.(map[string]any); ok {
			responsesReq.Metadata = metadata
		}
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
		Input:  convertMessagesToInputItems(openAIRequest.Messages),
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
		responsesReq.Tools = convertToolsToResponseTools(openAIRequest.Tools)
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

// ConvertGeminiToResponsesRequest converts a Gemini request to Responses API format
func ConvertGeminiToResponsesRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	// Parse Gemini request
	geminiReq, err := utils.UnmarshalGeminiChatRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Convert to OpenAI messages format first
	var messages []relaymodel.Message

	// Convert system instruction
	if geminiReq.SystemInstruction != nil && len(geminiReq.SystemInstruction.Parts) > 0 {
		var systemText strings.Builder
		for _, part := range geminiReq.SystemInstruction.Parts {
			if part.Text != "" {
				systemText.WriteString(part.Text)
			}
		}

		if systemText.Len() > 0 {
			messages = append(messages, relaymodel.Message{
				Role:    "system",
				Content: systemText.String(),
			})
		}
	}

	// Convert contents
	var pendingTools []relaymodel.ToolCall
	for _, content := range geminiReq.Contents {
		msgs := convertGeminiContentToOpenAI(content, &pendingTools)
		messages = append(messages, msgs...)
	}

	// Create Responses API request
	responsesReq := relaymodel.CreateResponseRequest{
		Model:  meta.ActualModel,
		Input:  convertMessagesToInputItems(messages),
		Stream: utils.IsGeminiStreamRequest(req.URL.Path),
	}

	// Map generation config
	if geminiReq.GenerationConfig != nil {
		if geminiReq.GenerationConfig.Temperature != nil {
			responsesReq.Temperature = geminiReq.GenerationConfig.Temperature
		}

		if geminiReq.GenerationConfig.TopP != nil {
			responsesReq.TopP = geminiReq.GenerationConfig.TopP
		}

		if geminiReq.GenerationConfig.MaxOutputTokens != nil {
			responsesReq.MaxOutputTokens = geminiReq.GenerationConfig.MaxOutputTokens
		}
	}

	// Convert tools
	if len(geminiReq.Tools) > 0 {
		var tools []relaymodel.ResponseTool
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

						// Clean parameters to remove null/empty required field
						parameters = cleanToolParameters(parameters)

						tools = append(tools, relaymodel.ResponseTool{
							Type:        "function",
							Name:        name,
							Description: description,
							Parameters:  parameters,
						})
					}
				}
			}
		}

		responsesReq.Tools = tools
	}

	// Convert tool config
	if geminiReq.ToolConfig != nil {
		switch geminiReq.ToolConfig.FunctionCallingConfig.Mode {
		case "AUTO":
			responsesReq.ToolChoice = "auto"
		case "NONE":
			responsesReq.ToolChoice = "none"
		case "ANY":
			responsesReq.ToolChoice = "required"
		}
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

// ConvertResponsesToChatCompletionResponse converts Responses API response to ChatCompletion format
func ConvertResponsesToChatCompletionResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var responsesResp relaymodel.Response

	err = sonic.Unmarshal(responseBody, &responsesResp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert to ChatCompletion format
	chatResp := relaymodel.TextResponse{
		ID:      responsesResp.ID,
		Object:  "chat.completion",
		Created: responsesResp.CreatedAt,
		Model:   responsesResp.Model,
		Choices: []*relaymodel.TextResponseChoice{},
	}

	// Convert output items to choices
	for _, outputItem := range responsesResp.Output {
		choice := relaymodel.TextResponseChoice{
			Index: 0, // Responses API doesn't have index, default to 0
			Message: relaymodel.Message{
				Role:    outputItem.Role,
				Content: "",
			},
		}

		// Convert content
		var (
			contentParts []string
			toolCalls    []relaymodel.ToolCall
		)

		for _, content := range outputItem.Content {
			if (content.Type == "text" || content.Type == "output_text") && content.Text != "" {
				contentParts = append(contentParts, content.Text)
			}
			// Add tool call conversion if needed in the future
		}

		if len(contentParts) > 0 {
			choice.Message.Content = strings.Join(contentParts, "\n")
		}

		if len(toolCalls) > 0 {
			choice.Message.ToolCalls = toolCalls
		}

		// Set finish reason based on status
		switch responsesResp.Status {
		case relaymodel.ResponseStatusCompleted:
			choice.FinishReason = relaymodel.FinishReasonStop
		case relaymodel.ResponseStatusIncomplete:
			choice.FinishReason = relaymodel.FinishReasonLength
		case relaymodel.ResponseStatusFailed:
			choice.FinishReason = relaymodel.FinishReasonStop
		}

		chatResp.Choices = append(chatResp.Choices, &choice)
	}

	// Convert usage
	if responsesResp.Usage != nil {
		chatResp.Usage = responsesResp.Usage.ToChatUsage()
	}

	// Marshal and return
	chatRespData, err := sonic.Marshal(chatResp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(chatRespData)))
	_, _ = c.Writer.Write(chatRespData)

	if responsesResp.Usage != nil {
		return responsesResp.Usage.ToModelUsage(), nil
	}

	return model.Usage{}, nil
}

// ConvertResponsesToChatCompletionStreamResponse converts Responses API stream to ChatCompletion stream
func ConvertResponsesToChatCompletionStreamResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)
	scanner := bufio.NewScanner(resp.Body)

	buf := utils.GetScannerBuffer()
	defer utils.PutScannerBuffer(buf)

	scanner.Buffer(*buf, cap(*buf))

	var usage model.Usage

	state := &chatCompletionStreamState{
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

		// Handle event and get response
		var chatStreamResp *relaymodel.ChatCompletionsStreamResponse

		switch event.Type {
		case relaymodel.EventResponseCreated:
			chatStreamResp = state.handleResponseCreated(&event)
		case relaymodel.EventOutputTextDelta:
			chatStreamResp = state.handleOutputTextDelta(&event)
		case relaymodel.EventOutputItemAdded:
			chatStreamResp = state.handleOutputItemAdded(&event)
		case relaymodel.EventFunctionCallArgumentsDelta:
			chatStreamResp = state.handleFunctionCallArgumentsDelta(&event)
		case relaymodel.EventOutputItemDone:
			chatStreamResp = state.handleOutputItemDone(&event)
		case relaymodel.EventResponseCompleted, relaymodel.EventResponseDone:
			if event.Response != nil && event.Response.Usage != nil {
				usage = event.Response.Usage.ToModelUsage()
			}

			chatStreamResp = state.handleResponseCompleted(&event)
		}

		// Send the converted chunk
		if chatStreamResp != nil {
			chunkData, err := sonic.Marshal(chatStreamResp)
			if err != nil {
				log.Error("error marshalling chat stream response: " + err.Error())
				continue
			}

			render.OpenaiBytesData(c, chunkData)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading response stream: " + err.Error())
	}

	return usage, nil
}

// ConvertResponsesToClaudeResponse converts Responses API response to Claude format
func ConvertResponsesToClaudeResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var responsesResp relaymodel.Response

	err = sonic.Unmarshal(responseBody, &responsesResp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert to Claude format
	claudeResp := relaymodel.ClaudeResponse{
		ID:      responsesResp.ID,
		Type:    "message",
		Role:    "assistant",
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
				if (content.Type == "text" || content.Type == "output_text") && content.Text != "" {
					claudeResp.Content = append(claudeResp.Content, relaymodel.ClaudeContent{
						Type:     "thinking",
						Thinking: content.Text,
					})
				}
			}
		default:
			// Handle regular message content
			for _, content := range outputItem.Content {
				if (content.Type == "text" || content.Type == "output_text") && content.Text != "" {
					claudeResp.Content = append(claudeResp.Content, relaymodel.ClaudeContent{
						Type: "text",
						Text: content.Text,
					})
				}
			}
		}
	}

	// Set stop reason based on status
	switch responsesResp.Status {
	case relaymodel.ResponseStatusCompleted:
		claudeResp.StopReason = "end_turn"
	case relaymodel.ResponseStatusIncomplete:
		claudeResp.StopReason = "max_tokens"
	default:
		claudeResp.StopReason = "end_turn"
	}

	// Convert usage
	if responsesResp.Usage != nil {
		claudeResp.Usage = responsesResp.Usage.ToClaudeUsage()
	}

	// Marshal and return
	claudeRespData, err := sonic.Marshal(claudeResp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(claudeRespData)))
	_, _ = c.Writer.Write(claudeRespData)

	if responsesResp.Usage != nil {
		return responsesResp.Usage.ToModelUsage(), nil
	}

	return model.Usage{}, nil
}

// ConvertResponsesToClaudeStreamResponse converts Responses API stream to Claude stream
func ConvertResponsesToClaudeStreamResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)
	scanner := bufio.NewScanner(resp.Body)

	buf := utils.GetScannerBuffer()
	defer utils.PutScannerBuffer(buf)

	scanner.Buffer(*buf, cap(*buf))

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

	return usage, nil
}

// ConvertResponsesToGeminiResponse converts Responses API response to Gemini format
func ConvertResponsesToGeminiResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var responsesResp relaymodel.Response

	err = sonic.Unmarshal(responseBody, &responsesResp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert to Gemini format
	geminiResp := relaymodel.GeminiChatResponse{
		ModelVersion: responsesResp.Model,
		Candidates:   []*relaymodel.GeminiChatCandidate{},
	}

	// Convert output items to Gemini candidates
	for _, outputItem := range responsesResp.Output {
		candidate := &relaymodel.GeminiChatCandidate{
			Index: 0,
			Content: relaymodel.GeminiChatContent{
				Role:  "model",
				Parts: []*relaymodel.GeminiPart{},
			},
		}

		// Handle different output types
		switch outputItem.Type {
		case "reasoning":
			// Convert reasoning to thought parts
			for _, content := range outputItem.Content {
				if (content.Type == "text" || content.Type == "output_text") && content.Text != "" {
					candidate.Content.Parts = append(
						candidate.Content.Parts,
						&relaymodel.GeminiPart{
							Text:    content.Text,
							Thought: true,
						},
					)
				}
			}

		case "function_call":
			// Handle function_call type
			if outputItem.Name != "" {
				var args map[string]any
				if outputItem.Arguments != "" {
					err := sonic.Unmarshal([]byte(outputItem.Arguments), &args)
					if err == nil {
						candidate.Content.Parts = append(
							candidate.Content.Parts,
							&relaymodel.GeminiPart{
								FunctionCall: &relaymodel.GeminiFunctionCall{
									Name: outputItem.Name,
									Args: args,
								},
							},
						)
					}
				}
			}

		default:
			// Handle message type with text content
			for _, content := range outputItem.Content {
				if (content.Type == "text" || content.Type == "output_text") && content.Text != "" {
					candidate.Content.Parts = append(
						candidate.Content.Parts,
						&relaymodel.GeminiPart{
							Text: content.Text,
						},
					)
				}
			}
		}

		// Only add candidate if it has content
		if len(candidate.Content.Parts) > 0 {
			// Set finish reason
			switch responsesResp.Status {
			case relaymodel.ResponseStatusCompleted:
				candidate.FinishReason = "STOP"
			case relaymodel.ResponseStatusIncomplete:
				candidate.FinishReason = "MAX_TOKENS"
			default:
				candidate.FinishReason = "STOP"
			}

			geminiResp.Candidates = append(geminiResp.Candidates, candidate)
		}
	}

	usage := model.Usage{}

	// Convert usage
	if responsesResp.Usage != nil {
		usage = responsesResp.Usage.ToModelUsage()
		geminiUsage := responsesResp.Usage.ToGeminiUsage()
		geminiResp.UsageMetadata = &geminiUsage
	}

	// Marshal and return
	geminiRespData, err := sonic.Marshal(geminiResp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(geminiRespData)))
	_, _ = c.Writer.Write(geminiRespData)

	return usage, nil
}

// ConvertResponsesToGeminiStreamResponse converts Responses API stream to Gemini stream
func ConvertResponsesToGeminiStreamResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)
	scanner := bufio.NewScanner(resp.Body)

	buf := utils.GetScannerBuffer()
	defer utils.PutScannerBuffer(buf)

	scanner.Buffer(*buf, cap(*buf))

	var usage model.Usage

	state := &geminiStreamState{
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
		// Note: Gemini format requires complete JSON for function calls,
		// so we handle function_call_arguments.done (complete), not function_call_arguments.delta (streaming)
		switch event.Type {
		case relaymodel.EventOutputItemAdded:
			state.handleOutputItemAdded(&event)
		case relaymodel.EventOutputTextDelta:
			state.handleOutputTextDelta(&event)
		case relaymodel.EventFunctionCallArgumentsDone:
			state.handleFunctionCallArgumentsDone(&event)
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

	return usage, nil
}
