package model

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const (
	GroupCacheKey = "group:%s"
)

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
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := common.RedisKeyf(GroupCacheKey, group.ID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, group)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

func CacheGetGroup(id string) (*GroupCache, error) {
	if !common.RedisEnabled {
		group, err := GetGroupByID(id, true)
		if err != nil {
			return nil, err
		}

		return group.ToGroupCache(), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cacheKey := common.RedisKeyf(GroupCacheKey, id)
	groupCache := &GroupCache{}

	err := common.RDB.HGetAll(ctx, cacheKey).Scan(groupCache)
	if err == nil && groupCache.Status != 0 {
		groupCache.ID = id
		return groupCache, nil
	} else if err != nil && !errors.Is(err, redis.Nil) {
		log.Errorf("get group (%s) from redis error: %s", id, err.Error())
	}

	group, err := GetGroupByID(id, true)
	if err != nil {
		return nil, err
	}

	gc := group.ToGroupCache()

	if err := CacheSetGroup(gc); err != nil {
		log.Error("redis set group error: " + err.Error())
	}

	return gc, nil
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
