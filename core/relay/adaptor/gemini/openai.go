package gemini

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

var toolChoiceTypeMap = map[string]string{
	relaymodel.ToolChoiceNone:     relaymodel.GeminiFunctionCallingModeNone,
	relaymodel.ToolChoiceAuto:     relaymodel.GeminiFunctionCallingModeAuto,
	relaymodel.ToolChoiceRequired: relaymodel.GeminiFunctionCallingModeAny,
}

var mimeTypeMap = map[string]string{
	"json_object": "application/json",
	"text":        "text/plain",
}

type CountTokensResponse struct {
	Error       *relaymodel.GeminiError `json:"error,omitempty"`
	TotalTokens int                     `json:"totalTokens"`
}

func buildSafetySettings(safetySetting string) []relaymodel.GeminiChatSafetySettings {
	if safetySetting == "" {
		safetySetting = relaymodel.GeminiSafetyThresholdBlockNone
	}

	return []relaymodel.GeminiChatSafetySettings{
		{Category: relaymodel.GeminiSafetyCategoryHarassment, Threshold: safetySetting},
		{Category: relaymodel.GeminiSafetyCategoryHateSpeech, Threshold: safetySetting},
		{Category: relaymodel.GeminiSafetyCategorySexuallyExplicit, Threshold: safetySetting},
		{Category: relaymodel.GeminiSafetyCategoryDangerousContent, Threshold: safetySetting},
		{Category: relaymodel.GeminiSafetyCategoryCivicIntegrity, Threshold: safetySetting},
	}
}

func buildGenerationConfig(
	meta *meta.Meta,
	httpReq *http.Request,
	req *relaymodel.GeneralOpenAIRequest,
	textRequest *relaymodel.GeneralOpenAIRequest,
) *relaymodel.GeminiChatGenerationConfig {
	// First unmarshal generationConfig from request body if present
	var reqWithConfig struct {
		GenerationConfig *relaymodel.GeminiChatGenerationConfig `json:"generationConfig,omitempty"`
	}

	_ = common.UnmarshalRequestReusable(httpReq, &reqWithConfig)

	var config relaymodel.GeminiChatGenerationConfig
	if reqWithConfig.GenerationConfig != nil {
		config = *reqWithConfig.GenerationConfig
	}

	// Override with OpenAI-style parameters if provided
	if config.Temperature != nil && textRequest.Temperature != nil {
		config.Temperature = textRequest.Temperature
	}

	if config.TopP != nil && textRequest.TopP != nil {
		config.TopP = textRequest.TopP
	}

	// Convert MaxTokens (int) to MaxOutputTokens (*int)
	if config.MaxOutputTokens == nil && textRequest.MaxTokens != 0 {
		config.MaxOutputTokens = &textRequest.MaxTokens
	}

	if len(config.ResponseModalities) == 0 &&
		strings.Contains(meta.ActualModel, "image") {
		config.ResponseModalities = []string{
			"Text",
			"Image",
		}
	}

	if config.ResponseMimeType == "" && textRequest.ResponseFormat != nil {
		if mimeType, ok := mimeTypeMap[textRequest.ResponseFormat.Type]; ok {
			config.ResponseMimeType = mimeType
		}

		if textRequest.ResponseFormat.JSONSchema != nil {
			config.ResponseSchema = textRequest.ResponseFormat.JSONSchema.Schema
			cleanJSONSchema(config.ResponseSchema)
			config.ResponseMimeType = mimeTypeMap["json_object"]
		}
	}

	if config.ThinkingConfig == nil && req.Thinking != nil {
		thinkingConfig := relaymodel.GeminiThinkingConfig{}
		switch req.Thinking.Type {
		case relaymodel.ClaudeThinkingTypeEnabled:
			thinkingConfig.IncludeThoughts = true
			thinkingConfig.ThinkingBudget = req.Thinking.BudgetTokens
		case relaymodel.ClaudeThinkingTypeDisabled:
			thinkingConfig.IncludeThoughts = false
		}

		config.ThinkingConfig = &thinkingConfig
	}

	// https://ai.google.dev/gemini-api/docs/thinking
	if config.ThinkingConfig == nil &&
		(strings.Contains(meta.ActualModel, "-2.5") ||
			strings.Contains(meta.ActualModel, "-3")) &&
		// disable vertexai image model include thoughts
		// because error call gemini-3-pro-image-preview model
		(meta.Channel.Type == model.ChannelTypeVertexAI && !strings.Contains(meta.ActualModel, "image")) {
		config.ThinkingConfig = &relaymodel.GeminiThinkingConfig{
			IncludeThoughts: true,
		}
	}

	return &config
}

