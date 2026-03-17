//go:build enterprise

package models

import (
	"time"

	"gorm.io/gorm"
)

// QuotaPolicy defines a progressive quota tier strategy.
// When a group's period usage reaches a tier threshold, the system can
// adjust RPM/TPM multipliers or block access entirely.
type QuotaPolicy struct {
	ID        int            `json:"id"         gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"          gorm:"index"`
	Name      string         `json:"name"       gorm:"size:128;not null"`

	// Tier thresholds as ratios of period quota (0.0-1.0)
	// Tier1: usage < Tier1Ratio → normal access
	// Tier2: Tier1Ratio <= usage < Tier2Ratio → reduced RPM/TPM
	// Tier3: usage >= Tier2Ratio → further reduction or block
	Tier1Ratio float64 `json:"tier1_ratio" gorm:"default:0.7"`
	Tier2Ratio float64 `json:"tier2_ratio" gorm:"default:0.9"`

	// RPM/TPM multipliers for each tier
	Tier1RPMMultiplier float64 `json:"tier1_rpm_multiplier" gorm:"default:1.0"`
	Tier1TPMMultiplier float64 `json:"tier1_tpm_multiplier" gorm:"default:1.0"`
	Tier2RPMMultiplier float64 `json:"tier2_rpm_multiplier" gorm:"default:0.5"`
	Tier2TPMMultiplier float64 `json:"tier2_tpm_multiplier" gorm:"default:0.5"`
	Tier3RPMMultiplier float64 `json:"tier3_rpm_multiplier" gorm:"default:0.1"`
	Tier3TPMMultiplier float64 `json:"tier3_tpm_multiplier" gorm:"default:0.1"`

	// Whether to completely block requests at tier3
	BlockAtTier3 bool `json:"block_at_tier3" gorm:"default:false"`
}

func (QuotaPolicy) TableName() string {
	return "enterprise_quota_policies"
}

// GroupQuotaPolicy binds a QuotaPolicy to a Group.
type GroupQuotaPolicy struct {
	ID            int            `json:"id"             gorm:"primaryKey"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-"              gorm:"index"`
	GroupID       string         `json:"group_id"       gorm:"size:64;uniqueIndex;not null"`
	QuotaPolicyID int            `json:"quota_policy_id" gorm:"index;not null"`
	QuotaPolicy   *QuotaPolicy   `json:"quota_policy"   gorm:"foreignKey:QuotaPolicyID"`
}

func (GroupQuotaPolicy) TableName() string {
	return "enterprise_group_quota_policies"
}

// DepartmentQuotaPolicy binds a QuotaPolicy to a department (all users in dept inherit it).
type DepartmentQuotaPolicy struct {
	ID            int            `json:"id"             gorm:"primaryKey"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-"              gorm:"index"`
	DepartmentID  string         `json:"department_id"  gorm:"size:64;uniqueIndex;not null"`
	QuotaPolicyID int            `json:"quota_policy_id" gorm:"index;not null"`
	QuotaPolicy   *QuotaPolicy   `json:"quota_policy"   gorm:"foreignKey:QuotaPolicyID"`
}

func (DepartmentQuotaPolicy) TableName() string {
	return "enterprise_department_quota_policies"
}

// UserQuotaPolicy binds a QuotaPolicy to a specific user (overrides department policy).
type UserQuotaPolicy struct {
	ID            int            `json:"id"             gorm:"primaryKey"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-"              gorm:"index"`
	OpenID        string         `json:"open_id"        gorm:"size:64;uniqueIndex;not null"`
	QuotaPolicyID int            `json:"quota_policy_id" gorm:"index;not null"`
	QuotaPolicy   *QuotaPolicy   `json:"quota_policy"   gorm:"foreignKey:QuotaPolicyID"`
}

func (UserQuotaPolicy) TableName() string {
	return "enterprise_user_quota_policies"
}
