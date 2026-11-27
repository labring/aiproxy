package openai

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

type OpenAIRequestHook func(*relaymodel.GeneralOpenAIRequest) error

// ConvertGeminiRequest converts a Gemini native request to OpenAI format
func ConvertGeminiRequest(
	meta *meta.Meta,
	req *http.Request,
	hooks ...OpenAIRequestHook,
) (adaptor.ConvertResult, error) {
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
		openaiReq.StreamOptions = &relaymodel.StreamOptions{
			IncludeUsage: true,
		}
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
	convertGeminiGenerationConfigToOpenAI(meta, geminiReq, &openaiReq)

	// Convert tools
	openaiReq.Tools = convertGeminiToolsToOpenAI(geminiReq)

	// Convert tool config
	openaiReq.ToolChoice = convertGeminiToolConfigToOpenAI(geminiReq)

	for _, hook := range hooks {
		if hook == nil {
			continue
		}

		if err := hook(&openaiReq); err != nil {
			return adaptor.ConvertResult{}, err
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

// ForwardGeminiTopK returns a hook that copies the topK of the Gemini request
// generation config into the converted request. top_k is not part of the
// OpenAI API, so ConvertGeminiRequest leaves it out; pass the hook for
// upstreams whose API documents top_k.
func ForwardGeminiTopK(req *http.Request) OpenAIRequestHook {
	return forwardTopK(req, "generationConfig", "topK")
}

// forwardTopK reads the top-k field straight from the request body instead of
// decoding the whole document. The field is left out unless it is a number
// that fits an int32.
func forwardTopK(req *http.Request, path ...any) OpenAIRequestHook {
	return func(openAIRequest *relaymodel.GeneralOpenAIRequest) error {
		body, err := common.GetRequestBodyReusable(req)
		if err != nil {
			return err
		}

		node, err := common.GetJSONNodeNoCopy(body, path...)
		if errors.Is(err, ast.ErrNotExist) {
			return nil
		}

		if err != nil {
			return err
		}

		if node.TypeSafe() != ast.V_NUMBER {
			return nil
		}

		topK, err := node.Float64()
		if err != nil || topK < math.MinInt32 || topK > math.MaxInt32 {
			return nil
		}

		openAIRequest.TopK = int(topK)

		return nil
	}
}

// ConvertOpenAIToGeminiResponse converts OpenAI response back to Gemini format
func ConvertOpenAIToGeminiResponse(
	meta *meta.Meta,
	openaiResp *relaymodel.TextResponse,
) *relaymodel.GeminiChatResponse {
	geminiResp := &relaymodel.GeminiChatResponse{
		ModelVersion: responseModelName(meta),
	}

	if openaiResp.Usage.TotalTokens > 0 {
		geminiUsage := openaiResp.Usage.ToGeminiUsage()
		geminiResp.UsageMetadata = &geminiUsage
	}

	for _, choice := range openaiResp.Choices {
		candidate := &relaymodel.GeminiChatCandidate{
			Index: int64(choice.Index),
			Content: relaymodel.GeminiChatContent{
				Role:  relaymodel.GeminiRoleModel,
				Parts: []*relaymodel.GeminiPart{},
			},
		}

		// Convert finish reason
		switch choice.FinishReason {
		case relaymodel.FinishReasonStop:
			candidate.FinishReason = relaymodel.GeminiFinishReasonStop
		case relaymodel.FinishReasonLength:
			candidate.FinishReason = relaymodel.GeminiFinishReasonMaxTokens
		case relaymodel.FinishReasonToolCalls:
			candidate.FinishReason = relaymodel.GeminiFinishReasonStop
		case relaymodel.FinishReasonContentFilter:
			candidate.FinishReason = relaymodel.GeminiFinishReasonSafety
		default:
			candidate.FinishReason = relaymodel.GeminiFinishReasonStop
		}

		// Convert content
		if choice.Message.Content != nil {
			switch content := choice.Message.Content.(type) {
			case string:
				if content != "" {
					candidate.Content.Parts = append(
						candidate.Content.Parts,
						&relaymodel.GeminiPart{
							Text: content,
						},
					)
				}
			case []relaymodel.MessageContent:
				for _, part := range content {
					if part.Type == relaymodel.ContentTypeText {
						candidate.Content.Parts = append(
							candidate.Content.Parts,
							&relaymodel.GeminiPart{
								Text: part.Text,
							},
						)
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
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.OriginModel, meta.ActualModel)
	defer cleanup()

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

		if openaiResp.Usage != nil {
			usage = openaiResp.Usage.ToModelUsage()
		}

		// Convert to Gemini stream format
		geminiResp := streamState.ConvertOpenAIStreamToGemini(meta, &openaiResp)
		if geminiResp != nil {
			_ = render.GeminiObjectData(c, geminiResp)
		}
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
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
	geminiResp := &relaymodel.GeminiChatResponse{
		ModelVersion: responseModelName(meta),
		Candidates:   []*relaymodel.GeminiChatCandidate{},
	}

	if openaiResp.Usage != nil {
		geminiUsage := openaiResp.Usage.ToGeminiUsage()
		geminiResp.UsageMetadata = &geminiUsage
	}

	hasContent := geminiResp.UsageMetadata != nil

	for _, choice := range openaiResp.Choices {
		candidate := &relaymodel.GeminiChatCandidate{
			Index: int64(choice.Index),
			Content: relaymodel.GeminiChatContent{
				Role:  relaymodel.GeminiRoleModel,
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
				candidate.FinishReason = relaymodel.GeminiFinishReasonStop
			case relaymodel.FinishReasonLength:
				candidate.FinishReason = relaymodel.GeminiFinishReasonMaxTokens
			case relaymodel.FinishReasonToolCalls:
				candidate.FinishReason = relaymodel.GeminiFinishReasonStop
			case relaymodel.FinishReasonContentFilter:
				candidate.FinishReason = relaymodel.GeminiFinishReasonSafety
			default:
				candidate.FinishReason = relaymodel.GeminiFinishReasonStop
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
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	var openaiResp relaymodel.TextResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	geminiResp := ConvertOpenAIToGeminiResponse(meta, &openaiResp)

	jsonResponse, err := sonic.Marshal(geminiResp)
	if err != nil {
		responseErr := relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)

		return adaptor.DoResponseResult{
			Usage: openaiResp.Usage.ToModelUsage(),
		}, responseErr
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{Usage: openaiResp.Usage.ToModelUsage()}, nil
}

func convertGeminiSystemToOpenAI(geminiReq *relaymodel.GeminiChatRequest) []relaymodel.Message {
	if geminiReq.SystemInstruction == nil || len(geminiReq.SystemInstruction.Parts) == 0 {
		return nil
	}

	var systemText strings.Builder
	for _, part := range geminiReq.SystemInstruction.Parts {
		if part.Text != "" {
			systemText.WriteString(part.Text)
		}
	}

	if systemText.String() != "" {
		return []relaymodel.Message{{
			Role:    relaymodel.RoleSystem,
			Content: systemText.String(),
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

					// Clean parameters to remove null or empty required field
					// Some OpenAI-compatible APIs reject null or empty required arrays
					parameters = CleanToolParameters(parameters)

					function := relaymodel.Function{
						Name:        name,
						Description: description,
						Parameters:  parameters,
					}
					tools = append(tools, relaymodel.Tool{
						Type:     relaymodel.ToolChoiceTypeFunction,
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
	case relaymodel.GeminiFunctionCallingModeAuto:
		return relaymodel.ToolChoiceAuto
	case relaymodel.GeminiFunctionCallingModeNone:
		return relaymodel.ToolChoiceNone
	case relaymodel.GeminiFunctionCallingModeAny:
		if len(geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames) > 0 {
			return map[string]any{
				"type": relaymodel.ToolChoiceTypeFunction,
				"function": map[string]any{
					"name": geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames[0],
				},
			}
		}

		return relaymodel.ToolChoiceRequired
	}

	return nil
}

func convertGeminiResponseFormatToOpenAI(
	config *relaymodel.GeminiChatGenerationConfig,
) *relaymodel.ResponseFormat {
	if config.ResponseMimeType == "" {
		return nil
	}

	if config.ResponseSchema != nil {
		return &relaymodel.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &relaymodel.JSONSchema{
				Name:   "response",
				Schema: config.ResponseSchema,
			},
		}
	}

	switch config.ResponseMimeType {
	case "application/json":
		return &relaymodel.ResponseFormat{Type: "json_object"}
	case "text/plain":
		return &relaymodel.ResponseFormat{Type: "text"}
	}

	return nil
}

func convertGeminiGenerationConfigToOpenAI(
	meta *meta.Meta,
	geminiReq *relaymodel.GeminiChatRequest,
	openaiReq *relaymodel.GeneralOpenAIRequest,
) {
	if geminiReq.GenerationConfig != nil {
		openaiReq.Temperature = geminiReq.GenerationConfig.Temperature

		openaiReq.TopP = geminiReq.GenerationConfig.TopP
		if geminiReq.GenerationConfig.MaxOutputTokens != nil {
			openaiReq.MaxTokens = *geminiReq.GenerationConfig.MaxOutputTokens
		}

		if len(geminiReq.GenerationConfig.StopSequences) > 0 {
			openaiReq.Stop = geminiReq.GenerationConfig.StopSequences
		}

		openaiReq.ResponseFormat = convertGeminiResponseFormatToOpenAI(geminiReq.GenerationConfig)

		applyReasoningToOpenAIRequestForModel(
			meta,
			openaiReq,
			utils.ParseGeminiReasoning(geminiReq.GenerationConfig.ThinkingConfig),
		)
	}
}

func convertGeminiContentToOpenAI(
	content *relaymodel.GeminiChatContent,
	pendingTools *[]relaymodel.ToolCall,
) []relaymodel.Message {
	var messages []relaymodel.Message

	// Map role
	role := content.Role
	if role == "" {
		role = relaymodel.RoleUser
	}

	switch role {
	case relaymodel.GeminiRoleModel:
		role = relaymodel.RoleAssistant
	case relaymodel.GeminiRoleUser:
		role = relaymodel.RoleUser
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
				Type: relaymodel.ToolChoiceTypeFunction,
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
					Type: relaymodel.ToolChoiceTypeFunction,
					Function: relaymodel.Function{
						Name:      name,
						Arguments: "{}", // Assume empty args as we can't reconstruct them
					},
				}

				syntheticMsg := relaymodel.Message{
					Role:      relaymodel.RoleAssistant,
					ToolCalls: []relaymodel.ToolCall{syntheticCall},
				}

				messages = append(messages, syntheticMsg)
			}

			responseContent, _ := sonic.MarshalString(part.FunctionResponse.Response)

			toolMsg := relaymodel.Message{
				Role:       relaymodel.RoleTool,
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
			currentContentParts = append(
				currentContentParts,
				convertGeminiInlineDataToOpenAIContent(part.InlineData),
			)
			hasContent = true
		case part.FileData != nil:
			currentContentParts = append(
				currentContentParts,
				convertGeminiFileDataToOpenAIContent(part.FileData),
			)
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

func convertGeminiInlineDataToOpenAIContent(
	inlineData *relaymodel.GeminiInlineData,
) relaymodel.MessageContent {
	dataURL := inlineData.Data
	if !strings.HasPrefix(dataURL, "http") && !strings.HasPrefix(dataURL, "data:") {
		dataURL = "data:" + inlineData.MimeType + ";base64," + inlineData.Data
	}

	switch {
	case strings.HasPrefix(inlineData.MimeType, "audio/"):
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeInputAudio,
			InputAudio: &relaymodel.InputAudio{
				URL: dataURL,
			},
		}
	case strings.HasPrefix(inlineData.MimeType, "video/"):
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeVideoURL,
			VideoURL: &relaymodel.VideoURL{
				URL: dataURL,
			},
		}
	default:
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: dataURL,
			},
		}
	}
}

func convertGeminiFileDataToOpenAIContent(
	fileData *relaymodel.GeminiFileData,
) relaymodel.MessageContent {
	mimeType := fileData.MimeType
	if mimeType == "" {
		mimeType = inferGeminiFileDataMimeType(fileData.FileURI)
	}

	switch {
	case strings.HasPrefix(mimeType, "audio/"):
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeInputAudio,
			InputAudio: &relaymodel.InputAudio{
				URL: fileData.FileURI,
			},
		}
	case strings.HasPrefix(mimeType, "video/"):
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeVideoURL,
			VideoURL: &relaymodel.VideoURL{
				URL: fileData.FileURI,
			},
		}
	default:
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: fileData.FileURI,
			},
		}
	}
}

func inferGeminiFileDataMimeType(fileURI string) string {
	if after, ok := strings.CutPrefix(fileURI, "data:"); ok {
		mediaType := after
		if beforeParams, _, ok := strings.Cut(mediaType, ";"); ok {
			return beforeParams
		}

		if beforeData, _, ok := strings.Cut(mediaType, ","); ok {
			return beforeData
		}
	}

	path := fileURI
	if parsed, err := url.Parse(fileURI); err == nil && parsed.Path != "" {
		path = parsed.Path
	}

	if ext := filepath.Ext(path); ext != "" {
		return mime.TypeByExtension(strings.ToLower(ext))
	}

	return ""
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
				Role:    relaymodel.RoleSystem,
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
		Input:  ConvertMessagesToInputItems(messages),
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

		responsesReq.Text = convertChatResponseFormatToResponseText(
			convertGeminiResponseFormatToOpenAI(geminiReq.GenerationConfig),
		)
	}

	if geminiReq.GenerationConfig != nil {
		applyReasoningToResponsesRequestForModel(
			meta,
			&responsesReq,
			utils.ParseGeminiReasoning(geminiReq.GenerationConfig.ThinkingConfig),
		)
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
						parameters = CleanToolParameters(parameters)

						tools = append(tools, relaymodel.ResponseTool{
							Type:        relaymodel.ToolChoiceTypeFunction,
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
	responsesReq.ToolChoice = convertChatToolChoiceToResponseToolChoice(
		convertGeminiToolConfigToOpenAI(geminiReq),
	)

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

// ConvertResponsesToGeminiResponse converts Responses API response to Gemini format
func ConvertResponsesToGeminiResponse(
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

	// Convert to Gemini format
	geminiResp := relaymodel.GeminiChatResponse{
		ModelVersion: responseModelName(meta),
		Candidates:   []*relaymodel.GeminiChatCandidate{},
	}

	// Convert output items to Gemini candidates
	for _, outputItem := range responsesResp.Output {
		candidate := &relaymodel.GeminiChatCandidate{
			Index: 0,
			Content: relaymodel.GeminiChatContent{
				Role:  relaymodel.GeminiRoleModel,
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
					err := sonic.UnmarshalString(outputItem.Arguments.String(), &args)
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
			candidate.FinishReason = responseToGeminiFinishReason(&responsesResp)

			geminiResp.Candidates = append(geminiResp.Candidates, candidate)
		}
	}

	// Convert usage
	if responsesResp.Usage != nil {
		geminiUsage := responsesResp.Usage.ToGeminiUsage()
		geminiResp.UsageMetadata = &geminiUsage
	}

	// Marshal and return
	geminiRespData, err := sonic.Marshal(geminiResp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(geminiRespData)))
	_, _ = c.Writer.Write(geminiRespData)

	return adaptor.DoResponseResult{
		Usage:      responsesResp.ToModelUsage(),
		UpstreamID: responsesResp.ID,
		AsyncUsage: responseNeedsAsyncUsage(&responsesResp),
	}, nil
}

// ConvertResponsesToGeminiStreamResponse converts Responses API stream to Gemini stream
func ConvertResponsesToGeminiStreamResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.OriginModel, meta.ActualModel)
	defer cleanup()

	var (
		errorState  responsesStreamErrorState
		wroteStream bool
	)

	state := &geminiStreamState{
		meta: meta,
		c:    c,
	}

	stopStream := false

	for scanner.Scan() && !stopStream {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)

		// Parse the stream event
		var event relaymodel.ResponseStreamEvent

		err := sonic.Unmarshal(data, &event)
		if err != nil {
			log.Error("error unmarshalling response stream: " + err.Error())
			continue
		}

		errorState.update(&event)

		if err := errorState.errorBeforeEvent(&event); err != nil {
			return errorState.result(), err
		}

		if event.Type == relaymodel.EventResponseFailed || event.Type == relaymodel.EventError {
			if wroteStream {
				log.Error(
					"response stream failed after data was sent: " + responseStreamErrorMessage(
						&event,
					),
				)

				stopStream = true

				continue
			}

			err, handled := errorState.handleFailure(&event)
			if handled && err == nil {
				continue
			}

			if handled {
				return errorState.result(), err
			}
		}

		// Handle events
		// Note: Gemini format requires complete JSON for function calls,
		// so we handle function_call_arguments.done (complete), not function_call_arguments.delta (streaming)
		switch event.Type {
		case relaymodel.EventOutputItemAdded:
			state.handleOutputItemAdded(&event)
		case relaymodel.EventOutputTextDelta:
			if state.handleOutputTextDelta(&event) {
				wroteStream = true
			}
		case relaymodel.EventFunctionCallArgumentsDone:
			if state.handleFunctionCallArgumentsDone(&event) {
				wroteStream = true
			}
		case relaymodel.EventResponseCompleted,
			relaymodel.EventResponseDone,
			relaymodel.EventResponseIncomplete:
			if state.handleResponseCompleted(&event) {
				wroteStream = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading response stream: " + err.Error())
	}

	if errorState.pendingFailure != nil && !wroteStream {
		return errorState.result(), responseStreamError(errorState.pendingFailure)
	}

	return errorState.result(), nil
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
	if event.Item.Type == relaymodel.InputItemTypeFunctionCall && event.Item.Name != "" {
		if s.functionCallNames == nil {
			s.functionCallNames = make(map[string]string)
		}

		s.functionCallNames[event.Item.ID] = event.Item.Name
	}
}

// handleOutputTextDelta handles response.output_text.delta event for Gemini
func (s *geminiStreamState) handleOutputTextDelta(event *relaymodel.ResponseStreamEvent) bool {
	if event.Delta == "" {
		return false
	}

	// Send text delta
	geminiResp := relaymodel.GeminiChatResponse{
		ModelVersion: responseModelName(s.meta),
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Index: 0,
				Content: relaymodel.GeminiChatContent{
					Role: relaymodel.GeminiRoleModel,
					Parts: []*relaymodel.GeminiPart{
						{
							Text: event.Delta,
						},
					},
				},
			},
		},
	}

	_ = render.GeminiObjectData(s.c, geminiResp)

	return true
}

// handleFunctionCallArgumentsDone handles response.function_call_arguments.done event for Gemini
func (s *geminiStreamState) handleFunctionCallArgumentsDone(
	event *relaymodel.ResponseStreamEvent,
) bool {
	if event.Arguments == "" || event.ItemID == "" {
		return false
	}

	// Get function name from tracked state
	functionName := s.functionCallNames[event.ItemID]
	if functionName == "" {
		return false
	}

	// Parse arguments
	var args map[string]any
	if err := sonic.UnmarshalString(event.Arguments.String(), &args); err != nil {
		return false
	}

	// Send complete function call
	geminiResp := relaymodel.GeminiChatResponse{
		ModelVersion: responseModelName(s.meta),
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Index: 0,
				Content: relaymodel.GeminiChatContent{
					Role: relaymodel.GeminiRoleModel,
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

	_ = render.GeminiObjectData(s.c, geminiResp)

	return true
}

// handleResponseCompleted handles response.completed/done event for Gemini
func (s *geminiStreamState) handleResponseCompleted(event *relaymodel.ResponseStreamEvent) bool {
	if event.Response == nil || event.Response.Usage == nil {
		return false
	}

	// Send final response with usage
	geminiUsage := event.Response.Usage.ToGeminiUsage()
	geminiResp := relaymodel.GeminiChatResponse{
		ModelVersion:  responseModelName(s.meta),
		UsageMetadata: &geminiUsage,
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Index:        0,
				FinishReason: responseToGeminiFinishReason(event.Response),
				Content: relaymodel.GeminiChatContent{
					Role:  relaymodel.GeminiRoleModel,
					Parts: []*relaymodel.GeminiPart{},
				},
			},
		},
	}

	_ = render.GeminiObjectData(s.c, geminiResp)

	return true
}

// responseToGeminiFinishReason maps a Responses API result to a Gemini finish reason,
// honoring incomplete_details.reason like responseToChatFinishReason does.
func responseToGeminiFinishReason(response *relaymodel.Response) string {
	switch responseToChatFinishReason(response) {
	case relaymodel.FinishReasonLength:
		return relaymodel.GeminiFinishReasonMaxTokens
	case relaymodel.FinishReasonContentFilter:
		return relaymodel.GeminiFinishReasonSafety
	default:
		return relaymodel.GeminiFinishReasonStop
	}
}
