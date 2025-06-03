package aws

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/aws/utils"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type Adaptor struct{}

func (a *Adaptor) DefaultBaseURL() string {
	return ""
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	aa := GetAdaptor(meta.ActualModel)
	if aa == nil {
		return adaptor.ConvertResult{}, errors.New("adaptor not found")
	}
	meta.Set("awsAdapter", aa)
	return aa.ConvertRequest(meta, store, req)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	_ *http.Response,
) (usage model.Usage, err adaptor.Error) {
	adaptor, ok := meta.Get("awsAdapter")
	if !ok {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"awsAdapter not found",
			nil,
			http.StatusInternalServerError,
		)
	}
	v, ok := adaptor.(utils.AwsAdapter)
	if !ok {
		panic(fmt.Sprintf("aws adapter type error: %T, %v", v, v))
	}
	return v.DoResponse(meta, store, c)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	models := make([]model.ModelConfig, 0, len(adaptors))
	for _, model := range adaptors {
		models = append(models, model.config)
	}
	return adaptor.Metadata{
		Models:  models,
		KeyHelp: "region|ak|sk",
	}
}

func (a *Adaptor) GetRequestURL(_ *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    "",
	}, nil
}

func (a *Adaptor) SetupRequestHeader(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	_ *http.Request,
) error {
	return nil
}

func (a *Adaptor) DoRequest(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	_ *http.Request,
) (*http.Response, error) {
	return nil, nil
}
