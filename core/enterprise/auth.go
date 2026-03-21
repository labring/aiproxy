//go:build enterprise

package enterprise

import (
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// Context keys for enterprise auth.
const (
	CtxEnterpriseRole = "enterprise_role"
	CtxEnterpriseUser = "enterprise_user"
)

// EnterpriseAuth is a middleware that authenticates requests using either
// the admin key or a Feishu user token. It sets enterprise_role and
// enterprise_user (for Feishu users) in the gin context.
//
// This middleware lives in the enterprise module (build-tagged) so it
// does not affect the core module's independence.
func EnterpriseAuth(c *gin.Context) {
	accessToken := c.Request.Header.Get("Authorization")
	if accessToken == "" {
		accessToken = c.Query("key")
	}

	if accessToken == "" {
		middleware.ErrorResponse(c, http.StatusUnauthorized, "unauthorized: no access token provided")
		c.Abort()

		return
	}

	accessToken = strings.TrimPrefix(accessToken, "Bearer ")
	accessToken = strings.TrimPrefix(accessToken, "sk-")

	// Path 1: AdminKey → full access
	if config.AdminKey != "" && accessToken == config.AdminKey {
		c.Set(CtxEnterpriseRole, models.RoleAdmin)
		c.Next()

		return
	}

	// Path 2: Regular token → look up FeishuUser
	tokenCache, err := model.GetAndValidateToken(accessToken)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusUnauthorized, "unauthorized: "+err.Error())
		c.Abort()

		return
	}

	var feishuUser models.FeishuUser

	if err := model.DB.Where("token_id = ?", tokenCache.ID).First(&feishuUser).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		c.Abort()

		return
	}

	if feishuUser.Status != 1 {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: enterprise user is disabled")
		c.Abort()

		return
	}

	role := feishuUser.Role
	if role == "" {
		role = models.RoleViewer
	}

	c.Set(CtxEnterpriseRole, role)
	c.Set(CtxEnterpriseUser, &feishuUser)
	c.Next()
}

// GetEnterpriseRole returns the enterprise role from the gin context.
func GetEnterpriseRole(c *gin.Context) string {
	role, _ := c.Get(CtxEnterpriseRole)
	if r, ok := role.(string); ok {
		return r
	}

	return ""
}

// GetEnterpriseUser returns the FeishuUser from the gin context, or nil if not set.
func GetEnterpriseUser(c *gin.Context) *models.FeishuUser {
	user, exists := c.Get(CtxEnterpriseUser)
	if !exists {
		return nil
	}

	if u, ok := user.(*models.FeishuUser); ok {
		return u
	}

	return nil
}

// rolePermCache is an in-memory cache of role→permission set, loaded at startup
// and refreshed whenever permissions are modified via the API.
var rolePermCache atomic.Pointer[map[string]map[string]bool]

// LoadRolePermissions loads all role permissions from DB into memory cache.
func LoadRolePermissions() {
	var perms []models.RolePermission
	if err := model.DB.Find(&perms).Error; err != nil {
		log.Errorf("failed to load role permissions: %v", err)
		return
	}

	m := make(map[string]map[string]bool)
	for _, p := range perms {
		if m[p.Role] == nil {
			m[p.Role] = make(map[string]bool)
		}

		m[p.Role][p.Permission] = true
	}

	// Admin always has all permissions (enforced in code)
	adminPerms := make(map[string]bool)
	for _, p := range models.AllPermissions {
		adminPerms[p] = true
	}

	m[models.RoleAdmin] = adminPerms

	rolePermCache.Store(&m)
	log.Infof("loaded role permissions: %d records", len(perms))
}

// HasPermission checks if a role has a specific permission.
// If checking a _view permission, having the corresponding _manage permission also grants access.
func HasPermission(role, permission string) bool {
	if role == models.RoleAdmin {
		return true
	}

	cache := rolePermCache.Load()
	if cache == nil {
		return false
	}

	if (*cache)[role][permission] {
		return true
	}

	// manage implies view: if checking xxx_view, also accept xxx_manage
	if strings.HasSuffix(permission, "_view") {
		manageKey := strings.TrimSuffix(permission, "_view") + "_manage"
		return (*cache)[role][manageKey]
	}

	return false
}

// GetRolePermissions returns the list of permissions for a role from the cache.
func GetRolePermissions(role string) []string {
	if role == models.RoleAdmin {
		return models.AllPermissions
	}

	cache := rolePermCache.Load()
	if cache == nil {
		return nil
	}

	permSet := (*cache)[role]
	perms := make([]string, 0, len(permSet))
	for p := range permSet {
		perms = append(perms, p)
	}

	return perms
}

// RequirePermission returns a middleware that checks for a specific permission.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetEnterpriseRole(c)
		if !HasPermission(role, permission) {
			middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: insufficient permissions")
			c.Abort()

			return
		}

		c.Next()
	}
}

// RequireRole returns a middleware that checks for a specific role (e.g., admin-only operations).
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetEnterpriseRole(c)
		if role != requiredRole {
			middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: requires "+requiredRole+" role")
			c.Abort()

			return
		}

		c.Next()
	}
}
