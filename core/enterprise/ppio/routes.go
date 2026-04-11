//go:build enterprise

package ppio

import "github.com/gin-gonic/gin"

// RegisterRoutes registers PPIO sync routes under /api/enterprise/ppio.
// permMW maps permission keys to gin middleware for access control.
func RegisterRoutes(group *gin.RouterGroup, permMW map[string]gin.HandlerFunc) {
	// Read-only endpoints — access_control_view
	ppioView := group.Group("/ppio", permMW["access_control_view"])
	ppioView.GET("/channels", ListChannelsHandler)
	ppioView.GET("/config", GetConfigHandler)
	ppioView.GET("/sync/diagnostic", DiagnosticHandler)
	ppioView.GET("/sync/history", HistoryHandler)
	ppioView.GET("/model-coverage", ModelCoverageHandler)

	// Write endpoints — access_control_manage
	ppioManage := group.Group("/ppio", permMW["access_control_manage"])
	ppioManage.PUT("/api-key", UpdateAPIKeyHandler)
	ppioManage.PUT("/config", UpdateConfigHandler)
	ppioManage.PUT("/mgmt-token", UpdateMgmtTokenHandler)
	ppioManage.PUT("/auto-sync", UpdateAutoSyncHandler)
	ppioManage.POST("/sync/preview", PreviewHandler)
	ppioManage.POST("/sync/execute", ExecuteHandler)
}
