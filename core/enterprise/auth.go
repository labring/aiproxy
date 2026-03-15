//go:build enterprise

package enterprise

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