func buildTools(textRequest *relaymodel.GeneralOpenAIRequest) []relaymodel.GeminiChatTools {
	if textRequest.Tools != nil {
		functions := make([]relaymodel.Function, 0, len(textRequest.Tools))
		for _, tool := range textRequest.Tools {
			cleanedFunction := cleanFunctionParameters(tool.Function)
			functions = append(functions, cleanedFunction)
		}

		return []relaymodel.GeminiChatTools{{FunctionDeclarations: functions}}
	}

	if textRequest.Functions != nil {
		return []relaymodel.GeminiChatTools{{FunctionDeclarations: textRequest.Functions}}
	}

	return nil
}

func cleanFunctionParameters(function relaymodel.Function) relaymodel.Function {
	if function.Parameters == nil {
		return function
	}

	parameters, ok := function.Parameters.(map[string]any)
	if !ok {
		return function
	}

	cleanJSONSchema(parameters)

	if properties, ok := parameters["properties"].(map[string]any); ok {
		if len(properties) == 0 {
			function.Parameters = nil
			return function
		}
	}

	function.Parameters = parameters

	return function
}

var unsupportedFields = []string{
	"additionalProperties",
	"$schema",
	"$id",
	"$ref",
	"$defs",
	"exclusiveMinimum",
	"exclusiveMaximum",
}

var supportedFormats = map[string]struct{}{
	"enum":      {},
	"date-time": {},
}

func cleanJSONSchema(schema map[string]any) {
	for _, field := range unsupportedFields {
		delete(schema, field)
	}

	if format, exists := schema["format"]; exists {
		if formatStr, ok := format.(string); ok {
			if _, ok := supportedFormats[formatStr]; !ok {
				delete(schema, "format")
			}
		}
	}

	for _, field := range schema {
		switch v := field.(type) {
		case map[string]any:
			cleanJSONSchema(v)
		case []any:
			for _, item := range v {
				if itemMap, ok := item.(map[string]any); ok {
					cleanJSONSchema(itemMap)
				}
			}
		}
	}
}

func buildToolConfig(textRequest *relaymodel.GeneralOpenAIRequest) *relaymodel.GeminiToolConfig {
	if textRequest.ToolChoice == nil {
		return nil
	}

	defaultMode := relaymodel.GeminiFunctionCallingModeAuto
	if strings.Contains(textRequest.Model, "gemini-3") {
		defaultMode = ""
	}

	toolConfig := relaymodel.GeminiToolConfig{
		FunctionCallingConfig: relaymodel.GeminiFunctionCallingConfig{
			Mode: defaultMode,
		},
	}
	switch mode := textRequest.ToolChoice.(type) {
	case string:
		if toolChoiceType, ok := toolChoiceTypeMap[mode]; ok {
			toolConfig.FunctionCallingConfig.Mode = toolChoiceType
		}
	case map[string]any:
		toolConfig.FunctionCallingConfig.Mode = relaymodel.GeminiFunctionCallingModeAny
		if fn, ok := mode["function"].(map[string]any); ok {
			if fnName, ok := fn["name"].(string); ok {
				toolConfig.FunctionCallingConfig.AllowedFunctionNames = []string{fnName}
			}
		}
	}

	return &toolConfig
}

