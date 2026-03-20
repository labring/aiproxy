//go:build enterprise

package ppio

import "github.com/gin-gonic/gin"

// RegisterRoutes registers PPIO sync routes under /api/enterprise/ppio
func RegisterRoutes(group *gin.RouterGroup) {
	ppioGroup := group.Group("/ppio")

	// Channel list and config management
	ppioGroup.GET("/channels", ListChannelsHandler)
	ppioGroup.GET("/config", GetConfigHandler)
	ppioGroup.PUT("/config", UpdateConfigHandler)
	ppioGroup.PUT("/mgmt-token", UpdateMgmtTokenHandler)

	// Diagnostic and preview (read-only operations)
	ppioGroup.GET("/sync/diagnostic", DiagnosticHandler)
	ppioGroup.POST("/sync/preview", PreviewHandler)

	// Execute sync (SSE streaming)
	ppioGroup.POST("/sync/execute", ExecuteHandler)

	// Sync history
	ppioGroup.GET("/sync/history", HistoryHandler)
}
