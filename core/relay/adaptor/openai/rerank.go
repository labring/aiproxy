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
	node, err := common.UnmarshalRequest2ValidatedNode(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := patchRerankMultimodalContent(&node); err != nil {
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

func patchRerankMultimodalContent(node *ast.Node) error {
	if query := node.Get("query"); query.Exists() {
		if err := patchRerankContentItem(query); err != nil {
			return err
		}
	}

	documents := node.Get("documents")
	if !documents.Exists() || documents.TypeSafe() != ast.V_ARRAY {
		return nil
	}

	var patchErr error

	err := documents.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
		patchErr = patchRerankContentItem(item)
		return patchErr == nil
	})
	if err != nil {
		return err
	}

	return patchErr
}

func patchRerankContentItem(item *ast.Node) error {
	if item == nil || !item.Exists() || item.TypeSafe() != ast.V_OBJECT {
		return nil
	}

	if image, ok, err := rerankStringOrURLValue(item.Get("image_url")); err != nil || ok {
		if err != nil {
			return err
		}

		*item = ast.NewObject([]ast.Pair{
			ast.NewPair("image", ast.NewString(image)),
		})

		return nil
	}

	if text, ok, err := rerankStringOrURLValue(item.Get("text")); err != nil || ok {
		if err != nil {
			return err
		}

		*item = ast.NewObject([]ast.Pair{
			ast.NewPair("text", ast.NewString(text)),
		})

		return nil
	}

	if image, ok, err := rerankStringOrURLValue(item.Get("image")); err != nil || ok {
		if err != nil {
			return err
		}

		*item = ast.NewObject([]ast.Pair{
			ast.NewPair("image", ast.NewString(image)),
		})

		return nil
	}

	_, err := item.Unset("type")

	return err
}

func rerankStringOrURLValue(node *ast.Node) (string, bool, error) {
	if node == nil || !node.Exists() {
		return "", false, nil
	}

	switch node.TypeSafe() {
	case ast.V_STRING:
		value, err := node.String()
		return value, true, err
	case ast.V_OBJECT:
		urlNode := node.Get("url")
		if !urlNode.Exists() || urlNode.TypeSafe() != ast.V_STRING {
			return "", false, nil
		}

		value, err := urlNode.String()

		return value, true, err
	default:
		return "", false, nil
	}
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
