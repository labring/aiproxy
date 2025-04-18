package anthropic

import (
	"bufio"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/common/render"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

const (
	toolUseType          = "tool_use"
	conetentTypeText     = "text"
	conetentTypeThinking = "thinking"
	conetentTypeImage    = "image"
)

func stopReasonClaude2OpenAI(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return relaymodel.FinishReasonStop
	case "max_tokens":
		return relaymodel.FinishReasonLength
	case toolUseType:
		return relaymodel.FinishReasonToolCalls
	case "null":
		return ""
	default:
		return reason
	}
}

type onlyThinkingRequest struct {
	Thinking *Thinking `json:"thinking,omitempty"`
}

func OpenAIConvertRequest(meta *meta.Meta, req *http.Request) (*Request, error) {
	var textRequest OpenAIRequest
	err := common.UnmarshalBodyReusable(req, &textRequest)
	if err != nil {
		return nil, err
	}

	var onlyThinking onlyThinkingRequest
	err = common.UnmarshalBodyReusable(req, &onlyThinking)
	if err != nil {
		return nil, err
	}

	textRequest.Model = meta.ActualModel
	claudeTools := make([]Tool, 0, len(textRequest.Tools))

	for _, tool := range textRequest.Tools {
		if tool.Type != "function" {
			claudeTools = append(claudeTools, Tool{
				Type:            tool.Type,
				Name:            tool.Name,
				DisplayWidthPx:  tool.DisplayWidthPx,
				DisplayHeightPx: tool.DisplayHeightPx,
				DisplayNumber:   tool.DisplayNumber,
				CacheControl:    tool.CacheControl,
			})
		} else {
			if params, ok := tool.Function.Parameters.(map[string]any); ok {
				t, _ := params["type"].(string)
				if t == "" {
					t = "object"
				}
				claudeTools = append(claudeTools, Tool{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					InputSchema: &InputSchema{
						Type:       t,
						Properties: params["properties"],
						Required:   params["required"],
					},
					CacheControl: tool.CacheControl,
				})
			}
		}
	}

	claudeRequest := Request{
		Model:       meta.ActualModel,
		MaxTokens:   textRequest.MaxTokens,
		Temperature: textRequest.Temperature,
		TopP:        textRequest.TopP,
		TopK:        textRequest.TopK,
		Stream:      textRequest.Stream,
		Tools:       claudeTools,
	}

	if claudeRequest.MaxTokens == 0 {
		claudeRequest.MaxTokens = 4096
	}

	if onlyThinking.Thinking != nil {
		claudeRequest.Thinking = onlyThinking.Thinking
	} else if strings.Contains(meta.OriginModel, "think") {
		claudeRequest.Thinking = &Thinking{
			Type: "enabled",
		}
	}

	if claudeRequest.Thinking != nil {
		if claudeRequest.Thinking.BudgetTokens == 0 ||
			claudeRequest.Thinking.BudgetTokens >= claudeRequest.MaxTokens {
			claudeRequest.Thinking.BudgetTokens = claudeRequest.MaxTokens / 2
		}
		if claudeRequest.Thinking.BudgetTokens < 1024 {
			claudeRequest.Thinking.BudgetTokens = 1024
		}
		claudeRequest.Temperature = nil
	}

	if len(claudeTools) > 0 {
		claudeToolChoice := struct {
			Type string `json:"type"`
			Name string `json:"name,omitempty"`
		}{Type: "auto"} // default value https://docs.anthropic.com/en/docs/build-with-claude/tool-use#controlling-claudes-output
		if choice, ok := textRequest.ToolChoice.(map[string]any); ok {
			if function, ok := choice["function"].(map[string]any); ok {
				claudeToolChoice.Type = "tool"
				name, _ := function["name"].(string)
				claudeToolChoice.Name = name
			}
		} else if toolChoiceType, ok := textRequest.ToolChoice.(string); ok {
			if toolChoiceType == "any" {
				claudeToolChoice.Type = toolChoiceType
			}
		}
		claudeRequest.ToolChoice = claudeToolChoice
	}

	for _, message := range textRequest.Messages {
		if message.Role == "system" {
			claudeRequest.System = append(claudeRequest.System, Content{
				Type:         conetentTypeText,
				Text:         message.StringContent(),
				CacheControl: message.CacheControl,
			})
			continue
		}
		claudeMessage := Message{
			Role: message.Role,
		}
		var content Content
		content.CacheControl = message.CacheControl
		if message.IsStringContent() {
			content.Type = conetentTypeText
			content.Text = message.StringContent()
			if message.Role == "tool" {
				claudeMessage.Role = "user"
				content.Type = "tool_result"
				content.Content = content.Text
				content.Text = ""
				content.ToolUseID = message.ToolCallID
			}
			claudeMessage.Content = append(claudeMessage.Content, content)
		} else {
			var contents []Content
			openaiContent := message.ParseContent()
			for _, part := range openaiContent {
				var content Content
				switch part.Type {
				case relaymodel.ContentTypeText:
					content.Type = conetentTypeText
					content.Text = part.Text
				case relaymodel.ContentTypeImageURL:
					content.Type = conetentTypeImage
					content.Source = &ImageSource{
						Type: "base64",
					}
					mimeType, data, err := image.GetImageFromURL(req.Context(), part.ImageURL.URL)
					if err != nil {
						return nil, err
					}
					content.Source.MediaType = mimeType
					content.Source.Data = data
				}
				contents = append(contents, content)
			}
			claudeMessage.Content = contents
		}

		for _, toolCall := range message.ToolCalls {
			inputParam := make(map[string]any)
			_ = sonic.UnmarshalString(toolCall.Function.Arguments, &inputParam)
			claudeMessage.Content = append(claudeMessage.Content, Content{
				Type:  toolUseType,
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: inputParam,
			})
		}
		claudeRequest.Messages = append(claudeRequest.Messages, claudeMessage)
	}

	return &claudeRequest, nil
}

