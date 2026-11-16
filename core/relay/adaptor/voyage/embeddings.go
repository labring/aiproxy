package voyage

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// ConvertEmbeddingsRequest adapts an OpenAI-style embeddings request to
// POST /v1/embeddings:
//   - `input` is always sent as a list;
//   - `encoding_format: "float"` is dropped (Voyage only accepts `base64`);
//   - `dimensions` becomes `output_dimension` unless that is already set;
//   - `input_type` and `truncation` are passed through untouched.
func ConvertEmbeddingsRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(err.Error())
	}

	if _, err := node.Set("model", ast.NewString(meta.ActualModel)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := normalizeEncodingFormat(&node); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := mapDimensions(&node); err != nil {
		return adaptor.ConvertResult{}, err
	}

	inputNode := node.Get("input")
	if !inputNode.Exists() {
		return adaptor.ConvertResult{}, convertRequestError("input is required")
	}

	if inputNode.TypeSafe() == ast.V_STRING {
		text, err := inputNode.String()
		if err != nil {
			return adaptor.ConvertResult{}, err
		}

		if _, err := node.Set("input", ast.NewArray([]ast.Node{ast.NewString(text)})); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	return marshalConvertResult(&node)
}

// normalizeEncodingFormat drops `encoding_format: "float"`, the OpenAI default
// that Voyage rejects with a 400. `base64` is a valid Voyage value and is kept;
// any other value is rejected here instead of being silently rewritten.
func normalizeEncodingFormat(node *ast.Node) error {
	format := node.Get("encoding_format")
	if !format.Exists() || format.TypeSafe() == ast.V_NULL {
		return nil
	}

	value, err := format.String()
	if err != nil {
		return convertRequestError("encoding_format must be a string")
	}

	switch value {
	case "float":
		_, err = node.Unset("encoding_format")
		return err
	case "base64":
		return nil
	default:
		return convertRequestError("unsupported encoding_format: " + value)
	}
}

// mapDimensions maps the OpenAI `dimensions` field to Voyage `output_dimension`.
// When both are present the explicit `output_dimension` wins.
func mapDimensions(node *ast.Node) error {
	dimensions := node.Get("dimensions")
	if !dimensions.Exists() {
		return nil
	}

	if !node.Get("output_dimension").Exists() && dimensions.TypeSafe() != ast.V_NULL {
		if _, err := node.Set("output_dimension", *dimensions); err != nil {
			return err
		}
	}

	_, err := node.Unset("dimensions")

	return err
}

func marshalConvertResult(node *ast.Node) (adaptor.ConvertResult, error) {
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

// EmbeddingsHandler relays the Voyage embeddings response. Voyage reports
// usage.total_tokens only; a response without it fails instead of being billed
// as zero or estimated.
func EmbeddingsHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	node, err := common.UnmarshalResponse2Node(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	totalTokens, ok := readTotalTokens(&node)
	if !ok {
		return adaptor.DoResponseResult{}, missingUsageError("embeddings")
	}

	usage := relaymodel.EmbeddingUsage{PromptTokens: totalTokens, TotalTokens: totalTokens}

	return writeEmbeddingsResponse(meta, c, &node, usage)
}

// readTotalTokens returns usage.total_tokens; ok is false when the field is
// missing, is not a JSON integer (null, booleans, strings and fractions are
// refused instead of being coerced) or is negative. A reported 0 is valid:
// Voyage counts short `input_type: query` inputs (e.g. "hello") as 0 tokens and
// bills them as such, so the upstream figure is the one to follow.
func readTotalTokens(node *ast.Node) (int64, bool) {
	field := node.GetByPath("usage", "total_tokens")
	if field == nil || field.TypeSafe() != ast.V_NUMBER {
		return 0, false
	}

	raw, err := field.Raw()
	if err != nil {
		return 0, false
	}

	total, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || total < 0 {
		return 0, false
	}

	return total, true
}

func writeEmbeddingsResponse(
	meta *meta.Meta,
	c *gin.Context,
	node *ast.Node,
	usage relaymodel.EmbeddingUsage,
) (adaptor.DoResponseResult, adaptor.Error) {
	modelUsage := usage.ToModelUsage()

	if _, err := node.Set("usage", ast.NewAny(usage)); err != nil {
		return adaptor.DoResponseResult{Usage: modelUsage}, relaymodel.WrapperOpenAIError(
			err, "set_usage_failed", http.StatusInternalServerError,
		)
	}

	if _, err := node.Set("model", ast.NewString(meta.OriginModel)); err != nil {
		return adaptor.DoResponseResult{Usage: modelUsage}, relaymodel.WrapperOpenAIError(
			err, "set_model_failed", http.StatusInternalServerError,
		)
	}

	data, err := node.MarshalJSON()
	if err != nil {
		return adaptor.DoResponseResult{Usage: modelUsage}, relaymodel.WrapperOpenAIError(
			err, "marshal_response_body_failed", http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))

	if _, err := c.Writer.Write(data); err != nil {
		common.GetLogger(c).Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: modelUsage}, nil
}
