package xunfei

import (
	"io"
	"net/http"

	"github.com/labring/aiproxy/model"
	"github.com/labring/aiproxy/relay/adaptor"
	"github.com/labring/aiproxy/relay/adaptor/openai"
	"github.com/labring/aiproxy/relay/meta"
)

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetBaseURL() string {
	return baseURL
}

const baseURL = "https://spark-api-open.xf-yun.com/v1"

func (a *Adaptor) ConvertRequest(meta *meta.Meta, req *http.Request) (string, http.Header, io.Reader, error) {
	domain := getXunfeiDomain(meta.ActualModel)
	model := meta.ActualModel
	meta.ActualModel = domain
	defer func() {
		meta.ActualModel = model
	}()
	method, h, body, err := a.Adaptor.ConvertRequest(meta, req)
	if err != nil {
		return "", nil, nil, err
	}
	return method, h, body, nil
}

func (a *Adaptor) GetModelList() []*model.ModelConfig {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return "xunfei"
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}
