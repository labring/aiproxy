package zhipu

import (
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// https://open.bigmodel.cn/doc/api#chatglm_std
// chatglm_std, chatglm_lite
// https://open.bigmodel.cn/api/paas/v3/model-api/chatglm_std/invoke
// https://open.bigmodel.cn/api/paas/v3/model-api/chatglm_std/sse-invoke

func EmbeddingsHandler(c *gin.Context, resp *http.Response) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	var zhipuResponse EmbeddingResponse
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&zhipuResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	fullTextResponse := embeddingResponseZhipu2OpenAI(&zhipuResponse)
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

func embeddingResponseZhipu2OpenAI(response *EmbeddingResponse) *relaymodel.EmbeddingResponse {
	openAIEmbeddingResponse := relaymodel.EmbeddingResponse{
		Object: "list",
		Data:   make([]*relaymodel.EmbeddingResponseItem, 0, len(response.Embeddings)),
		Model:  response.Model,
		Usage: relaymodel.Usage{
			PromptTokens:     response.PromptTokens,
			CompletionTokens: response.CompletionTokens,
			TotalTokens:      response.TotalTokens,
		},
	}

	for _, item := range response.Embeddings {
		openAIEmbeddingResponse.Data = append(
			openAIEmbeddingResponse.Data,
			&relaymodel.EmbeddingResponseItem{
				Object:    `embedding`,
				Index:     item.Index,
				Embedding: item.Embedding,
			},
		)
	}
	return &openAIEmbeddingResponse
}
