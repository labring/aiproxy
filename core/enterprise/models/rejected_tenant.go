//go:build enterprise

package models

import "time"

// RejectedTenantLogin records login attempts from tenants not in the whitelist.
// Same tenant_id merges into one record with attempt_count incremented.
type RejectedTenantLogin struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	TenantID      string    `json:"tenant_id" gorm:"uniqueIndex;size:64;not null"`
	UserName      string    `json:"user_name" gorm:"size:128"`
	UserEmail     string    `json:"user_email" gorm:"size:256"`
	AttemptCount  int       `json:"attempt_count" gorm:"default:1"`
	LastAttemptAt time.Time `json:"last_attempt_at"`
	CreatedAt     time.Time `json:"created_at"`
}
