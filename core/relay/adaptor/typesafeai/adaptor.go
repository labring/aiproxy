package typesafeai

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

func init() {
	registry.Register(model.ChannelTypeTypeSafeAI, &Adaptor{})
}

// Adaptor implements the adaptor.Adaptor interface for the Typesafe AI SystemOne API.
type Adaptor struct{}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme:       "Support native Endpoint: /v1/systemone",
		Models:       ModelList,
		ConfigSchema: nil,
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
	pu, err := url.Parse(meta.Channel.BaseURL)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	result := pu.JoinPath("/systemone")

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    result.String(),
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

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return adaptor.ConvertResult{}, WrapperSystemOneErrorWithMessage(
			"failed to read request body: "+err.Error(),
			"request_body_error",
			http.StatusInternalServerError,
		)
	}

	var systemOneReq SystemOneRequest
	if err := sonic.Unmarshal(body, &systemOneReq); err != nil {
		return adaptor.ConvertResult{}, WrapperSystemOneErrorWithMessage(
			"invalid request body: "+err.Error(),
			"request_body_error",
			http.StatusBadRequest,
		)
	}

	// Validate required fields
	if systemOneReq.State == nil {
		return adaptor.ConvertResult{}, WrapperSystemOneErrorWithMessage(
			"missing required field: state",
			"validation_error",
			http.StatusBadRequest,
		)
	}

	if len(systemOneReq.Questions) == 0 {
		return adaptor.ConvertResult{}, WrapperSystemOneErrorWithMessage(
			"missing required field: questions",
			"validation_error",
			http.StatusBadRequest,
		)
	}

	// Apply model mapping
	systemOneReq.Model = meta.ActualModel

	newBody, err := sonic.Marshal(systemOneReq)
	if err != nil {
		return adaptor.ConvertResult{}, WrapperSystemOneErrorWithMessage(
			"failed to marshal request body: "+err.Error(),
			"request_body_error",
			http.StatusInternalServerError,
		)
	}

	return adaptor.ConvertResult{
		Body: bytes.NewReader(newBody),
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
	if !IsSuccessfulResponseStatus(resp.StatusCode) {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, WrapperSystemOneError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Parse usage from the response
	var systemOneResp SystemOneResponse
	if err := sonic.Unmarshal(responseBody, &systemOneResp); err != nil {
		return adaptor.DoResponseResult{}, WrapperSystemOneError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	usage := model.Usage{
		InputTokens:  model.ZeroNullInt64(systemOneResp.Usage.InputTokens),
		OutputTokens: model.ZeroNullInt64(systemOneResp.Usage.OutputTokens),
		TotalTokens:  model.ZeroNullInt64(systemOneResp.Usage.TotalTokens),
	}

	// Write raw response to gin context
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
