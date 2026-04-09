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
)

const (
	TokenCacheKey = "token:%s"
)

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
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, common.RedisKeyf(TokenCacheKey, key)).Err()
}

func CacheSetToken(token *TokenCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := common.RedisKeyf(TokenCacheKey, token.Key)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, token)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

func CacheGetTokenByKey(key string) (*TokenCache, error) {
	if !common.RedisEnabled {
		token, err := GetTokenByKey(key)
		if err != nil {
			return nil, err
		}

		return token.ToTokenCache(), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cacheKey := common.RedisKeyf(TokenCacheKey, key)
	tokenCache := &TokenCache{}

	err := common.RDB.HGetAll(ctx, cacheKey).Scan(tokenCache)
	if err == nil && tokenCache.ID != 0 {
		tokenCache.Key = key
		return tokenCache, nil
	} else if err != nil && !errors.Is(err, redis.Nil) {
		log.Errorf("get token (%s) from redis error: %s", key, err.Error())
	}

	token, err := GetTokenByKey(key)
	if err != nil {
		return nil, err
	}

	tc := token.ToTokenCache()

	if err := CacheSetToken(tc); err != nil {
		log.Error("redis set token error: " + err.Error())
	}

	return tc, nil
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
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateTokenUsedAmountOnlyIncreaseScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(TokenCacheKey, key)},
		amount,
	).Err()
}

func CacheResetTokenPeriodUsage(
	key string,
	periodLastUpdateTime time.Time,
	periodLastUpdateAmount float64,
) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cacheKey := common.RedisKeyf(TokenCacheKey, key)
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
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateTokenNameScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(TokenCacheKey, key)},
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
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateTokenStatusScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(TokenCacheKey, key)},
		status,
	).Err()
}
