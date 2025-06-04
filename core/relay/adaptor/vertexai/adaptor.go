package vertexai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct{}

func (a *Adaptor) DefaultBaseURL() string {
	return ""
}

type Config struct {
	Region    string
	ProjectID string
	ADCJSON   string
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	aa := GetAdaptor(meta.ActualModel)
	if aa == nil {
		return adaptor.ConvertResult{}, errors.New("adaptor not found")
	}

	return aa.ConvertRequest(meta, store, request)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	adaptor := GetAdaptor(meta.ActualModel)
	if adaptor == nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			meta.ActualModel+" adaptor not found",
			"adaptor_not_found",
			http.StatusInternalServerError,
		)
	}
	return adaptor.DoResponse(meta, store, c, resp)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"Claude support native Endpoint: /v1/messages",
		},
		KeyHelp: "region|adcJSON",
		Models:  modelList,
	}
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	var suffix string
	if strings.HasPrefix(meta.ActualModel, "gemini") {
		if meta.GetBool("stream") {
			suffix = "streamGenerateContent?alt=sse"
		} else {
			suffix = "generateContent"
		}
	} else {
		if meta.GetBool("stream") {
			suffix = "streamRawPredict?alt=sse"
		} else {
			suffix = "rawPredict"
		}
	}

	config, err := getConfigFromKey(meta.Channel.Key)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	if meta.Channel.BaseURL != "" {
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/v1/projects/%s/locations/%s/publishers/google/models/%s:%s",
				meta.Channel.BaseURL,
				config.ProjectID,
				config.Region,
				meta.ActualModel,
				suffix,
			),
		}, nil
	}
	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL: fmt.Sprintf(
			"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:%s",
			config.Region,
			config.ProjectID,
			config.Region,
			meta.ActualModel,
			suffix,
		),
	}, nil
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	config, err := getConfigFromKey(meta.Channel.Key)
	if err != nil {
		return err
	}
	token, err := getToken(context.Background(), config.ADCJSON)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (a *Adaptor) DoRequest(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequest(req)
}
