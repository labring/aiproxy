package openai

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/bytedance/sonic/ast"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
)

func ConvertEmbeddingsRequest(
	meta *meta.Meta,
	req *http.Request,
	callback func(node *ast.Node) error,
	inputToSlices bool,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalBody2Node(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if callback != nil {
		err = callback(&node)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if inputToSlices {
		inputNode := node.Get("input")
		if inputNode.Exists() {
			inputString, err := inputNode.String()
			if err != nil {
				if !errors.Is(err, ast.ErrUnsupportType) {
					return adaptor.ConvertResult{}, err
				}
			} else {
				_, err = node.SetAny("input", []string{inputString})
				if err != nil {
					return adaptor.ConvertResult{}, err
				}
			}
		}
	}

	jsonData, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}
	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
}
