package ollama

import (
	"bytes"
	"net/http"
	"strconv"
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
)

func ConvertRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	var request relaymodel.GeneralOpenAIRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	ollamaRequest := ChatRequest{
		Model: meta.ActualModel,
		Options: &Options{
			Seed:             int(request.Seed),
			Temperature:      request.Temperature,
			TopP:             request.TopP,
			FrequencyPenalty: request.FrequencyPenalty,
			PresencePenalty:  request.PresencePenalty,
			NumPredict:       request.MaxTokens,
			NumCtx:           request.NumCtx,
			Stop:             request.Stop,
		},
		Stream:   request.Stream,
		Messages: make([]Message, 0, len(request.Messages)),
		Prompt:   request.Prompt,
		Tools:    make([]*Tool, 0, len(request.Tools)),
	}

	if request.ResponseFormat != nil {
		if request.ResponseFormat.Type == "json_schema" &&
			request.ResponseFormat.JSONSchema != nil &&
			request.ResponseFormat.JSONSchema.Schema != nil {
			ollamaRequest.Format = request.ResponseFormat.JSONSchema.Schema
		} else if request.ResponseFormat.Type == "json_object" {
			ollamaRequest.Format = "json"
		}
	}

	for _, message := range request.Messages {
		openaiContent := message.ParseContent()

		var (
			imageUrls   []string
			contentText string
		)

		for _, part := range openaiContent {
			switch part.Type {
			case relaymodel.ContentTypeText:
				contentText = part.Text
			case relaymodel.ContentTypeImageURL:
				_, data, err := image.GetImageFromURL(req.Context(), part.ImageURL.URL)
				if err != nil {
					return adaptor.ConvertResult{}, err
				}

				imageUrls = append(imageUrls, data)
			}
		}

		m := Message{
			Role:       message.Role,
			Content:    contentText,
			Images:     imageUrls,
			ToolCallID: message.ToolCallID,
			ToolCalls:  make([]*Tool, 0, len(message.ToolCalls)),
		}
		for _, tool := range message.ToolCalls {
			t := &Tool{
				ID:   tool.ID,
				Type: tool.Type,
				Function: Function{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
			_ = sonic.UnmarshalString(tool.Function.Arguments, &t.Function.Arguments)
			m.ToolCalls = append(m.ToolCalls, t)
		}

		ollamaRequest.Messages = append(ollamaRequest.Messages, m)
	}

	for _, tool := range request.Tools {
		ollamaRequest.Tools = append(ollamaRequest.Tools, &Tool{
			Type: tool.Type,
			Function: Function{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}

	data, err := sonic.Marshal(ollamaRequest)
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

func getToolCalls(ollamaResponse *ChatResponse) []relaymodel.ToolCall {
	if ollamaResponse.Message == nil || len(ollamaResponse.Message.ToolCalls) == 0 {
		return nil
	}

	toolCalls := make([]relaymodel.ToolCall, 0, len(ollamaResponse.Message.ToolCalls))
	for _, tool := range ollamaResponse.Message.ToolCalls {
		argString, err := sonic.MarshalString(tool.Function.Arguments)
		if err != nil {
			continue
		}

		toolCalls = append(toolCalls, relaymodel.ToolCall{
			ID:   openai.CallID(),
			Type: "function",
			Function: relaymodel.Function{
				Name:      tool.Function.Name,
				Arguments: argString,
			},
		})
	}

	return toolCalls
}

func response2OpenAI(meta *meta.Meta, response *ChatResponse) *relaymodel.TextResponse {
	choice := relaymodel.TextResponseChoice{
		Text: response.Response,
	}
	if response.Message != nil {
		choice.Message = relaymodel.Message{
			Role:      response.Message.Role,
			Content:   response.Message.Content,
			ToolCalls: getToolCalls(response),
			ReasoningContent: response.Thinking,
		}
	}

	if response.Done {
		choice.FinishReason = response.DoneReason
	}

	fullTextResponse := relaymodel.TextResponse{
		ID:      openai.ChatCompletionID(),
		Model:   meta.OriginModel,
		Object:  relaymodel.ChatCompletionObject,
		Created: time.Now().Unix(),
		Choices: []*relaymodel.TextResponseChoice{&choice},
		Usage: relaymodel.ChatUsage{
			PromptTokens:     response.PromptEvalCount,
			CompletionTokens: response.EvalCount,
			TotalTokens:      response.PromptEvalCount + response.EvalCount,
		},
	}

	return &fullTextResponse
}

func streamResponse2OpenAI(
	meta *meta.Meta,
	ollamaResponse *ChatResponse,
) *relaymodel.ChatCompletionsStreamResponse {
	choice := relaymodel.ChatCompletionsStreamResponseChoice{
		Text: ollamaResponse.Response,
	}
	if ollamaResponse.Message != nil {
		choice.Delta = relaymodel.Message{
			Role:      ollamaResponse.Message.Role,
			Content:   ollamaResponse.Message.Content,
			ToolCalls: getToolCalls(ollamaResponse),
		}
	}

	if ollamaResponse.Done {
		choice.FinishReason = ollamaResponse.DoneReason
	}

	response := relaymodel.ChatCompletionsStreamResponse{
		ID:      openai.ChatCompletionID(),
		Object:  relaymodel.ChatCompletionChunkObject,
		Created: time.Now().Unix(),
		Model:   meta.OriginModel,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{&choice},
	}

	if ollamaResponse.EvalCount != 0 {
		response.Usage = &relaymodel.ChatUsage{
			PromptTokens:     ollamaResponse.PromptEvalCount,
			CompletionTokens: ollamaResponse.EvalCount,
			TotalTokens:      ollamaResponse.PromptEvalCount + ollamaResponse.EvalCount,
		}
	}

	return &response
}

func StreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	var usage *relaymodel.ChatUsage

	scanner, cleanup := utils.NewScanner(resp.Body)
	defer cleanup()

	for scanner.Scan() {
		data := scanner.Bytes()

		var ollamaResponse ChatResponse

		err := sonic.Unmarshal(data, &ollamaResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := streamResponse2OpenAI(meta, &ollamaResponse)
		if response.Usage != nil {
			usage = response.Usage
		}

		_ = render.OpenaiObjectData(c, response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	render.OpenaiDone(c)

	if usage == nil {
		return meta.RequestUsage, nil
	}

	return usage.ToModelUsage(), nil
}

func ConvertEmbeddingRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	request, err := utils.UnmarshalGeneralOpenAIRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	request.Model = meta.ActualModel

	data, err := sonic.Marshal(&EmbeddingRequest{
		Model: request.Model,
		Input: request.ParseInput(),
		Options: &Options{
			Seed:             int(request.Seed),
			Temperature:      request.Temperature,
			TopP:             request.TopP,
			FrequencyPenalty: request.FrequencyPenalty,
			PresencePenalty:  request.PresencePenalty,
		},
	})
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

func EmbeddingHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var ollamaResponse EmbeddingResponse

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&ollamaResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if ollamaResponse.Error != "" {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			ollamaResponse.Error,
			relaymodel.ErrorTypeUpstream,
			resp.StatusCode,
		)
	}

	fullTextResponse := embeddingResponseOllama2OpenAI(meta, &ollamaResponse)

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

func embeddingResponseOllama2OpenAI(
	meta *meta.Meta,
	response *EmbeddingResponse,
) *relaymodel.EmbeddingResponse {
	openAIEmbeddingResponse := relaymodel.EmbeddingResponse{
		Object: "list",
		Data:   make([]*relaymodel.EmbeddingResponseItem, 0, len(response.Embeddings)),
		Model:  meta.OriginModel,
		Usage: relaymodel.EmbeddingUsage{
			PromptTokens: response.PromptEvalCount,
			TotalTokens:  response.PromptEvalCount,
		},
	}
	for i, embedding := range response.Embeddings {
		openAIEmbeddingResponse.Data = append(
			openAIEmbeddingResponse.Data,
			&relaymodel.EmbeddingResponseItem{
				Object:    "embedding",
				Index:     i,
				Embedding: embedding,
			},
		)
	}

	return &openAIEmbeddingResponse
}

func Handler(meta *meta.Meta, c *gin.Context, resp *http.Response) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var ollamaResponse ChatResponse

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&ollamaResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	fullTextResponse := response2OpenAI(meta, &ollamaResponse)

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
