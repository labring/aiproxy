//nolint:testpackage
package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/clause"
)

func TestStoreIDNamespaces(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "response:resp_123", ResponseStoreID("resp_123"))
	assert.Equal(t, "response:resp_123", ResponseStoreID("response:resp_123"))
	assert.Equal(t, "video_job:job_123", VideoJobStoreID("job_123"))
	assert.Equal(t, "video_generation:gen_123", VideoGenerationStoreID("gen_123"))
	assert.Contains(t, CacheFollowStoreID("gpt-5", CacheKeyTypeStable), "cachefollow:")
	assert.Contains(
		t,
		getStoreCacheNotFoundKey("group-1", 7, ResponseStoreID("resp_123"), ChannelScopeGlobal),
		"storev2notfound:group-1:7:global:response:resp_123",
	)
	assert.Equal(t, "", StoreID(StorePrefixResponse, ""))
}

func TestPromptCacheStoreID(t *testing.T) {
	t.Parallel()

	id := PromptCacheStoreID("gpt-5", "cache-key", CacheKeyTypeStable)
	assert.Contains(t, id, "prompt_cache_key:")
	assert.NotEqual(t, "prompt_cache_key:cache-key", id)
}

func TestCacheFollowStoreID(t *testing.T) {
	t.Parallel()

	id := CacheFollowStoreID("gpt-5", CacheKeyTypeStable)
	assert.Contains(t, id, "cachefollow:")
	assert.NotEqual(t, "cachefollow:gpt-5", id)
}

func TestCacheFollowUserStoreID(t *testing.T) {
	t.Parallel()

	id := CacheFollowUserStoreID("gpt-5", "user-123", CacheKeyTypeStable)
	assert.Contains(t, id, "cachefollow_user:")
	assert.NotEqual(t, "cachefollow_user:user-123", id)
}

func TestGetStoreIgnoresExpired(t *testing.T) {
	withTestStoreDB(t, func() {
		_, err := SaveStore(&StoreV2{
			ID:        ResponseStoreID("resp_expired"),
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(-time.Minute),
		})
		require.NoError(t, err)

		_, err = GetStore("group-1", 1, ResponseStoreID("resp_expired"))
		require.Error(t, err)

		_, err = CacheGetStore("group-1", 1, ResponseStoreID("resp_expired"))
		require.Error(t, err)
	})
}

func TestSaveIfNotExistStoreReplacesExpiredStore(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := PromptCacheStoreID("gpt-5", "cache-key", CacheKeyTypeStable)

		_, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(-time.Minute),
		})
		require.NoError(t, err)

		saved, err := SaveIfNotExistStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(30 * time.Minute),
		})
		require.NoError(t, err)
		assert.Equal(t, 20, saved.ChannelID)

		current, err := GetStore("group-1", 1, storeID)
		require.NoError(t, err)
		assert.Equal(t, 20, current.ChannelID)
		assert.True(t, current.ExpiresAt.After(time.Now()))
	})
}

func TestSaveIfNotExistStoreLoadsExistingGroupScopedStore(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := PromptCacheStoreID("gpt-5", "cache-key", CacheKeyTypeStable)

		existing, err := SaveIfNotExistStoreByScope(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		}, ChannelScopeGroup)
		require.NoError(t, err)

		storeLocalCache.Flush()

		saved, err := SaveIfNotExistStoreByScope(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(2 * time.Minute),
		}, ChannelScopeGroup)
		require.NoError(t, err)
		assert.Equal(t, existing.ChannelID, saved.ChannelID)
		assert.Equal(t, storeID, saved.ID)
	})
}

func TestCacheGetStoreUsesLocalCache(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := ResponseStoreID("resp_local_hit")

		_, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)

		store, err := CacheGetStore("group-1", 1, storeID)
		require.NoError(t, err)
		assert.Equal(t, 10, store.ChannelID)

		require.NoError(
			t,
			LogDB.Delete(
				&StoreV2{},
				"group_id = ? and token_id = ? and id = ?",
				"group-1",
				1,
				storeID,
			).Error,
		)

		store, err = CacheGetStore("group-1", 1, storeID)
		require.NoError(t, err)
		assert.Equal(t, 10, store.ChannelID)
	})
}

func TestSaveStorePersistsChannelScope(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := ResponseStoreID("resp_group_scope")

		_, err := SaveStoreByScope(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		}, ChannelScopeGroup)
		require.NoError(t, err)

		store, err := GetStoreByScope("group-1", 1, storeID, ChannelScopeGroup)
		require.NoError(t, err)
		assert.Equal(t, 10, store.ChannelID)

		storeLocalCache.Flush()

		cached, err := CacheGetStoreByScope("group-1", 1, storeID, ChannelScopeGroup)
		require.NoError(t, err)
		assert.Equal(t, storeID, cached.ID)
		assert.Equal(t, 10, cached.ChannelID)
	})
}

