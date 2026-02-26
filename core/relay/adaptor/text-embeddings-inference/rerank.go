package textembeddingsinference

import (
	"bytes"
	"errors"
	"fmt"
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

func ConvertRerankRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("failed to parse request body: %w", err)
	}

	// Set the actual model in the request
	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Get the documents array and rename it to texts
	documentsNode := node.Get("documents")
	if !documentsNode.Exists() {
		return adaptor.ConvertResult{}, errors.New("documents field not found")
	}

	// Set the texts field with the documents value
	_, err = node.Set("texts", *documentsNode)
	if err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("failed to set texts field: %w", err)
	}

	// Remove the documents field
	_, err = node.Unset("documents")
	if err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf(
			"failed to remove documents field: %w",
			err,
		)
	}

	returnDocumentsNode := node.Get("return_documents")
	if returnDocumentsNode.Exists() {
		returnDocuments, err := returnDocumentsNode.Bool()
		if err != nil {
			return adaptor.ConvertResult{}, fmt.Errorf(
				"failed to unmarshal return_documents field: %w",
				err,
			)
		}

		_, err = node.Unset("return_documents")
		if err != nil {
			return adaptor.ConvertResult{}, fmt.Errorf(
				"failed to remove return_documents field: %w",
				err,
			)
		}

		_, err = node.Set("return_text", ast.NewBool(returnDocuments))
		if err != nil {
			return adaptor.ConvertResult{}, fmt.Errorf(
				"failed to set return_text field: %w",
				err,
			)
		}
	}

	// Convert back to JSON
	jsonData, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonData))},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
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

func RerankHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, RerankErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	respSlice := RerankResponse{}

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&respSlice)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	usage := model.Usage{
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
