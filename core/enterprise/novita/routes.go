//go:build enterprise

package novita

import "github.com/gin-gonic/gin"

// RegisterRoutes registers Novita sync routes under /api/enterprise/novita.
func RegisterRoutes(group *gin.RouterGroup, permMW map[string]gin.HandlerFunc) {
	// Read-only endpoints — access_control_view
	novitaView := group.Group("/novita", permMW["access_control_view"])
	novitaView.GET("/channels", ListChannelsHandler)
	novitaView.GET("/config", GetConfigHandler)
	novitaView.GET("/sync/diagnostic", DiagnosticHandler)
	novitaView.GET("/sync/history", HistoryHandler)
	novitaView.GET("/model-coverage", ModelCoverageHandler)

	// Write endpoints — access_control_manage
	novitaManage := group.Group("/novita", permMW["access_control_manage"])
	novitaManage.PUT("/config", UpdateConfigHandler)
	novitaManage.PUT("/mgmt-token", UpdateMgmtTokenHandler)
	novitaManage.PUT("/exchange-rate", UpdateExchangeRateHandler)
	novitaManage.POST("/sync/preview", PreviewHandler)
	novitaManage.POST("/sync/execute", ExecuteHandler)
}
