package openai

import (
	"bytes"
	"errors"
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

func ConvertEmbeddingsRequest(
	meta *meta.Meta,
	req *http.Request,
	inputToSlices bool,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

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
		if inputNode.Exists() && inputNode.TypeSafe() == ast.V_STRING {
			inputString, err := inputNode.String()
			if err != nil {
				return adaptor.ConvertResult{}, err
			}

			_, err = node.SetAny("input", []string{inputString})
			if err != nil {
				return adaptor.ConvertResult{}, err
			}
		}
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

func GetEmbeddingsUsageFromNode(
	node *ast.Node,
) (*relaymodel.EmbeddingUsage, error) {
	usageNode := node.Get("usage")
	if usageNode == nil || usageNode.TypeSafe() == ast.V_NULL {
		return nil, nil
	}

	usageRaw, err := usageNode.Raw()
	if err != nil {
		if !errors.Is(err, ast.ErrNotExist) {
			return nil, err
		}
		return nil, nil
	}

	var usage relaymodel.EmbeddingUsage

	err = sonic.UnmarshalString(usageRaw, &usage)
	if err != nil {
		return nil, err
	}

	return &usage, nil
}

func EmbeddingsHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
	preHandler PreHandler,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	node, err := common.UnmarshalResponse2Node(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if preHandler != nil {
		err := preHandler(meta, &node)
		if err != nil {
			return model.Usage{}, relaymodel.WrapperOpenAIError(
				err,
				"pre_handler_failed",
				http.StatusInternalServerError,
			)
		}
	}

	usage, err := GetEmbeddingsUsageFromNode(&node)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if usage == nil ||
		(usage.TotalTokens == 0 && usage.PromptTokens == 0) {
		usage = &relaymodel.EmbeddingUsage{
			PromptTokens: int64(meta.RequestUsage.InputTokens),
			TotalTokens:  int64(meta.RequestUsage.InputTokens),
		}
		if meta.RequestUsage.ImageInputTokens != 0 {
			usage.PromptTokensDetails = &relaymodel.EmbeddingPromptTokensDetails{
				ImageTokens: int64(meta.RequestUsage.ImageInputTokens),
			}
		}

		_, err = node.Set("usage", ast.NewAny(usage))
		if err != nil {
			return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
				err,
				"set_usage_failed",
				http.StatusInternalServerError,
			)
		}
	} else if usage.TotalTokens != 0 && usage.PromptTokens == 0 { // some channels don't return prompt tokens
		usage.PromptTokens = usage.TotalTokens

		_, err = node.Set("usage", ast.NewAny(usage))
		if err != nil {
			return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(err, "set_usage_failed", http.StatusInternalServerError)
		}
	}

	_, err = node.Set("model", ast.NewString(meta.OriginModel))
	if err != nil {
		return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"set_model_failed",
			http.StatusInternalServerError,
		)
	}

	newData, err := node.MarshalJSON()
	if err != nil {
		return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(newData)))

	_, err = c.Writer.Write(newData)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return usage.ToModelUsage(), nil
}
