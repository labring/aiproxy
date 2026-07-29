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
	TokenCacheKey = "token:%s"
)

func getTokenCacheKey(key string) string {
	return common.RedisKeyf(TokenCacheKey, key)
}

func updateTokenLocalCache(key string, update func(*TokenCache) bool) {
	cacheUpdateModelLocal(getTokenCacheKey(key), cloneTokenCache, update)
}

type TokenCache struct {
	Group              string           `json:"group"                redis:"g"`
	Key                string           `json:"-"                    redis:"-"`
	Name               string           `json:"name"                 redis:"n"`
	Scope              ChannelScope     `json:"scope"                redis:"sc"`
	Subnets            redisStringSlice `json:"subnets"              redis:"s"`
	Models             redisStringSlice `json:"models"               redis:"m"`
	Sets               redisStringSlice `json:"sets"                 redis:"sets"`
	GroupChannelModels redisStringSlice `json:"group_channel_models" redis:"gcm"`
	GroupChannelSets   redisStringSlice `json:"group_channel_sets"   redis:"gcs"`
	ID                 int              `json:"id"                   redis:"i"`
	Status             int              `json:"status"               redis:"st"`
	UsedAmount         float64          `json:"used_amount"          redis:"u"`

	Quota                  float64   `json:"quota"                     redis:"q"`
	PeriodQuota            float64   `json:"period_quota"              redis:"pq"`
	PeriodType             string    `json:"period_type"               redis:"pt"`
	PeriodLastUpdateTime   redisTime `json:"period_last_update_time"   redis:"plut"`
	PeriodLastUpdateAmount float64   `json:"period_last_update_amount" redis:"plua"`
}

func (t *TokenCache) GetConfiguredSets() []string {
	return cleanAvailableSets(t.Sets)
}

func (t *TokenCache) GetConfiguredGroupChannelSets() []string {
	return cleanAvailableSets(t.GroupChannelSets)
}

func (t *Token) ToTokenCache() *TokenCache {
	return &TokenCache{
		ID:                 t.ID,
		Group:              t.GroupID,
		Key:                t.Key,
		Name:               string(t.Name),
		Scope:              t.Scope,
		Models:             t.Models,
		Sets:               t.Sets,
		GroupChannelModels: t.GroupChannelModels,
		GroupChannelSets:   t.GroupChannelSets,
		Subnets:            t.Subnets,
		Status:             t.Status,
		UsedAmount:         t.UsedAmount,

		Quota:                  t.Quota,
		PeriodQuota:            t.PeriodQuota,
		PeriodType:             string(t.PeriodType),
		PeriodLastUpdateTime:   redisTime(t.PeriodLastUpdateTime),
		PeriodLastUpdateAmount: t.PeriodLastUpdateAmount,
	}
}

func CacheDeleteToken(key string) error {
	cacheDeleteModelLocal(getTokenCacheKey(key))

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, getTokenCacheKey(key)).Err()
}

func CacheSetToken(token *TokenCache) error {
	key := getTokenCacheKey(token.Key)
	cacheSetModelLocal(key, token, cloneTokenCache)
	return cacheSetTokenRedis(token)
}

func CacheGetTokenByKey(key string) (*TokenCache, error) {
	cacheKey := getTokenCacheKey(key)
	if tokenCache, notFound, ok := cacheGetModelLocal(cacheKey, cloneTokenCache); ok {
		if notFound {
			return nil, NotFoundError(ErrTokenNotFound)
		}

		return tokenCache, nil
	}

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		tokenCache := &TokenCache{}

		err := common.RDB.HGetAll(ctx, cacheKey).Scan(tokenCache)
		if err == nil && tokenCache.ID != 0 {
			tokenCache.Key = key
			cacheSetModelLocal(cacheKey, tokenCache, cloneTokenCache)
			return tokenCache, nil
		} else if err != nil && !errors.Is(err, redis.Nil) {
			log.Errorf("get token (%s) from redis error: %s", key, err.Error())
		}
	}

	tokenCache, notFound, loaded, err := loadWithLocalKeyLock(
		modelCacheLoadLocker,
		cacheKey,
		func() (*TokenCache, bool, bool) {
			return cacheGetModelLocal(cacheKey, cloneTokenCache)
		},
		func() (*TokenCache, error) {
			token, err := GetTokenByKey(key)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					cacheSetModelNotFoundLocalUnlocked(cacheKey)
				}

				return nil, err
			}

			tc := token.ToTokenCache()
			cacheSetModelLocalUnlocked(cacheKey, tc, cloneTokenCache)

			return tc, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if notFound {
		return nil, NotFoundError(ErrTokenNotFound)
	}

	if loaded {
		if err := cacheSetTokenRedis(tokenCache); err != nil {
			log.Error("redis set token error: " + err.Error())
		}
	}

	return tokenCache, nil
}

func cacheSetTokenRedis(token *TokenCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := getTokenCacheKey(token.Key)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, token)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

var updateTokenUsedAmountOnlyIncreaseScript = redis.NewScript(`
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

func CacheUpdateTokenUsedAmountOnlyIncrease(key string, amount float64) error {
	updateTokenLocalCache(key, func(token *TokenCache) bool {
		if amount < token.UsedAmount {
			return false
		}

		token.UsedAmount = amount

		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateTokenUsedAmountOnlyIncreaseScript.Run(
		ctx,
		common.RDB,
		[]string{getTokenCacheKey(key)},
		amount,
	).Err()
}

func CacheResetTokenPeriodUsage(
	key string,
	periodLastUpdateTime time.Time,
	periodLastUpdateAmount float64,
) error {
	updateTokenLocalCache(key, func(token *TokenCache) bool {
		token.PeriodLastUpdateTime = redisTime(periodLastUpdateTime)
		token.PeriodLastUpdateAmount = periodLastUpdateAmount
		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cacheKey := getTokenCacheKey(key)
	pipe := common.RDB.Pipeline()
	periodLastUpdateTimeBytes, _ := periodLastUpdateTime.MarshalBinary()
	pipe.HSet(ctx, cacheKey, "plut", periodLastUpdateTimeBytes)
	pipe.HSet(ctx, cacheKey, "plua", periodLastUpdateAmount)
	_, err := pipe.Exec(ctx)

	return err
}

var updateTokenNameScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "n") then
		redis.call("HSet", KEYS[1], "n", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdateTokenName(key, name string) error {
	updateTokenLocalCache(key, func(token *TokenCache) bool {
		token.Name = name
		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateTokenNameScript.Run(
		ctx,
		common.RDB,
		[]string{getTokenCacheKey(key)},
		name,
	).Err()
}

var updateTokenStatusScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "st") then
		redis.call("HSet", KEYS[1], "st", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdateTokenStatus(key string, status int) error {
	updateTokenLocalCache(key, func(token *TokenCache) bool {
		token.Status = status
		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateTokenStatusScript.Run(
		ctx,
		common.RDB,
		[]string{getTokenCacheKey(key)},
		status,
	).Err()
}
