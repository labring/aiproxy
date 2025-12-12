package openai

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
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
		Object:  relaymodel.ChatCompletionChunkObject,
		Created: event.Response.CreatedAt,
		Model:   event.Response.Model,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					Role: relaymodel.RoleAssistant,
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
		Object:  relaymodel.ChatCompletionChunkObject,
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
	if event.Item.Type == relaymodel.InputItemTypeFunctionCall {
		s.currentToolCallID = event.Item.ID
		s.currentToolCall = &relaymodel.ToolCall{
			ID:   event.Item.CallID,
			Type: relaymodel.ToolChoiceTypeFunction,
			Function: relaymodel.Function{
				Name:      event.Item.Name,
				Arguments: "",
			},
		}
		s.toolCallArgs = ""

		// Send tool call start
		return &relaymodel.ChatCompletionsStreamResponse{
			ID:      s.messageID,
			Object:  relaymodel.ChatCompletionChunkObject,
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
								Type:  relaymodel.ToolChoiceTypeFunction,
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

	if event.Item.Type == relaymodel.InputItemTypeMessage {
		return &relaymodel.ChatCompletionsStreamResponse{
			ID:      s.messageID,
			Object:  relaymodel.ChatCompletionChunkObject,
			Created: time.Now().Unix(),
			Model:   s.meta.ActualModel,
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: relaymodel.Message{
						Role: relaymodel.RoleAssistant,
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
		Object:  relaymodel.ChatCompletionChunkObject,
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
	if event.Item.Type == relaymodel.InputItemTypeFunctionCall && s.currentToolCall != nil &&
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
					Object:  relaymodel.ChatCompletionChunkObject,
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
		Object:  relaymodel.ChatCompletionChunkObject,
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

func ConvertCompletionsRequest(
	meta *meta.Meta,
	req *http.Request,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

		if err := callback(&node); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	jsonData, err := node.MarshalJSON()
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

func ConvertChatCompletionsRequest(
	meta *meta.Meta,
	req *http.Request,
	doNotPatchStreamOptionsIncludeUsage bool,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Clean tool parameters (remove null/empty required fields)
	// This should be done before other callbacks to ensure consistency
	if err := CleanToolParametersFromNode(&node); err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

		if err := callback(&node); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	if !doNotPatchStreamOptionsIncludeUsage {
		if err := patchStreamOptions(&node); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	jsonData, err := node.MarshalJSON()
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

func patchStreamOptions(node *ast.Node) error {
	streamNode := node.Get("stream")
	if !streamNode.Exists() {
		return nil
	}

	streamBool, err := streamNode.Bool()
	if err != nil {
		return errors.New("stream is not a boolean")
	}

	if !streamBool {
		return nil
	}

	streamOptionsNode := node.Get("stream_options")
	if !streamOptionsNode.Exists() {
		_, err = node.SetAny("stream_options", map[string]any{
			"include_usage": true,
		})
		return err
	}

	if streamOptionsNode.TypeSafe() != ast.V_OBJECT {
		return errors.New("stream_options is not an object")
	}

	_, err = streamOptionsNode.Set("include_usage", ast.NewBool(true))

	return err
}

func GetUsageOrChatChoicesResponseFromNode(
	node *ast.Node,
) (*relaymodel.ChatUsage, []*relaymodel.ChatCompletionsStreamResponseChoice, error) {
	usageNode := node.Get("usage")
	if usageNode != nil && usageNode.TypeSafe() != ast.V_NULL {
		usageRaw, err := usageNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			var usage relaymodel.ChatUsage

			err = sonic.UnmarshalString(usageRaw, &usage)
			if err != nil {
				return nil, nil, err
			}

			return &usage, nil, nil
		}
	}

	var choices []*relaymodel.ChatCompletionsStreamResponseChoice

	choicesNode := node.Get("choices")
	if choicesNode != nil && choicesNode.TypeSafe() != ast.V_NULL {
		choicesRaw, err := choicesNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			err = sonic.UnmarshalString(choicesRaw, &choices)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return nil, choices, nil
}

type PreHandler func(meta *meta.Meta, node *ast.Node) error

func StreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
	preHandler PreHandler,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	responseText := strings.Builder{}

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

	var usage relaymodel.ChatUsage

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		node, err := sonic.Get(data)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		if preHandler != nil {
			err := preHandler(meta, &node)
			if err != nil {
				log.Error("error pre handler: " + err.Error())
				continue
			}
		}

		u, ch, err := GetUsageOrChatChoicesResponseFromNode(&node)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		if u != nil {
			usage = *u

			responseText.Reset()
		}

		for _, choice := range ch {
			if usage.TotalTokens == 0 {
				if choice.Text != "" {
					responseText.WriteString(choice.Text)
				} else {
					responseText.WriteString(choice.Delta.StringContent())
				}
			}
		}

		_, err = node.Set("model", ast.NewString(meta.OriginModel))
		if err != nil {
			log.Error("error set model: " + err.Error())
		}

		_ = render.OpenaiObjectData(c, &node)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	if usage.TotalTokens == 0 && responseText.Len() > 0 {
		usage = ResponseText2Usage(
			responseText.String(),
			meta.ActualModel,
			int64(meta.RequestUsage.InputTokens),
		)
		_ = render.OpenaiObjectData(c, &relaymodel.ChatCompletionsStreamResponse{
			ID:      ChatCompletionID(),
			Model:   meta.OriginModel,
			Object:  relaymodel.ChatCompletionChunkObject,
			Created: time.Now().Unix(),
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{},
			Usage:   &usage,
		})
	} else if usage.TotalTokens != 0 && usage.PromptTokens == 0 { // some channels don't return prompt tokens & completion tokens
		usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
		usage.CompletionTokens = usage.TotalTokens - int64(meta.RequestUsage.InputTokens)
	}

	render.OpenaiDone(c)

	return usage.ToModelUsage(), nil
}

func GetUsageOrChoicesResponseFromNode(
	node *ast.Node,
) (*relaymodel.ChatUsage, []*relaymodel.TextResponseChoice, error) {
	usageNode := node.Get("usage")
	if usageNode != nil && usageNode.TypeSafe() != ast.V_NULL {
		usageRaw, err := usageNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			var usage relaymodel.ChatUsage

			err = sonic.UnmarshalString(usageRaw, &usage)
			if err != nil {
				return nil, nil, err
			}

			return &usage, nil, nil
		}
	}

	var choices []*relaymodel.TextResponseChoice

	choicesNode := node.Get("choices")
	if choicesNode != nil && choicesNode.TypeSafe() != ast.V_NULL {
		choicesRaw, err := choicesNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			err = sonic.UnmarshalString(choicesRaw, &choices)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return nil, choices, nil
}

func Handler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
	preHandler PreHandler,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	node, err := common.UnmarshalResponse2Node(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if preHandler != nil {
		err := preHandler(meta, &node)
		if err != nil {
			return model.Usage{}, relaymodel.WrapperOpenAIError(
				err,
				"pre_handler_failed",
				http.StatusInternalServerError,
			)
		}
	}

	usage, choices, err := GetUsageOrChoicesResponseFromNode(&node)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if usage == nil ||
		usage.TotalTokens == 0 ||
		(usage.PromptTokens == 0 && usage.CompletionTokens == 0) {
		var completionTokens int64
		for _, choice := range choices {
			if choice.Text != "" {
				completionTokens += CountTokenText(choice.Text, meta.ActualModel)
				continue
			}

			completionTokens += CountTokenText(choice.Message.StringContent(), meta.ActualModel)
		}

		usage = &relaymodel.ChatUsage{
			PromptTokens:     int64(meta.RequestUsage.InputTokens),
			CompletionTokens: completionTokens,
			TotalTokens:      int64(meta.RequestUsage.InputTokens) + completionTokens,
		}

		_, err = node.Set("usage", ast.NewAny(usage))
		if err != nil {
			return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
				err,
				"set_usage_failed",
				http.StatusInternalServerError,
			)
		}
	} else if usage.TotalTokens != 0 && usage.PromptTokens == 0 { // some channels don't return prompt tokens & completion tokens
		usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
		usage.CompletionTokens = usage.TotalTokens - int64(meta.RequestUsage.InputTokens)

		_, err = node.Set("usage", ast.NewAny(usage))
		if err != nil {
			return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(err, "set_usage_failed", http.StatusInternalServerError)
		}
	}

	_, err = node.Set("model", ast.NewString(meta.OriginModel))
	if err != nil {
		return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"set_model_failed",
			http.StatusInternalServerError,
		)
	}

	newData, err := sonic.Marshal(&node)
	if err != nil {
		return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(newData)))

	_, err = c.Writer.Write(newData)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return usage.ToModelUsage(), nil
}

// CleanToolParameters removes null or empty required field from tool parameters
// Responses API requires the 'required' field to be either:
// - A non-empty array of strings
// - Completely absent from the schema
// It cannot be null or an empty array
func CleanToolParameters(parameters any) any {
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

// CleanToolParametersFromNode cleans tool parameters in an AST node
// It removes null or empty required fields from tool function parameters
func CleanToolParametersFromNode(node *ast.Node) error {
	toolsNode := node.Get("tools")
	if !toolsNode.Exists() || toolsNode.TypeSafe() == ast.V_NULL {
		return nil
	}

	if toolsNode.TypeSafe() != ast.V_ARRAY {
		return nil
	}

	// Iterate through each tool using ForEach
	err := toolsNode.ForEach(func(path ast.Sequence, toolNode *ast.Node) bool {
		// Get function node
		functionNode := toolNode.Get("function")
		if !functionNode.Exists() {
			return true // Continue to next tool
		}

		// Get parameters node
		parametersNode := functionNode.Get("parameters")
		if !parametersNode.Exists() || parametersNode.TypeSafe() == ast.V_NULL {
			return true // Continue to next tool
		}

		// Get required node
		requiredNode := parametersNode.Get("required")
		if !requiredNode.Exists() {
			return true // Continue to next tool
		}

		// Check if required is null or empty array
		shouldRemove := false
		if requiredNode.TypeSafe() == ast.V_NULL {
			shouldRemove = true
		} else if requiredNode.TypeSafe() == ast.V_ARRAY {
			requiredArray, err := requiredNode.ArrayUseNode()
			if err == nil && len(requiredArray) == 0 {
				shouldRemove = true
			}
		}

		// Remove required field if needed
		if shouldRemove {
			// Use Unset to directly remove the required field
			_, _ = parametersNode.Unset("required")
		}

		return true // Continue to next tool
	})

	return err
}

// ConvertToolsToResponseTools converts OpenAI Tool format to Responses API format
func ConvertToolsToResponseTools(tools []relaymodel.Tool) []relaymodel.ResponseTool {
	responseTools := make([]relaymodel.ResponseTool, 0, len(tools))

	for _, tool := range tools {
		responseTool := relaymodel.ResponseTool{
			Type:        tool.Type,
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  CleanToolParameters(tool.Function.Parameters),
		}
		responseTools = append(responseTools, responseTool)
	}

	return responseTools
}

// ConvertMessagesToInputItems converts Message array to InputItem array for Responses API
func ConvertMessagesToInputItems(messages []relaymodel.Message) []relaymodel.InputItem {
	inputItems := make([]relaymodel.InputItem, 0, len(messages))

	for _, msg := range messages {
		// Handle tool responses (function results from tool role)
		if msg.Role == relaymodel.RoleTool && msg.ToolCallID != "" {
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
		if role == relaymodel.RoleTool {
			role = relaymodel.RoleUser
		}

		inputItem := relaymodel.InputItem{
			Type:    relaymodel.InputItemTypeMessage,
			Role:    role,
			Content: make([]relaymodel.InputContent, 0),
		}

		// Determine content type based on role
		// assistant uses 'output_text', others use 'input_text'
		contentType := relaymodel.InputContentTypeInputText
		if role == relaymodel.RoleAssistant {
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
		Input:  ConvertMessagesToInputItems(chatReq.Messages),
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
		responsesReq.Tools = ConvertToolsToResponseTools(chatReq.Tools)
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
		Object:  relaymodel.ChatCompletionObject,
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
	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

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
