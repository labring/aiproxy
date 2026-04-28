//nolint:testpackage
package model

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type queryBlocker struct {
	count   atomic.Int32
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func installQueryBlocker(t *testing.T, db *gorm.DB, schemaName string) *queryBlocker {
	t.Helper()

	blocker := &queryBlocker{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}

	name := "test:block_query:" + schemaName + ":" + t.Name()
	require.NoError(
		t,
		db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
			if tx.Statement == nil || tx.Statement.Schema == nil ||
				tx.Statement.Schema.Name != schemaName {
				return
			}

			n := blocker.count.Add(1)
			if n == 1 {
				blocker.once.Do(func() {
					close(blocker.started)
				})

				<-blocker.release
			}
		}),
	)

	t.Cleanup(func() {
		require.NoError(t, db.Callback().Query().Remove(name))
	})

	return blocker
}

func waitChannelClosed(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal(msg)
	}
}

func withTestModelCacheDB(t *testing.T, fn func()) {
	t.Helper()

	oldDB := DB
	oldLogDB := LogDB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "model_cache_test.db"))
	require.NoError(t, err)
	require.NoError(
		t,
		db.AutoMigrate(
			&Group{},
			&GroupModelConfig{},
			&Token{},
			&PublicMCP{},
			&PublicMCPReusingParam{},
			&GroupMCP{},
		),
	)

	DB = db
	LogDB = db
	common.RedisEnabled = false
	common.UsingSQLite = true

	modelLocalCache.Flush()

	t.Cleanup(func() {
		DB = oldDB
		LogDB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite

		modelLocalCache.Flush()

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	fn()
}

func TestCacheGetStoreUsesPerKeyLockOnDBFallback(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := ResponseStoreID("resp_db_lock_hit")
		_, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-lock",
			TokenID:   1,
			ChannelID: 11,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)
		storeLocalCache.Flush()

		blocker := installQueryBlocker(t, LogDB, "StoreV2")

		var (
			wg      sync.WaitGroup
			results [2]*StoreCache
			errs    [2]error
		)

		wg.Go(func() {
			results[0], errs[0] = CacheGetStore("group-lock", 1, storeID)
		})

		waitChannelClosed(t, blocker.started, "first store query did not start")

		wg.Go(func() {
			results[1], errs[1] = CacheGetStore("group-lock", 1, storeID)
		})

		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, int32(1), blocker.count.Load())

		close(blocker.release)
		wg.Wait()

		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		require.NotNil(t, results[0])
		require.NotNil(t, results[1])
		assert.Equal(t, 11, results[0].ChannelID)
		assert.Equal(t, 11, results[1].ChannelID)
		assert.Equal(t, int32(1), blocker.count.Load())
	})
}

func TestCacheGetStoreUsesPerKeyLockOnDBNotFoundFallback(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := ResponseStoreID("resp_db_lock_miss")
		blocker := installQueryBlocker(t, LogDB, "StoreV2")

		var (
			wg   sync.WaitGroup
			errs [2]error
		)

		wg.Go(func() {
			_, errs[0] = CacheGetStore("group-lock-miss", 1, storeID)
		})

		waitChannelClosed(t, blocker.started, "first store miss query did not start")

		wg.Go(func() {
			_, errs[1] = CacheGetStore("group-lock-miss", 1, storeID)
		})

		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, int32(1), blocker.count.Load())

		close(blocker.release)
		wg.Wait()

		require.Error(t, errs[0])
		require.Error(t, errs[1])
		assert.True(t, errors.Is(errs[0], gorm.ErrRecordNotFound))
		assert.True(t, errors.Is(errs[1], gorm.ErrRecordNotFound))
		assert.Equal(t, int32(1), blocker.count.Load())
	})
}

