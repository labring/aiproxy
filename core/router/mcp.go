package router

import (
	"github.com/gin-gonic/gin"
	mcp "github.com/labring/aiproxy/core/controller/mcp"
	"github.com/labring/aiproxy/core/middleware"
)

func SetMCPRouter(router *gin.Engine) {
	mcpRoute := router.Group("/mcp", middleware.MCPAuth)

	mcpRoute.GET("/public/:id/sse", mcp.PublicMCPSSEServer)
	mcpRoute.POST("/public/message", mcp.PublicMCPMessage)
	mcpRoute.GET("/public/:id", mcp.PublicMCPStreamable)
	mcpRoute.POST("/public/:id", mcp.PublicMCPStreamable)
	mcpRoute.DELETE("/public/:id", mcp.PublicMCPStreamable)

	mcpRoute.GET("/group/:id/sse", mcp.GroupMCPSSEServer)
	mcpRoute.POST("/group/message", mcp.GroupMCPMessage)
	mcpRoute.GET("/group/:id", mcp.GroupMCPStreamable)
	mcpRoute.POST("/group/:id", mcp.GroupMCPStreamable)
	mcpRoute.DELETE("/group/:id", mcp.GroupMCPStreamable)

	router.GET("/sse", middleware.MCPAuth, mcp.HostMCPSSEServer)
	router.GET("/message", middleware.MCPAuth, mcp.HostMCPMessage)
	router.GET("/mcp", middleware.MCPAuth, mcp.HostMCPStreamable)
	router.POST("/mcp", middleware.MCPAuth, mcp.HostMCPStreamable)
	router.DELETE("/mcp", middleware.MCPAuth, mcp.HostMCPStreamable)
}
