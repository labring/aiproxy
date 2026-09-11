//nolint:testpackage
package controller

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInitialChannelBypassChannelModelCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	original := config.EnableAdminBypassChannelModelCheck
	t.Cleanup(func() { config.EnableAdminBypassChannelModelCheck = original })

	channel := &model.Channel{ID: 42, Type: model.ChannelTypeOpenAI}

	tests := []struct {
		name      string
		feature   bool
		status    int
		wantError string
	}{
		{
			name:    "internal bypasses model and set checks",
			feature: true,
			status:  model.GroupStatusInternal,
		},
		{name: "feature off", status: model.GroupStatusInternal, wantError: "not found for model"},
		{
			name:      "regular group denied",
			feature:   true,
			status:    model.GroupStatusEnabled,
			wantError: "channel header is not allowed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.EnableAdminBypassChannelModelCheck = tt.feature
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				"POST",
				"/v1/chat/completions",
				nil,
			)
			c.Request.Header.Set(AIProxyChannelHeader, "42")
			c.Set(middleware.Group, model.GroupCache{Status: tt.status})
			c.Set(middleware.ModelCaches, &model.ModelCaches{
				ChannelsByID: map[int]*model.Channel{42: channel},
			})

			initial, err := getInitialChannel(c, "unsaved-model", mode.ChatCompletions)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}

			require.NoError(t, err)
			require.True(t, initial.designatedChannel)
			assert.Same(t, channel, initial.channel)
		})
	}
}

func TestGetChannelFromHeaderModelCheck(t *testing.T) {
	original := config.EnableAdminBypassChannelModelCheck
	t.Cleanup(func() { config.EnableAdminBypassChannelModelCheck = original })

	tests := []struct {
		name      string
		feature   bool
		status    int
		header    string
		modelName string
		mode      mode.Mode
		wantError string
	}{
		{
			name:    "enabled channel with no configured models",
			feature: true,
			status:  model.ChannelStatusEnabled,
		},
		{
			name:    "disabled channel with no configured models",
			feature: true,
			status:  model.ChannelStatusDisabled,
		},
		{name: "feature off", wantError: "not found for model"},
		{name: "configured model without feature", modelName: "configured-model"},
		{name: "unknown channel", feature: true, header: "43", wantError: "channel 43 not found"},
		{name: "invalid channel ID", feature: true, header: "invalid", wantError: "invalid syntax"},
		{
			name:      "unsupported mode",
			feature:   true,
			mode:      mode.Mode(-1),
			wantError: "not supported by adaptor",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.EnableAdminBypassChannelModelCheck = tt.feature
			channel := &model.Channel{ID: 42, Type: model.ChannelTypeOpenAI, Status: tt.status}

			mc := &model.ModelCaches{
				ChannelsByID: map[int]*model.Channel{42: channel},
			}
			if !tt.feature {
				mc.EnabledModel2ChannelsBySet = map[string]map[string][]*model.Channel{
					model.ChannelDefaultSet: {"configured-model": {channel}},
				}
			}

			header := tt.header
			if header == "" {
				header = "42"
			}

			modelName := tt.modelName
			if modelName == "" {
				modelName = "unsaved-model"
			}

			m := tt.mode
			if m == 0 {
				m = mode.ChatCompletions
			}

			got, err := GetChannelFromHeader(
				header,
				mc,
				[]string{model.ChannelDefaultSet},
				modelName,
				m,
			)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Same(t, channel, got)
		})
	}
}

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

		initial, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, initial.migratedChannels, 2)
		assert.Equal(t, 2, initial.channel.ID)
	})

	t.Run("uses prefer id order instead of priority", func(t *testing.T) {
		t.Parallel()

		mc := newModelCaches(100, 1)

		initial, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2, 1},
			map[int64]float64{},
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 2, initial.channel.ID)
	})

	t.Run("falls back from preferred when preferred exceeds max error rate", func(t *testing.T) {
		t.Parallel()

		mc := newModelCaches(10, 10)

		initial, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{2: 0.9, 1: 0.1},
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 1, initial.channel.ID)
	})

	t.Run("preferred path shares fallback semantics with default path", func(t *testing.T) {
		t.Parallel()

		mc := newModelCaches(10, 10)

		initial, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{2: 0.9},
			map[int64]struct{}{1: {}},
		)
		require.NoError(t, err)
		assert.Equal(t, 2, initial.channel.ID)
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

		channel, err := getRetryChannel(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.ID)
	})

	t.Run("retry skips preferred channel after it failed", func(t *testing.T) {
		t.Parallel()

		state := newRetryState()

		state.failedChannelIDs = map[int64]struct{}{2: {}}
		channel, err := getRetryChannel(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, 1, channel.ID)
	})

	t.Run(
		"starts a new round when failed channels consume all retry candidates",
		func(t *testing.T) {
			t.Parallel()

			state := newRetryState()

			state.preferChannelIDs = nil
			state.failedChannelIDs = map[int64]struct{}{1: {}, 2: {}}
			state.ignoreChannelIDs = nil
			state.meta = meta.NewMeta(
				state.migratedChannels[0],
				mode.Responses,
				"gpt-5",
				model.ModelConfig{},
			)

			channel, err := getRetryChannel(context.Background(), state)
			require.NoError(t, err)
			assert.NotNil(t, channel)
			assert.Empty(t, state.failedChannelIDs)
		},
	)
}

