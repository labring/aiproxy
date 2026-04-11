//go:build enterprise

package quota

import (
	"testing"

	"github.com/labring/aiproxy/core/enterprise/models"
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
		PeriodQuota:        100,
	}

	tests := []struct {
		name        string
		usageRatio  float64
		model       string
		wantBlocked bool
		wantRPM     float64
		wantTPM     float64
	}{
		// Tier 1 (usage < 0.7): no model blocking
		{
			name: "tier1 expensive model allowed",
			usageRatio: 0.5,
			model: "claude-opus-4-20250101", wantBlocked: false,
			wantRPM: 1.0, wantTPM: 1.0,
		},

		// Tier 2 (0.7 <= usage < 0.9): blocked models get rejected
		{
			name: "tier2 blocked model claude-opus",
			usageRatio: 0.75,
			model: "claude-opus-4-20250101", wantBlocked: true,
		},
		{
			name: "tier2 blocked model gpt-4o",
			usageRatio: 0.75,
			model: "gpt-4o", wantBlocked: true,
		},
		{
			name: "tier2 allowed model gpt-4o-mini",
			usageRatio: 0.75,
			model: "gpt-4o-mini", wantBlocked: false,
			wantRPM: 0.5, wantTPM: 0.5,
		},
		{
			name: "tier2 allowed model claude-sonnet",
			usageRatio: 0.75,
			model: "claude-sonnet-4-20250101", wantBlocked: false,
			wantRPM: 0.5, wantTPM: 0.5,
		},

		// Tier 3 (usage >= 0.9): more models blocked
		{
			name: "tier3 blocked model gpt-4o-mini",
			usageRatio: 0.95,
			model: "gpt-4o-mini", wantBlocked: true,
		},
		{
			name: "tier3 allowed model claude-sonnet",
			usageRatio: 0.95,
			model: "claude-sonnet-4-20250101", wantBlocked: false,
			wantRPM: 0.1, wantTPM: 0.1,
		},

		// Zero PeriodQuota: no blocking (tested via separate policy)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, rpm, tpm, blocked := applyPolicyTiers(policy, tt.usageRatio, tt.model)

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
		PeriodQuota:        100,
	}

	// BlockAtTier3 blocks everything at tier 3
	_, _, _, blocked := applyPolicyTiers(policy, 0.95, "claude-sonnet-4-20250101")
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
		PeriodQuota:        100,
	}

	_, rpm, tpm, blocked := applyPolicyTiers(policy, 0.8, "claude-opus-4-20250101")
	if blocked {
		t.Error("should not block with empty blocked models")
	}
	if rpm != 0.5 || tpm != 0.5 {
		t.Errorf("rpm=%v tpm=%v, want 0.5/0.5", rpm, tpm)
	}
}

func TestApplyPolicyTiers_ZeroPeriodQuota(t *testing.T) {
	// Zero PeriodQuota means no quota enforcement
	policy := &models.QuotaPolicy{
		Tier1Ratio:  0.7,
		Tier2Ratio:  0.9,
		PeriodQuota: 0,
	}

	_, rpm, tpm, blocked := applyPolicyTiers(policy, 0, "claude-opus-4-20250101")
	if blocked {
		t.Error("zero PeriodQuota should not block")
	}
	if rpm != 1.0 || tpm != 1.0 {
		t.Errorf("rpm=%v tpm=%v, want 1.0/1.0", rpm, tpm)
	}
}

func TestComputeGroupUsageRatio(t *testing.T) {
	// Zero quota → ratio 0
	policy := &models.QuotaPolicy{PeriodQuota: 0}
	if r := computeGroupUsageRatio("test-group", policy); r != 0 {
		t.Errorf("zero quota: got %v, want 0", r)
	}
}
