//go:build enterprise

package enterprise

import (
	"github.com/labring/aiproxy/core/enterprise/feishu"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
)

// IsTenantAllowed checks if a tenant is allowed to access the system.
// Priority:
// 1. Database config (if env_override is false)
// 2. Environment variable FEISHU_ALLOWED_TENANTS
// 3. If both empty, allow all
func IsTenantAllowed(tenantKey string) bool {
	// Get config from database
	var config models.TenantWhitelistConfig
	err := model.DB.FirstOrCreate(&config, models.TenantWhitelistConfig{ID: 1}).Error
	if err != nil {
		// Fallback to environment variable
		return feishu.IsTenantAllowedByEnv(tenantKey)
	}

	// Use environment variable if env_override is enabled
	if config.EnvOverride {
		return feishu.IsTenantAllowedByEnv(tenantKey)
	}

	// Wildcard mode: allow all
	if config.WildcardMode {
		return true
	}

	// Check database whitelist
	var count int64
	model.DB.Model(&models.TenantWhitelist{}).
		Where("tenant_id = ?", tenantKey).
		Count(&count)

	if count > 0 {
		return true
	}

	// If database has records but tenant not found, deny
	var totalCount int64
	model.DB.Model(&models.TenantWhitelist{}).Count(&totalCount)
	if totalCount > 0 {
		return false
	}

	// No database records, fallback to environment variable
	return feishu.IsTenantAllowedByEnv(tenantKey)
}