func TestCacheGetGroupUsesPerKeyLockOnDBFallback(t *testing.T) {
	withTestModelCacheDB(t, func() {
		require.NoError(t, DB.Create(&Group{
			ID:            "group-db-lock",
			Status:        GroupStatusEnabled,
			AvailableSets: []string{"default"},
		}).Error)

		blocker := installQueryBlocker(t, DB, "Group")

		var (
			wg      sync.WaitGroup
			results [2]*GroupCache
			errs    [2]error
		)

		wg.Go(func() {
			results[0], errs[0] = CacheGetGroup("group-db-lock")
		})

		waitChannelClosed(t, blocker.started, "first group query did not start")

		wg.Go(func() {
			results[1], errs[1] = CacheGetGroup("group-db-lock")
		})

		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, int32(1), blocker.count.Load())

		close(blocker.release)
		wg.Wait()

		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		require.NotNil(t, results[0])
		require.NotNil(t, results[1])
		assert.Equal(t, "group-db-lock", results[0].ID)
		assert.Equal(t, "group-db-lock", results[1].ID)
		assert.Equal(t, int32(1), blocker.count.Load())
	})
}

func TestCacheUpdateGroupStatusUpdatesLocalCache(t *testing.T) {
	withTestModelCacheDB(t, func() {
		cache := &GroupCache{
			ID:     "group-local-update",
			Status: GroupStatusEnabled,
		}
		require.NoError(t, CacheSetGroup(cache))

		require.NoError(t, CacheUpdateGroupStatus(cache.ID, GroupStatusDisabled))

		got, err := CacheGetGroup(cache.ID)
		require.NoError(t, err)
		assert.Equal(t, GroupStatusDisabled, got.Status)
	})
}

func TestCacheUpdateTokenStatusUpdatesLocalCache(t *testing.T) {
	withTestModelCacheDB(t, func() {
		cache := &TokenCache{
			Key:    "token-local-update",
			ID:     1,
			Group:  "g1",
			Name:   "token",
			Status: TokenStatusEnabled,
		}
		require.NoError(t, CacheSetToken(cache))

		require.NoError(t, CacheUpdateTokenStatus(cache.Key, TokenStatusDisabled))

		got, err := CacheGetTokenByKey(cache.Key)
		require.NoError(t, err)
		assert.Equal(t, TokenStatusDisabled, got.Status)
	})
}

func TestCacheUpdatePublicMCPStatusUpdatesLocalCache(t *testing.T) {
	withTestModelCacheDB(t, func() {
		cache := &PublicMCPCache{
			ID:     "public-mcp-local-update",
			Status: PublicMCPStatusEnabled,
		}
		require.NoError(t, CacheSetPublicMCP(cache))

		require.NoError(t, CacheUpdatePublicMCPStatus(cache.ID, PublicMCPStatusDisabled))

		got, err := CacheGetPublicMCP(cache.ID)
		require.NoError(t, err)
		assert.Equal(t, PublicMCPStatusDisabled, got.Status)
	})
}

func TestCacheUpdateModelLocalSerializesConcurrentUpdates(t *testing.T) {
	type testCounter struct {
		Value int
	}

	clone := func(counter *testCounter) *testCounter {
		if counter == nil {
			return nil
		}

		cloned := *counter

		return &cloned
	}

	key := "test:model:update-lock"

	modelLocalCache.Flush()
	t.Cleanup(modelLocalCache.Flush)

	cacheSetModelLocal(key, &testCounter{}, clone)

	started := make(chan struct{})
	release := make(chan struct{})

	var once sync.Once

	var wg sync.WaitGroup

	wg.Go(func() {
		updated := cacheUpdateModelLocal(key, clone, func(counter *testCounter) bool {
			once.Do(func() {
				close(started)
				<-release
			})

			counter.Value++

			return true
		})
		require.True(t, updated)
	})

	waitChannelClosed(t, started, "first local update did not start")

	wg.Go(func() {
		updated := cacheUpdateModelLocal(key, clone, func(counter *testCounter) bool {
			counter.Value++
			return true
		})
		require.True(t, updated)
	})

	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	counter, notFound, ok := cacheGetModelLocal(key, clone)
	require.True(t, ok)
	require.False(t, notFound)
	require.NotNil(t, counter)
	assert.Equal(t, 2, counter.Value)
}
