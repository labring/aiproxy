//go:build enterprise

package enterprise

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/enterprise/analytics"
	"github.com/labring/aiproxy/core/enterprise/feishu"
	"github.com/labring/aiproxy/core/enterprise/ppio"
	"github.com/labring/aiproxy/core/enterprise/quota"
	"github.com/labring/aiproxy/core/middleware"
)

// RegisterRoutes registers all enterprise API routes under /api/enterprise.
// Each sub-module registers its own routes within this group.
func RegisterRoutes(router *gin.Engine) {
	enterprise := router.Group("/api/enterprise")

	// Public routes (no admin auth needed, e.g., OAuth callbacks)
	RegisterPublicRoutes(enterprise)

	// Admin-authenticated routes (feishu sync only)
	admin := enterprise.Group("")
	admin.Use(middleware.AdminAuth)

	RegisterAdminRoutes(admin)

	// Enterprise-authenticated routes (AdminKey or Feishu user token)
	// Analytics and quota routes are accessible by both admins and enterprise users.
	enterpriseAuth := enterprise.Group("")
	enterpriseAuth.Use(EnterpriseAuth)

	analytics.RegisterRoutes(enterpriseAuth)
	quota.RegisterRoutes(enterpriseAuth)
	RegisterTenantWhitelistRoutes(enterpriseAuth)
	RegisterEnterpriseAuthRoutes(enterpriseAuth)
}

// RegisterPublicRoutes registers routes that don't require admin authentication.
func RegisterPublicRoutes(public *gin.RouterGroup) {
	feishu.RegisterRoutes(public, nil, nil)
}

// RegisterAdminRoutes registers routes that require admin authentication.
func RegisterAdminRoutes(admin *gin.RouterGroup) {
	// Note: Feishu sync has been moved to EnterpriseAuth to allow Feishu admin users
	// feishu.RegisterRoutes(nil, admin, nil)

	// PPIO model sync routes (admin only)
	ppio.RegisterRoutes(admin)
}

// RegisterEnterpriseAuthRoutes registers routes that require enterprise authentication.
func RegisterEnterpriseAuthRoutes(enterpriseAuth *gin.RouterGroup) {
	feishu.RegisterRoutes(nil, nil, enterpriseAuth)
}
