//go:build enterprise

package quota

import (
	"testing"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
)

func TestApplyPolicyTiers_ModelBlocking(t *testing.T) {
	policy := &models.QuotaPolicy{
		Tier1Ratio:         0.7,
		Tier2Ratio:         0.9,
		Tier1RPMMultiplier: 1.0,
		Tier1TPMMultiplier: 1.0,
		Tier2RPMMultiplier: 0.5,
		Tier2TPMMultiplier: 0.5,
		Tier3RPMMultiplier: 0.1,
		Tier3TPMMultiplier: 0.1,
		BlockAtTier3:       false,
		Tier2BlockedModels: `["claude-opus-4*","gpt-4o"]`,
		Tier3BlockedModels: `["claude-opus-4*","gpt-4o","gpt-4o-mini"]`,
	}

	tests := []struct {
		name        string
		usedAmount  float64
		periodQuota float64
		model       string
		wantBlocked bool
		wantRPM     float64
		wantTPM     float64
	}{
		// Tier 1 (usage < 0.7): no model blocking
		{
			name: "tier1 expensive model allowed",
			usedAmount: 50, periodQuota: 100,
			model: "claude-opus-4-20250101", wantBlocked: false,
			wantRPM: 1.0, wantTPM: 1.0,
		},

		// Tier 2 (0.7 <= usage < 0.9): blocked models get rejected
		{
			name: "tier2 blocked model claude-opus",
			usedAmount: 75, periodQuota: 100,
			model: "claude-opus-4-20250101", wantBlocked: true,
		},
		{
			name: "tier2 blocked model gpt-4o",
			usedAmount: 75, periodQuota: 100,
			model: "gpt-4o", wantBlocked: true,
		},
		{
			name: "tier2 allowed model gpt-4o-mini",
			usedAmount: 75, periodQuota: 100,
			model: "gpt-4o-mini", wantBlocked: false,
			wantRPM: 0.5, wantTPM: 0.5,
		},
		{
			name: "tier2 allowed model claude-sonnet",
			usedAmount: 75, periodQuota: 100,
			model: "claude-sonnet-4-20250101", wantBlocked: false,
			wantRPM: 0.5, wantTPM: 0.5,
		},

		// Tier 3 (usage >= 0.9): more models blocked
		{
			name: "tier3 blocked model gpt-4o-mini",
			usedAmount: 95, periodQuota: 100,
			model: "gpt-4o-mini", wantBlocked: true,
		},
		{
			name: "tier3 allowed model claude-sonnet",
			usedAmount: 95, periodQuota: 100,
			model: "claude-sonnet-4-20250101", wantBlocked: false,
			wantRPM: 0.1, wantTPM: 0.1,
		},

		// Zero PeriodQuota: no blocking
		{
			name: "zero quota no blocking",
			usedAmount: 999, periodQuota: 0,
			model: "claude-opus-4-20250101", wantBlocked: false,
			wantRPM: 1.0, wantTPM: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := model.TokenCache{
				UsedAmount:             tt.usedAmount,
				PeriodLastUpdateAmount: 0,
				PeriodQuota:            tt.periodQuota,
			}

			_, rpm, tpm, blocked := applyPolicyTiers(policy, token, tt.model)

			if blocked != tt.wantBlocked {
				t.Fatalf("blocked = %v, want %v", blocked, tt.wantBlocked)
			}

			if !blocked {
				if rpm != tt.wantRPM {
					t.Errorf("rpm = %v, want %v", rpm, tt.wantRPM)
				}
				if tpm != tt.wantTPM {
					t.Errorf("tpm = %v, want %v", tpm, tt.wantTPM)
				}
			}
		})
	}
}

func TestApplyPolicyTiers_BlockAtTier3_WithModelBlock(t *testing.T) {
	// BlockAtTier3=true should still block even non-listed models
	policy := &models.QuotaPolicy{
		Tier1Ratio:         0.7,
		Tier2Ratio:         0.9,
		Tier1RPMMultiplier: 1.0,
		Tier1TPMMultiplier: 1.0,
		Tier3RPMMultiplier: 0.1,
		Tier3TPMMultiplier: 0.1,
		BlockAtTier3:       true,
		Tier3BlockedModels: `["gpt-4o"]`,
	}

	token := model.TokenCache{
		UsedAmount:             95,
		PeriodLastUpdateAmount: 0,
		PeriodQuota:            100,
	}

	// BlockAtTier3 blocks everything at tier 3
	_, _, _, blocked := applyPolicyTiers(policy, token, "claude-sonnet-4-20250101")
	if !blocked {
		t.Error("BlockAtTier3=true should block all models at tier 3")
	}
}

func TestApplyPolicyTiers_EmptyBlockedModels(t *testing.T) {
	// No blocked models configured — should behave as before
	policy := &models.QuotaPolicy{
		Tier1Ratio:         0.7,
		Tier2Ratio:         0.9,
		Tier2RPMMultiplier: 0.5,
		Tier2TPMMultiplier: 0.5,
		Tier3RPMMultiplier: 0.1,
		Tier3TPMMultiplier: 0.1,
	}

	token := model.TokenCache{
		UsedAmount:             80,
		PeriodLastUpdateAmount: 0,
		PeriodQuota:            100,
	}

	_, rpm, tpm, blocked := applyPolicyTiers(policy, token, "claude-opus-4-20250101")
	if blocked {
		t.Error("should not block with empty blocked models")
	}
	if rpm != 0.5 || tpm != 0.5 {
		t.Errorf("rpm=%v tpm=%v, want 0.5/0.5", rpm, tpm)
	}
}
