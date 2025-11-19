package noop

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/plugin"
)

var _ plugin.Plugin = (*Noop)(nil)

type Noop struct{}

func (n *Noop) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	do adaptor.GetRequestURL,
) (adaptor.RequestURL, error) {
	return do.GetRequestURL(meta, store, c)
}

func (n *Noop) SetupRequestHeader(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
	do adaptor.SetupRequestHeader,
) error {
	return do.SetupRequestHeader(meta, store, c, req)
}

func (n *Noop) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
	do adaptor.ConvertRequest,
) (adaptor.ConvertResult, error) {
	return do.ConvertRequest(meta, store, req)
}

func (n *Noop) DoRequest(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
	do adaptor.DoRequest,
) (*http.Response, error) {
	return do.DoRequest(meta, store, c, req)
}

func (n *Noop) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
) (model.Usage, adaptor.Error) {
	return do.DoResponse(meta, store, c, resp)
}
