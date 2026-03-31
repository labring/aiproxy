//go:build enterprise

package models

import (
	"time"

	log "github.com/sirupsen/logrus"
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

// NovitaSyncHistory mirrors novita.SyncHistory for migration purposes.
// Defined here to avoid circular dependency (novita → model → models → novita).
type NovitaSyncHistory struct {
	ID          int64     `gorm:"primaryKey"`
	SyncedAt    time.Time `gorm:"autoCreateTime;index"`
	Operator    string
	SyncOptions string
	Result      string
	Status      string
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (NovitaSyncHistory) TableName() string {
	return "novita_sync_history"
}

// EnterpriseAutoMigrate runs database migrations for all enterprise tables.
func EnterpriseAutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&FeishuUser{},
		&FeishuDepartment{},
		&QuotaPolicy{},
		&GroupQuotaPolicy{},
		&TenantWhitelist{},
		&TenantWhitelistConfig{},
		&DepartmentQuotaPolicy{},
		&UserQuotaPolicy{},
		&PPIOSyncHistory{},
		&NovitaSyncHistory{},
		&FeishuSyncHistory{},
		&RejectedTenantLogin{},
		&RolePermission{},
		&QuotaAlertHistory{},
	); err != nil {
		return err
	}

	// Seed default role permissions if table is empty
	seedDefaultRolePermissions(db)

	// Migrate V1 (single key) → V2 (view/manage split)
	migratePermissionsV2(db)

	return nil
}

// seedDefaultRolePermissions inserts default permissions if the table is empty.
func seedDefaultRolePermissions(db *gorm.DB) {
	var count int64
	db.Model(&RolePermission{}).Count(&count)

	if count > 0 {
		return
	}

	var records []RolePermission
	for role, perms := range DefaultRolePermissions {
		for _, perm := range perms {
			records = append(records, RolePermission{
				Role:       role,
				Permission: perm,
			})
		}
	}

	if err := db.Create(&records).Error; err != nil {
		log.Errorf("failed to seed default role permissions: %v", err)
		return
	}

	log.Infof("seeded %d default role permissions", len(records))
}

// migratePermissionsV2 converts old single-key permissions (e.g. "dashboard")
// to the new view/manage split (e.g. "dashboard_view" + "dashboard_manage").
// It only runs when old-format records exist but new-format ones do not.
func migratePermissionsV2(db *gorm.DB) {
	// Check if old format exists
	var oldCount int64
	db.Model(&RolePermission{}).Where("permission = ?", "dashboard").Count(&oldCount)

	if oldCount == 0 {
		return // already migrated or fresh install
	}

	// Check if new format already exists
	var newCount int64
	db.Model(&RolePermission{}).Where("permission = ?", "dashboard_view").Count(&newCount)

	if newCount > 0 {
		return // already migrated
	}

	log.Info("migrating permissions from V1 to V2 (view/manage split)...")

	var oldPerms []RolePermission
	db.Find(&oldPerms)

	var newRecords []RolePermission
	for _, op := range oldPerms {
		// Each old permission becomes both view and manage
		newRecords = append(newRecords,
			RolePermission{Role: op.Role, Permission: ViewPermission(op.Permission)},
			RolePermission{Role: op.Role, Permission: ManagePermission(op.Permission)},
		)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		// Delete all old records
		if err := tx.Where("1=1").Delete(&RolePermission{}).Error; err != nil {
			return err
		}

		// Insert new records
		if len(newRecords) > 0 {
			if err := tx.Create(&newRecords).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		log.Errorf("failed to migrate permissions to V2: %v", err)
		return
	}

	log.Infof("migrated %d old permission records to %d new records", len(oldPerms), len(newRecords))
}
