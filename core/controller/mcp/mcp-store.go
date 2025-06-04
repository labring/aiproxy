package controller

import (
	"context"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/mcpproxy"
	"github.com/redis/go-redis/v9"
)

// Global variables for session management
var (
	memStore       mcpproxy.SessionManager = mcpproxy.NewMemStore()
	redisStore     mcpproxy.SessionManager
	redisStoreOnce = &sync.Once{}
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
	rdb *redis.Client
}

func newRedisStoreManager(rdb *redis.Client) mcpproxy.SessionManager {
	return &redisStoreManager{
		rdb: rdb,
	}
}

var redisStoreManagerScript = redis.NewScript(`
local key = KEYS[1]
local value = redis.call('GET', key)
if not value then
	return nil
end
redis.call('EXPIRE', key, 300)
return value
`)

func (r *redisStoreManager) New() string {
	return common.ShortUUID()
}

func (r *redisStoreManager) Get(sessionID string) (string, bool) {
	ctx := context.Background()

	result, err := redisStoreManagerScript.Run(ctx, r.rdb, []string{"mcp:session:" + sessionID}).
		Result()
	if err != nil || result == nil {
		return "", false
	}

	res, ok := result.(string)
	return res, ok
}

func (r *redisStoreManager) Set(sessionID, endpoint string) {
	ctx := context.Background()
	r.rdb.Set(ctx, "mcp:session:"+sessionID, endpoint, time.Minute*5)
}

func (r *redisStoreManager) Delete(session string) {
	ctx := context.Background()
	r.rdb.Del(ctx, "mcp:session:"+session)
}
