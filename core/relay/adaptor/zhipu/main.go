package zhipu

import (
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// https://open.bigmodel.cn/doc/api#chatglm_std
// chatglm_std, chatglm_lite
// https://open.bigmodel.cn/api/paas/v3/model-api/chatglm_std/invoke
// https://open.bigmodel.cn/api/paas/v3/model-api/chatglm_std/sse-invoke

func EmbeddingsHandler(
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	var zhipuResponse EmbeddingResponse

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&zhipuResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	fullTextResponse := embeddingResponseZhipu2OpenAI(&zhipuResponse)

	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return adaptor.DoResponseResult{
				Usage: fullTextResponse.Usage.ToModelUsage(),
			}, relaymodel.WrapperOpenAIError(
				err,
				"marshal_response_body_failed",
				http.StatusInternalServerError,
			)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{Usage: fullTextResponse.Usage.ToModelUsage()}, nil
}

func embeddingResponseZhipu2OpenAI(response *EmbeddingResponse) *relaymodel.EmbeddingResponse {
	openAIEmbeddingResponse := relaymodel.EmbeddingResponse{
		Object: "list",
		Data:   make([]*relaymodel.EmbeddingResponseItem, 0, len(response.Embeddings)),
		Model:  response.Model,
		Usage:  response.Usage,
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
