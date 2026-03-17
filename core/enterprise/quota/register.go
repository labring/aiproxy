//go:build enterprise

package quota

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
)

// RegisterRoutes registers all quota management routes under the given admin router group.
func RegisterRoutes(admin *gin.RouterGroup) {
	policies := admin.Group("/quota/policies")
	{
		policies.GET("", ListPolicies)
		policies.POST("", CreatePolicy)
		policies.GET("/:id", GetPolicy)
		policies.PUT("/:id", UpdatePolicy)
		policies.DELETE("/:id", DeletePolicy)
	}

	bind := admin.Group("/quota")
	{
		bind.POST("/bind", BindPolicyToGroup)
		bind.DELETE("/bind/:group_id", UnbindPolicyFromGroup)
		bind.POST("/bind-department", BindPolicyToDepartment)
		bind.DELETE("/bind-department/:department_id", UnbindPolicyFromDepartment)
		bind.POST("/bind-user", BindPolicyToUser)
		bind.DELETE("/bind-user/:open_id", UnbindPolicyFromUser)
	}
}

// Init sets up the enterprise quota hook so the middleware can call into
// the progressive quota tier logic during request distribution.
func Init() {
	middleware.EnterpriseQuotaCheck = CheckQuotaTier
}
