// Package aws provides the AWS adaptor for the relay service.
package aws

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/render"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/aws/utils"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type awsModelItem struct {
	ID string
	model.ModelConfig
}

// AwsModelIDMap maps internal model identifiers to AWS model identifiers.
// It currently supports only llama-3-8b and llama-3-70b instruction models.
// For more details, see: https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html
var AwsModelIDMap = map[string]awsModelItem{
	"llama3-8b-8192": {
		ModelConfig: model.ModelConfig{
			Model: "llama3-8b-8192",
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerMeta,
		},
		ID: "meta.llama3-8b-instruct-v1:0",
	},
	"llama3-70b-8192": {
		ModelConfig: model.ModelConfig{
			Model: "llama3-70b-8192",
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerMeta,
		},
		ID: "meta.llama3-70b-instruct-v1:0",
	},
}

func awsModelID(requestModel string) (string, error) {
	if awsModelID, ok := AwsModelIDMap[requestModel]; ok {
		return awsModelID.ID, nil
	}

	return "", errors.Errorf("model %s not found", requestModel)
}

// promptTemplate with range
const promptTemplate = `<|begin_of_text|>{{range .Messages}}<|start_header_id|>{{.Role}}<|end_header_id|>{{.StringContent}}<|eot_id|>{{end}}<|start_header_id|>assistant<|end_header_id|>
`

var promptTpl = template.Must(template.New("llama3-chat").Parse(promptTemplate))

func RenderPrompt(messages []*relaymodel.Message) string {
	var buf bytes.Buffer
	err := promptTpl.Execute(&buf, struct{ Messages []*relaymodel.Message }{messages})
	if err != nil {
		log.Error("error rendering prompt messages: " + err.Error())
	}
	return buf.String()
}

func ConvertRequest(textRequest *relaymodel.GeneralOpenAIRequest) *Request {
	llamaRequest := Request{
		MaxGenLen:   textRequest.MaxTokens,
		Temperature: textRequest.Temperature,
		TopP:        textRequest.TopP,
	}
	if llamaRequest.MaxGenLen == 0 {
		llamaRequest.MaxGenLen = 2048
	}
	prompt := RenderPrompt(textRequest.Messages)
	llamaRequest.Prompt = prompt
	return &llamaRequest
}

func Handler(meta *meta.Meta, c *gin.Context) (model.Usage, adaptor.Error) {
	awsModelID, err := awsModelID(meta.ActualModel)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	awsReq := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(awsModelID),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	llamaReq, ok := meta.Get(ConvertedRequest)
	if !ok {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"request not found",
			nil,
			http.StatusInternalServerError,
		)
	}

	awsReq.Body, err = sonic.Marshal(llamaReq)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	awsClient, err := utils.AwsClientFromMeta(meta)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	awsResp, err := awsClient.InvokeModel(c.Request.Context(), awsReq)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	var llamaResponse Response
	err = sonic.Unmarshal(awsResp.Body, &llamaResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	openaiResp := ResponseLlama2OpenAI(meta, llamaResponse)

	jsonData, err := sonic.Marshal(llamaResponse)
	if err != nil {
		return openaiResp.ToModelUsage(), relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonData)))
	_, _ = c.Writer.Write(jsonData)
	return openaiResp.ToModelUsage(), nil
}

func ResponseLlama2OpenAI(meta *meta.Meta, llamaResponse Response) relaymodel.TextResponse {
	var responseText string
	if len(llamaResponse.Generation) > 0 {
		responseText = llamaResponse.Generation
	}
	choice := relaymodel.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:    "assistant",
			Content: responseText,
			Name:    nil,
		},
		FinishReason: llamaResponse.StopReason,
	}
	fullTextResponse := relaymodel.TextResponse{
		ID:      openai.ChatCompletionID(),
		Object:  relaymodel.ChatCompletionObject,
		Created: time.Now().Unix(),
		Choices: []*relaymodel.TextResponseChoice{&choice},
		Model:   meta.OriginModel,
		Usage: relaymodel.Usage{
			PromptTokens:     llamaResponse.PromptTokenCount,
			CompletionTokens: llamaResponse.GenerationTokenCount,
			TotalTokens:      llamaResponse.PromptTokenCount + llamaResponse.GenerationTokenCount,
		},
	}
	return fullTextResponse
}

func StreamHandler(meta *meta.Meta, c *gin.Context) (model.Usage, adaptor.Error) {
	log := middleware.GetLogger(c)

	createdTime := time.Now().Unix()
	awsModelID, err := awsModelID(meta.ActualModel)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	awsReq := &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(awsModelID),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	llamaReq, ok := meta.Get(ConvertedRequest)
	if !ok {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"request not found",
			nil,
			http.StatusInternalServerError,
		)
	}

	awsReq.Body, err = sonic.Marshal(llamaReq)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	awsClient, err := utils.AwsClientFromMeta(meta)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	awsResp, err := awsClient.InvokeModelWithResponseStream(c.Request.Context(), awsReq)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}
	stream := awsResp.GetStream()
	defer stream.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	var usage relaymodel.Usage
	c.Stream(func(_ io.Writer) bool {
		event, ok := <-stream.Events()
		if !ok {
			render.Done(c)
			return false
		}

		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			var llamaResp StreamResponse
			err := sonic.Unmarshal(v.Value.Bytes, &llamaResp)
			if err != nil {
				log.Error("error unmarshalling stream response: " + err.Error())
				return false
			}

			if llamaResp.PromptTokenCount > 0 {
				usage.PromptTokens = llamaResp.PromptTokenCount
			}
			if llamaResp.StopReason == relaymodel.FinishReasonStop {
				usage.CompletionTokens = llamaResp.GenerationTokenCount
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
			response := StreamResponseLlama2OpenAI(&llamaResp)
			response.ID = openai.ChatCompletionID()
			response.Model = meta.OriginModel
			response.Created = createdTime
			err = render.ObjectData(c, response)
			if err != nil {
				log.Error("error stream response: " + err.Error())
				return true
			}
			return true
		case *types.UnknownUnionMember:
			log.Error("unknown tag: " + v.Tag)
			return false
		default:
			log.Errorf("union is nil or unknown type: %v", v)
			return false
		}
	})

	return usage.ToModelUsage(), nil
}

func StreamResponseLlama2OpenAI(
	llamaResponse *StreamResponse,
) *relaymodel.ChatCompletionsStreamResponse {
	var choice relaymodel.ChatCompletionsStreamResponseChoice
	choice.Delta.Content = llamaResponse.Generation
	choice.Delta.Role = "assistant"
	finishReason := llamaResponse.StopReason
	if finishReason != "null" {
		choice.FinishReason = finishReason
	}
	var openaiResponse relaymodel.ChatCompletionsStreamResponse
	openaiResponse.Object = relaymodel.ChatCompletionChunkObject
	openaiResponse.Choices = []*relaymodel.ChatCompletionsStreamResponseChoice{&choice}
	return &openaiResponse
}
