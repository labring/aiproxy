package anthropic

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

// ConvertGeminiRequest converts a Gemini native request to Claude format
func ConvertGeminiRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	claudeReq, err := ConvertGeminiRequestToStruct(meta, req)
	if err != nil {
		return adaptor.ConvertResult{}, err
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

func ConvertGeminiRequestToStruct(
	meta *meta.Meta,
	req *http.Request,
) (*relaymodel.ClaudeRequest, error) {
	// Parse Gemini request
	geminiReq, err := utils.UnmarshalGeminiChatRequest(req)
	if err != nil {
		return nil, err
	}

	// Convert to Claude format
	claudeReq := relaymodel.ClaudeRequest{
		Model:     meta.ActualModel,
		MaxTokens: ModelDefaultMaxTokens(meta.ActualModel),
		Messages:  []relaymodel.ClaudeMessage{},
		System:    convertGeminiSystemInstruction(geminiReq),
	}

	// Check if this is a streaming request by checking the URL path
	// URL format: /v1beta/models/{model}:streamGenerateContent
	if utils.IsGeminiStreamRequest(req.URL.Path) {
		claudeReq.Stream = true
	}

	// Convert contents to messages
	toolCallMap := make(map[string][]string)
	for i, content := range geminiReq.Contents {
		msg := convertGeminiContent(content, i, toolCallMap)
		if len(msg.Content) == 0 {
			continue
		}

		if len(claudeReq.Messages) > 0 {
			lastMsg := &claudeReq.Messages[len(claudeReq.Messages)-1]
			if lastMsg.Role == msg.Role {
				lastMsg.Content = append(lastMsg.Content, msg.Content...)
				continue
			}
		}

		claudeReq.Messages = append(claudeReq.Messages, msg)
	}

	// Convert generation config
	if geminiReq.GenerationConfig != nil {
		if geminiReq.GenerationConfig.Temperature != nil {
			claudeReq.Temperature = geminiReq.GenerationConfig.Temperature
		} else if geminiReq.GenerationConfig.TopP != nil {
			claudeReq.TopP = geminiReq.GenerationConfig.TopP
		}

		if geminiReq.GenerationConfig.MaxOutputTokens != nil {
			claudeReq.MaxTokens = *geminiReq.GenerationConfig.MaxOutputTokens
		}
	}

	// Convert tools
	claudeReq.Tools = convertGeminiTools(geminiReq)
	claudeReq.ToolChoice = convertGeminiToolConfig(geminiReq)

	return &claudeReq, nil
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
			Role:  relaymodel.GeminiRoleModel,
			Parts: []*relaymodel.GeminiPart{},
		},
	}

	// Convert stop reason
	switch claudeResp.StopReason {
	case relaymodel.ClaudeStopReasonEndTurn:
		candidate.FinishReason = relaymodel.GeminiFinishReasonStop
	case relaymodel.ClaudeStopReasonMaxTokens:
		candidate.FinishReason = relaymodel.GeminiFinishReasonMaxTokens
	case relaymodel.ClaudeStopReasonToolUse:
		candidate.FinishReason = relaymodel.GeminiFinishReasonStop
	default:
		candidate.FinishReason = relaymodel.GeminiFinishReasonStop
	}

	// Convert content
	for _, content := range claudeResp.Content {
		switch content.Type {
		case relaymodel.ClaudeContentTypeText:
			if content.Text != "" {
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text: content.Text,
				})
			}
		case relaymodel.ClaudeContentTypeThinking:
			if content.Thinking != "" {
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text:    content.Thinking,
					Thought: true,
				})
			}
		case relaymodel.ClaudeContentTypeToolUse:
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
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var claudeResp relaymodel.ClaudeResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperAnthropicError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	geminiResp := ConvertClaudeToGeminiResponse(meta, &claudeResp)

	jsonResponse, err := sonic.Marshal(geminiResp)
	if err != nil {
		return adaptor.DoResponseResult{
				Usage: claudeResp.Usage.ToOpenAIUsage().ToModelUsage(),
			}, relaymodel.WrapperAnthropicError(
				err,
				"marshal_response_body_failed",
				http.StatusInternalServerError,
			)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))

	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{Usage: claudeResp.Usage.ToOpenAIUsage().ToModelUsage()}, nil
}