func buildMessageParts(message relaymodel.MessageContent) *relaymodel.GeminiPart {
	part := &relaymodel.GeminiPart{
		Text: message.Text,
	}
	if message.ImageURL != nil {
		part.InlineData = &relaymodel.GeminiInlineData{
			Data: message.ImageURL.URL,
		}
	}

	return part
}

// Add this helper function to track tool calls
func buildContents(
	textRequest *relaymodel.GeneralOpenAIRequest,
) (*relaymodel.GeminiChatContent, []*relaymodel.GeminiChatContent, []*relaymodel.GeminiPart) {
	contents := make([]*relaymodel.GeminiChatContent, 0, len(textRequest.Messages))

	var (
		imageTasks    []*relaymodel.GeminiPart
		systemContent *relaymodel.GeminiChatContent
	)

	// Track tool calls by ID to get their names for tool results
	toolCallMap := make(map[string]string) // tool_call_id -> tool_name

	for _, message := range textRequest.Messages {
		content := relaymodel.GeminiChatContent{
			Role: message.Role,
		}

		// Track tool calls from assistant messages
		switch {
		case message.Role == relaymodel.RoleAssistant && len(message.ToolCalls) > 0:
			for _, toolCall := range message.ToolCalls {
				toolCallMap[toolCall.ID] = toolCall.Function.Name

				var args map[string]any
				if toolCall.Function.Arguments != "" {
					if err := sonic.UnmarshalString(
						toolCall.Function.Arguments,
						&args,
					); err != nil {
						args = make(map[string]any)
					}
				} else {
					args = make(map[string]any)
				}

				part := &relaymodel.GeminiPart{
					FunctionCall: &relaymodel.GeminiFunctionCall{
						Name: toolCall.Function.Name,
						Args: args,
					},
				}

				// Restore Gemini thought signature if present in extra_content (OpenAI format)
				if toolCall.ExtraContent != nil &&
					toolCall.ExtraContent.Google != nil &&
					toolCall.ExtraContent.Google.ThoughtSignature != "" {
					part.ThoughtSignature = toolCall.ExtraContent.Google.ThoughtSignature
				} else {
					// If thought signature is missing (e.g., from non-Gemini sources or clients that don't preserve it),
					// use a dummy signature to skip Gemini's validation as per their FAQ:
					// https://ai.google.dev/gemini-api/docs/thought-signatures#faqs
					part.ThoughtSignature = ThoughtSignatureDummySkipValidator
				}

				content.Parts = append(content.Parts, part)
			}
		case message.Role == "tool" && message.ToolCallID != "":
			// Handle tool results - get the tool name from our map
			toolName := toolCallMap[message.ToolCallID]
			if toolName == "" {
				// Fallback: try to get from message.Name if available
				if message.Name != nil {
					toolName = *message.Name
				} else {
					// If still no name, use a default or the tool ID
					toolName = "tool_" + message.ToolCallID
				}
			}

			var contentMap map[string]any
			if message.Content != nil {
				switch content := message.Content.(type) {
				case map[string]any:
					contentMap = content
				case string:
					if err := sonic.UnmarshalString(content, &contentMap); err != nil {
						contentMap = map[string]any{"result": content}
					}
				}
			} else {
				contentMap = make(map[string]any)
			}

			content.Parts = append(content.Parts, &relaymodel.GeminiPart{
				FunctionResponse: &relaymodel.GeminiFunctionResponse{
					Name: toolName, // Now properly set
					Response: map[string]any{
						"name":    toolName, // Now properly set
						"content": contentMap,
					},
				},
			})
		case message.Role == relaymodel.RoleSystem:
			systemContent = &relaymodel.GeminiChatContent{
				Role: relaymodel.RoleUser, // Gemini uses "user" for system content
				Parts: []*relaymodel.GeminiPart{{
					Text: message.StringContent(),
				}},
			}

			continue
		default:
			// Handle regular messages
			openaiContent := message.ParseContent()
			if len(openaiContent) == 0 {
				continue
			}

			for _, part := range openaiContent {
				msgPart := buildMessageParts(part)

				if msgPart.Text == "" && msgPart.InlineData == nil {
					continue
				}

				if msgPart.InlineData != nil {
					imageTasks = append(imageTasks, msgPart)
				}

				content.Parts = append(content.Parts, msgPart)
			}
		}

		// Adjust role for Gemini
		switch content.Role {
		case relaymodel.RoleAssistant:
			content.Role = relaymodel.GeminiRoleModel
		case "tool":
			content.Role = relaymodel.GeminiRoleUser
		}

		if len(content.Parts) > 0 {
			contents = append(contents, &content)
		}
	}

	// Merge consecutive messages with the same role to avoid Gemini API errors
	// Gemini expects alternating user/model messages, but we might receive multiple
	// consecutive user messages (e.g., multiple tool results)
	mergedContents := make([]*relaymodel.GeminiChatContent, 0, len(contents))
	for i, content := range contents {
		if i > 0 && mergedContents[len(mergedContents)-1].Role == content.Role {
			// Merge with previous message of the same role
			mergedContents[len(mergedContents)-1].Parts = append(
				mergedContents[len(mergedContents)-1].Parts,
				content.Parts...,
			)
		} else {
			mergedContents = append(mergedContents, content)
		}
	}

	return systemContent, mergedContents, imageTasks
}

