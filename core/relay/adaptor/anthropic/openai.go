package anthropic

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
	"golang.org/x/sync/semaphore"
)

const (
	toolUseType             = "tool_use"
	serverToolUseType       = "server_tool_use"
	webSearchToolResult     = "web_search_tool_result"
	codeExecutionToolResult = "code_execution_tool_result"
	conetentTypeText        = "text"
	conetentTypeThinking    = "thinking"
	conetentTypeImage       = "image"
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

//nolint:gocyclo
func OpenAIConvertRequest(meta *meta.Meta, req *http.Request) (*relaymodel.ClaudeRequest, error) {
	var textRequest relaymodel.ClaudeOpenAIRequest

	err := common.UnmarshalRequestReusable(req, &textRequest)
	if err != nil {
		return nil, err
	}

	onlyThinking, err := utils.UnmarshalGeneralThinking(req)
	if err != nil {
		return nil, err
	}

	textRequest.Model = meta.ActualModel
	claudeTools := make([]relaymodel.ClaudeTool, 0, len(textRequest.Tools))

	for _, tool := range textRequest.Tools {
		if tool.Type != "function" {
			claudeTools = append(claudeTools, relaymodel.ClaudeTool{
				Type:            tool.Type,
				Name:            tool.Name,
				DisplayWidthPx:  tool.DisplayWidthPx,
				DisplayHeightPx: tool.DisplayHeightPx,
				DisplayNumber:   tool.DisplayNumber,
				CacheControl:    tool.CacheControl.ResetTTL(),

				MaxUses:        tool.MaxUses,
				AllowedDomains: tool.AllowedDomains,
				BlockedDomains: tool.BlockedDomains,
				UserLocation:   tool.UserLocation,
			})
		} else {
			if params, ok := tool.Function.Parameters.(map[string]any); ok {
				t, _ := params["type"].(string)
				if t == "" {
					t = "object"
				}

				claudeTools = append(claudeTools, relaymodel.ClaudeTool{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					InputSchema: &relaymodel.ClaudeInputSchema{
						Type:       t,
						Properties: params["properties"],
						Required:   params["required"],
					},
					CacheControl: tool.CacheControl.ResetTTL(),

					MaxUses:        tool.MaxUses,
					AllowedDomains: tool.AllowedDomains,
					BlockedDomains: tool.BlockedDomains,
					UserLocation:   tool.UserLocation,
				})
			}
		}
	}

	claudeRequest := relaymodel.ClaudeRequest{
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
		if claudeRequest.Thinking.Type == "disabled" {
			claudeRequest.Thinking = nil
		}
	} else if strings.Contains(meta.OriginModel, "think") {
		claudeRequest.Thinking = &relaymodel.ClaudeThinking{
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
		}{Type: "auto"}
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

	var imageTasks []*relaymodel.ClaudeContent

	hasToolCalls := false

	for _, message := range textRequest.Messages {
		if message.Role == "system" {
			claudeRequest.System = append(claudeRequest.System, relaymodel.ClaudeContent{
				Type:         conetentTypeText,
				Text:         message.StringContent(),
				CacheControl: message.CacheControl.ResetTTL(),
			})

			continue
		}

		claudeMessage := relaymodel.ClaudeMessage{
			Role: message.Role,
		}

		var content relaymodel.ClaudeContent

		content.CacheControl = message.CacheControl.ResetTTL()
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

			//nolint:staticcheck
			if !(message.Role == "assistant" && content.Text == "" && len(message.ToolCalls) > 0) {
				claudeMessage.Content = append(claudeMessage.Content, content)
			}
		} else {
			var contents []relaymodel.ClaudeContent

			openaiContent := message.ParseContent()
			for _, part := range openaiContent {
				if message.Role == "assistant" && part.Text == "" && len(message.ToolCalls) > 0 {
					continue
				}

				var content relaymodel.ClaudeContent
				switch part.Type {
				case relaymodel.ContentTypeText:
					content.Type = conetentTypeText
					content.Text = part.Text
				case relaymodel.ContentTypeImageURL:
					content.Type = conetentTypeImage
					content.Source = &relaymodel.ClaudeImageSource{
						Type: "url",
						URL:  part.ImageURL.URL,
					}
					imageTasks = append(imageTasks, &content)
				}

				contents = append(contents, content)
			}

			claudeMessage.Content = contents
		}

		for _, toolCall := range message.ToolCalls {
			hasToolCalls = true
			inputParam := make(map[string]any)
			_ = sonic.UnmarshalString(toolCall.Function.Arguments, &inputParam)
			claudeMessage.Content = append(claudeMessage.Content, relaymodel.ClaudeContent{
				Type:  toolUseType,
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: inputParam,
			})
		}

		claudeRequest.Messages = append(claudeRequest.Messages, claudeMessage)
	}

	if len(imageTasks) > 0 {
		err := batchPatchImage2Base64(req.Context(), imageTasks)
		if err != nil {
			return nil, err
		}
	}

	if hasToolCalls {
		claudeRequest.Thinking = nil
	}

	return &claudeRequest, nil
}

func batchPatchImage2Base64(ctx context.Context, imageTasks []*relaymodel.ClaudeContent) error {
	sem := semaphore.NewWeighted(3)

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		processErrs []error
	)

	for _, task := range imageTasks {
		if task.Source.URL == "" {
			continue
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			_ = sem.Acquire(ctx, 1)
			defer sem.Release(1)

			mimeType, data, err := image.GetImageFromURL(ctx, task.Source.URL)
			if err != nil {
				mu.Lock()

				processErrs = append(processErrs, err)

				mu.Unlock()

				return
			}

			task.Source.Type = "base64"
			task.Source.URL = ""
			task.Source.MediaType = mimeType
			task.Source.Data = data
		}()
	}

	wg.Wait()

	if len(processErrs) != 0 {
		return errors.Join(processErrs...)
	}

	return nil
}

