package model

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	GroupCacheKey = "group:%s"
)

func getGroupCacheKey(id string) string {
	return common.RedisKeyf(GroupCacheKey, id)
}

func updateGroupLocalCache(id string, update func(*GroupCache) bool) {
	cacheUpdateModelLocal(getGroupCacheKey(id), cloneGroupCache, update)
}

type GroupCache struct {
	ID            string                   `json:"-"              redis:"-"`
	Status        int                      `json:"status"         redis:"st"`
	UsedAmount    float64                  `json:"used_amount"    redis:"ua"`
	RPMRatio      float64                  `json:"rpm_ratio"      redis:"rpm_r"`
	TPMRatio      float64                  `json:"tpm_ratio"      redis:"tpm_r"`
	AvailableSets redisStringSlice         `json:"available_sets" redis:"ass"`
	ModelConfigs  redisGroupModelConfigMap `json:"model_configs"  redis:"mc"`

	BalanceAlertEnabled   bool    `json:"balance_alert_enabled"   redis:"bae"`
	BalanceAlertThreshold float64 `json:"balance_alert_threshold" redis:"bat"`
}

func (g *GroupCache) GetAvailableSets() []string {
	if len(g.AvailableSets) == 0 {
		return []string{ChannelDefaultSet}
	}
	return g.AvailableSets
}

func (g *Group) ToGroupCache() *GroupCache {
	modelConfigs := make(redisGroupModelConfigMap, len(g.GroupModelConfigs))
	for _, modelConfig := range g.GroupModelConfigs {
		modelConfigs[modelConfig.Model] = modelConfig
	}

	return &GroupCache{
		ID:            g.ID,
		Status:        g.Status,
		UsedAmount:    g.UsedAmount,
		RPMRatio:      g.RPMRatio,
		TPMRatio:      g.TPMRatio,
		AvailableSets: g.AvailableSets,
		ModelConfigs:  modelConfigs,

		BalanceAlertEnabled:   g.BalanceAlertEnabled,
		BalanceAlertThreshold: g.BalanceAlertThreshold,
	}
}

func CacheDeleteGroup(id string) error {
	cacheDeleteModelLocal(getGroupCacheKey(id))

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, common.RedisKeyf(GroupCacheKey, id)).Err()
}

var updateGroupRPMRatioScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "rpm_r") then
		redis.call("HSet", KEYS[1], "rpm_r", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdateGroupRPMRatio(id string, rpmRatio float64) error {
	updateGroupLocalCache(id, func(group *GroupCache) bool {
		group.RPMRatio = rpmRatio
		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateGroupRPMRatioScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(GroupCacheKey, id)},
		rpmRatio,
	).Err()
}

var updateGroupTPMRatioScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "tpm_r") then
		redis.call("HSet", KEYS[1], "tpm_r", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdateGroupTPMRatio(id string, tpmRatio float64) error {
	updateGroupLocalCache(id, func(group *GroupCache) bool {
		group.TPMRatio = tpmRatio
		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateGroupTPMRatioScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(GroupCacheKey, id)},
		tpmRatio,
	).Err()
}

var updateGroupStatusScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "st") then
		redis.call("HSet", KEYS[1], "st", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdateGroupStatus(id string, status int) error {
	updateGroupLocalCache(id, func(group *GroupCache) bool {
		group.Status = status
		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateGroupStatusScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(GroupCacheKey, id)},
		status,
	).Err()
}

func CacheSetGroup(group *GroupCache) error {
	key := getGroupCacheKey(group.ID)
	cacheSetModelLocal(key, group, cloneGroupCache)
	return cacheSetGroupRedis(group)
}

func CacheGetGroup(id string) (*GroupCache, error) {
	cacheKey := getGroupCacheKey(id)
	if groupCache, notFound, ok := cacheGetModelLocal(cacheKey, cloneGroupCache); ok {
		if notFound {
			return nil, NotFoundError(ErrGroupNotFound)
		}

		return groupCache, nil
	}

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		groupCache := &GroupCache{}

		err := common.RDB.HGetAll(ctx, cacheKey).Scan(groupCache)
		if err == nil && groupCache.Status != 0 {
			groupCache.ID = id
			cacheSetModelLocal(cacheKey, groupCache, cloneGroupCache)
			return groupCache, nil
		} else if err != nil && !errors.Is(err, redis.Nil) {
			log.Errorf("get group (%s) from redis error: %s", id, err.Error())
		}
	}

	groupCache, notFound, loaded, err := loadWithLocalKeyLock(
		modelCacheLoadLocker,
		cacheKey,
		func() (*GroupCache, bool, bool) {
			return cacheGetModelLocal(cacheKey, cloneGroupCache)
		},
		func() (*GroupCache, error) {
			group, err := GetGroupByID(id, true)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					cacheSetModelNotFoundLocalUnlocked(cacheKey)
				}

				return nil, err
			}

			gc := group.ToGroupCache()
			cacheSetModelLocalUnlocked(cacheKey, gc, cloneGroupCache)

			return gc, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if notFound {
		return nil, NotFoundError(ErrGroupNotFound)
	}

	if loaded {
		if err := cacheSetGroupRedis(groupCache); err != nil {
			log.Error("redis set group error: " + err.Error())
		}
	}

	return groupCache, nil
}

func cacheSetGroupRedis(group *GroupCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := getGroupCacheKey(group.ID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, group)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

var updateGroupUsedAmountOnlyIncreaseScript = redis.NewScript(`
	local used_amount = redis.call("HGet", KEYS[1], "ua")
	if used_amount == false then
		return redis.status_reply("ok")
	end
	if ARGV[1] < used_amount then
		return redis.status_reply("ok")
	end
	redis.call("HSet", KEYS[1], "ua", ARGV[1])
	return redis.status_reply("ok")
`)

func CacheUpdateGroupUsedAmountOnlyIncrease(id string, amount float64) error {
	updateGroupLocalCache(id, func(group *GroupCache) bool {
		if amount < group.UsedAmount {
			return false
		}

		group.UsedAmount = amount

		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateGroupUsedAmountOnlyIncreaseScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(GroupCacheKey, id)},
		amount,
	).Err()
}
