package gemini

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/common"
	"github.com/labring/aiproxy/common/config"
	"github.com/labring/aiproxy/common/conv"
	"github.com/labring/aiproxy/common/image"
	"github.com/labring/aiproxy/common/render"
	"github.com/labring/aiproxy/middleware"
	"github.com/labring/aiproxy/relay/adaptor/openai"
	"github.com/labring/aiproxy/relay/meta"
	"github.com/labring/aiproxy/relay/model"
	"github.com/labring/aiproxy/relay/utils"
	log "github.com/sirupsen/logrus"
)

// https://ai.google.dev/docs/gemini_api_overview?hl=zh-cn

const (
	VisionMaxImageNum = 16
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

func buildSafetySettings() []ChatSafetySettings {
	safetySetting := config.GetGeminiSafetySetting()
	return []ChatSafetySettings{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: safetySetting},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: safetySetting},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: safetySetting},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: safetySetting},
		{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: safetySetting},
	}
}

func buildGenerationConfig(meta *meta.Meta, textRequest *model.GeneralOpenAIRequest) *ChatGenerationConfig {
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

	return &config
}

func buildTools(textRequest *model.GeneralOpenAIRequest) []ChatTools {
	if textRequest.Tools != nil {
		functions := make([]model.Function, 0, len(textRequest.Tools))
		for _, tool := range textRequest.Tools {
			if parameters, ok := tool.Function.Parameters.(map[string]any); ok {
				if properties, ok := parameters["properties"].(map[string]any); ok {
					if len(properties) == 0 {
						tool.Function.Parameters = nil
					}
				}
			}
			functions = append(functions, tool.Function)
		}
		return []ChatTools{{FunctionDeclarations: functions}}
	}
	if textRequest.Functions != nil {
		return []ChatTools{{FunctionDeclarations: textRequest.Functions}}
	}
	return nil
}

func buildToolConfig(textRequest *model.GeneralOpenAIRequest) *ToolConfig {
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
	case map[string]interface{}:
		toolConfig.FunctionCallingConfig.Mode = "ANY"
		if fn, ok := mode["function"].(map[string]interface{}); ok {
			if fnName, ok := fn["name"].(string); ok {
				toolConfig.FunctionCallingConfig.AllowedFunctionNames = []string{fnName}
			}
		}
	}
	return &toolConfig
}

func buildMessageParts(ctx context.Context, part model.MessageContent) ([]Part, error) {
	if part.Type == model.ContentTypeText {
		return []Part{{Text: part.Text}}, nil
	}

	if part.Type == model.ContentTypeImageURL {
		mimeType, data, err := image.GetImageFromURL(ctx, part.ImageURL.URL)
		if err != nil {
			return nil, err
		}
		return []Part{{
			InlineData: &InlineData{
				MimeType: mimeType,
				Data:     data,
			},
		}}, nil
	}

	return nil, nil
}

func buildContents(ctx context.Context, textRequest *model.GeneralOpenAIRequest) (*ChatContent, []*ChatContent, error) {
	contents := make([]*ChatContent, 0, len(textRequest.Messages))
	imageNum := 0

	var systemContent *ChatContent

	for _, message := range textRequest.Messages {
		content := ChatContent{
			Role:  message.Role,
			Parts: make([]Part, 0),
		}

		switch {
		case message.Role == "assistant" && len(message.ToolCalls) > 0:
			for _, toolCall := range message.ToolCalls {
				var args map[string]any
				if toolCall.Function.Arguments != "" {
					if err := sonic.UnmarshalString(toolCall.Function.Arguments, &args); err != nil {
						args = make(map[string]any)
					}
				} else {
					args = make(map[string]any)
				}
				content.Parts = append(content.Parts, Part{
					FunctionCall: &FunctionCall{
						Name: toolCall.Function.Name,
						Args: args,
					},
				})
			}
		case message.Role == "tool" && message.ToolCallID != "":
			var contentMap map[string]any
			if message.Content != nil {
				switch content := message.Content.(type) {
				case map[string]any:
					contentMap = content
				case string:
					if err := sonic.UnmarshalString(content, &contentMap); err != nil {
						log.Error("unmarshal content failed: " + err.Error())
					}
				}
			} else {
				contentMap = make(map[string]any)
			}
			content.Parts = append(content.Parts, Part{
				FunctionResponse: &FunctionResponse{
					Name: *message.Name,
					Response: struct {
						Name    string         `json:"name"`
						Content map[string]any `json:"content"`
					}{
						Name:    *message.Name,
						Content: contentMap,
					},
				},
			})
		default:
			openaiContent := message.ParseContent()
			for _, part := range openaiContent {
				if part.Type == model.ContentTypeImageURL {
					imageNum++
					if imageNum > VisionMaxImageNum {
						continue
					}
				}

				parts, err := buildMessageParts(ctx, part)
				if err != nil {
					return nil, nil, err
				}
				content.Parts = append(content.Parts, parts...)
			}
		}

		switch content.Role {
		case "assistant":
			content.Role = "model"
		case "tool":
			content.Role = "user"
		case "system":
			systemContent = &content
			continue
		}
		contents = append(contents, &content)
	}

	return systemContent, contents, nil
}

