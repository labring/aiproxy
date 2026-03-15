//go:build enterprise

package enterprise

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/enterprise/analytics"
	"github.com/labring/aiproxy/core/enterprise/feishu"
	"github.com/labring/aiproxy/core/enterprise/quota"
	"github.com/labring/aiproxy/core/middleware"
)

// RegisterRoutes registers all enterprise API routes under /api/enterprise.
// Each sub-module registers its own routes within this group.
func RegisterRoutes(router *gin.Engine) {
	enterprise := router.Group("/api/enterprise")

	// Public routes (no admin auth needed, e.g., OAuth callbacks)
	RegisterPublicRoutes(enterprise)

	// Admin-authenticated routes (feishu sync, quota management)
	admin := enterprise.Group("")
	admin.Use(middleware.AdminAuth)

	RegisterAdminRoutes(admin)

	// Enterprise-authenticated routes (AdminKey or Feishu user token)
	// Analytics routes are accessible by both admins and enterprise users.
	enterpriseAuth := enterprise.Group("")
	enterpriseAuth.Use(EnterpriseAuth)

	analytics.RegisterRoutes(enterpriseAuth)
}

// RegisterPublicRoutes registers routes that don't require admin authentication.
func RegisterPublicRoutes(public *gin.RouterGroup) {
	feishu.RegisterRoutes(public, nil)
}

// RegisterAdminRoutes registers routes that require admin authentication.
func RegisterAdminRoutes(admin *gin.RouterGroup) {
	feishu.RegisterRoutes(nil, admin)
	quota.RegisterRoutes(admin)
}
