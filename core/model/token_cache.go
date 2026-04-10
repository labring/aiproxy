package model

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
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
	Group      string           `json:"group"       redis:"g"`
	Key        string           `json:"-"           redis:"-"`
	Name       string           `json:"name"        redis:"n"`
	Subnets    redisStringSlice `json:"subnets"     redis:"s"`
	Models     redisStringSlice `json:"models"      redis:"m"`
	ID         int              `json:"id"          redis:"i"`
	Status     int              `json:"status"      redis:"st"`
	UsedAmount float64          `json:"used_amount" redis:"u"`

	Quota                  float64   `json:"quota"                     redis:"q"`
	PeriodQuota            float64   `json:"period_quota"              redis:"pq"`
	PeriodType             string    `json:"period_type"               redis:"pt"`
	PeriodLastUpdateTime   redisTime `json:"period_last_update_time"   redis:"plut"`
	PeriodLastUpdateAmount float64   `json:"period_last_update_amount" redis:"plua"`

	availableSets []string
	modelsBySet   map[string][]string
}

func (t *TokenCache) SetAvailableSets(availableSets []string) {
	t.availableSets = availableSets
}

func (t *TokenCache) SetModelsBySet(modelsBySet map[string][]string) {
	t.modelsBySet = modelsBySet
}

func (t *TokenCache) FindModel(model string) string {
	var findModel string
	if len(t.Models) != 0 {
		if !slices.ContainsFunc(t.Models, func(e string) bool {
			ok := strings.EqualFold(e, model)
			if ok {
				findModel = e
			}

			return ok
		}) {
			return findModel
		}
	}

	return containsModel(model, t.availableSets, t.modelsBySet)
}

func containsModel(model string, sets []string, modelsBySet map[string][]string) string {
	var findModel string
	for _, set := range sets {
		if slices.ContainsFunc(modelsBySet[set], func(e string) bool {
			ok := strings.EqualFold(e, model)
			if ok {
				findModel = e
			}

			return ok
		}) {
			return findModel
		}
	}

	return findModel
}

func (t *TokenCache) Range(fn func(model string) bool) {
	ranged := make(map[string]struct{})
	if len(t.Models) != 0 {
		for _, model := range t.Models {
			if _, ok := ranged[model]; ok {
				continue
			}

			model = containsModel(model, t.availableSets, t.modelsBySet)
			if model == "" {
				continue
			}

			ranged[model] = struct{}{}
			if !fn(model) {
				return
			}
		}

		return
	}

	for _, set := range t.availableSets {
		for _, model := range t.modelsBySet[set] {
			if _, ok := ranged[model]; !ok {
				if !fn(model) {
					return
				}
			}

			ranged[model] = struct{}{}
		}
	}
}

func (t *Token) ToTokenCache() *TokenCache {
	return &TokenCache{
		ID:         t.ID,
		Group:      t.GroupID,
		Key:        t.Key,
		Name:       string(t.Name),
		Models:     t.Models,
		Subnets:    t.Subnets,
		Status:     t.Status,
		UsedAmount: t.UsedAmount,

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
