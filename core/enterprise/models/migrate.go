//go:build enterprise

package models

import "gorm.io/gorm"

// EnterpriseAutoMigrate runs database migrations for all enterprise tables.
func EnterpriseAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&FeishuUser{},
		&FeishuDepartment{},
		&QuotaPolicy{},
		&GroupQuotaPolicy{},
	)
}