func processImageTasks(ctx context.Context, imageTasks []*relaymodel.GeminiPart) error {
	if len(imageTasks) == 0 {
		return nil
	}

	sem := semaphore.NewWeighted(3)

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		processErrs []error
	)

	for _, task := range imageTasks {
		if task.InlineData == nil || task.InlineData.Data == "" {
			continue
		}

		wg.Go(func() {
			_ = sem.Acquire(ctx, 1)
			defer sem.Release(1)

			mimeType, data, err := image.GetImageFromURL(ctx, task.InlineData.Data)
			if err != nil {
				mu.Lock()

				processErrs = append(processErrs, err)

				mu.Unlock()

				return
			}

			task.InlineData.MimeType = mimeType
			task.InlineData.Data = data
		})
	}

	wg.Wait()

	if len(processErrs) != 0 {
		return errors.Join(processErrs...)
	}

	return nil
}

// Setting safety to the lowest possible values since Gemini is already powerless enough
func ConvertRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	adaptorConfig := Config{}

	err := meta.ChannelConfigs.LoadConfig(&adaptorConfig)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	textRequest, err := utils.UnmarshalGeneralOpenAIRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	textRequest.Model = meta.ActualModel
	meta.Set("stream", textRequest.Stream)

	systemContent, contents, imageTasks := buildContents(textRequest)

	// Process image tasks concurrently
	if len(imageTasks) > 0 {
		if err := processImageTasks(req.Context(), imageTasks); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	config := buildGenerationConfig(meta, req, textRequest, textRequest)

	// Build actual request
	geminiRequest := relaymodel.GeminiChatRequest{
		Contents:          contents,
		SystemInstruction: systemContent,
		SafetySettings:    buildSafetySettings(adaptorConfig.Safety),
		GenerationConfig:  config,
		Tools:             buildTools(textRequest),
		ToolConfig:        buildToolConfig(textRequest),
	}

	data, err := sonic.Marshal(geminiRequest)
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

// Type aliases for usage-related types to use unified definitions from relaymodel
var finishReason2OpenAI = map[string]string{
	relaymodel.GeminiFinishReasonStop:      relaymodel.FinishReasonStop,
	relaymodel.GeminiFinishReasonMaxTokens: relaymodel.FinishReasonLength,
}

func FinishReason2OpenAI(reason string) string {
	if openaiReason, ok := finishReason2OpenAI[reason]; ok {
		return openaiReason
	}
	return reason
}

func getToolCall(item *relaymodel.GeminiPart) (*relaymodel.ToolCall, error) {
	if item.FunctionCall == nil {
		return nil, nil
	}

	argsBytes, err := sonic.Marshal(item.FunctionCall.Args)
	if err != nil {
		return nil, err
	}

	toolCall := relaymodel.ToolCall{
		ID:   openai.CallID(),
		Type: "function",
		Function: relaymodel.Function{
			Arguments: conv.BytesToString(argsBytes),
			Name:      item.FunctionCall.Name,
		},
	}

	// Preserve Gemini thought signature if present (OpenAI format)
	if item.ThoughtSignature != "" {
		toolCall.ExtraContent = &relaymodel.ExtraContent{
			Google: &relaymodel.GoogleExtraContent{
				ThoughtSignature: item.ThoughtSignature,
			},
		}
	}

	return &toolCall, nil
}

func responseChat2OpenAI(
	meta *meta.Meta,
	response *relaymodel.GeminiChatResponse,
) *relaymodel.TextResponse {
	fullTextResponse := relaymodel.TextResponse{
		ID:      openai.ChatCompletionID(),
		Model:   meta.OriginModel,
		Object:  relaymodel.ChatCompletionObject,
		Created: time.Now().Unix(),
		Choices: make([]*relaymodel.TextResponseChoice, 0, len(response.Candidates)),
	}
	if response.UsageMetadata != nil {
		fullTextResponse.Usage = response.UsageMetadata.ToUsage()
	}

	for i, candidate := range response.Candidates {
		choice := relaymodel.TextResponseChoice{
			Index: i,
			Message: relaymodel.Message{
				Role: relaymodel.RoleAssistant,
			},
			FinishReason: FinishReason2OpenAI(candidate.FinishReason),
		}
		if len(candidate.Content.Parts) > 0 {
			var (
				contents         []relaymodel.MessageContent
				reasoningContent strings.Builder
				builder          strings.Builder
			)

			hasImage := false
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil {
					hasImage = true
					break
				}
			}

			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					toolCall, err := getToolCall(part)
					if err != nil {
						log.Error("get tool call failed: " + err.Error())
					}

					if toolCall != nil {
						choice.Message.ToolCalls = append(choice.Message.ToolCalls, *toolCall)
					}
				}

				if part.Text != "" {
					if hasImage {
						if part.Thought {
							reasoningContent.WriteString(part.Text)

							if part.ThoughtSignature != "" {
								choice.Message.Signature = part.ThoughtSignature
							}
						} else {
							contents = append(contents, relaymodel.MessageContent{
								Type: relaymodel.ContentTypeText,
								Text: part.Text,
							})
						}
					} else {
						if part.Thought {
							reasoningContent.WriteString(part.Text)

							if part.ThoughtSignature != "" {
								choice.Message.Signature = part.ThoughtSignature
							}
						} else {
							builder.WriteString(part.Text)
						}
					}
				}

				if part.InlineData != nil {
					contents = append(contents, relaymodel.MessageContent{
						Type: relaymodel.ContentTypeImageURL,
						ImageURL: &relaymodel.ImageURL{
							URL: fmt.Sprintf(
								"data:%s;base64,%s",
								part.InlineData.MimeType,
								part.InlineData.Data,
							),
						},
					})
				}
			}

			if hasImage {
				choice.Message.Content = contents
			} else {
				choice.Message.Content = builder.String()
			}

			choice.Message.ReasoningContent = reasoningContent.String()
		}

		fullTextResponse.Choices = append(fullTextResponse.Choices, &choice)
	}

	return &fullTextResponse
}

