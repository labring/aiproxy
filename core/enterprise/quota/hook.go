//go:build enterprise

package quota

import (
	"context"
	"errors"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CheckQuotaTier evaluates the token's period usage against the quota policy
// with multi-level priority: user policy > department policy > group policy.
// Returns: effectiveModel, rpmMultiplier, tpmMultiplier, blocked
func CheckQuotaTier(
	group model.GroupCache,
	token model.TokenCache,
	requestModel string,
) (string, float64, float64, bool) {
	ctx := context.Background()

	// Check if this group is associated with a Feishu user
	// If so, use multi-level policy (user > department > group)
	var feishuUser models.FeishuUser
	err := model.DB.Where("group_id = ?", group.ID).First(&feishuUser).Error
	if err == nil {
		// This is a Feishu user group, check multi-level policies
		policy, err := GetPolicyForUser(ctx, feishuUser.OpenID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("failed to get multi-level quota policy for user %s: %v", feishuUser.OpenID, err)
		}
		if policy != nil {
			// Apply the user/department/group policy
			return applyPolicyTiers(policy, token, requestModel)
		}
		// No policy found for this Feishu user, fall through to default
		return requestModel, 1.0, 1.0, false
	}

	// Not a Feishu user or user not found, use traditional group-level policy
	policy, err := GetGroupQuotaPolicy(ctx, group.ID)
	if err != nil {
		log.Errorf("failed to get quota policy for group %s: %v", group.ID, err)
		return requestModel, 1.0, 1.0, false
	}

	if policy == nil {
		return requestModel, 1.0, 1.0, false
	}

	return applyPolicyTiers(policy, token, requestModel)
}

// applyPolicyTiers applies the tiered policy logic based on usage ratio.
func applyPolicyTiers(policy *models.QuotaPolicy, token model.TokenCache, requestModel string) (string, float64, float64, bool) {

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
		if policy.BlockAtTier3 || policy.IsModelBlockedAtTier(3, requestModel) {
			return requestModel, 0, 0, true
		}

		return requestModel, policy.Tier3RPMMultiplier, policy.Tier3TPMMultiplier, false
	case usageRatio >= policy.Tier1Ratio:
		// Tier 2: Tier1Ratio <= usage < Tier2Ratio
		if policy.IsModelBlockedAtTier(2, requestModel) {
			return requestModel, 0, 0, true
		}

		return requestModel, policy.Tier2RPMMultiplier, policy.Tier2TPMMultiplier, false
	default:
		// Tier 1: usage < Tier1Ratio
		return requestModel, policy.Tier1RPMMultiplier, policy.Tier1TPMMultiplier, false
	}
}
