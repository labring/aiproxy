//go:build enterprise

package quota

import (
	"context"
	"errors"
	"sync"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// pendingSyncs tracks token IDs that already have an in-flight auto-sync goroutine,
// preventing repeated goroutine spawns on every request for the same unsynced token.
var pendingSyncs sync.Map

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
			effModel, rpmMul, tpmMul, blocked := applyPolicyTiers(policy, token, requestModel)
			// Trigger async notification if usage entered a higher tier
			usageRatio := computeUsageRatio(token)
			tier := ComputeTier(policy, usageRatio, blocked)
			if tier >= 2 {
				go MaybeNotifyUser(
					feishuUser.OpenID,
					feishuUser.Name,
					token.PeriodType,
					tier,
					usageRatio,
					token.PeriodQuota,
					tierThreshold(policy, tier),
				)
			}
			// Admin webhook alert: independent threshold, checked on every request
			go MaybeNotifyAdmin(
				feishuUser.OpenID,
				feishuUser.Name,
				token.PeriodType,
				usageRatio,
				token.PeriodQuota,
			)

			return effModel, rpmMul, tpmMul, blocked
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

// computeUsageRatio returns the fraction of the period quota that has been consumed.
func computeUsageRatio(token model.TokenCache) float64 {
	if token.PeriodQuota <= 0 {
		return 0
	}

	used := token.UsedAmount - token.PeriodLastUpdateAmount
	if used < 0 {
		used = 0
	}

	return used / token.PeriodQuota
}

// ComputeTier returns the effective tier (1–4) for the given usage state.
// tier 1 = normal, 2 = tier2 throttle, 3 = tier3 throttle, 4 = exhausted/blocked.
func ComputeTier(policy *models.QuotaPolicy, usageRatio float64, blocked bool) int {
	switch {
	case blocked || usageRatio >= 1.0:
		if blocked && policy.BlockAtTier3 {
			return 4 // exhausted
		}

		return 3
	case usageRatio >= policy.Tier2Ratio:
		return 3
	case usageRatio >= policy.Tier1Ratio:
		return 2
	default:
		return 1
	}
}

// tierThreshold returns the ratio threshold that triggered the given tier notification.
func tierThreshold(policy *models.QuotaPolicy, tier int) float64 {
	switch tier {
	case 2:
		return policy.Tier1Ratio
	case 3:
		return policy.Tier2Ratio
	default: // 4 (exhaust)
		return 1.0
	}
}

// applyPolicyTiers applies the tiered policy logic based on usage ratio.
func applyPolicyTiers(policy *models.QuotaPolicy, token model.TokenCache, requestModel string) (string, float64, float64, bool) {
	// If token has no PeriodQuota but the policy does, the token was never synced
	// (e.g. user-created key). Use the policy's quota directly for enforcement,
	// and trigger a one-shot async sync to fix the token record.
	if token.PeriodQuota <= 0 && policy.PeriodQuota > 0 {
		if _, alreadyPending := pendingSyncs.LoadOrStore(token.ID, struct{}{}); !alreadyPending {
			go func() {
				defer pendingSyncs.Delete(token.ID)
				periodQuota := policy.PeriodQuota
				periodType := PolicyPeriodTypeToTokenPeriodType(policy.PeriodType)
				if _, err := model.UpdateToken(token.ID, model.UpdateTokenRequest{
					PeriodQuota: &periodQuota,
					PeriodType:  &periodType,
				}); err != nil {
					log.Errorf("auto-sync policy to unsynced token %d: %v", token.ID, err)
				}
			}()
		}
		token.PeriodQuota = policy.PeriodQuota
	}

	// Guard against zero PeriodQuota to avoid division by zero
	if token.PeriodQuota <= 0 {
		return requestModel, 1.0, 1.0, false
	}

	usageRatio := computeUsageRatio(token)

	// Resolve model pricing once for price-based blocking
	var inputPrice, outputPrice float64
	if mc := model.LoadModelCaches(); mc != nil {
		if cfg, ok := mc.ModelConfig.GetModelConfig(requestModel); ok {
			inputPrice = float64(cfg.Price.InputPrice)
			outputPrice = float64(cfg.Price.OutputPrice)
		}
	}

	switch {
	case usageRatio >= policy.Tier2Ratio:
		// Tier 3: usage >= Tier2Ratio
		if policy.BlockAtTier3 ||
			policy.IsModelBlockedAtTier(3, requestModel) ||
			policy.IsModelBlockedByPrice(3, inputPrice, outputPrice) {
			return requestModel, 0, 0, true
		}

		return requestModel, policy.Tier3RPMMultiplier, policy.Tier3TPMMultiplier, false
	case usageRatio >= policy.Tier1Ratio:
		// Tier 2: Tier1Ratio <= usage < Tier2Ratio
		if policy.IsModelBlockedAtTier(2, requestModel) ||
			policy.IsModelBlockedByPrice(2, inputPrice, outputPrice) {
			return requestModel, 0, 0, true
		}

		return requestModel, policy.Tier2RPMMultiplier, policy.Tier2TPMMultiplier, false
	default:
		// Tier 1: usage < Tier1Ratio
		return requestModel, policy.Tier1RPMMultiplier, policy.Tier1TPMMultiplier, false
	}
}
