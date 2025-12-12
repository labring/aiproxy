package coze

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/coze/constant/messagetype"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

// https://www.coze.com/open

func stopReasonCoze2OpenAI(reason *string) relaymodel.FinishReason {
	if reason == nil {
		return ""
	}

	switch *reason {
	case relaymodel.ClaudeStopReasonEndTurn:
		return relaymodel.FinishReasonLength
	case relaymodel.ClaudeStopReasonStopSequence:
		return relaymodel.FinishReasonStop
	case relaymodel.ClaudeStopReasonMaxTokens:
		return relaymodel.FinishReasonLength
	default:
		return *reason
	}
}

func StreamResponse2OpenAI(
	meta *meta.Meta,
	cozeResponse *StreamResponse,
) *relaymodel.ChatCompletionsStreamResponse {
	var (
		stopReason string
		choice     relaymodel.ChatCompletionsStreamResponseChoice
	)

	if cozeResponse.Message != nil {
		if cozeResponse.Message.Type != messagetype.Answer {
			return nil
		}

		choice.Delta.Content = cozeResponse.Message.Content
	}

	choice.Delta.Role = "assistant"

	finishReason := stopReasonCoze2OpenAI(&stopReason)
	if finishReason != "null" {
		choice.FinishReason = finishReason
	}

	openaiResponse := relaymodel.ChatCompletionsStreamResponse{
		ID:      cozeResponse.ConversationID,
		Model:   meta.OriginModel,
		Created: time.Now().Unix(),
		Object:  relaymodel.ChatCompletionChunkObject,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{&choice},
	}

	return &openaiResponse
}

func Response2OpenAI(meta *meta.Meta, cozeResponse *Response) *relaymodel.TextResponse {
	var responseText string
	for _, message := range cozeResponse.Messages {
		if message.Type == messagetype.Answer {
			responseText = message.Content
			break
		}
	}

	choice := relaymodel.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:    "assistant",
			Content: responseText,
			Name:    nil,
		},
		FinishReason: relaymodel.FinishReasonStop,
	}
	fullTextResponse := relaymodel.TextResponse{
		ID:      openai.ChatCompletionID(),
		Model:   meta.OriginModel,
		Object:  relaymodel.ChatCompletionObject,
		Created: time.Now().Unix(),
		Choices: []*relaymodel.TextResponseChoice{&choice},
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

	log := common.GetLogger(c)

	responseText := strings.Builder{}
	createdTime := time.Now().Unix()

	scanner, cleanup := utils.NewScanner(resp.Body)
	defer cleanup()

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var cozeResponse StreamResponse

		err := sonic.Unmarshal(data, &cozeResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := StreamResponse2OpenAI(meta, &cozeResponse)
		if response == nil {
			continue
		}

		for _, choice := range response.Choices {
			responseText.WriteString(choice.Delta.StringContent())
		}

		response.Model = meta.OriginModel
		response.Created = createdTime

		_ = render.OpenaiObjectData(c, response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	render.OpenaiDone(c)

	return openai.ResponseText2Usage(
		responseText.String(),
		meta.ActualModel,
		int64(meta.RequestUsage.InputTokens),
	).ToModelUsage(), nil
}

func Handler(meta *meta.Meta, c *gin.Context, resp *http.Response) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	var cozeResponse Response

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&cozeResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if cozeResponse.Code != 0 {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			cozeResponse.Msg,
			cozeResponse.Code,
			resp.StatusCode,
		)
	}

	fullTextResponse := Response2OpenAI(meta, &cozeResponse)

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

	_, err = c.Writer.Write(jsonResponse)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	var responseText string
	if len(fullTextResponse.Choices) > 0 {
		responseText = fullTextResponse.Choices[0].Message.StringContent()
	}

	return openai.ResponseText2Usage(responseText, meta.ActualModel, int64(meta.RequestUsage.InputTokens)).
			ToModelUsage(),
		nil
}
