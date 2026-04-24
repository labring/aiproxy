//nolint:testpackage
package model

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCacheSetAndGetPublicMCPViaRedis(t *testing.T) {
	withTestMCPRedisEnv(t, func(ctx context.Context, client *redis.Client) {
		cache := &PublicMCPCache{
			ID:     "public-mcp-redis",
			Status: PublicMCPStatusEnabled,
			Type:   PublicMCPTypeOpenAPI,
			Price: MCPPrice{
				DefaultToolsCallPrice: 1.23,
				ToolsCallPrices: map[string]float64{
					"tool-a": 2.34,
				},
			},
			OpenAPIConfig: &MCPOpenAPIConfig{
				OpenAPISpec: "https://example.com/openapi.json",
			},
			EmbedConfig: &MCPEmbeddingConfig{
				Init: map[string]string{
					"foo": "bar",
				},
			},
		}

		require.NoError(t, CacheSetPublicMCP(cache))

		modelLocalCache.Flush()

		got, err := CacheGetPublicMCP(cache.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, cache.ID, got.ID)
		assert.Equal(t, PublicMCPStatusEnabled, got.Status)
		assert.Equal(t, PublicMCPTypeOpenAPI, got.Type)
		assert.Equal(t, cache.Price.DefaultToolsCallPrice, got.Price.DefaultToolsCallPrice)
		assert.Equal(t, cache.Price.ToolsCallPrices, got.Price.ToolsCallPrices)
		require.NotNil(t, got.OpenAPIConfig)
		assert.Equal(t, cache.OpenAPIConfig.OpenAPISpec, got.OpenAPIConfig.OpenAPISpec)
		require.NotNil(t, got.EmbedConfig)
		assert.Equal(t, cache.EmbedConfig.Init, got.EmbedConfig.Init)

		exists, err := client.Exists(ctx, getPublicMCPCacheKey(cache.ID)).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, exists)
	})
}

func withTestMCPRedisEnv(t *testing.T, fn func(context.Context, *redis.Client)) {
	t.Helper()

	ctx := context.Background()

	oldDB := DB
	oldLogDB := LogDB
	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "mcp_cache_redis_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PublicMCP{}, &PublicMCPReusingParam{}, &GroupMCP{}))

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
