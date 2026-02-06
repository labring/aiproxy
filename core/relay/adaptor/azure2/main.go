package azure2

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/azure"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
)

type Adaptor struct {
	azure.Adaptor
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	return azure.GetRequestURL(meta, false)
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return a.Adaptor.Adaptor.ConvertRequest(meta, store, req)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: fmt.Sprintf(
			"Model names can contain '.' character\nAPI version is optional, default is '%s'\nGemini support",
			azure.DefaultAPIVersion,
		),
		KeyHelp: "key or key|api-version",
		Models:  openai.ModelList,
	}
}
