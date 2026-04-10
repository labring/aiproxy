//nolint:testpackage
package model

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCacheGetStoreWritesRedisNotFoundCache(t *testing.T) {
	withTestStoreCacheRedisEnv(t, func(ctx context.Context, client *redis.Client) {
		storeID := ResponseStoreID("resp_redis_not_found")

		_, err := CacheGetStore("group-redis-miss", 1, storeID)
		require.Error(t, err)

		exists, err := client.Exists(
			ctx,
			getStoreCacheNotFoundKey("group-redis-miss", 1, storeID),
		).Result()
		require.NoError(t, err)
		require.EqualValues(t, 1, exists)
	})
}

func withTestStoreCacheRedisEnv(t *testing.T, fn func(context.Context, *redis.Client)) {
	t.Helper()

	ctx := context.Background()

	oldDB := DB
	oldLogDB := LogDB
	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "store_cache_redis_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&StoreV2{}))

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

	DB = db
	LogDB = db
	common.RDB = client
	common.RedisEnabled = true
	common.UsingSQLite = true

	storeLocalCache.Flush()

	t.Cleanup(func() {
		DB = oldDB
		LogDB = oldLogDB
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite

		storeLocalCache.Flush()

		_ = client.Close()
		_ = container.Terminate(ctx)

		sqlDB, sqlErr := db.DB()
		require.NoError(t, sqlErr)
		require.NoError(t, sqlDB.Close())
	})

	fn(ctx, client)
}
