package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// ConvertDecisionsRequest only applies model mapping. Preserve native input items,
// typed choices, cache controls, and future fields; OpenAI validates their contract.
func ConvertDecisionsRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	requestBody, err := common.GetRequestBodyReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Validate the complete JSON before editing it. The reusable AST shortcut
	// skips validation and can silently normalize malformed request bodies.
	var node ast.Node
	if err := json.Unmarshal(requestBody, &node); err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Set rejects arrays/scalars, but turns null into an object. Reject null
	// explicitly so malformed root values cannot become valid mapped requests.
	if node.TypeSafe() == ast.V_NULL {
		return adaptor.ConvertResult{}, errors.New("decisions request must be a JSON object")
	}

	if _, err = node.Set("model", ast.NewString(meta.ActualModel)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	data, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(data))},
		},
		Body: bytes.NewReader(data),
	}, nil
}

// DecisionsHandler returns native answers without rebuilding their union types.
// Usage is extracted separately so cache reads/writes remain part of input_tokens
// and are not added to the token total again during accounting.
func DecisionsHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	defer resp.Body.Close()

	body, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err, "read_response_body_failed", http.StatusInternalServerError,
		)
	}

	if !adaptor.IsSuccessfulResponseStatus(mode.Decisions, resp.StatusCode) {
		var failure struct {
			Error *relaymodel.OpenAIError `json:"error"`
		}

		// Preserve null code/param values and additional native error fields while
		// still exposing the error code to the existing retry/monitor pipeline.
		if err := sonic.Unmarshal(body, &failure); err == nil && failure.Error != nil {
			return adaptor.DoResponseResult{}, adaptor.NewError(
				resp.StatusCode,
				decisionsErrorResponse{
					RawMessage: body,
					code:       failure.Error.Code,
				},
			)
		}

		return adaptor.DoResponseResult{}, ErrorHanlderWithBody(resp.StatusCode, body)
	}

	var response *struct {
		Usage relaymodel.ResponseUsage `json:"usage"`
	}

	if err := sonic.Unmarshal(body, &response); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err, "unmarshal_response_body_failed", http.StatusInternalServerError,
		)
	}

	if response == nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"decisions response must be a JSON object",
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	body, err = rewriteTopLevelModel(body, responseModelName(meta))
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err, "rewrite_response_model_failed", http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = c.Writer.Write(body)

	return adaptor.DoResponseResult{Usage: response.Usage.ToModelUsage()}, nil
}

// decisionsErrorResponse keeps OpenAI's error JSON intact, including nulls and
// extension fields, without losing the error code used for relay retries.
type decisionsErrorResponse struct {
	json.RawMessage
	code any
}

func (e decisionsErrorResponse) ErrorCode() any {
	return e.code
}

func (e decisionsErrorResponse) String() string {
	return string(e.RawMessage)
}