// Setting safety to the lowest possible values since Gemini is already powerless enough
func ConvertRequest(meta *meta.Meta, req *http.Request) (string, http.Header, io.Reader, error) {
	textRequest, err := utils.UnmarshalGeneralOpenAIRequest(req)
	if err != nil {
		return "", nil, nil, err
	}

	textRequest.Model = meta.ActualModel
	meta.Set("stream", textRequest.Stream)

	systemContent, contents, err := buildContents(req.Context(), textRequest)
	if err != nil {
		return "", nil, nil, err
	}

	// Build actual request
	geminiRequest := ChatRequest{
		Contents:          contents,
		SystemInstruction: systemContent,
		SafetySettings:    buildSafetySettings(),
		GenerationConfig:  buildGenerationConfig(meta, textRequest),
		Tools:             buildTools(textRequest),
		ToolConfig:        buildToolConfig(textRequest),
	}

	data, err := sonic.Marshal(geminiRequest)
	if err != nil {
		return "", nil, nil, err
	}

	return http.MethodPost, nil, bytes.NewReader(data), nil
}

type ChatResponse struct {
	Candidates     []*ChatCandidate   `json:"candidates"`
	PromptFeedback ChatPromptFeedback `json:"promptFeedback"`
	UsageMetadata  *UsageMetadata     `json:"usageMetadata"`
	ModelVersion   string             `json:"modelVersion"`
}

type UsageMetadata struct {
	PromptTokenCount     int64 `json:"promptTokenCount"`
	CandidatesTokenCount int64 `json:"candidatesTokenCount"`
	TotalTokenCount      int64 `json:"totalTokenCount"`
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

func getToolCall(item *Part) (*model.Tool, error) {
	if item.FunctionCall == nil {
		return nil, nil
	}
	argsBytes, err := sonic.Marshal(item.FunctionCall.Args)
	if err != nil {
		return nil, err
	}
	toolCall := model.Tool{
		ID:   openai.CallID(),
		Type: "function",
		Function: model.Function{
			Arguments: conv.BytesToString(argsBytes),
			Name:      item.FunctionCall.Name,
		},
	}
	return &toolCall, nil
}

func responseChat2OpenAI(meta *meta.Meta, response *ChatResponse) *model.TextResponse {
	fullTextResponse := model.TextResponse{
		ID:      openai.ChatCompletionID(),
		Model:   meta.OriginModel,
		Object:  model.ChatCompletion,
		Created: time.Now().Unix(),
		Choices: make([]*model.TextResponseChoice, 0, len(response.Candidates)),
	}
	if response.UsageMetadata != nil {
		fullTextResponse.Usage = model.Usage{
			PromptTokens:     response.UsageMetadata.PromptTokenCount,
			CompletionTokens: response.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      response.UsageMetadata.TotalTokenCount,
		}
	}
	for i, candidate := range response.Candidates {
		choice := model.TextResponseChoice{
			Index: i,
			Message: model.Message{
				Role:    "assistant",
				Content: "",
			},
			FinishReason: candidate.FinishReason,
		}
		if len(candidate.Content.Parts) > 0 {
			var contents []model.MessageContent
			var builder strings.Builder
			hasImage := false
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil {
					hasImage = true
					break
				}
			}
			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					toolCall, err := getToolCall(&part)
					if err != nil {
						log.Error("get tool call failed: " + err.Error())
					}
					if toolCall != nil {
						choice.Message.ToolCalls = append(choice.Message.ToolCalls, toolCall)
					}
				}
				if part.Text != "" {
					if hasImage {
						contents = append(contents, model.MessageContent{
							Type: model.ContentTypeText,
							Text: part.Text,
						})
					} else {
						builder.WriteString(part.Text)
					}
				}
				if part.InlineData != nil {
					contents = append(contents, model.MessageContent{
						Type: model.ContentTypeImageURL,
						ImageURL: &model.ImageURL{
							URL: fmt.Sprintf("data:%s;base64,%s", part.InlineData.MimeType, part.InlineData.Data),
						},
					})
				}
			}
			if hasImage {
				choice.Message.Content = contents
			} else {
				choice.Message.Content = builder.String()
			}
		}
		fullTextResponse.Choices = append(fullTextResponse.Choices, &choice)
	}
	return &fullTextResponse
}