// https://docs.anthropic.com/claude/reference/messages-streaming
func StreamResponse2OpenAI(meta *meta.Meta, respData []byte) (*relaymodel.ChatCompletionsStreamResponse, *relaymodel.ErrorWithStatusCode) {
	var usage *relaymodel.Usage
	var content string
	var thinking string
	var stopReason string
	tools := make([]*relaymodel.Tool, 0)

	var claudeResponse StreamResponse
	err := sonic.Unmarshal(respData, &claudeResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "unmarshal_response", http.StatusInternalServerError)
	}

	switch claudeResponse.Type {
	case "error":
		return nil, OpenAIErrorHandlerWithBody(
			http.StatusBadRequest,
			respData,
		)
	case "ping", "message_stop", "content_block_stop":
		return nil, nil
	case "content_block_start":
		if claudeResponse.ContentBlock != nil {
			content = claudeResponse.ContentBlock.Text
			if claudeResponse.ContentBlock.Type == toolUseType {
				tools = append(tools, &relaymodel.Tool{
					ID:   claudeResponse.ContentBlock.ID,
					Type: "function",
					Function: relaymodel.Function{
						Name: claudeResponse.ContentBlock.Name,
					},
				})
			}
		}
	case "content_block_delta":
		if claudeResponse.Delta != nil {
			switch claudeResponse.Delta.Type {
			case "input_json_delta":
				tools = append(tools, &relaymodel.Tool{
					Type: "function",
					Function: relaymodel.Function{
						Arguments: claudeResponse.Delta.PartialJSON,
					},
				})
			case "thinking_delta":
				thinking = claudeResponse.Delta.Thinking
			case "signature_delta":
			default:
				content = claudeResponse.Delta.Text
			}
		}
	case "message_start":
		if claudeResponse.Message == nil {
			return nil, nil
		}
		claudeUsage := claudeResponse.Message.Usage
		usage = &relaymodel.Usage{
			PromptTokens:     claudeUsage.InputTokens + claudeUsage.CacheReadInputTokens + claudeUsage.CacheCreationInputTokens,
			CompletionTokens: claudeUsage.OutputTokens,
			PromptTokensDetails: &relaymodel.PromptTokensDetails{
				CachedTokens:        claudeUsage.CacheReadInputTokens,
				CacheCreationTokens: claudeUsage.CacheCreationInputTokens,
			},
		}
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	case "message_delta":
		if claudeResponse.Usage != nil {
			usage = &relaymodel.Usage{
				PromptTokens:     claudeResponse.Usage.InputTokens + claudeResponse.Usage.CacheReadInputTokens + claudeResponse.Usage.CacheCreationInputTokens,
				CompletionTokens: claudeResponse.Usage.OutputTokens,
				PromptTokensDetails: &relaymodel.PromptTokensDetails{
					CachedTokens:        claudeResponse.Usage.CacheReadInputTokens,
					CacheCreationTokens: claudeResponse.Usage.CacheCreationInputTokens,
				},
			}
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
		if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
			stopReason = *claudeResponse.Delta.StopReason
		}
	}

	choice := relaymodel.ChatCompletionsStreamResponseChoice{
		Delta: relaymodel.Message{
			Content:          content,
			ReasoningContent: thinking,
			ToolCalls:        tools,
			Role:             "assistant",
		},
		FinishReason: stopReasonClaude2OpenAI(stopReason),
	}

	openaiResponse := relaymodel.ChatCompletionsStreamResponse{
		ID:      openai.ChatCompletionID(),
		Object:  relaymodel.ChatCompletionChunk,
		Created: time.Now().Unix(),
		Model:   meta.OriginModel,
		Usage:   usage,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{&choice},
	}

	return &openaiResponse, nil
}