// GeminiStreamHandler handles streaming responses and converts them to Gemini format
func GeminiStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
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

		var claudeResp relaymodel.ClaudeStreamResponse
		if err := sonic.Unmarshal(data, &claudeResp); err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		// Convert to Gemini stream format
		geminiResp := streamState.ConvertClaudeStreamToGemini(
			meta,
			&claudeResp,
		)
		if geminiResp != nil {
			_ = render.GeminiObjectData(c, geminiResp)

			if geminiResp.UsageMetadata != nil {
				usage = geminiResp.UsageMetadata.ToModelUsage()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}

// GeminiStreamState maintains state during streaming response conversion
type GeminiStreamState struct {
	CurrentToolName string
	CurrentToolID   string
	CurrentToolArgs strings.Builder
}

func NewGeminiStreamState() *GeminiStreamState {
	return &GeminiStreamState{}
}

//nolint:gocyclo
func (s *GeminiStreamState) ConvertClaudeStreamToGemini(
	meta *meta.Meta,
	claudeResp *relaymodel.ClaudeStreamResponse,
) *relaymodel.GeminiChatResponse {
	geminiResp := &relaymodel.GeminiChatResponse{
		ModelVersion: meta.ActualModel,
		Candidates:   []*relaymodel.GeminiChatCandidate{},
	}

	candidate := &relaymodel.GeminiChatCandidate{
		Index: 0,
		Content: relaymodel.GeminiChatContent{
			Role:  relaymodel.GeminiRoleModel,
			Parts: []*relaymodel.GeminiPart{},
		},
	}

	switch claudeResp.Type {
	case relaymodel.ClaudeStreamTypeMessageStart:
		if claudeResp.Message != nil && claudeResp.Message.Usage.InputTokens > 0 {
			geminiResp.UsageMetadata = &relaymodel.GeminiUsageMetadata{
				PromptTokenCount: claudeResp.Message.Usage.InputTokens,
			}
		}

		return geminiResp

	case relaymodel.ClaudeStreamTypeContentBlockDelta:
		if claudeResp.Delta != nil {
			switch {
			case claudeResp.Delta.Type == relaymodel.ClaudeDeltaTypeTextDelta && claudeResp.Delta.Text != "":
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text: claudeResp.Delta.Text,
				})
			case claudeResp.Delta.Type == relaymodel.ClaudeDeltaTypeThinkingDelta && claudeResp.Delta.Thinking != "":
				candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
					Text:    claudeResp.Delta.Thinking,
					Thought: true,
				})
			case claudeResp.Delta.Type == relaymodel.ClaudeDeltaTypeInputJSONDelta && claudeResp.Delta.PartialJSON != "":
				s.CurrentToolArgs.WriteString(claudeResp.Delta.PartialJSON)
				return nil
			}
		}

	case relaymodel.ClaudeStreamTypeContentBlockStart:
		if claudeResp.ContentBlock != nil {
			if claudeResp.ContentBlock.Type == relaymodel.ClaudeContentTypeToolUse {
				s.CurrentToolName = claudeResp.ContentBlock.Name
				s.CurrentToolID = claudeResp.ContentBlock.ID
				s.CurrentToolArgs.Reset()

				// If input is already provided (non-streaming case sometimes), use it
				if inputMap, ok := claudeResp.ContentBlock.Input.(map[string]any); ok &&
					len(inputMap) > 0 {
					inputJSON, _ := sonic.MarshalString(inputMap)
					s.CurrentToolArgs.WriteString(inputJSON)
				}

				return nil
			}
		}

	case relaymodel.ClaudeStreamTypeContentBlockStop:
		if s.CurrentToolName != "" {
			argsStr := s.CurrentToolArgs.String()

			args := make(map[string]any)
			if argsStr != "" {
				_ = sonic.UnmarshalString(argsStr, &args)
			}

			candidate.Content.Parts = append(candidate.Content.Parts, &relaymodel.GeminiPart{
				FunctionCall: &relaymodel.GeminiFunctionCall{
					Name: s.CurrentToolName,
					Args: args,
				},
			})

			s.CurrentToolName = ""
			s.CurrentToolID = ""
			s.CurrentToolArgs.Reset()
		} else {
			return nil
		}

	case relaymodel.ClaudeStreamTypeMessageDelta:
		if claudeResp.Delta != nil && claudeResp.Delta.StopReason != nil {
			switch *claudeResp.Delta.StopReason {
			case relaymodel.ClaudeStopReasonEndTurn:
				candidate.FinishReason = relaymodel.GeminiFinishReasonStop
			case relaymodel.ClaudeStopReasonMaxTokens:
				candidate.FinishReason = relaymodel.GeminiFinishReasonMaxTokens
			case relaymodel.ClaudeStopReasonToolUse:
				candidate.FinishReason = relaymodel.GeminiFinishReasonStop
			default:
				candidate.FinishReason = relaymodel.GeminiFinishReasonStop
			}
		}

		if claudeResp.Usage != nil {
			geminiResp.UsageMetadata = &relaymodel.GeminiUsageMetadata{
				PromptTokenCount:     claudeResp.Usage.InputTokens,
				CandidatesTokenCount: claudeResp.Usage.OutputTokens,
				TotalTokenCount:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
			}
		}

	case relaymodel.ClaudeStreamTypeMessageStop:
		return nil
	}

	if len(candidate.Content.Parts) > 0 || candidate.FinishReason != "" ||
		geminiResp.UsageMetadata != nil {
		geminiResp.Candidates = append(geminiResp.Candidates, candidate)
		return geminiResp
	}

	return nil
}