func TestGetRetryChannelStartsNewRoundAfterCandidatesAreExhausted(t *testing.T) {
	t.Parallel()

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

	state := &retryState{
		meta:             meta.NewMeta(ch1, mode.Responses, "gpt-5", model.ModelConfig{}),
		migratedChannels: []*model.Channel{ch1, ch2},
		failedChannelIDs: map[int64]struct{}{1: {}, 2: {}},
		ignoreChannelIDs: nil,
		preferChannelIDs: []int{1},
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Contains(t, []int{1, 2}, channel.ID)
	assert.Empty(t, state.failedChannelIDs)
	assert.Empty(t, state.preferChannelIDs)

	state.failedChannelIDs[int64(channel.ID)] = struct{}{}
	nextChannel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, nextChannel)
	assert.NotEqual(t, channel.ID, nextChannel.ID)
}

func TestGetRetryChannelKeepsPermissionFailuresIgnoredAcrossRounds(t *testing.T) {
	t.Parallel()

	ch1 := &model.Channel{ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled}
	ch2 := &model.Channel{ID: 2, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled}
	state := &retryState{
		meta:             meta.NewMeta(ch1, mode.Responses, "gpt-5", model.ModelConfig{}),
		migratedChannels: []*model.Channel{ch1, ch2},
		failedChannelIDs: map[int64]struct{}{1: {}},
		ignoreChannelIDs: map[int64]struct{}{2: {}},
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 1, channel.ID)
	assert.Empty(t, state.failedChannelIDs)
}

func TestGetRetryChannelKeepsDesignatedChannelPinned(t *testing.T) {
	t.Parallel()

	ch1 := &model.Channel{ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled}
	ch2 := &model.Channel{ID: 2, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled}
	state := &retryState{
		designatedChannel: ch1,
		meta:              meta.NewMeta(ch1, mode.Responses, "gpt-5", model.ModelConfig{}),
		migratedChannels:  []*model.Channel{ch2},
		failedChannelIDs:  map[int64]struct{}{1: {}, 2: {}},
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, ch1.ID, channel.ID)

	state.ignoreChannelIDs = map[int64]struct{}{1: {}}
	channel, err = getRetryChannel(context.Background(), state)
	require.ErrorIs(t, err, ErrChannelsExhausted)
	assert.Nil(t, channel)
}

func TestFilterChannelsAppliesRetryEligibilityRules(t *testing.T) {
	t.Parallel()

	enabled := &model.Channel{
		ID:     1,
		Status: model.ChannelStatusEnabled,
	}
	disabled := &model.Channel{
		ID:     2,
		Status: model.ChannelStatusDisabled,
	}
	highError := &model.Channel{
		ID:     3,
		Status: model.ChannelStatusEnabled,
	}
	exactThreshold := &model.Channel{
		ID:     6,
		Status: model.ChannelStatusEnabled,
	}
	ignored := &model.Channel{
		ID:     4,
		Status: model.ChannelStatusEnabled,
	}
	multiIgnored := &model.Channel{
		ID:     5,
		Status: model.ChannelStatusEnabled,
	}

	filtered := filterChannels(
		[]*model.Channel{nil, enabled, disabled, highError, ignored, multiIgnored, exactThreshold},
		map[int64]float64{3: maxRetryErrorRate + 0.01, 6: maxRetryErrorRate},
		maxRetryErrorRate,
		map[int64]struct{}{4: {}},
		map[int64]struct{}{5: {}},
	)

	gotIDs := make([]int, len(filtered))
	for i, channel := range filtered {
		gotIDs[i] = channel.ID
	}

	assert.Equal(t, []int{enabled.ID, exactThreshold.ID}, gotIDs)
}

