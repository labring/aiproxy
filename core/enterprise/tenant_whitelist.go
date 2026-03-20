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

	// Auto-clean rejected tenant login record after successful whitelist addition
	model.DB.Where("tenant_id = ?", req.TenantID).Delete(&models.RejectedTenantLogin{})

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

// TenantSummaryItem represents a single tenant row in the summary view.
type TenantSummaryItem struct {
	TenantID          string `json:"tenant_id"`
	Name              string `json:"name"`
	IsWhitelisted     bool   `json:"is_whitelisted"`
	WhitelistID       uint   `json:"whitelist_id,omitempty"`
	AddedBy           string `json:"added_by,omitempty"`
	SuccessfulMembers int64  `json:"successful_members"`
	// RejectedAttempts is the total attempt_count from rejected_tenant_logins for this tenant.
	RejectedAttempts int    `json:"rejected_attempts"`
	RejectedRecordID *uint  `json:"rejected_record_id,omitempty"`
}

// GetTenantSummary returns a unified view of all known tenants:
// those in the whitelist, those with successful feishu logins, and those with rejected attempts.
func GetTenantSummary(c *gin.Context) {
	// --- Load whitelist ---
	var whitelistRows []models.TenantWhitelist
	if err := model.DB.Find(&whitelistRows).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	whitelistByTenant := make(map[string]*models.TenantWhitelist, len(whitelistRows))
	for i := range whitelistRows {
		whitelistByTenant[whitelistRows[i].TenantID] = &whitelistRows[i]
	}

	// --- Load feishu_users member counts per tenant ---
	type tenantCount struct {
		TenantID string
		Count    int64
	}

	var memberCounts []tenantCount
	// GORM automatically applies deleted_at IS NULL for soft-delete models
	model.DB.Model(&models.FeishuUser{}).
		Select("tenant_id, COUNT(*) AS count").
		Group("tenant_id").
		Scan(&memberCounts)

	memberByTenant := make(map[string]int64, len(memberCounts))
	for _, mc := range memberCounts {
		memberByTenant[mc.TenantID] = mc.Count
	}

	// --- Load rejected_tenant_logins ---
	var rejectedRows []models.RejectedTenantLogin
	model.DB.Find(&rejectedRows)

	rejectedByTenant := make(map[string]*models.RejectedTenantLogin, len(rejectedRows))
	for i := range rejectedRows {
		rejectedByTenant[rejectedRows[i].TenantID] = &rejectedRows[i]
	}

	// --- Merge into a unified list, ordered: whitelisted first, then active, then rejected-only ---
	seen := make(map[string]struct{})
	var items []TenantSummaryItem

	addItem := func(tenantID string) {
		if tenantID == "" {
			return
		}

		if _, ok := seen[tenantID]; ok {
			return
		}
		seen[tenantID] = struct{}{}

		item := TenantSummaryItem{TenantID: tenantID}

		if wl, ok := whitelistByTenant[tenantID]; ok {
			item.IsWhitelisted = true
			item.WhitelistID = wl.ID
			item.Name = wl.Name
			item.AddedBy = wl.AddedBy
		}

		if count, ok := memberByTenant[tenantID]; ok {
			item.SuccessfulMembers = count
		}

		if rej, ok := rejectedByTenant[tenantID]; ok {
			item.RejectedAttempts = rej.AttemptCount
			item.RejectedRecordID = &rej.ID
			// Use user info as name hint if whitelist name is empty
			if item.Name == "" {
				item.Name = rej.UserName
			}
		}

		items = append(items, item)
	}

	// Priority order: whitelist entries first
	for _, wl := range whitelistRows {
		addItem(wl.TenantID)
	}
	// Then tenants with successful logins
	for _, mc := range memberCounts {
		addItem(mc.TenantID)
	}
	// Then rejected-only tenants
	for _, rej := range rejectedRows {
		addItem(rej.TenantID)
	}

	middleware.SuccessResponse(c, gin.H{
		"tenants": items,
	})
}

// GetRejectedTenantLogins returns all rejected tenant login records.
func GetRejectedTenantLogins(c *gin.Context) {
	var rejected []models.RejectedTenantLogin
	if err := model.DB.Order("last_attempt_at DESC").Find(&rejected).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"rejected": rejected,
	})
}

// DismissRejectedTenantLogin deletes a rejected tenant login record.
func DismissRejectedTenantLogin(c *gin.Context) {
	id := c.Param("id")

	var record models.RejectedTenantLogin
	if err := model.DB.First(&record, id).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, "record not found")
		return
	}

	if err := model.DB.Delete(&record).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{"message": "dismissed"})
}

// RegisterTenantWhitelistRoutes registers tenant whitelist management routes.
func RegisterTenantWhitelistRoutes(router *gin.RouterGroup) {
	router.GET("/tenant-whitelist", GetTenantWhitelist)
	router.GET("/tenant-whitelist/summary", GetTenantSummary)
	router.GET("/tenant-whitelist/rejected", GetRejectedTenantLogins)
	router.POST("/tenant-whitelist", AddTenantToWhitelist)
	router.PUT("/tenant-whitelist/config", UpdateWhitelistConfig)
	router.DELETE("/tenant-whitelist/:id", RemoveTenantFromWhitelist)
	router.DELETE("/tenant-whitelist/rejected/:id", DismissRejectedTenantLogin)
}