func streamResponseChat2OpenAI(meta *meta.Meta, geminiResponse *ChatResponse) *model.ChatCompletionsStreamResponse {
	response := &model.ChatCompletionsStreamResponse{
		ID:      openai.ChatCompletionID(),
		Created: time.Now().Unix(),
		Model:   meta.OriginModel,
		Object:  model.ChatCompletionChunk,
		Choices: make([]*model.ChatCompletionsStreamResponseChoice, 0, len(geminiResponse.Candidates)),
	}
	if geminiResponse.UsageMetadata != nil {
		response.Usage = &model.Usage{
			PromptTokens:     geminiResponse.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResponse.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResponse.UsageMetadata.TotalTokenCount,
		}
	}
	for i, candidate := range geminiResponse.Candidates {
		choice := model.ChatCompletionsStreamResponseChoice{
			Index: i,
			Delta: model.Message{
				Content: "",
			},
			FinishReason: &candidate.FinishReason,
		}
		if len(candidate.Content.Parts) > 0 {
			var contents []model.MessageContent
			var builder strings.Builder
			hasImage := false
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil {
					hasImage = true
					break
				}
			}
			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					toolCall, err := getToolCall(&part)
					if err != nil {
						log.Error("get tool call failed: " + err.Error())
					}
					if toolCall != nil {
						choice.Delta.ToolCalls = append(choice.Delta.ToolCalls, toolCall)
					}
				}
				if part.Text != "" {
					if hasImage {
						contents = append(contents, model.MessageContent{
							Type: model.ContentTypeText,
							Text: part.Text,
						})
					} else {
						builder.WriteString(part.Text)
					}
				}
				if part.InlineData != nil {
					contents = append(contents, model.MessageContent{
						Type: model.ContentTypeImageURL,
						ImageURL: &model.ImageURL{
							URL: fmt.Sprintf("data:%s;base64,%s", part.InlineData.MimeType, part.InlineData.Data),
						},
					})
				}
			}
			if hasImage {
				choice.Delta.Content = contents
			} else {
				choice.Delta.Content = builder.String()
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
	return scannerBufferPool.Get().(*[]byte)
}

func PutImageScannerBuffer(buf *[]byte) {
	if cap(*buf) != imageScannerBufferSize {
		return
	}
	scannerBufferPool.Put(buf)
}

func StreamHandler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *model.ErrorWithStatusCode) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, openai.ErrorHanlder(resp)
	}

	log := middleware.GetLogger(c)

	responseText := strings.Builder{}

	scanner := bufio.NewScanner(resp.Body)
	if strings.Contains(meta.ActualModel, "image") {
		buf := GetImageScannerBuffer()
		defer PutImageScannerBuffer(buf)
		scanner.Buffer(*buf, cap(*buf))
	} else {
		buf := openai.GetScannerBuffer()
		defer openai.PutScannerBuffer(buf)
		scanner.Buffer(*buf, cap(*buf))
	}

	common.SetEventStreamHeaders(c)

	usage := model.Usage{
		PromptTokens: meta.InputTokens,
	}

	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) < 6 || conv.BytesToString(data[:6]) != "data: " {
			continue
		}
		data = data[6:]

		if conv.BytesToString(data) == "[DONE]" {
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

		responseText.WriteString(response.Choices[0].Delta.StringContent())

		_ = render.ObjectData(c, response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	render.Done(c)

	return &usage, nil
}

func Handler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *model.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	var geminiResponse ChatResponse
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&geminiResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	fullTextResponse := responseChat2OpenAI(meta, &geminiResponse)
	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return &fullTextResponse.Usage, nil
}
