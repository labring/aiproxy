package model

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/labring/aiproxy/core/common"
	gcache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	storeLocalTTL     = time.Second
	storeLocalMissTTL = 500 * time.Millisecond
	storeRedisMissTTL = 15 * time.Second

	StoreCacheKey         = "storev2:%s:%d:%s"
	StoreCacheNotFoundKey = "storev2notfound:%s:%d:%s"
)

var (
	storeLocalCache      = gcache.New(2*time.Second, 5*time.Second)
	storeCacheLoadLocker = common.NewKeyedLocker()
)

type StoreCache struct {
	ID        string    `json:"id"         redis:"i"`
	GroupID   string    `json:"group_id"   redis:"g"`
	TokenID   int       `json:"token_id"   redis:"t"`
	ChannelID int       `json:"channel_id" redis:"c"`
	Model     string    `json:"model"      redis:"m"`
	CreatedAt time.Time `json:"created_at" redis:"a"`
	UpdatedAt time.Time `json:"updated_at" redis:"u"`
	ExpiresAt time.Time `json:"expires_at" redis:"e"`
}

type localStoreCacheItem struct {
	Store    *StoreCache
	NotFound bool
}

func (s *StoreV2) ToStoreCache() *StoreCache {
	return &StoreCache{
		ID:        s.ID,
		GroupID:   s.GroupID,
		TokenID:   s.TokenID,
		ChannelID: s.ChannelID,
		Model:     s.Model,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		ExpiresAt: s.ExpiresAt,
	}
}

func cloneStoreCache(store *StoreCache) *StoreCache {
	if store == nil {
		return nil
	}

	cloned := *store

	return &cloned
}

func (s *StoreCache) ToStoreV2() *StoreV2 {
	if s == nil {
		return nil
	}

	return &StoreV2{
		ID:        s.ID,
		GroupID:   s.GroupID,
		TokenID:   s.TokenID,
		ChannelID: s.ChannelID,
		Model:     s.Model,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		ExpiresAt: s.ExpiresAt,
	}
}

func getStoreCacheKey(group string, tokenID int, id string) string {
	return common.RedisKeyf(StoreCacheKey, group, tokenID, id)
}

func getStoreCacheNotFoundKey(group string, tokenID int, id string) string {
	return common.RedisKeyf(StoreCacheNotFoundKey, group, tokenID, id)
}

func getStoreLocalTTL(store *StoreCache) time.Duration {
	ttl := storeLocalTTL
	if store == nil || store.ExpiresAt.IsZero() {
		return jitterStoreLocalTTL(ttl)
	}

	storeTTL := time.Until(store.ExpiresAt)
	if storeTTL <= 0 {
		return 0
	}

	if storeTTL < ttl {
		return jitterStoreLocalTTL(storeTTL)
	}

	return jitterStoreLocalTTL(ttl)
}

func jitterStoreLocalTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}

	jitter := ttl / 10
	if jitter <= 0 {
		return ttl
	}

	return ttl + time.Duration(rand.Int64N(int64(jitter)*2+1)) - jitter
}

func cacheSetStoreLocal(key string, store *StoreCache) {
	common.WithKeyLock(storeCacheLoadLocker, key, func() {
		cacheSetStoreLocalUnlocked(key, store)
	})
}

func cacheSetStoreLocalUnlocked(key string, store *StoreCache) {
	ttl := getStoreLocalTTL(store)
	if ttl <= 0 {
		storeLocalCache.Delete(key)
		return
	}

	storeLocalCache.Set(key, localStoreCacheItem{Store: cloneStoreCache(store)}, ttl)
}

func cacheSetStoreNotFoundLocal(key string) {
	common.WithKeyLock(storeCacheLoadLocker, key, func() {
		cacheSetStoreNotFoundLocalUnlocked(key)
	})
}

func cacheSetStoreNotFoundLocalUnlocked(key string) {
	storeLocalCache.Set(key, localStoreCacheItem{NotFound: true}, storeLocalMissTTL)
}

func cacheGetStoreLocal(key string) (*StoreCache, bool, bool) {
	v, ok := storeLocalCache.Get(key)
	if !ok {
		return nil, false, false
	}

	item, ok := v.(localStoreCacheItem)
	if !ok {
		panic("store local cache type mismatch")
	}

	if item.NotFound {
		return nil, true, true
	}

	return cloneStoreCache(item.Store), false, true
}