func TestGetRetryChannelVisitsEveryEligibleChannelBeforeNextRound(t *testing.T) {
	t.Parallel()

	channels := []*model.Channel{
		{ID: 1, Status: model.ChannelStatusEnabled},
		{ID: 2, Status: model.ChannelStatusEnabled},
		{ID: 3, Status: model.ChannelStatusEnabled},
	}
	state := &retryState{
		meta:             meta.NewMeta(channels[0], mode.Responses, "gpt-5", model.ModelConfig{}),
		migratedChannels: channels,
		failedChannelIDs: map[int64]struct{}{1: {}, 2: {}, 3: {}},
	}
	seen := make(map[int]struct{}, len(channels))

	for len(seen) < len(channels) {
		channel, err := getRetryChannel(context.Background(), state)
		require.NoError(t, err)
		require.NotNil(t, channel)
		_, alreadySeen := seen[channel.ID]
		assert.False(t, alreadySeen, "channel %d was selected twice in one round", channel.ID)
		seen[channel.ID] = struct{}{}
		state.failedChannelIDs[int64(channel.ID)] = struct{}{}
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Empty(t, state.failedChannelIDs)
}

func TestFilterChannelsDisablesErrorRateFilterAtZero(t *testing.T) {
	t.Parallel()

	channels := []*model.Channel{
		{ID: 1, Status: model.ChannelStatusEnabled},
		{ID: 2, Status: model.ChannelStatusDisabled},
	}

	filtered := filterChannels(
		channels,
		map[int64]float64{1: 1},
		0,
	)

	require.Len(t, filtered, 1)
	assert.Equal(t, 1, filtered[0].ID)
}

func TestGetRetryChannelReturnsExhaustedWhenNoRoundCanBeReset(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		ID:     1,
		Status: model.ChannelStatusDisabled,
	}
	state := &retryState{
		meta:             meta.NewMeta(channel, mode.Responses, "gpt-5", model.ModelConfig{}),
		migratedChannels: []*model.Channel{channel},
		failedChannelIDs: map[int64]struct{}{},
	}

	got, err := getRetryChannel(context.Background(), state)
	require.ErrorIs(t, err, ErrChannelsExhausted)
	assert.Nil(t, got)
	assert.Empty(t, state.failedChannelIDs)
}

func TestGetRetryChannelReturnsExhaustedWhenRoundResetHasNoEligibleChannel(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		ID:     1,
		Status: model.ChannelStatusEnabled,
	}
	state := &retryState{
		meta:             meta.NewMeta(channel, mode.Responses, "gpt-5", model.ModelConfig{}),
		migratedChannels: []*model.Channel{channel},
		failedChannelIDs: map[int64]struct{}{1: {}},
		ignoreChannelIDs: map[int64]struct{}{1: {}},
	}

	got, err := getRetryChannel(context.Background(), state)
	require.ErrorIs(t, err, ErrChannelsExhausted)
	assert.Nil(t, got)
	assert.Empty(t, state.failedChannelIDs)
}

func TestGetRetryChannelRoundResetPreservesBackoffState(t *testing.T) {
	t.Parallel()

	ch1 := &model.Channel{ID: 1, Status: model.ChannelStatusEnabled}
	ch2 := &model.Channel{ID: 2, Status: model.ChannelStatusEnabled}
	base := time.Unix(100, 0)
	state := &retryState{
		meta:             meta.NewMeta(ch1, mode.Responses, "gpt-5", model.ModelConfig{}),
		migratedChannels: []*model.Channel{ch1, ch2},
		failedChannelIDs: map[int64]struct{}{1: {}, 2: {}},
		channelRetryInfo: map[int]channelRetryInfo{
			1: {failures: 2, lastEndAt: base},
		},
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Empty(t, state.failedChannelIDs)
	assert.Equal(t, channelRetryInfo{failures: 2, lastEndAt: base}, state.channelRetryInfo[1])
}

func TestGetPriorityWeight(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{Priority: 10}

	t.Run("applies stronger than linear penalty for higher error rates", func(t *testing.T) {
		t.Parallel()

		lowErrorWeight := getPriorityWeight(channel, 0.05)
		highErrorWeight := getPriorityWeight(channel, 0.5)

		assert.InDelta(t, 444.444444, lowErrorWeight, 0.0001)
		assert.InDelta(t, 27.777778, highErrorWeight, 0.0001)
		assert.Greater(t, lowErrorWeight/highErrorWeight, 10.0)
	})

	t.Run(
		"uses base smoothing for low error rates and clamps very high error rates",
		func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, 1000.0, getPriorityWeight(channel, 0), 0.0001)
			assert.InDelta(t, 826.446281, getPriorityWeight(channel, 0.01), 0.0001)
			assert.Equal(t, getPriorityWeight(channel, 2), getPriorityWeight(channel, 1))
		},
	)
}

