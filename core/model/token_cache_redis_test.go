//nolint:testpackage
package model

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestTokenCacheRedisRoundTrip guards the redis serialization of TokenCache:
// before ChannelScope implemented encoding.BinaryMarshaler, every
// CacheSetToken/CacheGetTokenByKey redis write failed with
// "redis: can't marshal model.ChannelScope (implement encoding.BinaryMarshaler)".
func TestTokenCacheRedisRoundTrip(t *testing.T) {
	withTestTokenCacheRedisEnv(t, func(ctx context.Context, client *redis.Client) {
		token := &TokenCache{
			ID:                 1,
			Group:              "round-trip-group",
			Key:                "sk-token-cache-round-trip",
			Name:               "round-trip",
			Scope:              ChannelScopeGroup,
			Subnets:            redisStringSlice{"127.0.0.1/32"},
			Models:             redisStringSlice{"gpt-4"},
			Sets:               redisStringSlice{"set-a"},
			GroupChannelModels: redisStringSlice{"gpt-4o"},
			GroupChannelSets:   redisStringSlice{"gcs-a"},
			Status:             1,
			UsedAmount:         1.25,
			Quota:              100.5,
			PeriodQuota:        50,
			PeriodType:         "day",
			PeriodLastUpdateTime: redisTime(
				time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
			),
			PeriodLastUpdateAmount: 10,
		}

		require.NoError(t, CacheSetToken(token))

		modelLocalCache.Flush()

		got, err := CacheGetTokenByKey(token.Key)
		require.NoError(t, err)
		require.Equal(t, token.Scope, got.Scope)
		require.Equal(t, token.ID, got.ID)
		require.Equal(t, token.Group, got.Group)
		require.Equal(t, token.Name, got.Name)
		require.Equal(t, token.Subnets, got.Subnets)
		require.Equal(t, token.Models, got.Models)
		require.Equal(t, token.Sets, got.Sets)
		require.Equal(t, token.GroupChannelModels, got.GroupChannelModels)
		require.Equal(t, token.GroupChannelSets, got.GroupChannelSets)
		require.Equal(t, token.Status, got.Status)
		require.Equal(t, token.UsedAmount, got.UsedAmount)
		require.Equal(t, token.Quota, got.Quota)
		require.Equal(t, token.PeriodQuota, got.PeriodQuota)
		require.Equal(t, token.PeriodType, got.PeriodType)
		require.Equal(t, token.PeriodLastUpdateTime, got.PeriodLastUpdateTime)
		require.Equal(t, token.PeriodLastUpdateAmount, got.PeriodLastUpdateAmount)
	})
}

func withTestTokenCacheRedisEnv(t *testing.T, fn func(context.Context, *redis.Client)) {
	t.Helper()

	ctx := context.Background()

	oldDB := DB
	oldLogDB := LogDB
	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "token_cache_redis_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Token{}))

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
		Addr: net.JoinHostPort(host, port.Port()),
		DB:   0,
	})
	require.NoError(t, client.Ping(ctx).Err())

	DB = db
	LogDB = db
	common.RDB = client
	common.RedisEnabled = true
	common.UsingSQLite = true

	modelLocalCache.Flush()

	t.Cleanup(func() {
		DB = oldDB
		LogDB = oldLogDB
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite

		modelLocalCache.Flush()

		_ = client.Close()
		_ = container.Terminate(ctx)

		sqlDB, sqlErr := db.DB()
		require.NoError(t, sqlErr)
		require.NoError(t, sqlDB.Close())
	})

	fn(ctx, client)
}