func TestSaveStoreByScopePreservesGeneratedID(t *testing.T) {
	withTestStoreDB(t, func() {
		input := &StoreV2{
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		}

		saved, err := SaveStoreByScope(input, ChannelScopeGroup)
		require.NoError(t, err)
		require.NotEmpty(t, saved.ID)
		assert.Equal(t, saved.ID, input.ID)

		current, err := GetStoreByScope("group-1", 1, saved.ID, ChannelScopeGroup)
		require.NoError(t, err)
		assert.Equal(t, saved.ID, current.ID)
		assert.Equal(t, 20, current.ChannelID)

		cached, err := CacheGetStoreByScope("group-1", 1, saved.ID, ChannelScopeGroup)
		require.NoError(t, err)
		assert.Equal(t, saved.ID, cached.ID)
		assert.Equal(t, 20, cached.ChannelID)
	})
}

func TestSaveIfNotExistStoreByScopePreservesGeneratedID(t *testing.T) {
	withTestStoreDB(t, func() {
		input := &StoreV2{
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		}

		saved, err := SaveIfNotExistStoreByScope(input, ChannelScopeGroup)
		require.NoError(t, err)
		require.NotEmpty(t, saved.ID)
		assert.Equal(t, saved.ID, input.ID)

		current, err := GetStoreByScope("group-1", 1, saved.ID, ChannelScopeGroup)
		require.NoError(t, err)
		assert.Equal(t, saved.ID, current.ID)
		assert.Equal(t, 20, current.ChannelID)

		cached, err := CacheGetStoreByScope("group-1", 1, saved.ID, ChannelScopeGroup)
		require.NoError(t, err)
		assert.Equal(t, saved.ID, cached.ID)
		assert.Equal(t, 20, cached.ChannelID)
	})
}

func TestSaveStoreDefaultsChannelScopeToGlobal(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := ResponseStoreID("resp_global_scope")

		_, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)

		store, err := GetStore("group-1", 1, storeID)
		require.NoError(t, err)
		assert.Equal(t, 10, store.ChannelID)

		cached, err := CacheGetStore("group-1", 1, storeID)
		require.NoError(t, err)
		assert.Equal(t, storeID, cached.ID)
	})
}

func TestStoreScopesUseIndependentIdentities(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := CacheFollowStoreID("gpt-5", CacheKeyTypeStable)

		_, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)

		_, err = SaveStoreByScope(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		}, ChannelScopeGroup)
		require.NoError(t, err)

		globalStore, err := CacheGetStoreByScope("group-1", 1, storeID, ChannelScopeGlobal)
		require.NoError(t, err)
		assert.Equal(t, storeID, globalStore.ID)
		assert.Equal(t, 10, globalStore.ChannelID)

		groupStore, err := CacheGetStoreByScope("group-1", 1, storeID, ChannelScopeGroup)
		require.NoError(t, err)
		assert.Equal(t, storeID, groupStore.ID)
		assert.Equal(t, 20, groupStore.ChannelID)

		var count int64
		require.NoError(t, LogDB.Model(&StoreV2{}).Count(&count).Error)
		assert.Equal(t, int64(1), count)

		require.NoError(t, LogDB.Model(&GroupChannelStoreV2{}).Count(&count).Error)
		assert.Equal(t, int64(1), count)
	})
}

func TestCacheGetStoreCachesNotFoundLocally(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := ResponseStoreID("resp_local_miss")

		_, err := CacheGetStore("group-1", 1, storeID)
		require.Error(t, err)

		err = LogDB.Create(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		}).Error
		require.NoError(t, err)

		_, err = CacheGetStore("group-1", 1, storeID)
		require.Error(t, err)

		time.Sleep(storeLocalMissTTL + 100*time.Millisecond)

		store, err := CacheGetStore("group-1", 1, storeID)
		require.NoError(t, err)
		assert.Equal(t, 10, store.ChannelID)
	})
}

func TestCacheGetStoreIgnoresInvalidLocalCache(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := ResponseStoreID("resp_invalid_local_cache")
		cacheKey := getStoreCacheKey("group-1", 1, storeID, ChannelScopeGlobal)

		require.NoError(t, LogDB.Create(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		}).Error)

		storeLocalCache.Set(cacheKey, localStoreCacheItem{}, storeLocalTTL)

		store, err := CacheGetStore("group-1", 1, storeID)
		require.NoError(t, err)
		assert.Equal(t, storeID, store.ID)
		assert.Equal(t, 10, store.ChannelID)
	})
}

func TestSaveStoreWithOptionSkipsUpdateWithinMinInterval(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := CacheFollowStoreID("gpt-5", CacheKeyTypeRecent)
		createdAt := time.Now().Add(-5 * time.Second)

		initial, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)

		saved, err := SaveStoreWithOption(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(2 * time.Minute),
		}, SaveStoreOption{
			MinUpdateInterval: 15 * time.Second,
		})
		require.NoError(t, err)
		assert.Equal(t, 10, saved.ChannelID)
		assert.WithinDuration(t, initial.CreatedAt, saved.CreatedAt, time.Second)
		assert.WithinDuration(t, initial.UpdatedAt, saved.UpdatedAt, time.Second)
	})
}

