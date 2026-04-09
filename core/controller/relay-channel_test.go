//nolint:testpackage
package controller

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChannelWithFallbackPreferred(t *testing.T) {
	t.Parallel()

	newModelCaches := func(priority1, priority2 int32) *model.ModelCaches {
		ch1 := &model.Channel{
			ID:       1,
			Type:     model.ChannelTypeOpenAI,
			Status:   model.ChannelStatusEnabled,
			Priority: priority1,
		}
		ch2 := &model.Channel{
			ID:       2,
			Type:     model.ChannelTypeOpenAI,
			Status:   model.ChannelStatusEnabled,
			Priority: priority2,
		}

		return &model.ModelCaches{
			EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
				model.ChannelDefaultSet: {
					"gpt-5": {ch1, ch2},
				},
			},
		}
	}

	t.Run("uses preferred channel when healthy", func(t *testing.T) {
		t.Parallel()

		mc := newModelCaches(10, 10)

		channel, migratedChannels, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, migratedChannels, 2)
		assert.Equal(t, 2, channel.ID)
	})

	t.Run("uses prefer id order instead of priority", func(t *testing.T) {
		t.Parallel()

		mc := newModelCaches(100, 1)

		channel, _, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2, 1},
			map[int64]float64{},
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.ID)
	})

	t.Run("falls back from preferred when preferred exceeds max error rate", func(t *testing.T) {
		t.Parallel()

		mc := newModelCaches(10, 10)

		channel, _, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{2: 0.9, 1: 0.1},
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 1, channel.ID)
	})

	t.Run("preferred path shares fallback semantics with default path", func(t *testing.T) {
		t.Parallel()

		mc := newModelCaches(10, 10)

		channel, _, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{2: 0.9},
			map[int64]struct{}{1: {}},
		)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.ID)
	})
}

func TestGetRetryChannelPrefersPreferredChannels(t *testing.T) {
	t.Parallel()

	newRetryState := func() *retryState {
		ch1 := &model.Channel{
			ID:       1,
			Type:     model.ChannelTypeOpenAI,
			Status:   model.ChannelStatusEnabled,
			Priority: 10,
		}
		ch2 := &model.Channel{
			ID:       2,
			Type:     model.ChannelTypeOpenAI,
			Status:   model.ChannelStatusEnabled,
			Priority: 10,
		}

		return &retryState{
			preferChannelIDs: []int{2},
			errorRates:       map[int64]float64{},
			meta: meta.NewMeta(
				ch1,
				mode.Responses,
				"gpt-5",
				model.ModelConfig{},
			),
			migratedChannels: []*model.Channel{ch1, ch2},
			failedChannelIDs: map[int64]struct{}{},
		}
	}

	t.Run("retry prefers preferred channel when available", func(t *testing.T) {
		t.Parallel()

		state := newRetryState()

		channel, err := getRetryChannel(state, 0, 2)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.ID)
	})

	t.Run("retry skips preferred channel after it failed", func(t *testing.T) {
		t.Parallel()

		state := newRetryState()

		state.failedChannelIDs = map[int64]struct{}{2: {}}
		channel, err := getRetryChannel(state, 0, 2)
		require.NoError(t, err)
		assert.Equal(t, 1, channel.ID)
	})

	t.Run(
		"last retry can fall back to failed channel when no other choice exists",
		func(t *testing.T) {
			t.Parallel()

			state := newRetryState()

			state.preferChannelIDs = nil
			state.failedChannelIDs = map[int64]struct{}{1: {}, 2: {}}
			state.ignoreChannelIDs = nil
			state.errorRates = map[int64]float64{}
			state.meta = meta.NewMeta(
				state.migratedChannels[0],
				mode.Responses,
				"gpt-5",
				model.ModelConfig{},
			)

			channel, err := getRetryChannel(state, 1, 2)
			require.NoError(t, err)
			assert.Contains(t, []int{1, 2}, channel.ID)
		},
	)
}

func TestGetPreferChannelIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := model.SaveStore(&model.StoreV2{
			ID:        model.PromptCacheStoreID("gpt-5", "cache-key", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 11,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		_, err = model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 22,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
		c.Set(middleware.Token, model.TokenCache{ID: 7})
		c.Set(middleware.PromptCacheKey, "cache-key")
		c.Set(middleware.ModelConfig, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true},
			},
		})

		assert.Equal(t, []int{11}, getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions))
	})
}

func TestGetPreferChannelIDsDeduplicatesPromptCacheAndCacheFollow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := model.SaveStore(&model.StoreV2{
			ID:        model.PromptCacheStoreID("gpt-5", "cache-key", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 11,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		_, err = model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 11,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
		c.Set(middleware.Token, model.TokenCache{ID: 7})
		c.Set(middleware.PromptCacheKey, "cache-key")
		c.Set(middleware.ModelConfig, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true},
			},
		})

		assert.Equal(t, []int{11}, getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions))
	})
}

func TestGetPreferChannelIDsDoesNotFallbackToCacheFollowWhenPromptCacheKeyExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 22,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
		c.Set(middleware.Token, model.TokenCache{ID: 7})
		c.Set(middleware.PromptCacheKey, "missing-cache-key")
		c.Set(middleware.ModelConfig, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true},
			},
		})

		assert.Nil(t, getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions))
	})
}

func TestGetPreferChannelIDsDisabledByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 22,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
		c.Set(middleware.Token, model.TokenCache{ID: 7})
		c.Set(middleware.ModelConfig, model.ModelConfig{Model: "gpt-5"})

		assert.Nil(t, getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions))
	})
}

func TestGetPreferChannelIDsReadsStableBeforeLast(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 11,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		_, err = model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeLast),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 22,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
		c.Set(middleware.Token, model.TokenCache{ID: 7})
		c.Set(middleware.ModelConfig, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true},
			},
		})

		assert.Equal(t, []int{11, 22}, getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions))
	})
}

func withTestStoreDB(t *testing.T, fn func()) {
	t.Helper()

	oldLogDB := model.LogDB
	oldDB := model.DB

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "relay_channel_store_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.StoreV2{}))

	model.LogDB = db
	model.DB = db

	t.Cleanup(func() {
		model.LogDB = oldLogDB
		model.DB = oldDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	fn()
}
