package ali

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
)

type RerankResponse struct {
	Usage     *RerankUsage `json:"usage"`
	RequestID string       `json:"request_id"`
	Output    RerankOutput `json:"output"`
}
type RerankOutput struct {
	Results []*relaymodel.RerankResult `json:"results"`
}
type RerankUsage struct {
	TotalTokens int64 `json:"total_tokens"`
}

func ConvertRerankRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	reqMap := make(map[string]any)

	err := common.UnmarshalRequestReusable(req, &reqMap)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	reqMap["model"] = meta.ActualModel
	reqMap["input"] = map[string]any{
		"query":     reqMap["query"],
		"documents": reqMap["documents"],
	}
	delete(reqMap, "query")
	delete(reqMap, "documents")

	parameters := make(map[string]any)
	for k, v := range reqMap {
		if k == "model" || k == "input" {
			continue
		}

		parameters[k] = v
		delete(reqMap, k)
	}

	reqMap["parameters"] = parameters

	jsonData, err := sonic.Marshal(reqMap)
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

	var rerankResponse RerankResponse

	err := common.UnmarshalResponse(resp, &rerankResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	rerankResp := relaymodel.RerankResponse{
		Meta: relaymodel.RerankMeta{
			Tokens: &relaymodel.RerankMetaTokens{
				InputTokens:  rerankResponse.Usage.TotalTokens,
				OutputTokens: 0,
			},
		},
		Results: rerankResponse.Output.Results,
		ID:      rerankResponse.RequestID,
	}

	var usage model.Usage
	if rerankResponse.Usage == nil {
		usage = model.Usage{
			InputTokens: meta.RequestUsage.InputTokens,
			TotalTokens: meta.RequestUsage.InputTokens,
		}
	} else {
		usage = model.Usage{
			InputTokens: model.ZeroNullInt64(rerankResponse.Usage.TotalTokens),
			TotalTokens: model.ZeroNullInt64(rerankResponse.Usage.TotalTokens),
		}
	}

	jsonResponse, err := sonic.Marshal(&rerankResp)
	if err != nil {
		return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIError(
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

	return adaptor.DoResponseResult{Usage: usage}, nil
}