func TestSaveIfNotExistStoreUsesCachedExistingFastPath(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := PromptCacheStoreID("gpt-5", "cache-key", CacheKeyTypeStable)

		existing, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)

		_, err = CacheGetStore("group-1", 1, storeID)
		require.NoError(t, err)

		require.NoError(
			t,
			LogDB.Delete(
				&StoreV2{},
				"group_id = ? and token_id = ? and id = ?",
				"group-1",
				1,
				storeID,
			).Error,
		)

		saved, err := SaveIfNotExistStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(2 * time.Minute),
		})
		require.NoError(t, err)
		assert.Equal(t, existing.ChannelID, saved.ChannelID)
	})
}

func TestSaveIfNotExistStoreIgnoresInvalidLocalCache(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := PromptCacheStoreID("gpt-5", "invalid-cache", CacheKeyTypeStable)
		cacheKey := getStoreCacheKey("group-1", 1, storeID, ChannelScopeGlobal)

		storeLocalCache.Set(cacheKey, localStoreCacheItem{}, storeLocalTTL)

		saved, err := SaveIfNotExistStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)
		assert.Equal(t, storeID, saved.ID)
		assert.Equal(t, 20, saved.ChannelID)

		current, err := GetStore("group-1", 1, storeID)
		require.NoError(t, err)
		assert.Equal(t, 20, current.ChannelID)
	})
}

func TestSaveStoreWithOptionUsesCachedFastPathWithinMinInterval(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := CacheFollowStoreID("gpt-5", CacheKeyTypeRecent)
		now := time.Now()

		existing, err := SaveStore(&StoreV2{
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

		_, err = CacheGetStore("group-1", 1, storeID)
		require.NoError(t, err)

		require.NoError(
			t,
			LogDB.Delete(
				&StoreV2{},
				"group_id = ? and token_id = ? and id = ?",
				"group-1",
				1,
				storeID,
			).Error,
		)

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
		assert.Equal(t, existing.ChannelID, saved.ChannelID)
		assert.WithinDuration(t, existing.CreatedAt, saved.CreatedAt, time.Second)
		assert.WithinDuration(t, existing.UpdatedAt, saved.UpdatedAt, time.Second)
	})
}

func TestSaveStoreWithOptionUpdatesAfterMinIntervalAndPreservesCreatedAt(t *testing.T) {
	withTestStoreDB(t, func() {
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

func TestSaveStorePreservesCreatedAtOnUpdate(t *testing.T) {
	withTestStoreDB(t, func() {
		storeID := ResponseStoreID("resp_created_at")
		createdAt := time.Now().Add(-time.Minute)

		initial, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 10,
			Model:     "gpt-5",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
			ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)

		saved, err := SaveStore(&StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   1,
			ChannelID: 20,
			Model:     "gpt-5",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(2 * time.Minute),
		})
		require.NoError(t, err)
		assert.Equal(t, 20, saved.ChannelID)
		assert.WithinDuration(t, initial.CreatedAt, saved.CreatedAt, time.Second)
		assert.True(t, saved.UpdatedAt.After(initial.UpdatedAt))
	})
}

func TestStoreUpsertUpdateWhereQualifiesCurrentTableColumns(t *testing.T) {
	t.Parallel()

	cutoff := time.Now().Add(-time.Minute)
	now := time.Now()

	where := storeUpsertUpdateWhere(cutoff, now)
	require.Len(t, where.Exprs, 1)

	orExpr, ok := where.Exprs[0].(clause.OrConditions)
	require.True(t, ok)
	require.Len(t, orExpr.Exprs, 2)

	updatedAtExpr, ok := orExpr.Exprs[0].(clause.Lte)
	require.True(t, ok)
	updatedAtColumn, ok := updatedAtExpr.Column.(clause.Column)
	require.True(t, ok)
	assert.Equal(t, clause.CurrentTable, updatedAtColumn.Table)
	assert.Equal(t, "updated_at", updatedAtColumn.Name)
	assert.Equal(t, cutoff, updatedAtExpr.Value)

	expiresAtExpr, ok := orExpr.Exprs[1].(clause.Lte)
	require.True(t, ok)
	expiresAtColumn, ok := expiresAtExpr.Column.(clause.Column)
	require.True(t, ok)
	assert.Equal(t, clause.CurrentTable, expiresAtColumn.Table)
	assert.Equal(t, "expires_at", expiresAtColumn.Name)
	assert.Equal(t, now, expiresAtExpr.Value)
}

func withTestStoreDB(t *testing.T, fn func()) {
	t.Helper()

	oldLogDB := LogDB
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "store_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&StoreV2{}, &GroupChannelStoreV2{}))

	LogDB = db
	DB = db
	common.RedisEnabled = false

	storeLocalCache.Flush()

	t.Cleanup(func() {
		LogDB = oldLogDB
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled

		storeLocalCache.Flush()

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	fn()
}
