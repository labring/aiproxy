package azure2

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/azure"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
)

type Adaptor struct {
	azure.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeAzure2, &Adaptor{})
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
	return azure.ConvertRequest(meta, store, req, false)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: fmt.Sprintf(
			"Azure AI Foundry / Azure OpenAI compatible endpoint\nModel names can contain '.' character\nAPI version is optional, default is '%s'\nSupports Gemini-compatible request conversion",
			azure.DefaultAPIVersion,
		),
		KeyHelp: "key or key|api-version",
		Models:  openai.ModelList,
	}
}
