//go:build enterprise

package enterprise

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// GetAllRolePermissions returns all roles and their permissions.
func GetAllRolePermissions(c *gin.Context) {
	var perms []models.RolePermission
	if err := model.DB.Find(&perms).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make(map[string][]string)
	for _, p := range perms {
		result[p.Role] = append(result[p.Role], p.Permission)
	}

	// Ensure admin always shows all permissions
	result[models.RoleAdmin] = models.AllPermissions

	middleware.SuccessResponse(c, gin.H{
		"roles": result,
	})
}

// GetMyPermissions returns the current user's role and permissions.
func GetMyPermissions(c *gin.Context) {
	role := GetEnterpriseRole(c)
	perms := GetRolePermissions(role)

	middleware.SuccessResponse(c, gin.H{
		"role":        role,
		"permissions": perms,
	})
}

// GetAllPermissionKeys returns all permission modules with view/manage keys.
func GetAllPermissionKeys(c *gin.Context) {
	type moduleInfo struct {
		Module      string `json:"module"`
		DisplayName string `json:"display_name"`
		ViewKey     string `json:"view_key"`
		ManageKey   string `json:"manage_key"`
	}

	modules := make([]moduleInfo, 0, len(models.AllModules))
	for _, m := range models.AllModules {
		modules = append(modules, moduleInfo{
			Module:      m,
			DisplayName: models.ModuleDisplayNames[m],
			ViewKey:     models.ViewPermission(m),
			ManageKey:   models.ManagePermission(m),
		})
	}

	middleware.SuccessResponse(c, gin.H{
		"modules": modules,
	})
}

// UpdateRolePermissions updates the permissions for a specific role.
func UpdateRolePermissions(c *gin.Context) {
	role := c.Param("role")

	// Admin permissions cannot be modified
	if role == models.RoleAdmin {
		middleware.ErrorResponse(c, http.StatusBadRequest, "admin permissions cannot be modified")
		return
	}

	// Validate role
	if role != models.RoleViewer && role != models.RoleAnalyst {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid role: must be viewer or analyst")
		return
	}

	var req struct {
		Permissions []string `json:"permissions" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate all permission keys
	validPerms := make(map[string]bool)
	for _, p := range models.AllPermissions {
		validPerms[p] = true
	}

	for _, p := range req.Permissions {
		if !validPerms[p] {
			middleware.ErrorResponse(c, http.StatusBadRequest, "invalid permission key: "+p)
			return
		}
	}

	// Replace permissions in a transaction
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		// Delete old permissions for this role
		if err := tx.Where("role = ?", role).Delete(&models.RolePermission{}).Error; err != nil {
			return err
		}

		// Insert new permissions
		if len(req.Permissions) > 0 {
			records := make([]models.RolePermission, 0, len(req.Permissions))
			for _, p := range req.Permissions {
				records = append(records, models.RolePermission{
					Role:       role,
					Permission: p,
				})
			}

			if err := tx.Create(&records).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Refresh in-memory cache
	LoadRolePermissions()

	middleware.SuccessResponse(c, gin.H{
		"role":        role,
		"permissions": req.Permissions,
	})
}
