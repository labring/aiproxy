package controller

import (
	"context"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/mcpproxy"
	gcache "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
)

// Global variables for session management
var (
	memStore       mcpproxy.SessionManager = mcpproxy.NewMemStore()
	redisStore     mcpproxy.SessionManager
	redisStoreOnce = &sync.Once{}

	redisStoreSessionTTL = 5 * time.Minute
	redisStoreLocalTTL   = 5 * time.Minute
)

func getStore() mcpproxy.SessionManager {
	if common.RedisEnabled {
		redisStoreOnce.Do(func() {
			redisStore = newRedisStoreManager(common.RDB)
		})
		return redisStore
	}

	return memStore
}

// Redis-based session manager
type redisStoreManager struct {
	rdb   *redis.Client
	local *gcache.Cache
}

func newRedisStoreManager(rdb *redis.Client) mcpproxy.SessionManager {
	return &redisStoreManager{
		rdb:   rdb,
		local: gcache.New(redisStoreLocalTTL, 2*redisStoreLocalTTL),
	}
}

var redisStoreManagerScript = redis.NewScript(`
local key = KEYS[1]
local value = redis.call('GET', key)
if not value then
	return nil
end
redis.call('EXPIRE', key, ARGV[1])
return value
`)

func (r *redisStoreManager) New() string {
	return common.ShortUUID()
}

func (r *redisStoreManager) Get(sessionID string) (string, bool) {
	if endpoint, ok := r.local.Get(sessionID); ok {
		value, ok := endpoint.(string)
		if !ok {
			r.local.Delete(sessionID)
			return "", false
		}

		ctx := context.Background()

		touched, err := r.rdb.Expire(ctx, common.RedisKey("mcp:session", sessionID), redisStoreSessionTTL).
			Result()
		if err != nil || !touched {
			r.local.Delete(sessionID)
			return "", false
		}

		r.local.Set(sessionID, value, redisStoreLocalTTL)

		return value, true
	}

	ctx := context.Background()

	result, err := redisStoreManagerScript.Run(
		ctx,
		r.rdb,
		[]string{common.RedisKey("mcp:session", sessionID)},
		int(redisStoreSessionTTL/time.Second),
	).Result()
	if err != nil || result == nil {
		return "", false
	}

	res, ok := result.(string)
	if ok {
		r.local.Set(sessionID, res, redisStoreLocalTTL)
	}

	return res, ok
}

func (r *redisStoreManager) Set(sessionID, endpoint string) {
	ctx := context.Background()

	r.local.Set(sessionID, endpoint, redisStoreLocalTTL)
	r.rdb.Set(ctx, common.RedisKey("mcp:session", sessionID), endpoint, redisStoreSessionTTL)
}

func (r *redisStoreManager) Delete(session string) {
	ctx := context.Background()

	r.local.Delete(session)
	r.rdb.Del(ctx, common.RedisKey("mcp:session", session))
}
