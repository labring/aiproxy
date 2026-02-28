package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
)

type AwsAdapter interface {
	ConvertRequest(
		meta *meta.Meta,
		store adaptor.Store,
		req *http.Request,
	) (adaptor.ConvertResult, error)
	DoRequest(
		meta *meta.Meta,
		store adaptor.Store,
		c *gin.Context,
		req *http.Request,
	) (*http.Response, error)
	DoResponse(
		meta *meta.Meta,
		store adaptor.Store,
		c *gin.Context,
	) (adaptor.DoResponseResult, adaptor.Error)
}
