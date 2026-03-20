//go:build enterprise

package models

import (
	"time"

	"gorm.io/gorm"
)

// PPIOSyncHistory mirrors ppio.SyncHistory for migration purposes.
// Defined here to avoid circular dependency (ppio → model → models → ppio).
type PPIOSyncHistory struct {
	ID          int64     `gorm:"primaryKey"`
	SyncedAt    time.Time `gorm:"autoCreateTime;index"`
	Operator    string
	SyncOptions string
	Result      string
	Status      string
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (PPIOSyncHistory) TableName() string {
	return "ppio_sync_history"
}

// EnterpriseAutoMigrate runs database migrations for all enterprise tables.
func EnterpriseAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&FeishuUser{},
		&FeishuDepartment{},
		&QuotaPolicy{},
		&GroupQuotaPolicy{},
		&TenantWhitelist{},
		&TenantWhitelistConfig{},
		&DepartmentQuotaPolicy{},
		&UserQuotaPolicy{},
		&PPIOSyncHistory{},
		&RejectedTenantLogin{},
	)
}
