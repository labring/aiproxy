//go:build enterprise

package quota

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	quotaPolicyCacheKey = "enterprise:quota_policy:group:%s"
	cacheTTL            = 10 * time.Minute
	redisTimeout        = 2 * time.Second
)

// GetGroupQuotaPolicy fetches the QuotaPolicy bound to a group.
// It checks Redis cache first, then falls back to the database.
// Returns nil if no policy is bound to the group.
func GetGroupQuotaPolicy(ctx context.Context, groupID string) (*models.QuotaPolicy, error) {
	if common.RedisEnabled {
		policy, err := getGroupQuotaPolicyFromCache(ctx, groupID)
		if err == nil && policy != nil {
			return policy, nil
		}

		if err != nil && !errors.Is(err, redis.Nil) {
			log.Errorf("get quota policy for group %s from redis error: %v", groupID, err)
		}
	}

	// Fallback to database
	policy, err := getGroupQuotaPolicyFromDB(groupID)
	if err != nil {
		return nil, err
	}

	if policy == nil {
		return nil, nil
	}

	// Update cache
	if common.RedisEnabled {
		if cacheErr := SetGroupQuotaPolicy(ctx, groupID, policy); cacheErr != nil {
			log.Errorf("set quota policy cache for group %s error: %v", groupID, cacheErr)
		}
	}

	return policy, nil
}

func getGroupQuotaPolicyFromCache(ctx context.Context, groupID string) (*models.QuotaPolicy, error) {
	ctx, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	key := common.RedisKeyf(quotaPolicyCacheKey, groupID)

	data, err := common.RDB.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var policy models.QuotaPolicy
	if err := sonic.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("unmarshal quota policy: %w", err)
	}

	return &policy, nil
}

func getGroupQuotaPolicyFromDB(groupID string) (*models.QuotaPolicy, error) {
	var binding models.GroupQuotaPolicy

	err := model.DB.
		Where("group_id = ?", groupID).
		Preload("QuotaPolicy").
		First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("query group quota policy: %w", err)
	}

	return binding.QuotaPolicy, nil
}

// SetGroupQuotaPolicy stores the QuotaPolicy in Redis cache.
func SetGroupQuotaPolicy(ctx context.Context, groupID string, policy *models.QuotaPolicy) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	data, err := sonic.Marshal(policy)
	if err != nil {
		return fmt.Errorf("marshal quota policy: %w", err)
	}

	key := common.RedisKeyf(quotaPolicyCacheKey, groupID)

	return common.RDB.Set(ctx, key, data, cacheTTL).Err()
}

// InvalidateGroupQuotaPolicy removes the cached QuotaPolicy for a group.
func InvalidateGroupQuotaPolicy(ctx context.Context, groupID string) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	key := common.RedisKeyf(quotaPolicyCacheKey, groupID)

	return common.RDB.Del(ctx, key).Err()
}

// GetPolicyForUser returns the effective quota policy for a FeishuUser.
// Priority: UserQuotaPolicy > DepartmentQuotaPolicy > GroupQuotaPolicy
func GetPolicyForUser(ctx context.Context, openID string) (*models.QuotaPolicy, error) {
	var user models.FeishuUser
	if err := model.DB.Where("open_id = ?", openID).First(&user).Error; err != nil {
		return nil, err
	}

	// 1. Check user-level policy (highest priority)
	var userPolicy models.UserQuotaPolicy
	err := model.DB.Preload("QuotaPolicy").Where("open_id = ?", openID).First(&userPolicy).Error
	if err == nil && userPolicy.QuotaPolicy != nil {
		return userPolicy.QuotaPolicy, nil
	}

	// 2. Check department-level policy
	if user.DepartmentID != "" {
		var deptPolicy models.DepartmentQuotaPolicy
		err = model.DB.Preload("QuotaPolicy").Where("department_id = ?", user.DepartmentID).First(&deptPolicy).Error
		if err == nil && deptPolicy.QuotaPolicy != nil {
			return deptPolicy.QuotaPolicy, nil
		}
	}

	// 3. Check group-level policy (lowest priority, backwards compatible)
	if user.GroupID != "" {
		return GetGroupQuotaPolicy(ctx, user.GroupID)
	}

	return nil, gorm.ErrRecordNotFound
}
