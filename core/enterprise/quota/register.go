//go:build enterprise

package quota

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
)

// RegisterRoutes registers all quota management routes under the given router group.
// permMW maps permission keys to gin middleware for access control.
func RegisterRoutes(group *gin.RouterGroup, permMW map[string]gin.HandlerFunc) {
	quotaViewMw := permMW["quota_manage_view"]
	quotaManageMw := permMW["quota_manage_manage"]

	// Read-only endpoints — quota_manage_view permission required
	policies := group.Group("/quota/policies", quotaViewMw)
	policies.GET("", ListPolicies)
	policies.GET("/:id", GetPolicy)

	bind := group.Group("/quota", quotaViewMw)
	bind.GET("/department-bindings", ListDepartmentPolicyBindings)
	bind.GET("/user-bindings", ListUserPolicyBindings)

	// Write operations — quota_manage_manage permission required
	adminPolicies := group.Group("/quota/policies", quotaManageMw)
	adminPolicies.POST("", CreatePolicy)
	adminPolicies.PUT("/:id", UpdatePolicy)
	adminPolicies.DELETE("/:id", DeletePolicy)

	adminBind := group.Group("/quota", quotaManageMw)
	adminBind.POST("/bind", BindPolicyToGroup)
	adminBind.DELETE("/bind/:group_id", UnbindPolicyFromGroup)
	adminBind.POST("/bind-department", BindPolicyToDepartment)
	adminBind.DELETE("/bind-department/:department_id", UnbindPolicyFromDepartment)
	adminBind.POST("/bind-user", BindPolicyToUser)
	adminBind.DELETE("/bind-user/:open_id", UnbindPolicyFromUser)
	adminBind.POST("/batch-bind-departments", BatchBindPolicyToDepartments)
	adminBind.POST("/batch-bind-users", BatchBindPolicyToUsers)

	notifCfg := group.Group("/quota/notif-config")
	notifCfg.GET("", quotaViewMw, GetNotifConfigHandler)
	notifCfg.PUT("", quotaManageMw, UpdateNotifConfigHandler)

	// Alert history
	alertHistory := group.Group("/quota", quotaViewMw)
	alertHistory.GET("/alert-history", ListAlertHistory)
}

// Init sets up the enterprise quota hook so the middleware can call into
// the progressive quota tier logic during request distribution.
func Init() {
	middleware.EnterpriseQuotaCheck = CheckQuotaTier
}
