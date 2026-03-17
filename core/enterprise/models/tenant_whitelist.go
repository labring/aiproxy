//go:build enterprise

package models

import "time"

// TenantWhitelist stores allowed Feishu tenant IDs.
// If no records exist and FEISHU_ALLOWED_TENANTS env is not set, all tenants are allowed.
type TenantWhitelist struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	TenantID  string    `gorm:"uniqueIndex;size:64;not null" json:"tenant_id"`
	TenantKey string    `gorm:"size:64" json:"tenant_key"` // Same as TenantID for compatibility
	Name      string    `gorm:"size:255" json:"name"`      // Organization name (optional)
	AddedBy   string    `gorm:"size:64" json:"added_by"`   // Admin who added this tenant
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (TenantWhitelist) TableName() string {
	return "tenant_whitelist"
}

// TenantWhitelistConfig stores global configuration for tenant access control.
type TenantWhitelistConfig struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	WildcardMode  bool      `gorm:"default:false" json:"wildcard_mode"` // If true, allow all tenants
	EnvOverride   bool      `gorm:"default:false" json:"env_override"`  // If true, use FEISHU_ALLOWED_TENANTS env var
	Description   string    `gorm:"type:text" json:"description"`
	LastUpdatedBy string    `gorm:"size:64" json:"last_updated_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (TenantWhitelistConfig) TableName() string {
	return "tenant_whitelist_config"
}
