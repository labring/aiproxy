//nolint:testpackage
package model

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSaveStoreWithOptionPostgresSkipsUpdateWithinMinInterval(t *testing.T) {
	withTestPostgresStoreDB(t, func() {
		storeID := CacheFollowStoreID("gpt-5", CacheKeyTypeRecent)
		now := time.Now()

		initial, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			CreatedAt: now.Add(-2 * time.Second),
			UpdatedAt: now.Add(-2 * time.Second),
			ExpiresAt: now.Add(time.Minute),
		})
		require.NoError(t, err)

		saved, err := SaveStoreWithOption(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			CreatedAt: now,
			UpdatedAt: now,
			ExpiresAt: now.Add(2 * time.Minute),
		}, SaveStoreOption{
			MinUpdateInterval: 15 * time.Second,
		})
		require.NoError(t, err)
		assert.Equal(t, initial.ChannelID, saved.ChannelID)
		assert.WithinDuration(t, initial.CreatedAt, saved.CreatedAt, time.Second)
		assert.WithinDuration(t, initial.UpdatedAt, saved.UpdatedAt, time.Second)
	})
}

func TestSaveStoreWithOptionPostgresUpdatesAfterMinInterval(t *testing.T) {
	withTestPostgresStoreDB(t, func() {
		storeID := CacheFollowStoreID("gpt-5", CacheKeyTypeRecent)
		createdAt := time.Now().Add(-time.Minute)
		initialUpdatedAt := time.Now().Add(-30 * time.Second)

		initial, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			CreatedAt: createdAt,
			UpdatedAt: initialUpdatedAt,
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)

		nextUpdatedAt := time.Now()
		saved, err := SaveStoreWithOption(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			CreatedAt: time.Now(),
			UpdatedAt: nextUpdatedAt,
			ExpiresAt: time.Now().Add(2 * time.Minute),
		}, SaveStoreOption{
			MinUpdateInterval: 15 * time.Second,
		})
		require.NoError(t, err)
		assert.Equal(t, 20, saved.ChannelID)
		assert.WithinDuration(t, initial.CreatedAt, saved.CreatedAt, time.Second)
		assert.True(t, saved.UpdatedAt.After(initial.UpdatedAt))
		assert.WithinDuration(t, nextUpdatedAt, saved.UpdatedAt, time.Second)
		assert.True(t, saved.ExpiresAt.After(initial.ExpiresAt))
	})
}

func withTestPostgresStoreDB(t *testing.T, fn func()) {
	t.Helper()

	ctx := context.Background()

	oldLogDB := LogDB
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "aiproxy_test",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}

	var (
		container testcontainers.Container
		err       error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("docker unavailable: %v", r)
			}
		}()

		container, err = testcontainers.GenericContainer(
			ctx,
			testcontainers.GenericContainerRequest{
				ContainerRequest: req,
				Started:          true,
			},
		)
	}()
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dsn := fmt.Sprintf(
		"postgres://postgres:postgres@%s/aiproxy_test?sslmode=disable&TimeZone=UTC",
		net.JoinHostPort(host, port.Port()),
	)

	db, err := OpenPostgreSQL(dsn)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&StoreV2{}))

	DB = db
	LogDB = db
	common.RedisEnabled = false
	common.UsingSQLite = false

	storeLocalCache.Flush()

	t.Cleanup(func() {
		DB = oldDB
		LogDB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite

		storeLocalCache.Flush()

		sqlDB, sqlErr := db.DB()
		require.NoError(t, sqlErr)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, container.Terminate(ctx))
	})

	fn()
}
