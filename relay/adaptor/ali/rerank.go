package ali

import (
	"bytes"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/common"
	"github.com/labring/aiproxy/middleware"
	"github.com/labring/aiproxy/relay/adaptor/openai"
	"github.com/labring/aiproxy/relay/meta"
	model "github.com/labring/aiproxy/relay/model"
)

type RerankResponse struct {
	Usage     *RerankUsage `json:"usage"`
	RequestID string       `json:"request_id"`
	Output    RerankOutput `json:"output"`
}
type RerankOutput struct {
	Results []*model.RerankResult `json:"results"`
}
type RerankUsage struct {
	TotalTokens int64 `json:"total_tokens"`
}

func ConvertRerankRequest(meta *meta.Meta, req *http.Request) (string, http.Header, io.Reader, error) {
	reqMap := make(map[string]any)
	err := common.UnmarshalBodyReusable(req, &reqMap)
	if err != nil {
		return "", nil, nil, err
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
		return "", nil, nil, err
	}
	return http.MethodPost, nil, bytes.NewReader(jsonData), nil
}

func RerankHandler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *model.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	var rerankResponse RerankResponse
	err = sonic.Unmarshal(responseBody, &rerankResponse)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	c.Writer.WriteHeader(resp.StatusCode)

	rerankResp := model.RerankResponse{
		Meta: model.RerankMeta{
			Tokens: &model.RerankMetaTokens{
				InputTokens:  rerankResponse.Usage.TotalTokens,
				OutputTokens: 0,
			},
		},
		Results: rerankResponse.Output.Results,
		ID:      rerankResponse.RequestID,
	}

	var usage *model.Usage
	if rerankResponse.Usage == nil {
		usage = &model.Usage{
			PromptTokens:     meta.InputTokens,
			CompletionTokens: 0,
			TotalTokens:      meta.InputTokens,
		}
	} else {
		usage = &model.Usage{
			PromptTokens: rerankResponse.Usage.TotalTokens,
			TotalTokens:  rerankResponse.Usage.TotalTokens,
		}
	}

	jsonResponse, err := sonic.Marshal(&rerankResp)
	if err != nil {
		return usage, openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	_, err = c.Writer.Write(jsonResponse)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}
	return usage, nil
}