func TestGetPriorityWeightHandlesNilErrorRatesMap(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{Priority: 10}

	assert.InDelta(t, 1000.0, getPriorityWeight(channel, getChannelErrorRate(nil, 123)), 0.0001)
}

func TestGetChannelWithFallbackHandlesNilInputs(t *testing.T) {
	t.Parallel()

	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {
				"gpt-5": {
					{
						ID:       1,
						Type:     model.ChannelTypeOpenAI,
						Status:   model.ChannelStatusEnabled,
						Priority: 10,
					},
				},
			},
		},
	}

	initial, err := getChannelWithFallback(
		mc,
		[]string{model.ChannelDefaultSet},
		"gpt-5",
		mode.Responses,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Len(t, initial.migratedChannels, 1)
	require.NotNil(t, initial.channel)
	assert.Equal(t, 1, initial.channel.ID)
}

func TestGetRetryChannelHandlesNilInputs(t *testing.T) {
	t.Parallel()

	ch1 := &model.Channel{
		ID:       1,
		Type:     model.ChannelTypeOpenAI,
		Status:   model.ChannelStatusEnabled,
		Priority: 10,
	}

	state := &retryState{
		preferChannelIDs: nil,
		ignoreChannelIDs: nil,
		meta: meta.NewMeta(
			ch1,
			mode.Responses,
			"gpt-5",
			model.ModelConfig{},
		),
		migratedChannels: []*model.Channel{ch1},
		failedChannelIDs: map[int64]struct{}{},
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 1, channel.ID)
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

func TestGetPreferChannelIDsFallsBackToUserWhenPromptCacheKeyExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowUserStoreID("gpt-5", "user-1", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 33,
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
		c.Set(middleware.PromptCacheKey, "missing-cache-key")
		c.Set(middleware.RequestUser, "user-1")
		c.Set(middleware.ModelConfig, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true},
			},
		})

		assert.Equal(t, []int{33}, getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions))
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

func TestGetPreferChannelIDsReadsStableBeforeRecent(t *testing.T) {
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
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeRecent),
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
				"cachefollow": {"enable": true, "enable_generic_follow": true},
			},
		})

		assert.Equal(t, []int{11, 22}, getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions))
	})
}

func TestGetPreferChannelIDsReadsPromptThenUserThenGeneric(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		storeIDs := []struct {
			id        string
			channelID int
		}{
			{
				id:        model.PromptCacheStoreID("gpt-5", "cache-key", model.CacheKeyTypeStable),
				channelID: 11,
			},
			{
				id:        model.PromptCacheStoreID("gpt-5", "cache-key", model.CacheKeyTypeRecent),
				channelID: 12,
			},
			{
				id: model.CacheFollowUserStoreID(
					"gpt-5",
					"user-1",
					model.CacheKeyTypeStable,
				),
				channelID: 21,
			},
			{
				id: model.CacheFollowUserStoreID(
					"gpt-5",
					"user-1",
					model.CacheKeyTypeRecent,
				),
				channelID: 22,
			},
			{
				id:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
				channelID: 31,
			},
			{
				id:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeRecent),
				channelID: 32,
			},
		}

		for _, item := range storeIDs {
			_, err := model.SaveStore(&model.StoreV2{
				ID:        item.id,
				GroupID:   "group-1",
				TokenID:   7,
				ChannelID: item.channelID,
				Model:     "gpt-5",
			})
			require.NoError(t, err)
		}

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
		c.Set(middleware.Token, model.TokenCache{ID: 7})
		c.Set(middleware.PromptCacheKey, "cache-key")
		c.Set(middleware.RequestUser, "user-1")
		c.Set(middleware.ModelConfig, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true, "enable_generic_follow": true},
			},
		})

		assert.Equal(
			t,
			[]int{11, 12, 21, 22, 31, 32},
			getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions),
		)
	})
}

func TestGetPreferChannelIDsReadsGenericOnlyWhenExplicitlyEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 31,
			Model:     "gpt-5",
		})
		require.NoError(t, err)

		_, err = model.SaveStore(&model.StoreV2{
			ID:        model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeRecent),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 32,
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
				"cachefollow": {"enable": true, "enable_generic_follow": true},
			},
		})

		assert.Equal(t, []int{31, 32}, getPreferChannelIDs(c, "gpt-5", mode.ChatCompletions))
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
