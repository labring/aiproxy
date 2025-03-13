package ollama

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/common"
	"github.com/labring/aiproxy/common/image"
	"github.com/labring/aiproxy/common/random"
	"github.com/labring/aiproxy/common/render"
	"github.com/labring/aiproxy/common/splitter"
	"github.com/labring/aiproxy/middleware"
	"github.com/labring/aiproxy/relay/adaptor/openai"
	"github.com/labring/aiproxy/relay/meta"
	relaymodel "github.com/labring/aiproxy/relay/model"
	"github.com/labring/aiproxy/relay/utils"
)

func ConvertRequest(meta *meta.Meta, req *http.Request) (string, http.Header, io.Reader, error) {
	var request relaymodel.GeneralOpenAIRequest
	err := common.UnmarshalBodyReusable(req, &request)
	if err != nil {
		return "", nil, nil, err
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
		},
		Stream:   request.Stream,
		Messages: make([]Message, 0, len(request.Messages)),
		Tools:    make([]*Tool, 0, len(request.Tools)),
	}

	for _, message := range request.Messages {
		openaiContent := message.ParseContent()
		var imageUrls []string
		var contentText string
		for _, part := range openaiContent {
			switch part.Type {
			case relaymodel.ContentTypeText:
				contentText = part.Text
			case relaymodel.ContentTypeImageURL:
				_, data, err := image.GetImageFromURL(req.Context(), part.ImageURL.URL)
				if err != nil {
					return "", nil, nil, err
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
		return "", nil, nil, err
	}

	return http.MethodPost, nil, bytes.NewReader(data), nil
}

func getToolCalls(ollamaResponse *ChatResponse) []*relaymodel.Tool {
	if len(ollamaResponse.Message.ToolCalls) == 0 {
		return nil
	}
	toolCalls := make([]*relaymodel.Tool, 0, len(ollamaResponse.Message.ToolCalls))
	for _, tool := range ollamaResponse.Message.ToolCalls {
		argString, err := sonic.MarshalString(tool.Function.Arguments)
		if err != nil {
			continue
		}
		toolCalls = append(toolCalls, &relaymodel.Tool{
			ID:   "call_" + random.GetUUID(),
			Type: "function",
			Function: relaymodel.Function{
				Name:      tool.Function.Name,
				Arguments: argString,
			},
		})
	}
	return toolCalls
}

func responseOllama2OpenAI(meta *meta.Meta, response *ChatResponse) *openai.TextResponse {
	choice := openai.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:      response.Message.Role,
			Content:   response.Message.Content,
			ToolCalls: getToolCalls(response),
		},
	}
	if response.Done {
		choice.FinishReason = response.DoneReason
	}
	fullTextResponse := openai.TextResponse{
		ID:      "chatcmpl-" + random.GetUUID(),
		Model:   meta.OriginModel,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Choices: []*openai.TextResponseChoice{&choice},
		Usage: relaymodel.Usage{
			PromptTokens:     response.PromptEvalCount,
			CompletionTokens: response.EvalCount,
			TotalTokens:      response.PromptEvalCount + response.EvalCount,
		},
	}
	return &fullTextResponse
}

func streamResponseOllama2OpenAI(meta *meta.Meta, ollamaResponse *ChatResponse) *openai.ChatCompletionsStreamResponse {
	choice := openai.ChatCompletionsStreamResponseChoice{
		Delta: relaymodel.Message{
			Role:      ollamaResponse.Message.Role,
			Content:   ollamaResponse.Message.Content,
			ToolCalls: getToolCalls(ollamaResponse),
		},
	}
	if ollamaResponse.Done {
		choice.FinishReason = &ollamaResponse.DoneReason
	}
	response := openai.ChatCompletionsStreamResponse{
		ID:      "chatcmpl-" + random.GetUUID(),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   meta.OriginModel,
		Choices: []*openai.ChatCompletionsStreamResponseChoice{&choice},
	}

	if ollamaResponse.EvalCount != 0 {
		response.Usage = &relaymodel.Usage{
			PromptTokens:     ollamaResponse.PromptEvalCount,
			CompletionTokens: ollamaResponse.EvalCount,
			TotalTokens:      ollamaResponse.PromptEvalCount + ollamaResponse.EvalCount,
		}
	}

	return &response
}

func StreamHandler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*relaymodel.Usage, *relaymodel.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	var usage *relaymodel.Usage
	scanner := bufio.NewScanner(resp.Body)

	common.SetEventStreamHeaders(c)

	var thinkSplitter *splitter.Splitter
	if meta.ChannelConfig.SplitThink {
		thinkSplitter = splitter.NewThinkSplitter()
	}

	for scanner.Scan() {
		data := scanner.Bytes()

		var ollamaResponse ChatResponse
		err := sonic.Unmarshal(data, &ollamaResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := streamResponseOllama2OpenAI(meta, &ollamaResponse)
		if response.Usage != nil {
			usage = response.Usage
		}

		if meta.ChannelConfig.SplitThink {
			openai.StreamSplitThinkModeld(response, thinkSplitter, func(data *openai.ChatCompletionsStreamResponse) {
				_ = render.ObjectData(c, data)
			})
			continue
		}

		_ = render.ObjectData(c, response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	render.Done(c)

	return usage, nil
}

func ConvertEmbeddingRequest(meta *meta.Meta, req *http.Request) (string, http.Header, io.Reader, error) {
	request, err := utils.UnmarshalGeneralOpenAIRequest(req)
	if err != nil {
		return "", nil, nil, err
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
		return "", nil, nil, err
	}
	return http.MethodPost, nil, bytes.NewReader(data), nil
}

func EmbeddingHandler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*relaymodel.Usage, *relaymodel.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var ollamaResponse EmbeddingResponse
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&ollamaResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	if ollamaResponse.Error != "" {
		return nil, openai.ErrorWrapperWithMessage(ollamaResponse.Error, openai.ErrorTypeUpstream, resp.StatusCode)
	}

	fullTextResponse := embeddingResponseOllama2OpenAI(meta, &ollamaResponse)
	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return &fullTextResponse.Usage, nil
}

func embeddingResponseOllama2OpenAI(meta *meta.Meta, response *EmbeddingResponse) *openai.EmbeddingResponse {
	openAIEmbeddingResponse := openai.EmbeddingResponse{
		Object: "list",
		Data:   make([]*openai.EmbeddingResponseItem, 0, len(response.Embeddings)),
		Model:  meta.OriginModel,
		Usage: relaymodel.Usage{
			PromptTokens: response.PromptEvalCount,
			TotalTokens:  response.PromptEvalCount,
		},
	}
	for i, embedding := range response.Embeddings {
		openAIEmbeddingResponse.Data = append(openAIEmbeddingResponse.Data, &openai.EmbeddingResponseItem{
			Object:    "embedding",
			Index:     i,
			Embedding: embedding,
		})
	}
	return &openAIEmbeddingResponse
}

func Handler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*relaymodel.Usage, *relaymodel.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var ollamaResponse ChatResponse
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&ollamaResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if ollamaResponse.Error != "" {
		return nil, openai.ErrorWrapperWithMessage(ollamaResponse.Error, openai.ErrorTypeUpstream, resp.StatusCode)
	}
	fullTextResponse := responseOllama2OpenAI(meta, &ollamaResponse)

	if meta.ChannelConfig.SplitThink {
		openai.SplitThinkModeld(fullTextResponse)
	}

	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return &fullTextResponse.Usage, nil
}
