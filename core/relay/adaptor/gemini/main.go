package gemini

import (
	"bufio"
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

// https://ai.google.dev/docs/gemini_api_overview?hl=zh-cn

// Dummy thought signatures for skipping Gemini's validation when the actual signature is unavailable
// See: https://ai.google.dev/gemini-api/docs/thought-signatures#faqs
const (
	ThoughtSignatureDummySkipValidator = "skip_thought_signature_validator"
	ThoughtSignatureDummyContextEng    = "context_engineering_is_the_way_to_go"
)

var toolChoiceTypeMap = map[string]string{
	"none":     "NONE",
	"auto":     "AUTO",
	"required": "ANY",
}

var mimeTypeMap = map[string]string{
	"json_object": "application/json",
	"text":        "text/plain",
}

type CountTokensResponse struct {
	Error       *Error `json:"error,omitempty"`
	TotalTokens int    `json:"totalTokens"`
}

func buildSafetySettings(safetySetting string) []ChatSafetySettings {
	if safetySetting == "" {
		safetySetting = "BLOCK_NONE"
	}

	return []ChatSafetySettings{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: safetySetting},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: safetySetting},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: safetySetting},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: safetySetting},
		{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: safetySetting},
	}
}

func buildGenerationConfig(
	meta *meta.Meta,
	req *relaymodel.GeneralOpenAIRequest,
	textRequest *relaymodel.GeneralOpenAIRequest,
) *ChatGenerationConfig {
	config := ChatGenerationConfig{
		Temperature:     textRequest.Temperature,
		TopP:            textRequest.TopP,
		MaxOutputTokens: textRequest.MaxTokens,
	}

	if strings.Contains(meta.ActualModel, "image") {
		config.ResponseModalities = []string{
			"Text",
			"Image",
		}
	}

	if textRequest.ResponseFormat != nil {
		if mimeType, ok := mimeTypeMap[textRequest.ResponseFormat.Type]; ok {
			config.ResponseMimeType = mimeType
		}

		if textRequest.ResponseFormat.JSONSchema != nil {
			config.ResponseSchema = textRequest.ResponseFormat.JSONSchema.Schema
			config.ResponseMimeType = mimeTypeMap["json_object"]
		}
	}

	if req.Thinking != nil {
		thinkingConfig := ThinkingConfig{}
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
	if strings.Contains(meta.ActualModel, "2.5") {
		if config.ThinkingConfig == nil {
			config.ThinkingConfig = &ThinkingConfig{}
		}

		config.ThinkingConfig.IncludeThoughts = true
	}

	return &config
}

func buildTools(textRequest *relaymodel.GeneralOpenAIRequest) []ChatTools {
	if textRequest.Tools != nil {
		functions := make([]relaymodel.Function, 0, len(textRequest.Tools))
		for _, tool := range textRequest.Tools {
			cleanedFunction := cleanFunctionParameters(tool.Function)
			functions = append(functions, cleanedFunction)
		}

		return []ChatTools{{FunctionDeclarations: functions}}
	}

	if textRequest.Functions != nil {
		return []ChatTools{{FunctionDeclarations: textRequest.Functions}}
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

func buildToolConfig(textRequest *relaymodel.GeneralOpenAIRequest) *ToolConfig {
	if textRequest.ToolChoice == nil {
		return nil
	}

	toolConfig := ToolConfig{
		FunctionCallingConfig: FunctionCallingConfig{
			Mode: "auto",
		},
	}
	switch mode := textRequest.ToolChoice.(type) {
	case string:
		if toolChoiceType, ok := toolChoiceTypeMap[mode]; ok {
			toolConfig.FunctionCallingConfig.Mode = toolChoiceType
		}
	case map[string]any:
		toolConfig.FunctionCallingConfig.Mode = "ANY"
		if fn, ok := mode["function"].(map[string]any); ok {
			if fnName, ok := fn["name"].(string); ok {
				toolConfig.FunctionCallingConfig.AllowedFunctionNames = []string{fnName}
			}
		}
	}

	return &toolConfig
}

func buildMessageParts(message relaymodel.MessageContent) *Part {
	part := &Part{
		Text: message.Text,
	}
	if message.ImageURL != nil {
		part.InlineData = &InlineData{
			Data: message.ImageURL.URL,
		}
	}

	return part
}

// Add this helper function to track tool calls
func buildContents(
	textRequest *relaymodel.GeneralOpenAIRequest,
) (*ChatContent, []*ChatContent, []*Part) {
	contents := make([]*ChatContent, 0, len(textRequest.Messages))

	var (
		imageTasks    []*Part
		systemContent *ChatContent
	)

	// Track tool calls by ID to get their names for tool results
	toolCallMap := make(map[string]string) // tool_call_id -> tool_name

	for _, message := range textRequest.Messages {
		content := ChatContent{
			Role: message.Role,
		}

		// Track tool calls from assistant messages
		switch {
		case message.Role == "assistant" && len(message.ToolCalls) > 0:
			for _, toolCall := range message.ToolCalls {
				toolCallMap[toolCall.ID] = toolCall.Function.Name

				var args map[string]any
				if toolCall.Function.Arguments != "" {
					if err := sonic.UnmarshalString(toolCall.Function.Arguments, &args); err != nil {
						args = make(map[string]any)
					}
				} else {
					args = make(map[string]any)
				}

				part := &Part{
					FunctionCall: &FunctionCall{
						Name: toolCall.Function.Name,
						Args: args,
					},
				}

				// Restore Gemini thought signature if present in extra_content (OpenAI format)
				if toolCall.ExtraContent != nil && toolCall.ExtraContent.Google != nil {
					if toolCall.ExtraContent.Google.ThoughtSignature != "" {
						part.ThoughtSignature = toolCall.ExtraContent.Google.ThoughtSignature
					}
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

			content.Parts = append(content.Parts, &Part{
				FunctionResponse: &FunctionResponse{
					Name: toolName, // Now properly set
					Response: struct {
						Name    string         `json:"name"`
						Content map[string]any `json:"content"`
					}{
						Name:    toolName, // Now properly set
						Content: contentMap,
					},
				},
			})
		case message.Role == "system":
			systemContent = &ChatContent{
				Role: "user", // Gemini uses "user" for system content
				Parts: []*Part{{
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
				if msgPart.InlineData != nil {
					imageTasks = append(imageTasks, msgPart)
				}

				if msgPart.Text == "" {
					continue
				}

				content.Parts = append(content.Parts, msgPart)
			}
		}

		// Adjust role for Gemini
		switch content.Role {
		case "assistant":
			content.Role = "model"
		case "tool":
			content.Role = "user"
		}

		if len(content.Parts) > 0 {
			contents = append(contents, &content)
		}
	}

	return systemContent, contents, imageTasks
}

func processImageTasks(ctx context.Context, imageTasks []*Part) error {
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

	config := buildGenerationConfig(meta, textRequest, textRequest)

	// Build actual request
	geminiRequest := ChatRequest{
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

type ChatResponse struct {
	Candidates     []*ChatCandidate   `json:"candidates"`
	PromptFeedback ChatPromptFeedback `json:"promptFeedback"`
	UsageMetadata  *UsageMetadata     `json:"usageMetadata"`
	ModelVersion   string             `json:"modelVersion"`
}

type UsageMetadata struct {
	PromptTokenCount     int64                `json:"promptTokenCount"`
	CandidatesTokenCount int64                `json:"candidatesTokenCount"`
	TotalTokenCount      int64                `json:"totalTokenCount"`
	ThoughtsTokenCount   int64                `json:"thoughtsTokenCount,omitempty"`
	PromptTokensDetails  []PromptTokensDetail `json:"promptTokensDetails"`

	// https://ai.google.dev/gemini-api/docs/caching?lang=rest
	CachedContentTokenCount int64               `json:"cachedContentTokenCount,omitempty"`
	CacheTokensDetails      []CacheTokensDetail `json:"cacheTokensDetails,omitempty"`
}

func (u *UsageMetadata) ToUsage() relaymodel.ChatUsage {
	chatUsage := relaymodel.ChatUsage{
		PromptTokens: u.PromptTokenCount,
		CompletionTokens: u.CandidatesTokenCount +
			u.ThoughtsTokenCount,
		TotalTokens: u.TotalTokenCount,
		PromptTokensDetails: &relaymodel.PromptTokensDetails{
			CachedTokens: u.CachedContentTokenCount,
		},
		CompletionTokensDetails: &relaymodel.CompletionTokensDetails{
			ReasoningTokens: u.ThoughtsTokenCount,
		},
	}

	return chatUsage
}

type PromptTokensDetail struct {
	Modality   string `json:"modality"`
	TokenCount int64  `json:"tokenCount"`
}

type CacheTokensDetail struct {
	Modality   string `json:"modality"`
	TokenCount int64  `json:"tokenCount"`
}

func (g *ChatResponse) GetResponseText() string {
	if g == nil {
		return ""
	}

	builder := strings.Builder{}
	for _, candidate := range g.Candidates {
		for i, part := range candidate.Content.Parts {
			if i > 0 {
				builder.WriteString("\n")
			}

			builder.WriteString(part.Text)
		}
	}

	return builder.String()
}

var finishReason2OpenAI = map[string]string{
	"STOP":       relaymodel.FinishReasonStop,
	"MAX_TOKENS": relaymodel.FinishReasonLength,
}

func FinishReason2OpenAI(reason string) string {
	if openaiReason, ok := finishReason2OpenAI[reason]; ok {
		return openaiReason
	}
	return reason
}

type ChatCandidate struct {
	FinishReason  string             `json:"finishReason"`
	Content       ChatContent        `json:"content"`
	SafetyRatings []ChatSafetyRating `json:"safetyRatings"`
	Index         int64              `json:"index"`
}

type ChatSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type ChatPromptFeedback struct {
	SafetyRatings []ChatSafetyRating `json:"safetyRatings"`
}

func getToolCall(item *Part) (*relaymodel.ToolCall, error) {
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

func responseChat2OpenAI(meta *meta.Meta, response *ChatResponse) *relaymodel.TextResponse {
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
				Role: "assistant",
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
						contents = append(contents, relaymodel.MessageContent{
							Type: relaymodel.ContentTypeText,
							Text: part.Text,
						})
					} else {
						if part.Thought {
							reasoningContent.WriteString(part.Text)
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
				choice.Message.ReasoningContent = reasoningContent.String()
			}
		}

		fullTextResponse.Choices = append(fullTextResponse.Choices, &choice)
	}

	return &fullTextResponse
}

func streamResponseChat2OpenAI(
	meta *meta.Meta,
	geminiResponse *ChatResponse,
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
						contents = append(contents, relaymodel.MessageContent{
							Type: relaymodel.ContentTypeText,
							Text: part.Text,
						})
					} else {
						if part.Thought {
							reasoningContent.WriteString(part.Text)
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
				choice.Delta.ReasoningContent = reasoningContent.String()
			}
		}

		response.Choices = append(response.Choices, &choice)
	}

	return response
}

const imageScannerBufferSize = 2 * 1024 * 1024

var scannerBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, imageScannerBufferSize)
		return &buf
	},
}

//nolint:forcetypeassert
func GetImageScannerBuffer() *[]byte {
	v, ok := scannerBufferPool.Get().(*[]byte)
	if !ok {
		panic(fmt.Sprintf("scanner buffer type error: %T, %v", v, v))
	}

	return v
}

func PutImageScannerBuffer(buf *[]byte) {
	if cap(*buf) != imageScannerBufferSize {
		return
	}

	scannerBufferPool.Put(buf)
}

func StreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	responseText := strings.Builder{}

	scanner := bufio.NewScanner(resp.Body)
	if strings.Contains(meta.ActualModel, "image") {
		buf := GetImageScannerBuffer()
		defer PutImageScannerBuffer(buf)

		scanner.Buffer(*buf, cap(*buf))
	} else {
		buf := utils.GetScannerBuffer()
		defer utils.PutScannerBuffer(buf)

		scanner.Buffer(*buf, cap(*buf))
	}

	usage := relaymodel.ChatUsage{
		PromptTokens: int64(meta.RequestUsage.InputTokens),
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

		var geminiResponse ChatResponse

		err := sonic.Unmarshal(data, &geminiResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := streamResponseChat2OpenAI(meta, &geminiResponse)
		if response.Usage != nil {
			usage = *response.Usage
		}

		if len(response.Choices) > 0 {
			responseText.WriteString(response.Choices[0].Delta.StringContent())
		}

		_ = render.OpenaiObjectData(c, response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	render.OpenaiDone(c)

	return usage.ToModelUsage(), nil
}

func Handler(meta *meta.Meta, c *gin.Context, resp *http.Response) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	var geminiResponse ChatResponse

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

	return fullTextResponse.Usage.ToModelUsage(), nil
}
