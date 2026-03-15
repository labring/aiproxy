package router

import (
	"github.com/gin-gonic/gin"
)

// enterpriseRouter is set by enterprise build tag to register enterprise routes.
var enterpriseRouter func(*gin.Engine)

func SetRouter(router *gin.Engine) {
	SetAPIRouter(router)
	SetRelayRouter(router)
	SetMCPRouter(router)
	SetStaticFileRouter(router)
	SetSwaggerRouter(router)

	if enterpriseRouter != nil {
		enterpriseRouter(router)
	}
}
