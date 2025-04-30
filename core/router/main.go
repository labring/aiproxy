package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetRouter(router *gin.Engine) {
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "AI Proxy is running!")
	})
	SetAPIRouter(router)
	SetRelayRouter(router)
	SetMCPRouter(router)
	SetStaticFileRouter(router)
	SetSwaggerRouter(router)
}
