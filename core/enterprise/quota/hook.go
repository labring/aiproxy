//go:build enterprise

package quota

import (
	"context"

	"github.com/labring/aiproxy/core/model"
	log "github.com/sirupsen/logrus"
)

// CheckQuotaTier evaluates the token's period usage against the group's quota policy
// and returns adjusted RPM/TPM multipliers or a block signal.
// Returns: effectiveModel, rpmMultiplier, tpmMultiplier, blocked
func CheckQuotaTier(
	group model.GroupCache,
	token model.TokenCache,
	requestModel string,
) (string, float64, float64, bool) {
	ctx := context.Background()

	policy, err := GetGroupQuotaPolicy(ctx, group.ID)
	if err != nil {
		log.Errorf("failed to get quota policy for group %s: %v", group.ID, err)
		return requestModel, 1.0, 1.0, false
	}

	if policy == nil {
		return requestModel, 1.0, 1.0, false
	}

	// Guard against zero PeriodQuota to avoid division by zero
	if token.PeriodQuota <= 0 {
		return requestModel, 1.0, 1.0, false
	}

	periodUsage := token.UsedAmount - token.PeriodLastUpdateAmount
	if periodUsage < 0 {
		periodUsage = 0
	}

	usageRatio := periodUsage / token.PeriodQuota

	switch {
	case usageRatio >= policy.Tier2Ratio:
		// Tier 3: usage >= Tier2Ratio
		if policy.BlockAtTier3 {
			return requestModel, 0, 0, true
		}

		return requestModel, policy.Tier3RPMMultiplier, policy.Tier3TPMMultiplier, false
	case usageRatio >= policy.Tier1Ratio:
		// Tier 2: Tier1Ratio <= usage < Tier2Ratio
		return requestModel, policy.Tier2RPMMultiplier, policy.Tier2TPMMultiplier, false
	default:
		// Tier 1: usage < Tier1Ratio
		return requestModel, policy.Tier1RPMMultiplier, policy.Tier1TPMMultiplier, false
	}
}
