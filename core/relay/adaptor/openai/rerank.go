package openai

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// need to keep model import for model.ZeroNullInt64

func ConvertRerankRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	jsonData, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonData))},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
}

func RerankHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var rerankResponse relaymodel.SlimRerankResponse

	err = sonic.Unmarshal(responseBody, &rerankResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))

	_, err = c.Writer.Write(responseBody)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	if rerankResponse.Meta.Tokens == nil {
		return adaptor.DoResponseResult{Usage: model.Usage{
			InputTokens: meta.RequestUsage.InputTokens,
			TotalTokens: meta.RequestUsage.InputTokens,
		}}, nil
	}

	if rerankResponse.Meta.Tokens.InputTokens <= 0 {
		rerankResponse.Meta.Tokens.InputTokens = int64(meta.RequestUsage.InputTokens)
	}

	return adaptor.DoResponseResult{Usage: model.Usage{
		InputTokens:  model.ZeroNullInt64(rerankResponse.Meta.Tokens.InputTokens),
		OutputTokens: model.ZeroNullInt64(rerankResponse.Meta.Tokens.OutputTokens),
		TotalTokens: model.ZeroNullInt64(
			rerankResponse.Meta.Tokens.InputTokens + rerankResponse.Meta.Tokens.OutputTokens,
		),
	}}, nil
}
