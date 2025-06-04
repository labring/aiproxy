package baidu

import (
	"io"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type RerankResponse struct {
	Error *Error           `json:"error"`
	Usage relaymodel.Usage `json:"usage"`
}

func RerankHandler(
	_ *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	reRankResp := &RerankResponse{}
	err = sonic.Unmarshal(respBody, reRankResp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	if reRankResp.Error != nil && reRankResp.Error.ErrorCode != 0 {
		return model.Usage{}, ErrorHandler(reRankResp.Error)
	}
	respMap := make(map[string]any)
	err = sonic.Unmarshal(respBody, &respMap)
	if err != nil {
		return reRankResp.Usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	delete(respMap, "model")
	delete(respMap, "usage")
	respMap["meta"] = &relaymodel.RerankMeta{
		Tokens: &relaymodel.RerankMetaTokens{
			InputTokens:  reRankResp.Usage.TotalTokens,
			OutputTokens: 0,
		},
	}
	respMap["result"] = respMap["results"]
	delete(respMap, "results")
	jsonData, err := sonic.Marshal(respMap)
	if err != nil {
		return reRankResp.Usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonData)))
	_, err = c.Writer.Write(jsonData)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}
	return reRankResp.Usage.ToModelUsage(), nil
}
