//go:build enterprise

package enterprise

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/enterprise/analytics"
	"github.com/labring/aiproxy/core/enterprise/feishu"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/enterprise/novita"
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

	// Build permission middleware map for sub-packages (avoids circular imports)
	// Keys use the new view/manage permission constants.
	permMW := map[string]gin.HandlerFunc{
		models.PermDashboardView:        RequirePermission(models.PermDashboardView),
		models.PermDepartmentDetailView: RequirePermission(models.PermDepartmentDetailView),
		models.PermRankingView:          RequirePermission(models.PermRankingView),
		models.PermExportManage:         RequirePermission(models.PermExportManage),
		models.PermCustomReportView:     RequirePermission(models.PermCustomReportView),
		models.PermCustomReportManage:   RequirePermission(models.PermCustomReportManage),
		models.PermQuotaManageView:      RequirePermission(models.PermQuotaManageView),
		models.PermQuotaManageManage:    RequirePermission(models.PermQuotaManageManage),
		models.PermUserManageView:       RequirePermission(models.PermUserManageView),
		models.PermUserManageManage:     RequirePermission(models.PermUserManageManage),
		models.PermAccessControlView:    RequirePermission(models.PermAccessControlView),
		models.PermAccessControlManage:  RequirePermission(models.PermAccessControlManage),
	}

	// My Access routes (all enterprise users, no special permission)
	enterpriseAuth.GET("/my-access", GetMyAccess)
	enterpriseAuth.GET("/my-access/stats", GetMyStats)
	enterpriseAuth.POST("/my-access/tokens", CreateMyToken)
	enterpriseAuth.DELETE("/my-access/tokens/:id", DisableMyToken)

	analytics.RegisterRoutes(enterpriseAuth, permMW)
	quota.RegisterRoutes(enterpriseAuth, permMW)
	ppio.RegisterRoutes(enterpriseAuth, permMW)
	novita.RegisterRoutes(enterpriseAuth, permMW)
	RegisterTenantWhitelistRoutes(enterpriseAuth, permMW)
	RegisterEnterpriseAuthRoutes(enterpriseAuth, permMW)
	RegisterRolePermissionRoutes(enterpriseAuth)
}

// RegisterPublicRoutes registers routes that don't require admin authentication.
func RegisterPublicRoutes(public *gin.RouterGroup) {
	feishu.RegisterRoutes(public, nil, nil, nil)
}

// RegisterAdminRoutes registers routes that require admin authentication.
func RegisterAdminRoutes(admin *gin.RouterGroup) {
	// Note: Feishu sync has been moved to EnterpriseAuth to allow Feishu admin users
	// feishu.RegisterRoutes(nil, admin, nil, nil)
}

// RegisterEnterpriseAuthRoutes registers routes that require enterprise authentication.
func RegisterEnterpriseAuthRoutes(enterpriseAuth *gin.RouterGroup, permMW map[string]gin.HandlerFunc) {
	feishu.RegisterRoutes(nil, nil, enterpriseAuth, &feishu.FeishuMiddleware{
		UserManageView:   permMW[models.PermUserManageView],
		UserManageManage: permMW[models.PermUserManageManage],
		AdminOnly:        RequireRole(models.RoleAdmin),
	})
}

// RegisterRolePermissionRoutes registers role permission management routes.
func RegisterRolePermissionRoutes(router *gin.RouterGroup) {
	rp := router.Group("/role-permissions")

	// Readable by all authenticated enterprise users
	rp.GET("", GetAllRolePermissions)
	rp.GET("/my", GetMyPermissions)
	rp.GET("/all-keys", GetAllPermissionKeys)

	// Write operations require admin role
	rp.PUT("/:role", RequireRole(models.RoleAdmin), UpdateRolePermissions)
}