func cachePeekStore(group string, tokenID int, id string) (*StoreCache, bool) {
	cacheKey := getStoreCacheKey(group, tokenID, id)

	if storeCache, notFound, ok := cacheGetStoreLocal(cacheKey); ok {
		if notFound {
			return nil, false
		}

		if !storeCache.ExpiresAt.IsZero() && !storeCache.ExpiresAt.After(time.Now()) {
			storeLocalCache.Delete(cacheKey)
			return nil, false
		}

		return storeCache, true
	}

	if !common.RedisEnabled {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	storeCache := &StoreCache{}
	if err := common.RDB.HGetAll(ctx, cacheKey).Scan(storeCache); err != nil {
		log.Error("redis peek store error: " + err.Error())
		return nil, false
	}

	if storeCache.ID == "" {
		return nil, false
	}

	if !storeCache.ExpiresAt.IsZero() && !storeCache.ExpiresAt.After(time.Now()) {
		_ = common.RDB.Del(ctx, cacheKey).Err()
		return nil, false
	}

	cacheSetStoreLocal(cacheKey, storeCache)

	return storeCache, true
}

func cacheSetStoreNotFound(ctx context.Context, key string) error {
	return common.RDB.Set(ctx, key, "1", storeRedisMissTTL).Err()
}

func CacheSetStore(store *StoreCache) error {
	key := getStoreCacheKey(store.GroupID, store.TokenID, store.ID)
	cacheSetStoreLocal(key, store)

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return cacheSetStore(
		ctx,
		key,
		getStoreCacheNotFoundKey(store.GroupID, store.TokenID, store.ID),
		store,
	)
}

func cacheSetStore(ctx context.Context, key, notFoundKey string, store *StoreCache) error {
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, store)
	pipe.Del(ctx, notFoundKey)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	if !store.ExpiresAt.IsZero() {
		storeTTL := time.Until(store.ExpiresAt)
		if storeTTL <= 0 {
			expireTime = time.Second
		} else if storeTTL < expireTime {
			expireTime = storeTTL
		}
	}

	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

func CacheGetStore(group string, tokenID int, id string) (*StoreCache, error) {
	cacheKey := getStoreCacheKey(group, tokenID, id)
	if storeCache, notFound, ok := cacheGetStoreLocal(cacheKey); ok {
		if notFound {
			return nil, NotFoundError(ErrStoreNotFound)
		}

		return storeCache, nil
	}

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		storeCache := &StoreCache{}

		err := common.RDB.HGetAll(ctx, cacheKey).Scan(storeCache)
		if err == nil && storeCache.ID != "" {
			if !storeCache.ExpiresAt.IsZero() && !storeCache.ExpiresAt.After(time.Now()) {
				_ = common.RDB.Del(ctx, cacheKey).Err()
			} else {
				cacheSetStoreLocal(cacheKey, storeCache)
				return storeCache, nil
			}
		}

		notFoundKey := getStoreCacheNotFoundKey(group, tokenID, id)

		exists, err := common.RDB.Exists(ctx, notFoundKey).Result()
		if err == nil && exists > 0 {
			cacheSetStoreNotFoundLocal(cacheKey)
			return nil, NotFoundError(ErrStoreNotFound)
		}
	}

	storeCache, notFound, loaded, err := loadWithLocalKeyLock(
		storeCacheLoadLocker,
		cacheKey,
		func() (*StoreCache, bool, bool) {
			return cacheGetStoreLocal(cacheKey)
		},
		func() (*StoreCache, error) {
			store, err := GetStore(group, tokenID, id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					cacheSetStoreNotFoundLocalUnlocked(cacheKey)
				}

				return nil, err
			}

			sc := store.ToStoreCache()
			cacheSetStoreLocalUnlocked(cacheKey, sc)

			return sc, nil
		},
	)
	if err != nil {
		if loaded && errors.Is(err, gorm.ErrRecordNotFound) && common.RedisEnabled {
			ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
			defer cancel()

			if cacheErr := cacheSetStoreNotFound(
				ctx,
				getStoreCacheNotFoundKey(group, tokenID, id),
			); cacheErr != nil {
				log.Error("redis set store not found cache error: " + cacheErr.Error())
			}
		}

		return nil, err
	}

	if notFound {
		return nil, NotFoundError(ErrStoreNotFound)
	}

	if loaded && common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		if err := cacheSetStore(
			ctx,
			cacheKey,
			getStoreCacheNotFoundKey(group, tokenID, id),
			storeCache,
		); err != nil {
			log.Error("redis set store error: " + err.Error())
		}
	}

	return storeCache, nil
}