func Response2OpenAI(meta *meta.Meta, claudeResponse *Response) *relaymodel.TextResponse {
	var content string
	var thinking string
	tools := make([]*relaymodel.Tool, 0)
	for _, v := range claudeResponse.Content {
		switch v.Type {
		case conetentTypeText:
			content = v.Text
		case conetentTypeThinking:
			thinking = v.Thinking
		case toolUseType:
			args, _ := sonic.MarshalString(v.Input)
			tools = append(tools, &relaymodel.Tool{
				ID:   v.ID,
				Type: "function",
				Function: relaymodel.Function{
					Name:      v.Name,
					Arguments: args,
				},
			})
		}
	}

	choice := relaymodel.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:             "assistant",
			Content:          content,
			ReasoningContent: thinking,
			Name:             nil,
			ToolCalls:        tools,
		},
		FinishReason: stopReasonClaude2OpenAI(claudeResponse.StopReason),
	}

	fullTextResponse := relaymodel.TextResponse{
		ID:      openai.ChatCompletionID(),
		Model:   meta.OriginModel,
		Object:  relaymodel.ChatCompletion,
		Created: time.Now().Unix(),
		Choices: []*relaymodel.TextResponseChoice{&choice},
		Usage: relaymodel.Usage{
			PromptTokens:     claudeResponse.Usage.InputTokens + claudeResponse.Usage.CacheReadInputTokens + claudeResponse.Usage.CacheCreationInputTokens,
			CompletionTokens: claudeResponse.Usage.OutputTokens,
			PromptTokensDetails: &relaymodel.PromptTokensDetails{
				CachedTokens:        claudeResponse.Usage.CacheReadInputTokens,
				CacheCreationTokens: claudeResponse.Usage.CacheCreationInputTokens,
			},
		},
	}
	if fullTextResponse.Usage.PromptTokens == 0 {
		fullTextResponse.Usage.PromptTokens = meta.InputTokens
	}
	fullTextResponse.Usage.TotalTokens = fullTextResponse.Usage.PromptTokens + fullTextResponse.Usage.CompletionTokens
	return &fullTextResponse
}

func OpenAIStreamHandler(m *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *relaymodel.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, OpenAIErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	scanner := bufio.NewScanner(resp.Body)
	buf := openai.GetScannerBuffer()
	defer openai.PutScannerBuffer(buf)
	scanner.Buffer(*buf, cap(*buf))

	responseText := strings.Builder{}

	var usage *relaymodel.Usage
	var writed bool

	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) < 6 || conv.BytesToString(data[:6]) != "data: " {
			continue
		}
		data = data[6:]

		if conv.BytesToString(data) == "[DONE]" {
			break
		}

		response, err := StreamResponse2OpenAI(m, data)
		if err != nil {
			if writed {
				log.Errorf("response error: %+v", err)
				continue
			}
			return usage.ToModelUsage(), err
		}
		if response == nil {
			continue
		}

		switch {
		case response.Usage != nil:
			if usage == nil {
				usage = &relaymodel.Usage{}
			}
			usage.Add(response.Usage)
			if usage.PromptTokens == 0 {
				usage.PromptTokens = m.InputTokens
				usage.TotalTokens += m.InputTokens
			}
			response.Usage = usage
			responseText.Reset()
		case usage == nil:
			for _, choice := range response.Choices {
				responseText.WriteString(choice.Delta.StringContent())
			}
		default:
			response.Usage = usage
		}

		_ = render.ObjectData(c, response)
		writed = true
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	if usage == nil {
		usage = &relaymodel.Usage{
			PromptTokens:     m.InputTokens,
			CompletionTokens: openai.CountTokenText(responseText.String(), m.OriginModel),
			TotalTokens:      m.InputTokens + openai.CountTokenText(responseText.String(), m.OriginModel),
		}
		_ = render.ObjectData(c, &relaymodel.ChatCompletionsStreamResponse{
			ID:      openai.ChatCompletionID(),
			Model:   m.OriginModel,
			Object:  relaymodel.ChatCompletionChunk,
			Created: time.Now().Unix(),
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{},
			Usage:   usage,
		})
	}

	render.Done(c)

	return usage.ToModelUsage(), nil
}

func OpenAIHandler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *relaymodel.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, OpenAIErrorHandler(resp)
	}

	defer resp.Body.Close()

	var claudeResponse Response
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&claudeResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	fullTextResponse := Response2OpenAI(meta, &claudeResponse)
	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return fullTextResponse.Usage.ToModelUsage(), nil
}
