package typesafeai

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

func init() {
	registry.Register(model.ChannelTypeTypeSafeAI, &Adaptor{})
}

// Adaptor implements the adaptor.Adaptor interface for the TypeSafe SystemOne API.
type Adaptor struct{}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "https://docs.typesafe.ai\nSupport native Endpoint: /v1/systemone",
		Models: ModelList,
	}
}

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	return adaptor.ModeFromMeta(mt) == mode.SystemOne
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	base := meta.Channel.BaseURL
	if base == "" {
		base = a.DefaultBaseURL()
	}

	requestURL, err := url.JoinPath(base, "/systemone")
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    requestURL,
	}, nil
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)

	return nil
}

// ConvertRequest rewrites the model and validates the required fields, keeping
// the rest of the request - including assets and unknown fields - as raw JSON.
func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	requestBody, err := common.GetRequestBodyReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	// Validate the complete JSON before editing it. The reusable AST shortcut
	// skips validation and can silently normalize malformed request bodies.
	var node ast.Node
	if err := json.Unmarshal(requestBody, &node); err != nil {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"invalid request body: "+err.Error(),
		)
	}

	// Get on a null root reports every field as missing, but Set would turn
	// that same root into a valid object; reject it explicitly.
	if node.TypeSafe() == ast.V_NULL {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"invalid request body: request must be a JSON object",
		)
	}

	stateNode := node.Get("state")
	if !stateNode.Exists() || stateNode.TypeSafe() == ast.V_NULL {
		return adaptor.ConvertResult{}, convertRequestError(meta, "missing required field: state")
	}

	questionsNode := node.Get("questions")
	if !questionsNode.Exists() || questionsNode.TypeSafe() != ast.V_OBJECT {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"missing required field: questions",
		)
	}

	// sonic's Len counts array elements, so walk the object to check it has at
	// least one named question.
	questionCount := 0

	_ = questionsNode.ForEach(func(_ ast.Sequence, _ *ast.Node) bool {
		questionCount++
		return true
	})

	if questionCount == 0 {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"missing required field: questions",
		)
	}

	if _, err := node.Set("model", ast.NewString(meta.ActualModel)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	return marshalConvertResult(&node)
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

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequestWithMeta(req, meta)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	var systemOneResp relaymodel.SystemOneResponse
	if err := sonic.Unmarshal(responseBody, &systemOneResp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"unmarshal response body failed: "+err.Error(),
		)
	}

	usage := model.Usage{
		InputTokens:  model.ZeroNullInt64(systemOneResp.Usage.InputTokens),
		OutputTokens: model.ZeroNullInt64(systemOneResp.Usage.OutputTokens),
		TotalTokens:  model.ZeroNullInt64(systemOneResp.Usage.TotalTokens),
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)

	return adaptor.DoResponseResult{
		Usage: usage,
	}, nil
}