func convertGeminiSystemInstruction(
	geminiReq *relaymodel.GeminiChatRequest,
) []relaymodel.ClaudeContent {
	var system []relaymodel.ClaudeContent
	if geminiReq.SystemInstruction != nil && len(geminiReq.SystemInstruction.Parts) > 0 {
		for _, part := range geminiReq.SystemInstruction.Parts {
			if part.Text != "" {
				system = append(system, relaymodel.ClaudeContent{
					Type: relaymodel.ClaudeContentTypeText,
					Text: part.Text,
				})
			}
		}
	}

	return system
}

func convertGeminiContent(
	content *relaymodel.GeminiChatContent,
	msgIndex int,
	toolCallMap map[string][]string,
) relaymodel.ClaudeMessage {
	msg := relaymodel.ClaudeMessage{}

	// Map role
	if content.Role == "" {
		content.Role = relaymodel.GeminiRoleUser
	}

	switch content.Role {
	case relaymodel.GeminiRoleModel:
		msg.Role = relaymodel.RoleAssistant
	case relaymodel.GeminiRoleUser:
		msg.Role = relaymodel.RoleUser
	default:
		msg.Role = content.Role
	}

	// Convert parts
	for i, part := range content.Parts {
		switch {
		case part.FunctionCall != nil:
			// Handle function call - convert to tool use
			// Generate a deterministic ID based on message index and part index
			id := fmt.Sprintf("toolu_%d_%d", msgIndex, i)
			// Store ID for matching with response
			toolCallMap[part.FunctionCall.Name] = append(toolCallMap[part.FunctionCall.Name], id)

			msg.Content = append(msg.Content, relaymodel.ClaudeContent{
				Type:  relaymodel.ClaudeContentTypeToolUse,
				ID:    id,
				Name:  part.FunctionCall.Name,
				Input: part.FunctionCall.Args,
			})
		case part.FunctionResponse != nil:
			// Handle function response - convert to tool result
			msg.Role = relaymodel.RoleUser
			content, _ := sonic.MarshalString(part.FunctionResponse.Response)

			// Retrieve the corresponding tool call ID
			id := ""
			if ids, ok := toolCallMap[part.FunctionResponse.Name]; ok && len(ids) > 0 {
				id = ids[0]
				// Remove the used ID
				toolCallMap[part.FunctionResponse.Name] = ids[1:]
			}

			if id != "" {
				msg.Content = append(msg.Content, relaymodel.ClaudeContent{
					Type:      relaymodel.ClaudeContentTypeToolResult,
					ToolUseID: id,
					Content:   content,
				})
			} else {
				// Orphaned result - convert to text to avoid validation error
				msg.Content = append(msg.Content, relaymodel.ClaudeContent{
					Type: relaymodel.ClaudeContentTypeText,
					Text: fmt.Sprintf(
						"Tool result for %s: %s",
						part.FunctionResponse.Name,
						content,
					),
				})
			}
		case part.Text != "":
			if part.Thought {
				// Handle thinking content
				msg.Content = append(msg.Content, relaymodel.ClaudeContent{
					Type:     relaymodel.ClaudeContentTypeThinking,
					Thinking: part.Text,
				})
			} else {
				// Handle text content
				msg.Content = append(msg.Content, relaymodel.ClaudeContent{
					Type: relaymodel.ClaudeContentTypeText,
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
				Type: relaymodel.ClaudeContentTypeImage,
				Source: &relaymodel.ClaudeImageSource{
					Type:      relaymodel.ClaudeImageSourceTypeBase64,
					MediaType: part.InlineData.MimeType,
					Data:      imageData,
				},
			})
		}
	}

	return msg
}

func convertGeminiTools(geminiReq *relaymodel.GeminiChatRequest) []relaymodel.ClaudeTool {
	if len(geminiReq.Tools) == 0 {
		return nil
	}

	var tools []relaymodel.ClaudeTool
	for _, geminiTool := range geminiReq.Tools {
		if fnDecls, ok := geminiTool.FunctionDeclarations.([]any); ok {
			for _, fnDecl := range fnDecls {
				if fn, ok := fnDecl.(map[string]any); ok {
					tools = append(tools, convertGeminiFunctionDeclaration(fn))
				}
			}
		}
	}

	return tools
}

func convertGeminiFunctionDeclaration(fn map[string]any) relaymodel.ClaudeTool {
	var (
		inputSchema *relaymodel.ClaudeInputSchema
		parameters  map[string]any
	)

	if params, ok := fn["parameters"].(map[string]any); ok {
		parameters = params
	} else if params, ok := fn["parametersJsonSchema"].(map[string]any); ok {
		parameters = params
	}

	if parameters != nil {
		inputSchema = &relaymodel.ClaudeInputSchema{
			Type: "object",
		}

		// Check if parameters is a schema or just properties
		_, hasType := parameters["type"]
		_, hasProps := parameters["properties"]
		_, hasRequired := parameters["required"]

		if hasType || hasProps || hasRequired {
			if t, ok := parameters["type"].(string); ok {
				inputSchema.Type = t
			}

			if props, ok := parameters["properties"]; ok {
				inputSchema.Properties = props
			}

			if req, ok := parameters["required"]; ok {
				inputSchema.Required = req
			}
		} else {
			// Legacy support: treat as properties map
			inputSchema.Properties = parameters
		}
	}

	name, _ := fn["name"].(string)
	description, _ := fn["description"].(string)

	return relaymodel.ClaudeTool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}
}

func convertGeminiToolConfig(geminiReq *relaymodel.GeminiChatRequest) any {
	if geminiReq.ToolConfig == nil {
		return nil
	}

	switch geminiReq.ToolConfig.FunctionCallingConfig.Mode {
	case relaymodel.GeminiFunctionCallingModeAuto:
		return map[string]any{"type": relaymodel.ToolChoiceAuto}
	case relaymodel.GeminiFunctionCallingModeAny:
		if len(geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames) > 0 {
			return map[string]any{
				"type": relaymodel.ToolChoiceTypeTool,
				"name": geminiReq.ToolConfig.FunctionCallingConfig.AllowedFunctionNames[0],
			}
		}

		return map[string]any{"type": relaymodel.ToolChoiceAny}
	}

	return nil
}
