package jina

import (
	"io"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func RerankHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	node, err := sonic.Get(responseBody)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	var usage relaymodel.Usage
	usageNode := node.Get("usage")
	usageStr, err := usageNode.Raw()
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_usage_failed",
			http.StatusInternalServerError,
		)
	}
	err = sonic.UnmarshalString(usageStr, &usage)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_usage_failed",
			http.StatusInternalServerError,
		)
	}
	if usage.PromptTokens == 0 && usage.TotalTokens != 0 {
		usage.PromptTokens = usage.TotalTokens
	} else if usage.PromptTokens == 0 {
		usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
		usage.TotalTokens = int64(meta.RequestUsage.InputTokens)
	}
	modelUsage := usage.ToModelUsage()
	_, err = node.SetAny("meta", map[string]any{
		"tokens": modelUsage,
	})
	if err != nil {
		return modelUsage, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_usage_failed",
			http.StatusInternalServerError,
		)
	}
	_, err = node.Unset("usage")
	if err != nil {
		return modelUsage, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_usage_failed",
			http.StatusInternalServerError,
		)
	}
	_, err = node.Set("model", ast.NewString(meta.OriginModel))
	if err != nil {
		return modelUsage, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_usage_failed",
			http.StatusInternalServerError,
		)
	}
	respData, err := node.MarshalJSON()
	if err != nil {
		return modelUsage, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_failed",
			http.StatusInternalServerError,
		)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(respData)))
	_, err = c.Writer.Write(respData)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}
	return modelUsage, nil
}
