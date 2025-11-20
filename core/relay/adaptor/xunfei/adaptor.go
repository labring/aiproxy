package xunfei

import (
	"net/http"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
)

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

const baseURL = "https://spark-api-open.xf-yun.com/v1"

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	domain := getXunfeiDomain(meta.ActualModel)
	model := meta.ActualModel

	meta.ActualModel = domain
	defer func() {
		meta.ActualModel = model
	}()

	return a.Adaptor.ConvertRequest(meta, store, req)
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme:  "Gemini support",
		KeyHelp: "app_id|app_token",
		Models:  ModelList,
	}
}
