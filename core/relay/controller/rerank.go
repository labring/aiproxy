package controller

import (
	"errors"
	"fmt"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
)

func getRerankRequestUsageFromNode(c *gin.Context) (RequestUsage, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(c.Request)
	if err != nil {
		return RequestUsage{}, err
	}

	modelNode := node.Get("model")
	if modelNode == nil || !modelNode.Exists() || modelNode.TypeSafe() == ast.V_NULL {
		return RequestUsage{}, errors.New("model parameter must be provided")
	}

	modelName, err := modelNode.String()
	if err != nil || modelName == "" {
		return RequestUsage{}, errors.New("model parameter must be provided")
	}

	query := node.Get("query")
	if query == nil || !query.Exists() || query.TypeSafe() == ast.V_NULL {
		return RequestUsage{}, errors.New("query must not be empty")
	}

	if query.TypeSafe() == ast.V_STRING {
		queryString, err := query.String()
		if err != nil || queryString == "" {
			return RequestUsage{}, errors.New("query must not be empty")
		}
	}

	documents := node.Get("documents")
	if documents == nil || !documents.Exists() || documents.TypeSafe() == ast.V_NULL {
		return RequestUsage{}, errors.New("document list must not be empty")
	}

	if documents.TypeSafe() != ast.V_ARRAY {
		return RequestUsage{}, errors.New("documents must be an array")
	}

	tokens, err := rerankContentTokens(query, modelName, false)
	if err != nil {
		return RequestUsage{}, err
	}

	var (
		count    int
		tokenErr error
	)

	err = documents.ForEach(func(_ ast.Sequence, document *ast.Node) bool {
		var itemTokens int64

		itemTokens, tokenErr = rerankContentTokens(document, modelName, true)
		if tokenErr != nil {
			return false
		}

		count++
		tokens += itemTokens

		return true
	})
	if err != nil {
		return RequestUsage{}, fmt.Errorf("documents must be an array: %w", err)
	}

	if tokenErr != nil {
		return RequestUsage{}, tokenErr
	}

	if count == 0 {
		return RequestUsage{}, errors.New("document list must not be empty")
	}

	return NewRequestUsage(model.Usage{
		InputTokens: model.ZeroNullInt64(tokens),
	}), nil
}

func rerankContentTokens(node *ast.Node, modelName string, allowVideo bool) (int64, error) {
	if node == nil || !node.Exists() || node.TypeSafe() == ast.V_NULL {
		return 0, nil
	}

	switch node.TypeSafe() {
	case ast.V_STRING:
		text, err := node.String()
		if err != nil {
			return 0, err
		}

		return openai.CountTokenInput(text, modelName), nil
	case ast.V_OBJECT:
		text, ok, err := rerankTextContentValue(node, allowVideo)
		if err != nil || !ok {
			return 0, err
		}

		return openai.CountTokenInput(text, modelName), nil
	default:
		return 0, nil
	}
}

func rerankTextContentValue(
	node *ast.Node,
	allowVideo bool,
) (string, bool, error) {
	text, ok, err := rerankStringOrURLValue(node.Get("text"))
	if err != nil || ok {
		return text, ok, err
	}

	if node.Get("image").Exists() || node.Get("image_url").Exists() {
		return "", false, nil
	}

	if allowVideo && (node.Get("video").Exists() || node.Get("video_url").Exists()) {
		return "", false, nil
	}

	return "", false, nil
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

func GetRerankRequestUsage(c *gin.Context, _ model.ModelConfig) (RequestUsage, error) {
	return getRerankRequestUsageFromNode(c)
}
