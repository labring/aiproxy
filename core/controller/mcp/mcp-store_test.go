//nolint:testpackage
package controller

import (
	"context"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisStoreManagerLocalHitRefreshesRedisTTL(t *testing.T) {
	ctx := context.Background()

	client, cleanup := setupRedisForMCPStoreTest(t, ctx)
	defer cleanup()

	oldSessionTTL := redisStoreSessionTTL
	oldLocalTTL := redisStoreLocalTTL
	redisStoreSessionTTL = 2 * time.Second
	redisStoreLocalTTL = 2 * time.Second
	t.Cleanup(func() {
		redisStoreSessionTTL = oldSessionTTL
		redisStoreLocalTTL = oldLocalTTL
	})

	storeManager := newRedisStoreManager(client)
	store, ok := storeManager.(*redisStoreManager)
	require.True(t, ok)
	store.Set("session-refresh", "endpoint-1")

	time.Sleep(1500 * time.Millisecond)

	beforeTTL, err := client.PTTL(ctx, common.RedisKey("mcp:session", "session-refresh")).Result()
	require.NoError(t, err)
	require.Less(t, beforeTTL, time.Second)

	endpoint, ok := store.Get("session-refresh")
	require.True(t, ok)
	require.Equal(t, "endpoint-1", endpoint)

	afterTTL, err := client.PTTL(ctx, common.RedisKey("mcp:session", "session-refresh")).Result()
	require.NoError(t, err)
	require.Greater(t, afterTTL, time.Second)
}

func TestRedisStoreManagerLocalHitDoesNotReturnStaleSession(t *testing.T) {
	ctx := context.Background()

	client, cleanup := setupRedisForMCPStoreTest(t, ctx)
	defer cleanup()

	oldSessionTTL := redisStoreSessionTTL
	oldLocalTTL := redisStoreLocalTTL
	redisStoreSessionTTL = 2 * time.Second
	redisStoreLocalTTL = 2 * time.Second
	t.Cleanup(func() {
		redisStoreSessionTTL = oldSessionTTL
		redisStoreLocalTTL = oldLocalTTL
	})

	storeManager := newRedisStoreManager(client)
	store, ok := storeManager.(*redisStoreManager)
	require.True(t, ok)
	store.Set("session-stale", "endpoint-2")

	require.NoError(t, client.Del(ctx, common.RedisKey("mcp:session", "session-stale")).Err())

	endpoint, ok := store.Get("session-stale")
	require.False(t, ok)
	require.Empty(t, endpoint)
}

func setupRedisForMCPStoreTest(t *testing.T, ctx context.Context) (*redis.Client, func()) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
		DB:   0,
	})
	require.NoError(t, client.Ping(ctx).Err())

	cleanup := func() {
		_ = client.Close()
		_ = container.Terminate(ctx)
	}

	return client, cleanup
}
