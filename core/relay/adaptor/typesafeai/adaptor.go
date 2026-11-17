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

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return adaptor.ConvertResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_request_body_failed",
			http.StatusInternalServerError,
		)
	}

	var systemOneReq relaymodel.SystemOneRequest
	if err := sonic.Unmarshal(body, &systemOneReq); err != nil {
		return adaptor.ConvertResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"invalid request body: "+err.Error(),
			"invalid_request_error",
			http.StatusBadRequest,
		)
	}

	if systemOneReq.State == nil {
		return adaptor.ConvertResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"missing required field: state",
			"invalid_request_error",
			http.StatusBadRequest,
		)
	}

	if len(systemOneReq.Questions) == 0 {
		return adaptor.ConvertResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"missing required field: questions",
			"invalid_request_error",
			http.StatusBadRequest,
		)
	}

	systemOneReq.Model = meta.ActualModel

	newBody, err := sonic.Marshal(systemOneReq)
	if err != nil {
		return adaptor.ConvertResult{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_request_body_failed",
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
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var systemOneResp relaymodel.SystemOneResponse
	if err := sonic.Unmarshal(responseBody, &systemOneResp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
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
