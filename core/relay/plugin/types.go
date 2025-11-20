package plugin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
)

// adaptor hook
type Plugin interface {
	GetRequestURL(
		meta *meta.Meta,
		store adaptor.Store,
		c *gin.Context,
		do adaptor.GetRequestURL,
	) (adaptor.RequestURL, error)

	SetupRequestHeader(
		meta *meta.Meta,
		store adaptor.Store,
		c *gin.Context,
		req *http.Request,
		do adaptor.SetupRequestHeader,
	) error

	ConvertRequest(
		meta *meta.Meta,
		store adaptor.Store,
		req *http.Request,
		do adaptor.ConvertRequest,
	) (adaptor.ConvertResult, error)

	DoRequest(
		meta *meta.Meta,
		store adaptor.Store,
		c *gin.Context,
		req *http.Request,
		do adaptor.DoRequest,
	) (*http.Response, error)

	DoResponse(
		meta *meta.Meta,
		store adaptor.Store,
		c *gin.Context,
		resp *http.Response,
		do adaptor.DoResponse,
	) (model.Usage, adaptor.Error)
}

func WrapperAdaptor(adaptor adaptor.Adaptor, plugins ...Plugin) adaptor.Adaptor {
	if len(plugins) == 0 {
		return adaptor
	}

	result := adaptor
	for i := len(plugins) - 1; i >= 0; i-- {
		result = &wrappedAdaptor{
			Adaptor: result,
			plugin:  plugins[i],
		}
	}

	return result
}

var _ adaptor.Adaptor = (*wrappedAdaptor)(nil)

type wrappedAdaptor struct {
	adaptor.Adaptor
	plugin Plugin
}

func (w *wrappedAdaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	return w.plugin.GetRequestURL(meta, store, c, w.Adaptor)
}

func (w *wrappedAdaptor) SetupRequestHeader(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) error {
	return w.plugin.SetupRequestHeader(meta, store, c, req, w.Adaptor)
}

func (w *wrappedAdaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return w.plugin.ConvertRequest(meta, store, req, w.Adaptor)
}

func (w *wrappedAdaptor) DoRequest(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return w.plugin.DoRequest(meta, store, c, req, w.Adaptor)
}

func (w *wrappedAdaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	return w.plugin.DoResponse(meta, store, c, resp, w.Adaptor)
}
