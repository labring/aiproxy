//go:build enterprise

package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
	log "github.com/sirupsen/logrus"
)

const (
	groupUsageCacheKey = "enterprise:group_period_usage:%s:%s"
	groupUsageCacheTTL = 30 * time.Second
)

// getCachedGroupPeriodUsage returns the total used_amount for a group in the current period.
// Uses Redis cache (30s TTL) to reduce DB pressure; falls back to direct DB query.
func getCachedGroupPeriodUsage(groupID string, periodType int) float64 {
	periodStart := PeriodStartByType(periodType)
	cacheKey := common.RedisKeyf(groupUsageCacheKey, groupID, periodCacheKey(periodType, periodStart))

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		val, err := common.RDB.Get(ctx, cacheKey).Float64()
		if err == nil {
			return val
		}
	}

	used := queryGroupPeriodUsage(groupID, periodStart)

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		if err := common.RDB.Set(ctx, cacheKey, used, groupUsageCacheTTL).Err(); err != nil {
			log.Errorf("cache group period usage for %s: %v", groupID, err)
		}
	}

	return used
}

func queryGroupPeriodUsage(groupID string, periodStart time.Time) float64 {
	var used float64
	if err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select("COALESCE(SUM(used_amount), 0)").
		Where("group_id = ?", groupID).
		Where("hour_timestamp >= ?", periodStart.Unix()).
		Scan(&used).Error; err != nil {
		log.Errorf("query group period usage for %s: %v", groupID, err)
		return 0
	}

	return used
}

func periodCacheKey(periodType int, periodStart time.Time) string {
	switch periodType {
	case models.PeriodTypeDaily:
		return fmt.Sprintf("daily:%s", periodStart.Format("2006-01-02"))
	case models.PeriodTypeWeekly:
		return fmt.Sprintf("weekly:%s", periodStart.Format("2006-01-02"))
	default:
		return fmt.Sprintf("monthly:%s", periodStart.Format("2006-01"))
	}
}
