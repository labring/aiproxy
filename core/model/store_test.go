package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreIDNamespaces(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "response:resp_123", ResponseStoreID("resp_123"))
	assert.Equal(t, "response:resp_123", ResponseStoreID("response:resp_123"))
	assert.Equal(t, "video_job:job_123", VideoJobStoreID("job_123"))
	assert.Equal(t, "video_generation:gen_123", VideoGenerationStoreID("gen_123"))
	assert.Equal(t, "", StoreID(StorePrefixResponse, ""))
}

func TestPromptCacheStoreID(t *testing.T) {
	t.Parallel()

	id := PromptCacheStoreID("gpt-5", "cache-key")
	assert.Contains(t, id, "prompt_cache_key:")
	assert.NotEqual(t, "prompt_cache_key:cache-key", id)
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
		storeID := PromptCacheStoreID("gpt-5", "cache-key")

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

func withTestStoreDB(t *testing.T, fn func()) {
	t.Helper()

	oldLogDB := LogDB
	oldDB := DB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "store_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&StoreV2{}))

	LogDB = db
	DB = db

	t.Cleanup(func() {
		LogDB = oldLogDB
		DB = oldDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	fn()
}
