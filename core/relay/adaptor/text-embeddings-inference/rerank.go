package textembeddingsinference

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ConvertRerankRequest(meta *meta.Meta, req *http.Request) (string, http.Header, io.Reader, error) {
	node, err := common.UnmarshalBody2Node(req)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	// Set the actual model in the request
	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return "", nil, nil, err
	}

	// Get the documents array and rename it to texts
	documentsNode := node.Get("documents")
	if !documentsNode.Exists() {
		return "", nil, nil, errors.New("documents field not found")
	}

	// Set the texts field with the documents value
	_, err = node.Set("texts", *documentsNode)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to set texts field: %w", err)
	}

	// Remove the documents field
	_, err = node.Unset("documents")
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to remove documents field: %w", err)
	}

	returnDocumentsNode := node.Get("return_documents")
	if returnDocumentsNode.Exists() {
		returnDocuments, err := returnDocumentsNode.Bool()
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to unmarshal return_documents field: %w", err)
		}
		_, err = node.Unset("return_documents")
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to remove return_documents field: %w", err)
		}
		_, err = node.Set("return_text", ast.NewBool(returnDocuments))
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to set return_text field: %w", err)
		}
	}

	// Convert back to JSON
	jsonData, err := node.MarshalJSON()
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	return http.MethodPost, nil, bytes.NewReader(jsonData), nil
}

type RerankResponse []RerankResponseItem

type RerankResponseItem struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
	Text  string  `json:"text,omitempty"`
}

func (rri *RerankResponseItem) ToRerankModel() *relaymodel.RerankResult {
	var document *relaymodel.Document
	if rri.Text != "" {
		document = &relaymodel.Document{
			Text: rri.Text,
		}
	}
	return &relaymodel.RerankResult{
		Index:          rri.Index,
		RelevanceScore: rri.Score,
		Document:       document,
	}
}

func RerankHandler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *relaymodel.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, RerankErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	respSlice := RerankResponse{}
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&respSlice)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}

	usage := &model.Usage{
		InputTokens: meta.RequestUsage.InputTokens,
		TotalTokens: meta.RequestUsage.InputTokens,
	}

	results := make([]*relaymodel.RerankResult, len(respSlice))
	for i, v := range respSlice {
		results[i] = v.ToRerankModel()
	}

	rerankResp := relaymodel.RerankResponse{
		Meta: relaymodel.RerankMeta{
			Tokens: &relaymodel.RerankMetaTokens{
				InputTokens: int64(usage.InputTokens),
			},
		},
		Results: results,
		ID:      meta.RequestID,
	}

	jsonResponse, err := sonic.Marshal(rerankResp)
	if err != nil {
		return usage, openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}

	_, err = c.Writer.Write(jsonResponse)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}
	return usage, nil
}
