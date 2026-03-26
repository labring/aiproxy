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
	"github.com/labring/aiproxy/core/enterprise/session"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// Context keys for enterprise auth.
const (
	CtxEnterpriseRole = "enterprise_role"
	CtxEnterpriseUser = "enterprise_user"
)

// extractToken reads the auth token from the request header or query param
// and returns (rawToken, plainToken). rawToken has "Bearer " stripped;
// plainToken additionally has "sk-" stripped (for AdminKey / legacy key comparison).
func extractToken(c *gin.Context) (rawToken, plainToken string, ok bool) {
	accessToken := c.Request.Header.Get("Authorization")
	if accessToken == "" {
		accessToken = c.Query("key")
	}

	if accessToken == "" {
		middleware.ErrorResponse(c, http.StatusUnauthorized, "unauthorized: no access token provided")
		c.Abort()

		return "", "", false
	}

	rawToken = strings.TrimPrefix(accessToken, "Bearer ")
	plainToken = strings.TrimPrefix(rawToken, "sk-")

	return rawToken, plainToken, true
}

// authenticateJWT parses a JWT, verifies the user exists and is active,
// and returns the user + claims. Returns nil user on any failure (already
// writes the HTTP error response).
func authenticateJWT(c *gin.Context, rawToken string) (*models.FeishuUser, *session.Claims) {
	claims, err := session.ParseJWT(rawToken)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusUnauthorized, "unauthorized: invalid session token")
		c.Abort()

		return nil, nil
	}

	var feishuUser models.FeishuUser
	if err := model.DB.Where("open_id = ?", claims.Subject).First(&feishuUser).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusUnauthorized, "unauthorized: user no longer exists")
		c.Abort()

		return nil, nil
	}

	if feishuUser.Status != 1 {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: enterprise user is disabled")
		c.Abort()

		return nil, nil
	}

	return &feishuUser, claims
}

// authenticateLegacyToken validates an API key token and looks up the associated
// FeishuUser. Returns nil on any failure (already writes the HTTP error response).
func authenticateLegacyToken(c *gin.Context, plainToken string) *models.FeishuUser {
	tokenCache, err := model.GetAndValidateToken(plainToken)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusUnauthorized, "unauthorized: "+err.Error())
		c.Abort()

		return nil
	}

	var feishuUser models.FeishuUser
	if err := model.DB.Where("token_id = ?", tokenCache.ID).First(&feishuUser).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		c.Abort()

		return nil
	}

	if feishuUser.Status != 1 {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: enterprise user is disabled")
		c.Abort()

		return nil
	}

	return &feishuUser
}

// effectiveRole returns the user's role, defaulting to viewer if empty.
func effectiveRole(user *models.FeishuUser) string {
	if user.Role == "" {
		return models.RoleViewer
	}

	return user.Role
}

// maybeRefreshJWT issues a new JWT via response header if the current one is close to expiry.
func maybeRefreshJWT(c *gin.Context, claims *session.Claims) {
	if !session.ShouldRefresh(claims) {
		return
	}

	newToken, err := session.GenerateJWT(claims.Subject, claims.Role, claims.GroupID)
	if err != nil {
		log.Warnf("failed to refresh JWT for %s: %v", claims.Subject, err)
		return
	}

	c.Header("X-New-Token", newToken)
}

// isJWT returns true if the token string looks like a JWT (three dot-separated parts).
func isJWT(token string) bool {
	return strings.Count(token, ".") == 2
}

// EnterpriseAdminAuth authenticates admin-panel requests.
// Accepts: AdminKey, JWT (admin role only), or legacy API key (admin role only).
func EnterpriseAdminAuth(c *gin.Context) {
	rawToken, plainToken, ok := extractToken(c)
	if !ok {
		return
	}

	// Path 1: AdminKey
	if config.AdminKey != "" && plainToken == config.AdminKey {
		c.Set(middleware.Token, &model.TokenCache{Key: config.AdminKey})
		c.Set(CtxEnterpriseRole, models.RoleAdmin)
		c.Next()

		return
	}

	// Path 2: JWT session token
	if isJWT(rawToken) {
		user, claims := authenticateJWT(c, rawToken)
		if user == nil {
			return
		}

		if user.Role != models.RoleAdmin {
			middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: admin role required for admin panel")
			c.Abort()

			return
		}

		c.Set(middleware.Token, &model.TokenCache{Key: config.AdminKey})
		c.Set(CtxEnterpriseRole, models.RoleAdmin)
		c.Set(CtxEnterpriseUser, user)
		maybeRefreshJWT(c, claims)
		c.Next()

		return
	}

	// Path 3 (legacy): API key token
	user := authenticateLegacyToken(c, plainToken)
	if user == nil {
		return
	}

	if user.Role != models.RoleAdmin {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: admin role required for admin panel")
		c.Abort()

		return
	}

	c.Set(middleware.Token, &model.TokenCache{Key: config.AdminKey})
	c.Set(CtxEnterpriseRole, models.RoleAdmin)
	c.Set(CtxEnterpriseUser, user)
	c.Next()
}

// EnterpriseAuth authenticates enterprise dashboard requests.
// Accepts: AdminKey (full access), JWT, or legacy API key token.
func EnterpriseAuth(c *gin.Context) {
	rawToken, plainToken, ok := extractToken(c)
	if !ok {
		return
	}

	// Path 1: AdminKey
	if config.AdminKey != "" && plainToken == config.AdminKey {
		c.Set(CtxEnterpriseRole, models.RoleAdmin)
		c.Next()

		return
	}

	// Path 2: JWT session token
	if isJWT(rawToken) {
		user, claims := authenticateJWT(c, rawToken)
		if user == nil {
			return
		}

		c.Set(CtxEnterpriseRole, effectiveRole(user))
		c.Set(CtxEnterpriseUser, user)
		maybeRefreshJWT(c, claims)
		c.Next()

		return
	}

	// Path 3 (legacy): API key token
	user := authenticateLegacyToken(c, plainToken)
	if user == nil {
		return
	}

	c.Set(CtxEnterpriseRole, effectiveRole(user))
	c.Set(CtxEnterpriseUser, user)
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
