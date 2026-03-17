//go:build enterprise

package enterprise

import (
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// GetTenantWhitelist returns the list of allowed tenants and configuration.
func GetTenantWhitelist(c *gin.Context) {
	var tenants []models.TenantWhitelist
	if err := model.DB.Order("created_at DESC").Find(&tenants).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	var config models.TenantWhitelistConfig
	// Get or create default config
	if err := model.DB.FirstOrCreate(&config, models.TenantWhitelistConfig{ID: 1}).Error; err != nil {
		log.Errorf("failed to get whitelist config: %v", err)
	}

	middleware.SuccessResponse(c, gin.H{
		"tenants": tenants,
		"config":  config,
	})
}

// AddTenantToWhitelist adds a new tenant to the whitelist.
func AddTenantToWhitelist(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		Name     string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get current user (from admin token or feishu user)
	addedBy := c.GetString("username")
	if addedBy == "" {
		addedBy = "admin"
	}

	tenant := models.TenantWhitelist{
		TenantID:  req.TenantID,
		TenantKey: req.TenantID, // For compatibility
		Name:      req.Name,
		AddedBy:   addedBy,
	}

	if err := model.DB.Create(&tenant).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	log.Infof("tenant %s added to whitelist by %s", req.TenantID, addedBy)
	middleware.SuccessResponse(c, tenant)
}

// RemoveTenantFromWhitelist removes a tenant from the whitelist.
func RemoveTenantFromWhitelist(c *gin.Context) {
	id := c.Param("id")

	var tenant models.TenantWhitelist
	if err := model.DB.First(&tenant, id).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, "tenant not found")
		return
	}

	if err := model.DB.Delete(&tenant).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	log.Infof("tenant %s removed from whitelist", tenant.TenantID)
	middleware.SuccessResponse(c, gin.H{"message": "tenant removed successfully"})
}

// UpdateWhitelistConfig updates the global whitelist configuration.
func UpdateWhitelistConfig(c *gin.Context) {
	var req struct {
		WildcardMode bool   `json:"wildcard_mode"`
		EnvOverride  bool   `json:"env_override"`
		Description  string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updatedBy := c.GetString("username")
	if updatedBy == "" {
		updatedBy = "admin"
	}

	var config models.TenantWhitelistConfig
	if err := model.DB.FirstOrCreate(&config, models.TenantWhitelistConfig{ID: 1}).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	config.WildcardMode = req.WildcardMode
	config.EnvOverride = req.EnvOverride
	config.Description = req.Description
	config.LastUpdatedBy = updatedBy

	if err := model.DB.Save(&config).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	log.Infof("whitelist config updated by %s: wildcard=%v, env_override=%v",
		updatedBy, config.WildcardMode, config.EnvOverride)

	middleware.SuccessResponse(c, config)
}

// RegisterTenantWhitelistRoutes registers tenant whitelist management routes.
func RegisterTenantWhitelistRoutes(router *gin.RouterGroup) {
	router.GET("/tenant-whitelist", GetTenantWhitelist)
	router.POST("/tenant-whitelist", AddTenantToWhitelist)
	router.DELETE("/tenant-whitelist/:id", RemoveTenantFromWhitelist)
	router.PUT("/tenant-whitelist/config", UpdateWhitelistConfig)
}