func streamResponseChat2OpenAI(
	meta *meta.Meta,
	geminiResponse *relaymodel.GeminiChatResponse,
) *relaymodel.ChatCompletionsStreamResponse {
	response := &relaymodel.ChatCompletionsStreamResponse{
		ID:      openai.ChatCompletionID(),
		Created: time.Now().Unix(),
		Model:   meta.OriginModel,
		Object:  relaymodel.ChatCompletionChunkObject,
		Choices: make(
			[]*relaymodel.ChatCompletionsStreamResponseChoice,
			0,
			len(geminiResponse.Candidates),
		),
	}
	if geminiResponse.UsageMetadata != nil {
		usage := geminiResponse.UsageMetadata.ToUsage()
		response.Usage = &usage
	}

	for i, candidate := range geminiResponse.Candidates {
		choice := relaymodel.ChatCompletionsStreamResponseChoice{
			Index: i,
			Delta: relaymodel.Message{
				Content: "",
			},
			FinishReason: FinishReason2OpenAI(candidate.FinishReason),
		}
		if len(candidate.Content.Parts) > 0 {
			var (
				contents         []relaymodel.MessageContent
				reasoningContent strings.Builder
				builder          strings.Builder
			)

			hasImage := false
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil {
					hasImage = true
					break
				}
			}

			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					toolCall, err := getToolCall(part)
					if err != nil {
						log.Error("get tool call failed: " + err.Error())
					}

					if toolCall != nil {
						choice.Delta.ToolCalls = append(choice.Delta.ToolCalls, *toolCall)
					}
				}

				if part.Text != "" {
					if hasImage {
						if part.Thought {
							reasoningContent.WriteString(part.Text)

							if part.ThoughtSignature != "" {
								choice.Delta.Signature = part.ThoughtSignature
							}
						} else {
							contents = append(contents, relaymodel.MessageContent{
								Type: relaymodel.ContentTypeText,
								Text: part.Text,
							})
						}
					} else {
						if part.Thought {
							reasoningContent.WriteString(part.Text)

							if part.ThoughtSignature != "" {
								choice.Delta.Signature = part.ThoughtSignature
							}
						} else {
							builder.WriteString(part.Text)
						}
					}
				}

				if part.InlineData != nil {
					contents = append(contents, relaymodel.MessageContent{
						Type: relaymodel.ContentTypeImageURL,
						ImageURL: &relaymodel.ImageURL{
							URL: fmt.Sprintf(
								"data:%s;base64,%s",
								part.InlineData.MimeType,
								part.InlineData.Data,
							),
						},
					})
				}
			}

			if hasImage {
				choice.Delta.Content = contents
			} else {
				choice.Delta.Content = builder.String()
			}

			choice.Delta.ReasoningContent = reasoningContent.String()
		}

		response.Choices = append(response.Choices, &choice)
	}

	return response
}

func StreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

	usage := model.Usage{}

	var websearchCount int64

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var geminiResponse relaymodel.GeminiChatResponse

		err := sonic.Unmarshal(data, &geminiResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := streamResponseChat2OpenAI(meta, &geminiResponse)
		if response.Usage != nil {
			usage = geminiResponse.UsageMetadata.ToModelUsage()
		}
		// Track web search count from grounding metadata
		if count := geminiResponse.GetWebSearchCount(); count > 0 {
			websearchCount += count
		}

		_ = render.OpenaiObjectData(c, response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	render.OpenaiDone(c)

	usage.WebSearchCount = model.ZeroNullInt64(websearchCount)

	return usage, nil
}

func Handler(meta *meta.Meta, c *gin.Context, resp *http.Response) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var geminiResponse relaymodel.GeminiChatResponse

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&geminiResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	fullTextResponse := responseChat2OpenAI(meta, &geminiResponse)

	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return fullTextResponse.Usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	modelUsage := fullTextResponse.Usage.ToModelUsage()
	modelUsage.WebSearchCount = model.ZeroNullInt64(geminiResponse.GetWebSearchCount())

	return modelUsage, nil
}
