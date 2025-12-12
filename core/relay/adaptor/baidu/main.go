package baidu

import (
	"bytes"
	"net/http"
	"strconv"

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

// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/flfmc9do2

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Temperature     *float64             `json:"temperature,omitempty"`
	TopP            *float64             `json:"top_p,omitempty"`
	PenaltyScore    *float64             `json:"penalty_score,omitempty"`
	System          string               `json:"system,omitempty"`
	UserID          string               `json:"user_id,omitempty"`
	Messages        []relaymodel.Message `json:"messages"`
	MaxOutputTokens int                  `json:"max_output_tokens,omitempty"`
	Stream          bool                 `json:"stream,omitempty"`
	DisableSearch   bool                 `json:"disable_search,omitempty"`
	EnableCitation  bool                 `json:"enable_citation,omitempty"`
}

func ConvertRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	request, err := utils.UnmarshalGeneralOpenAIRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	request.Model = meta.ActualModel
	baiduRequest := ChatRequest{
		Messages:        request.Messages,
		Temperature:     request.Temperature,
		TopP:            request.TopP,
		Stream:          request.Stream,
		DisableSearch:   false,
		EnableCitation:  false,
		MaxOutputTokens: request.MaxTokens,
		UserID:          request.User,
	}
	// Convert frequency penalty to penalty score range [1.0, 2.0]
	if request.FrequencyPenalty != nil {
		penaltyScore := *request.FrequencyPenalty
		if penaltyScore < -2.0 {
			penaltyScore = -2.0
		}

		if penaltyScore > 2.0 {
			penaltyScore = 2.0
		}
		// Map [-2.0, 2.0] to [1.0, 2.0]
		mappedScore := (penaltyScore+2.0)/4.0 + 1.0
		baiduRequest.PenaltyScore = &mappedScore
	}

	for i, message := range request.Messages {
		if message.Role == "system" {
			baiduRequest.System = message.StringContent()

			request.Messages = append(request.Messages[:i], request.Messages[i+1:]...)
			break
		}
	}

	data, err := sonic.Marshal(baiduRequest)
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

func response2OpenAI(meta *meta.Meta, response *ChatResponse) *relaymodel.TextResponse {
	choice := relaymodel.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:    "assistant",
			Content: response.Result,
		},
		FinishReason: relaymodel.FinishReasonStop,
	}

	fullTextResponse := relaymodel.TextResponse{
		ID:      response.ID,
		Object:  relaymodel.ChatCompletionObject,
		Created: response.Created,
		Model:   meta.OriginModel,
		Choices: []*relaymodel.TextResponseChoice{&choice},
	}
	if response.Usage != nil {
		fullTextResponse.Usage = *response.Usage
	}

	return &fullTextResponse
}

func streamResponse2OpenAI(
	meta *meta.Meta,
	baiduResponse *ChatStreamResponse,
) *relaymodel.ChatCompletionsStreamResponse {
	var choice relaymodel.ChatCompletionsStreamResponseChoice

	choice.Delta.Content = baiduResponse.Result
	if baiduResponse.IsEnd {
		choice.FinishReason = relaymodel.FinishReasonStop
	}

	response := relaymodel.ChatCompletionsStreamResponse{
		ID:      baiduResponse.ID,
		Object:  relaymodel.ChatCompletionChunkObject,
		Created: baiduResponse.Created,
		Model:   meta.OriginModel,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{&choice},
		Usage:   baiduResponse.Usage,
	}

	return &response
}

func StreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	defer resp.Body.Close()

	log := common.GetLogger(c)

	var usage relaymodel.ChatUsage

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

		var baiduResponse ChatStreamResponse

		err := sonic.Unmarshal(data, &baiduResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := streamResponse2OpenAI(meta, &baiduResponse)
		if response.Usage != nil {
			usage = *response.Usage
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
	defer resp.Body.Close()

	var baiduResponse ChatResponse

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&baiduResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if baiduResponse.Error != nil && baiduResponse.ErrorCode != 0 {
		return model.Usage{}, ErrorHandler(baiduResponse.Error)
	}

	fullTextResponse := response2OpenAI(meta, &baiduResponse)

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
