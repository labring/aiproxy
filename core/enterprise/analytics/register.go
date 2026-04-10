//go:build enterprise

package analytics

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all analytics routes under the given router group.
// permMiddleware maps permission keys to gin middleware for access control.
func RegisterRoutes(group *gin.RouterGroup, permMiddleware map[string]gin.HandlerFunc) {
	analytics := group.Group("/analytics")

	// Dashboard view permission
	dash := analytics.Group("", permMiddleware["dashboard_view"])
	dash.GET("/department", HandleDepartmentSummary)
	dash.GET("/model/distribution", HandleModelDistribution)
	dash.GET("/comparison", HandlePeriodComparison)

	// Department detail view permission
	detail := analytics.Group("", permMiddleware["department_detail_view"])
	detail.GET("/department/:id/trend", HandleDepartmentTrend)

	// Ranking view permission
	rank := analytics.Group("", permMiddleware["ranking_view"])
	rank.GET("/department/ranking", HandleDepartmentRanking)
	rank.GET("/user/ranking", HandleUserRanking)

	// Export is an action → requires manage permission
	exp := analytics.Group("", permMiddleware["export_manage"])
	exp.GET("/export", HandleExport)

	// Custom report: view fields with view, generate with manage
	crView := analytics.Group("", permMiddleware["custom_report_view"])
	crView.GET("/custom-report/fields", HandleCustomReportFields)

	crManage := analytics.Group("", permMiddleware["custom_report_manage"])
	crManage.POST("/custom-report", HandleCustomReport)

	// Report template CRUD
	crView.GET("/custom-report/templates", HandleListTemplates)
	crManage.POST("/custom-report/templates", HandleCreateTemplate)
	crManage.PUT("/custom-report/templates/:id", HandleUpdateTemplate)
	crManage.DELETE("/custom-report/templates/:id", HandleDeleteTemplate)
}