// https://docs.anthropic.com/claude/reference/messages-streaming
func StreamResponse2OpenAI(
	meta *meta.Meta,
	respData []byte,
) (*relaymodel.ChatCompletionsStreamResponse, adaptor.Error) {
	var (
		usage      *relaymodel.ChatUsage
		content    string
		thinking   string
		stopReason string
	)

	tools := make([]relaymodel.ToolCall, 0)

	var claudeResponse relaymodel.ClaudeStreamResponse

	err := sonic.Unmarshal(respData, &claudeResponse)
	if err != nil {
		return nil, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response",
			http.StatusInternalServerError,
		)
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
				tools = append(tools, relaymodel.ToolCall{
					Index: claudeResponse.Index,
					ID:    claudeResponse.ContentBlock.ID,
					Type:  "function",
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
				tools = append(tools, relaymodel.ToolCall{
					Index: claudeResponse.Index,
					Type:  "function",
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

		openAIUsage := claudeResponse.Message.Usage.ToOpenAIUsage()
		usage = &openAIUsage
	case "message_delta":
		if claudeResponse.Usage != nil {
			openAIUsage := claudeResponse.Usage.ToOpenAIUsage()
			usage = &openAIUsage
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
		Object:  relaymodel.ChatCompletionChunkObject,
		Created: time.Now().Unix(),
		Model:   meta.OriginModel,
		Usage:   usage,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{&choice},
	}

	return &openaiResponse, nil
}

func Response2OpenAI(
	meta *meta.Meta,
	respData []byte,
) (*relaymodel.TextResponse, adaptor.Error) {
	var claudeResponse relaymodel.ClaudeResponse

	err := sonic.Unmarshal(respData, &claudeResponse)
	if err != nil {
		return nil, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if claudeResponse.Type == "error" {
		return nil, OpenAIErrorHandlerWithBody(
			http.StatusBadRequest,
			respData,
		)
	}

	var (
		content  string
		thinking string
	)

	tools := make([]relaymodel.ToolCall, 0)
	for _, v := range claudeResponse.Content {
		switch v.Type {
		case conetentTypeText:
			content = v.Text
		case conetentTypeThinking:
			thinking = v.Thinking
		case toolUseType:
			args, _ := sonic.MarshalString(v.Input)
			tools = append(tools, relaymodel.ToolCall{
				ID:   v.ID,
				Type: "function",
				Function: relaymodel.Function{
					Name:      v.Name,
					Arguments: args,
				},
			})
		case serverToolUseType:
		case webSearchToolResult:
		case codeExecutionToolResult:
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
		Object:  relaymodel.ChatCompletionObject,
		Created: time.Now().Unix(),
		Choices: []*relaymodel.TextResponseChoice{&choice},
		Usage:   claudeResponse.Usage.ToOpenAIUsage(),
	}
	if fullTextResponse.Usage.PromptTokens == 0 {
		fullTextResponse.Usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
	}

	fullTextResponse.Usage.TotalTokens = fullTextResponse.Usage.PromptTokens + fullTextResponse.Usage.CompletionTokens

	return &fullTextResponse, nil
}

func OpenAIStreamHandler(
	m *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, OpenAIErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner := bufio.NewScanner(resp.Body)

	buf := utils.GetScannerBuffer()
	defer utils.PutScannerBuffer(buf)

	scanner.Buffer(*buf, cap(*buf))

	responseText := strings.Builder{}

	var (
		usage  *relaymodel.ChatUsage
		writed bool
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

		response, err := StreamResponse2OpenAI(m, data)
		if err != nil {
			if writed {
				log.Errorf("response error: %+v", err)
				continue
			}

			if usage == nil {
				usage = &relaymodel.ChatUsage{}
			}

			if response != nil && response.Usage != nil {
				usage.Add(response.Usage)
			}

			return usage.ToModelUsage(), err
		}

		if response == nil {
			continue
		}

		switch {
		case response.Usage != nil:
			if usage == nil {
				usage = &relaymodel.ChatUsage{}
			}

			usage.Add(response.Usage)

			if usage.PromptTokens == 0 {
				usage.PromptTokens = int64(m.RequestUsage.InputTokens)
				usage.TotalTokens += int64(m.RequestUsage.InputTokens)
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

		_ = render.OpenaiObjectData(c, response)
		writed = true
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	if usage == nil {
		usage = &relaymodel.ChatUsage{
			PromptTokens:     int64(m.RequestUsage.InputTokens),
			CompletionTokens: openai.CountTokenText(responseText.String(), m.OriginModel),
			TotalTokens: int64(
				m.RequestUsage.InputTokens,
			) + openai.CountTokenText(
				responseText.String(),
				m.OriginModel,
			),
		}
		_ = render.OpenaiObjectData(c, &relaymodel.ChatCompletionsStreamResponse{
			ID:      openai.ChatCompletionID(),
			Model:   m.OriginModel,
			Object:  relaymodel.ChatCompletionChunkObject,
			Created: time.Now().Unix(),
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{},
			Usage:   usage,
		})
	}

	render.OpenaiDone(c)

	return usage.ToModelUsage(), nil
}

func OpenAIHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, OpenAIErrorHandler(resp)
	}

	defer resp.Body.Close()

	body, err := common.GetResponseBody(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	fullTextResponse, adaptorErr := Response2OpenAI(meta, body)
	if adaptorErr != nil {
		return model.Usage{}, adaptorErr
	}

	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
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
