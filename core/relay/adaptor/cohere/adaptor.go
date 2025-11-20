package cohere

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct{}

const baseURL = "https://api.cohere.ai"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	url, err := url.JoinPath(meta.Channel.BaseURL, "/v1/chat")
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    url,
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
	request, err := utils.UnmarshalGeneralOpenAIRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	request.Model = meta.ActualModel

	requestBody := ConvertRequest(request)
	if requestBody == nil {
		return adaptor.ConvertResult{}, errors.New("request body is nil")
	}

	data, err := sonic.Marshal(requestBody)
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

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequest(req, meta.RequestTimeout)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	if utils.IsStreamResponse(resp) {
		usage, err = StreamHandler(meta, c, resp)
	} else {
		usage, err = Handler(meta, c, resp)
	}

	return usage, err
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Models: ModelList,
	}
}
