//go:build enterprise

package models

import (
	"encoding/json"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

// PeriodType constants for quota policy period configuration.
const (
	PeriodTypeDaily   = 1
	PeriodTypeWeekly  = 2
	PeriodTypeMonthly = 3
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

	// Models blocked when entering each tier (JSON array of model name patterns)
	// e.g. ["claude-opus-4*", "gpt-4o"] — supports glob patterns
	Tier2BlockedModels string `json:"tier2_blocked_models" gorm:"size:1024;default:''"`
	Tier3BlockedModels string `json:"tier3_blocked_models" gorm:"size:1024;default:''"`

	// Price-based model blocking: block models whose input/output price (¥/M tokens)
	// exceeds the threshold. Condition controls how thresholds combine ("and"/"or").
	// Threshold 0 = that dimension disabled. Both 0 = rule inactive.
	Tier2PriceInputThreshold  float64 `json:"tier2_price_input_threshold"  gorm:"default:0"`
	Tier2PriceOutputThreshold float64 `json:"tier2_price_output_threshold" gorm:"default:0"`
	Tier2PriceCondition       string  `json:"tier2_price_condition"        gorm:"size:4;default:'or'"`
	Tier3PriceInputThreshold  float64 `json:"tier3_price_input_threshold"  gorm:"default:0"`
	Tier3PriceOutputThreshold float64 `json:"tier3_price_output_threshold" gorm:"default:0"`
	Tier3PriceCondition       string  `json:"tier3_price_condition"        gorm:"size:4;default:'or'"`

	// Amount limit fields — synced to Token.PeriodQuota/PeriodType when binding
	PeriodQuota float64 `json:"period_quota" gorm:"default:0"` // 0 = no limit, unit: currency
	PeriodType  int     `json:"period_type"  gorm:"default:3"` // 1=daily, 2=weekly, 3=monthly
}

func (QuotaPolicy) TableName() string {
	return "enterprise_quota_policies"
}

func parseBlockedModels(raw string) []string {
	if raw == "" {
		return nil
	}

	var models []string
	if err := json.Unmarshal([]byte(raw), &models); err != nil {
		return nil
	}

	return models
}

// GetTier2BlockedModels parses the Tier2BlockedModels JSON field.
func (p *QuotaPolicy) GetTier2BlockedModels() []string {
	return parseBlockedModels(p.Tier2BlockedModels)
}

// GetTier3BlockedModels parses the Tier3BlockedModels JSON field.
func (p *QuotaPolicy) GetTier3BlockedModels() []string {
	return parseBlockedModels(p.Tier3BlockedModels)
}

// IsModelBlockedAtTier checks whether a model is blocked at the given tier.
// Supports glob patterns (e.g. "claude-opus-4*").
func (p *QuotaPolicy) IsModelBlockedAtTier(tier int, model string) bool {
	var patterns []string

	switch tier {
	case 2:
		patterns = p.GetTier2BlockedModels()
	case 3:
		patterns = p.GetTier3BlockedModels()
	default:
		return false
	}

	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, model); matched {
			return true
		}
	}

	return false
}

// IsModelBlockedByPrice checks whether a model should be blocked at the given
// tier based on its input/output price (¥/M tokens, normalized by caller).
func (p *QuotaPolicy) IsModelBlockedByPrice(tier int, inputPrice, outputPrice float64) bool {
	var inThresh, outThresh float64
	var cond string

	switch tier {
	case 2:
		inThresh = p.Tier2PriceInputThreshold
		outThresh = p.Tier2PriceOutputThreshold
		cond = p.Tier2PriceCondition
	case 3:
		inThresh = p.Tier3PriceInputThreshold
		outThresh = p.Tier3PriceOutputThreshold
		cond = p.Tier3PriceCondition
	default:
		return false
	}

	inActive := inThresh > 0
	outActive := outThresh > 0

	if !inActive && !outActive {
		return false
	}

	inMatch := inActive && inputPrice >= inThresh
	outMatch := outActive && outputPrice >= outThresh

	// Single dimension active — only check that one
	if !inActive {
		return outMatch
	}
	if !outActive {
		return inMatch
	}

	// Both dimensions active — apply condition
	if cond == "and" {
		return inMatch && outMatch
	}
	return inMatch || outMatch // default "or"
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

// QuotaAlertHistory records each quota tier notification sent to a user.
type QuotaAlertHistory struct {
	ID          int64     `json:"id"           gorm:"primaryKey"`
	CreatedAt   time.Time `json:"created_at"   gorm:"autoCreateTime;index"`
	OpenID      string    `json:"open_id"      gorm:"size:64;index;not null"`
	UserName    string    `json:"user_name"    gorm:"size:128"`
	Tier        int       `json:"tier"         gorm:"not null"`           // 2, 3, or 4 (exhausted)
	UsageRatio  float64   `json:"usage_ratio"  gorm:"not null"`           // 0.0-1.0+
	PeriodQuota float64   `json:"period_quota" gorm:"not null"`           // currency amount
	PeriodType  string    `json:"period_type"  gorm:"size:16;not null"`   // daily, weekly, monthly
	Title       string    `json:"title"        gorm:"size:256"`
	Body        string    `json:"body"         gorm:"size:1024"`
	Status      string    `json:"status"       gorm:"size:16;not null"`   // sent, failed
	Error       string    `json:"error,omitempty" gorm:"size:512"`
}

func (QuotaAlertHistory) TableName() string {
	return "enterprise_quota_alert_histories"
}
