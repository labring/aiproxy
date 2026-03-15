//go:build enterprise

package analytics

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all analytics routes under the given admin router group.
func RegisterRoutes(admin *gin.RouterGroup) {
	analytics := admin.Group("/analytics")
	{
		analytics.GET("/department", HandleDepartmentSummary)
		analytics.GET("/department/:id/trend", HandleDepartmentTrend)
		analytics.GET("/user/ranking", HandleUserRanking)
		analytics.GET("/export", HandleExport)
	}
}
