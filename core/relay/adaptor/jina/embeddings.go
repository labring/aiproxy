package jina

import (
	"net/http"

	"github.com/bytedance/sonic/ast"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
)

func ConvertEmbeddingsRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return openai.ConvertEmbeddingsRequest(meta, req, func(node *ast.Node) error {
		_, err := node.Unset("encoding_format")
		return err
	}, true)
}
