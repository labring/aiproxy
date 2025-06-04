package cohere

import (
	"bufio"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/render"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

var WebSearchConnector = Connector{ID: "web-search"}

func stopReasonCohere2OpenAI(reason *string) string {
	if reason == nil {
		return ""
	}
	switch *reason {
	case "COMPLETE":
		return relaymodel.FinishReasonStop
	default:
		return *reason
	}
}

func ConvertRequest(textRequest *relaymodel.GeneralOpenAIRequest) *Request {
	cohereRequest := Request{
		Model:            textRequest.Model,
		Message:          "",
		MaxTokens:        textRequest.MaxTokens,
		Temperature:      textRequest.Temperature,
		P:                textRequest.TopP,
		K:                textRequest.TopK,
		Stream:           textRequest.Stream,
		FrequencyPenalty: textRequest.FrequencyPenalty,
		PresencePenalty:  textRequest.PresencePenalty,
		Seed:             int(textRequest.Seed),
	}
	if cohereRequest.Model == "" {
		cohereRequest.Model = "command-r"
	}
	if strings.HasSuffix(cohereRequest.Model, "-internet") {
		cohereRequest.Model = strings.TrimSuffix(cohereRequest.Model, "-internet")
		cohereRequest.Connectors = append(cohereRequest.Connectors, WebSearchConnector)
	}
	for _, message := range textRequest.Messages {
		messageContent, _ := message.Content.(string)
		if message.Role == "user" {
			cohereRequest.Message = messageContent
		} else {
			var role string
			switch message.Role {
			case "assistant":
				role = "CHATBOT"
			case "system":
				role = "SYSTEM"
			default:
				role = "USER"
			}
			cohereRequest.ChatHistory = append(cohereRequest.ChatHistory, ChatMessage{
				Role:    role,
				Message: messageContent,
			})
		}
	}
	return &cohereRequest
}

func StreamResponse2OpenAI(
	meta *meta.Meta,
	cohereResponse *StreamResponse,
) *relaymodel.ChatCompletionsStreamResponse {
	var response *Response
	var responseText string
	var finishReason string

	switch cohereResponse.EventType {
	case "stream-start":
		return nil
	case "text-generation":
		responseText += cohereResponse.Text
	case "stream-end":
		usage := cohereResponse.Response.Meta.Tokens
		response = &Response{
			Meta: Meta{
				Tokens: Usage{
					InputTokens:  usage.InputTokens,
					OutputTokens: usage.OutputTokens,
				},
			},
		}
		finishReason = *cohereResponse.Response.FinishReason
	default:
		return nil
	}

	var choice relaymodel.ChatCompletionsStreamResponseChoice
	choice.Delta.Content = responseText
	choice.Delta.Role = "assistant"
	if finishReason != "" {
		choice.FinishReason = finishReason
	}
	openaiResponse := relaymodel.ChatCompletionsStreamResponse{
		ID:      "chatcmpl-" + cohereResponse.GenerationID,
		Model:   meta.OriginModel,
		Created: time.Now().Unix(),
		Object:  relaymodel.ChatCompletionChunkObject,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{&choice},
	}
	if response != nil {
		openaiResponse.Usage = &relaymodel.Usage{
			PromptTokens:     response.Meta.Tokens.InputTokens,
			CompletionTokens: response.Meta.Tokens.OutputTokens,
			TotalTokens:      response.Meta.Tokens.InputTokens + response.Meta.Tokens.OutputTokens,
		}
	}
	return &openaiResponse
}

func Response2OpenAI(meta *meta.Meta, cohereResponse *Response) *relaymodel.TextResponse {
	choice := relaymodel.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:    "assistant",
			Content: cohereResponse.Text,
			Name:    nil,
		},
		FinishReason: stopReasonCohere2OpenAI(cohereResponse.FinishReason),
	}
	fullTextResponse := relaymodel.TextResponse{
		ID:      openai.ChatCompletionID(),
		Model:   meta.OriginModel,
		Object:  relaymodel.ChatCompletionObject,
		Created: time.Now().Unix(),
		Choices: []*relaymodel.TextResponseChoice{&choice},
		Usage: relaymodel.Usage{
			PromptTokens:     cohereResponse.Meta.Tokens.InputTokens,
			CompletionTokens: cohereResponse.Meta.Tokens.OutputTokens,
			TotalTokens:      cohereResponse.Meta.Tokens.InputTokens + cohereResponse.Meta.Tokens.OutputTokens,
		},
	}
	return &fullTextResponse
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

	log := middleware.GetLogger(c)

	scanner := bufio.NewScanner(resp.Body)
	buf := openai.GetScannerBuffer()
	defer openai.PutScannerBuffer(buf)
	scanner.Buffer(*buf, cap(*buf))

	var usage relaymodel.Usage

	for scanner.Scan() {
		data := scanner.Text()
		data = strings.TrimSuffix(data, "\r")

		var cohereResponse StreamResponse
		err := sonic.Unmarshal(conv.StringToBytes(data), &cohereResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := StreamResponse2OpenAI(meta, &cohereResponse)
		if response.Usage != nil {
			usage = *response.Usage
		}

		_ = render.ObjectData(c, response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	render.Done(c)

	return usage.ToModelUsage(), nil
}

func Handler(meta *meta.Meta, c *gin.Context, resp *http.Response) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	var cohereResponse Response
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&cohereResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	if cohereResponse.ResponseID == "" {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			cohereResponse.Message,
			resp.StatusCode,
			resp.StatusCode,
		)
	}
	fullTextResponse := Response2OpenAI(meta, &cohereResponse)
	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return fullTextResponse.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)
	return fullTextResponse.ToModelUsage(), nil
}
