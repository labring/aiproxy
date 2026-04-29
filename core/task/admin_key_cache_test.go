package task

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestInitAdminKeyCacheLoadsCachedKeyOverLocalKey(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupRedisForAdminKeyCacheTest(t, ctx)
	defer cleanup()

	configureAdminKeyCacheTest(t, client)
	config.SetAdminKey("local-key")

	require.NoError(t, client.Set(ctx, getAdminKeyCacheKey(), "redis-key", 0).Err())
	require.NoError(t, InitAdminKeyCache(ctx))
	require.Equal(t, "redis-key", config.GetAdminKey())
}

func TestInitAdminKeyCacheBootstrapsLocalKey(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupRedisForAdminKeyCacheTest(t, ctx)
	defer cleanup()

	configureAdminKeyCacheTest(t, client)
	config.SetAdminKey("local-key")

	require.NoError(t, InitAdminKeyCache(ctx))

	cachedKey, err := client.Get(ctx, getAdminKeyCacheKey()).Result()
	require.NoError(t, err)
	require.Equal(t, "local-key", cachedKey)
}

func TestInitAdminKeyCacheNoopsWithoutScope(t *testing.T) {
	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled
	oldRedisKeyPrefix := config.RedisKeyPrefix
	oldAdminKey := config.GetAdminKey()
	oldInternalToken := config.GetInternalToken()
	oldSealosJWTKey, hadSealosJWTKey := os.LookupEnv("SEALOS_JWT_KEY")

	common.RDB = nil
	common.RedisEnabled = true
	config.RedisKeyPrefix = ""
	config.SetAdminKey("local-key")
	config.SetInternalToken("")
	require.NoError(t, os.Unsetenv("SEALOS_JWT_KEY"))

	t.Cleanup(func() {
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
		config.RedisKeyPrefix = oldRedisKeyPrefix
		config.SetAdminKey(oldAdminKey)
		config.SetInternalToken(oldInternalToken)
		if hadSealosJWTKey {
			require.NoError(t, os.Setenv("SEALOS_JWT_KEY", oldSealosJWTKey))
		} else {
			require.NoError(t, os.Unsetenv("SEALOS_JWT_KEY"))
		}
	})

	require.False(t, AdminKeyCacheEnabled())
	require.NoError(t, InitAdminKeyCache(context.Background()))
	require.Equal(t, "local-key", config.GetAdminKey())
}

func TestInitAdminKeyCacheReturnsRedisError(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})
	defer client.Close()

	configureAdminKeyCacheTest(t, client)
	config.SetAdminKey("local-key")

	err := InitAdminKeyCache(context.Background())
	require.Error(t, err)
}

func TestAdminKeyCacheTaskUpdatesLocalKey(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupRedisForAdminKeyCacheTest(t, ctx)
	defer cleanup()

	configureAdminKeyCacheTest(t, client)
	config.SetAdminKey("initial-key")

	require.NoError(t, InitAdminKeyCache(ctx))

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go AdminKeyCacheTask(taskCtx)

	require.NoError(t, client.Set(ctx, getAdminKeyCacheKey(), "rotated-key", 0).Err())
	require.Eventually(t, func() bool {
		return config.GetAdminKey() == "rotated-key"
	}, 3*time.Second, 50*time.Millisecond)
}

func configureAdminKeyCacheTest(t *testing.T, client *redis.Client) {
	t.Helper()

	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled
	oldRedisKeyPrefix := config.RedisKeyPrefix
	oldAdminKey := config.GetAdminKey()
	oldInternalToken := config.GetInternalToken()
	oldSealosJWTKey, hadSealosJWTKey := os.LookupEnv("SEALOS_JWT_KEY")

	common.RDB = client
	common.RedisEnabled = client != nil
	config.RedisKeyPrefix = ""
	config.SetAdminKey("")
	config.SetInternalToken("admin-key-cache-test-scope")
	require.NoError(t, os.Unsetenv("SEALOS_JWT_KEY"))

	t.Cleanup(func() {
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
		config.RedisKeyPrefix = oldRedisKeyPrefix
		config.SetAdminKey(oldAdminKey)
		config.SetInternalToken(oldInternalToken)
		if hadSealosJWTKey {
			require.NoError(t, os.Setenv("SEALOS_JWT_KEY", oldSealosJWTKey))
		} else {
			require.NoError(t, os.Unsetenv("SEALOS_JWT_KEY"))
		}
	})
}

func setupRedisForAdminKeyCacheTest(t *testing.T, ctx context.Context) (*redis.Client, func()) {
	t.Helper()

	server, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: server.Addr(),
		DB:   0,
	})
	require.NoError(t, client.Ping(ctx).Err())

	cleanup := func() {
		_ = client.Close()
		server.Close()
	}

	return client, cleanup
}
