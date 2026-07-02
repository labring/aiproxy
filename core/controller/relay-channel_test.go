//nolint:testpackage
package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	relaycontroller "github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getChannelWithFallback(
	cache *model.ModelCaches,
	preferChannelIDs []int,
	errorRates map[int64]float64,
	ignoreChannelIDs map[int64]struct{},
) (*model.Channel, []*model.Channel, error) {
	errorRateKeys := make(map[string]float64, len(errorRates))
	for id, rate := range errorRates {
		errorRateKeys[strconv.FormatInt(id, 10)] = rate
	}

	scoped, scopedChannels, err := getScopedChannelWithFallback(
		cache,
		[]string{model.ChannelDefaultSet},
		nil,
		false,
		"gpt-5",
		mode.Responses,
		channelIDsToKeys(preferChannelIDs),
		errorRateKeys,
		int64SetToStringSet(ignoreChannelIDs),
	)
	if err != nil {
		return nil, nil, err
	}

	channels := make([]*model.Channel, len(scopedChannels))
	for i, channel := range scopedChannels {
		channels[i] = channel.channel
	}

	return scoped.channel, channels, nil
}

func getPreferChannelIDs(c *gin.Context) []int {
	keys := getPreferChannelKeys(c, "gpt-5", mode.ChatCompletions)
	if len(keys) == 0 {
		return nil
	}

	ids := make([]int, 0, len(keys))
	for _, key := range keys {
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}

		ids = append(ids, id)
	}

	return ids
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

		channel, migratedChannels, err := getChannelWithFallback(
			mc,
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
			[]int{2},
			map[int64]float64{2: 0.9},
			map[int64]struct{}{1: {}},
		)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.ID)
	})
}

func TestGetScopedChannelWithFallbackUsesScopedPreferredKey(t *testing.T) {
	t.Parallel()

	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: "group-1",
		Models:  []string{"gpt-5"},
		Configs: map[string]model.ModelConfig{
			"gpt-5": {Model: "gpt-5", Type: mode.Responses},
		},
		List: []model.GroupScopeModelConfig{
			{
				GroupID:     "group-1",
				ModelConfig: model.ModelConfig{Model: "gpt-5", Type: mode.Responses},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig("group-1"))
	})

	globalChannel := &model.Channel{
		ID:       22,
		Type:     model.ChannelTypeOpenAI,
		Status:   model.ChannelStatusEnabled,
		Priority: 10,
	}
	groupChannel := &model.GroupChannel{
		ID:      22,
		GroupID: "group-1",
		Type:    model.ChannelTypeOpenAI,
		Status:  model.ChannelStatusEnabled,
		Models:  []string{"gpt-5"},
	}
	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {
				"gpt-5": {globalChannel},
			},
		},
	}

	selected, _, err := getScopedChannelWithFallback(
		mc,
		[]string{model.ChannelDefaultSet},
		[]*model.GroupChannel{groupChannel},
		false,
		"gpt-5",
		mode.Responses,
		[]string{model.GroupChannelMonitorKey("group-1", 22)},
		map[string]float64{},
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, model.ChannelScopeGroup, selected.scope)
	require.Equal(t, 22, selected.channel.ID)
}

func TestGetChannelFromHeaderUsesModelCacheChannel(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		ID:     901,
		Type:   model.ChannelTypeOpenAI,
		Status: model.ChannelStatusEnabled,
		Models: []string{"gpt-5"},
	}
	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {
				"gpt-5": {channel},
			},
		},
		ModelConfig: testModelConfigCache{
			"gpt-5": {Model: "gpt-5", Type: mode.ChatCompletions},
		},
	}

	selected, err := GetChannelFromHeader("901", mc, "gpt-5", mode.ChatCompletions)
	require.NoError(t, err)
	require.Same(t, channel, selected)
}

func TestGetChannelFromHeaderAllowsDisabledModelCacheChannel(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		ID:     902,
		Type:   model.ChannelTypeOpenAI,
		Status: model.ChannelStatusDisabled,
		Models: []string{"gpt-5"},
	}
	mc := &model.ModelCaches{
		DisabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {
				"gpt-5": {channel},
			},
		},
		ModelConfig: testModelConfigCache{
			"gpt-5": {Model: "gpt-5", Type: mode.ChatCompletions},
		},
	}

	selected, err := GetChannelFromHeader("902", mc, "gpt-5", mode.ChatCompletions)
	require.NoError(t, err)
	require.Same(t, channel, selected)
}

func TestGetScopedChannelFromRequestLoadsPinnedGroupChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		groupID := "group-pinned"
		require.NoError(t, model.DB.Create(&model.GroupChannel{
			ID:      22,
			GroupID: groupID,
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Key:     "group-key",
		}).Error)
		require.NoError(t, model.DB.Create(&model.GroupScopeModelConfig{
			GroupID: groupID,
			ModelConfig: model.ModelConfig{
				Model: "gpt-5",
				Type:  mode.Responses,
			},
		}).Error)

		globalChannel := &model.Channel{
			ID:     22,
			Type:   model.ChannelTypeGoogleGeminiOpenAI,
			Status: model.ChannelStatusEnabled,
			Models: []string{"gpt-5"},
		}
		mc := &model.ModelCaches{
			EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
				model.ChannelDefaultSet: {
					"gpt-5": {globalChannel},
				},
			},
		}

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: groupID})
		c.Set(middleware.ChannelID, 22)
		c.Set(middleware.ChannelScope, model.ChannelScopeGroup)

		selected, err := GetScopedChannelFromRequest(
			c,
			mc,
			[]string{model.ChannelDefaultSet},
			false,
			"gpt-5",
			mode.ResponsesGet,
		)
		require.NoError(t, err)
		require.NotNil(t, selected)
		require.Equal(t, model.ChannelScopeGroup, selected.scope)
		require.Equal(t, groupID, selected.groupID)
		require.Equal(t, model.ChannelTypeOpenAI, selected.channel.Type)
	})
}

func TestGetScopedChannelFromRequestAllowsPinnedGroupChannelModelMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		groupID := "group-mapping"
		require.NoError(t, model.DB.Create(&model.GroupChannel{
			ID:           23,
			GroupID:      groupID,
			Type:         model.ChannelTypeOpenAI,
			Status:       model.ChannelStatusEnabled,
			Models:       []string{"provider-model"},
			ModelMapping: map[string]string{"public-model": "provider-model"},
			Key:          "group-key",
		}).Error)
		require.NoError(t, model.DB.Create(&model.GroupScopeModelConfig{
			GroupID: groupID,
			ModelConfig: model.ModelConfig{
				Model: "public-model",
				Type:  mode.Responses,
			},
		}).Error)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: groupID})
		c.Set(middleware.ChannelID, 23)
		c.Set(middleware.ChannelScope, model.ChannelScopeGroup)

		selected, err := GetScopedChannelFromRequest(
			c,
			&model.ModelCaches{},
			[]string{model.ChannelDefaultSet},
			false,
			"public-model",
			mode.ResponsesGet,
		)
		require.NoError(t, err)
		require.NotNil(t, selected)
		require.Equal(t, model.ChannelScopeGroup, selected.scope)
		require.Equal(t, groupID, selected.groupID)
		require.Equal(t, "provider-model", selected.channel.ModelMapping["public-model"])
	})
}

func TestGetScopedChannelFromRequestAllowsInternalGroupChannelOutsideAvailableSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		groupID := "internal-group-stored-pin"
		require.NoError(t, model.DB.Create(&model.GroupChannel{
			ID:      24,
			GroupID: groupID,
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Sets:    []string{"private-set"},
			Key:     "group-key",
		}).Error)
		require.NoError(t, model.DB.Create(&model.GroupScopeModelConfig{
			GroupID: groupID,
			ModelConfig: model.ModelConfig{
				Model: "gpt-5",
				Type:  mode.Responses,
			},
		}).Error)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{
			ID:     groupID,
			Status: model.GroupStatusInternal,
		})
		c.Set(middleware.ChannelID, 24)
		c.Set(middleware.ChannelScope, model.ChannelScopeGroup)

		selected, err := GetScopedChannelFromRequest(
			c,
			&model.ModelCaches{},
			[]string{model.ChannelDefaultSet},
			true,
			"gpt-5",
			mode.ResponsesGet,
		)
		require.NoError(t, err)
		require.NotNil(t, selected)
		require.Equal(t, model.ChannelScopeGroup, selected.scope)
		require.Equal(t, groupID, selected.groupID)
		require.Equal(t, 24, selected.channel.ID)
	})
}

func TestGetScopedChannelFromRequestRejectsGroupChannelWhenUserSetIntersectionEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		groupID := "user-group-empty-set-pin"
		require.NoError(t, model.DB.Create(&model.GroupChannel{
			ID:      25,
			GroupID: groupID,
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Sets:    []string{"private-set"},
			Key:     "group-key",
		}).Error)
		require.NoError(t, model.DB.Create(&model.GroupScopeModelConfig{
			GroupID: groupID,
			ModelConfig: model.ModelConfig{
				Model: "gpt-5",
				Type:  mode.Responses,
			},
		}).Error)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{
			ID:     groupID,
			Status: model.GroupStatusEnabled,
		})
		c.Set(middleware.ChannelID, 25)
		c.Set(middleware.ChannelScope, model.ChannelScopeGroup)

		selected, err := GetScopedChannelFromRequest(
			c,
			&model.ModelCaches{},
			[]string{},
			false,
			"gpt-5",
			mode.ResponsesGet,
		)
		require.Error(t, err)
		require.Nil(t, selected)
		require.Contains(t, err.Error(), "pinned group channel 25 not supported")
	})
}

func TestGetInitialChannelAllowsInternalGroupChannelMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := "internal-initial-group-channel"
	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: groupID,
		Models:  []string{"gpt-5"},
		Configs: map[string]model.ModelConfig{
			"gpt-5": {Model: "gpt-5", Type: mode.Responses},
		},
		List: []model.GroupScopeModelConfig{
			{
				GroupID:     groupID,
				ModelConfig: model.ModelConfig{Model: "gpt-5", Type: mode.Responses},
			},
		},
	}))
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: groupID,
		Channels: []*model.GroupChannel{
			{
				ID:       31,
				GroupID:  groupID,
				Type:     model.ChannelTypeOpenAI,
				Status:   model.ChannelStatusEnabled,
				Models:   []string{"gpt-5"},
				Sets:     []string{model.ChannelDefaultSet},
				Priority: 1,
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig(groupID))
		require.NoError(t, model.CacheDeleteGroupChannels(groupID))
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/responses", nil)
	c.Set(middleware.Group, model.GroupCache{
		ID:            groupID,
		Status:        model.GroupStatusInternal,
		AvailableSets: []string{model.ChannelDefaultSet},
	})
	c.Set(middleware.Token, model.TokenCache{})
	c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
	c.Set(middleware.ModelCaches, &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{},
	})
	c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeOwn)

	initial, err := getInitialChannel(c, "gpt-5", mode.Responses)
	require.NoError(t, err)
	require.NotNil(t, initial)
	require.NotNil(t, initial.channel)
	require.Equal(t, model.ChannelScopeGroup, initial.channel.scope)
	require.Equal(t, groupID, initial.channel.groupID)
	require.Equal(t, 31, initial.channel.channel.ID)
	require.True(t, initial.groupRetryOnly)
}

func TestGetInitialChannelUsesGroupPinnedHeaderInOwnMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := "internal-pinned-group-channel"
	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: groupID,
		Models:  []string{"gpt-5"},
		Configs: map[string]model.ModelConfig{
			"gpt-5": {Model: "gpt-5", Type: mode.Responses},
		},
		List: []model.GroupScopeModelConfig{
			{
				GroupID:     groupID,
				ModelConfig: model.ModelConfig{Model: "gpt-5", Type: mode.Responses},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig(groupID))
	})

	withTestStoreDB(t, func() {
		require.NoError(t, model.DB.Create(&model.GroupChannel{
			ID:      41,
			GroupID: groupID,
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Sets:    []string{model.ChannelDefaultSet},
			Key:     "group-key",
		}).Error)

		globalChannel := &model.Channel{
			ID:     41,
			Type:   model.ChannelTypeAnthropic,
			Status: model.ChannelStatusEnabled,
			Models: []string{"gpt-5"},
		}

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/responses",
			nil,
		)
		c.Request.Header.Set(AIProxyChannelHeader, "41")
		c.Set(middleware.Group, model.GroupCache{
			ID:            groupID,
			Status:        model.GroupStatusInternal,
			AvailableSets: []string{model.ChannelDefaultSet},
		})
		c.Set(middleware.Token, model.TokenCache{})
		c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
		c.Set(middleware.ModelCaches, &model.ModelCaches{
			EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
				model.ChannelDefaultSet: {
					"gpt-5": {globalChannel},
				},
			},
			DisabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{},
		})
		c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeOwn)

		initial, err := getInitialChannel(c, "gpt-5", mode.Responses)
		require.NoError(t, err)
		require.NotNil(t, initial)
		require.NotNil(t, initial.channel)
		require.True(t, initial.designatedChannel)
		require.True(t, initial.groupRetryOnly)
		require.Equal(t, model.ChannelScopeGroup, initial.channel.scope)
		require.Equal(t, groupID, initial.channel.groupID)
		require.Equal(t, model.ChannelTypeOpenAI, initial.channel.channel.Type)
	})
}

func TestGetInitialChannelGroupPinnedHeaderHonorsUserSets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := "user-pinned-group-channel-sets"
	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: groupID,
		Models:  []string{"gpt-5"},
		Configs: map[string]model.ModelConfig{
			"gpt-5": {Model: "gpt-5", Type: mode.Responses},
		},
		List: []model.GroupScopeModelConfig{
			{
				GroupID:     groupID,
				ModelConfig: model.ModelConfig{Model: "gpt-5", Type: mode.Responses},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig(groupID))
	})

	withTestStoreDB(t, func() {
		require.NoError(t, model.DB.Create(&model.GroupChannel{
			ID:      43,
			GroupID: groupID,
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Sets:    []string{"beta"},
			Key:     "group-key",
		}).Error)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/responses",
			nil,
		)
		c.Request.Header.Set(AIProxyChannelHeader, "43")
		c.Set(middleware.Group, model.GroupCache{
			ID:            groupID,
			Status:        model.GroupStatusEnabled,
			AvailableSets: []string{model.ChannelDefaultSet, "beta"},
		})
		c.Set(middleware.Token, model.TokenCache{})
		c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
		c.Set(middleware.ModelCaches, &model.ModelCaches{})
		c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeOwn)

		initial, err := getInitialChannel(c, "gpt-5", mode.Responses)
		require.Error(t, err)
		require.Nil(t, initial)
		require.Contains(t, err.Error(), "group channel 43 not supported")
	})
}

func TestGetInitialChannelGroupPinnedHeaderInternalIgnoresSets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := "internal-pinned-group-channel-sets"
	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: groupID,
		Models:  []string{"gpt-5"},
		Configs: map[string]model.ModelConfig{
			"gpt-5": {Model: "gpt-5", Type: mode.Responses},
		},
		List: []model.GroupScopeModelConfig{
			{
				GroupID:     groupID,
				ModelConfig: model.ModelConfig{Model: "gpt-5", Type: mode.Responses},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig(groupID))
	})

	withTestStoreDB(t, func() {
		require.NoError(t, model.DB.Create(&model.GroupChannel{
			ID:      44,
			GroupID: groupID,
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Sets:    []string{"beta"},
			Key:     "group-key",
		}).Error)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/responses",
			nil,
		)
		c.Request.Header.Set(AIProxyChannelHeader, "44")
		c.Set(middleware.Group, model.GroupCache{
			ID:            groupID,
			Status:        model.GroupStatusInternal,
			AvailableSets: []string{model.ChannelDefaultSet},
		})
		c.Set(middleware.Token, model.TokenCache{})
		c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
		c.Set(middleware.ModelCaches, &model.ModelCaches{})
		c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeOwn)

		initial, err := getInitialChannel(c, "gpt-5", mode.Responses)
		require.NoError(t, err)
		require.NotNil(t, initial)
		require.Equal(t, 44, initial.channel.channel.ID)
		require.Equal(t, model.ChannelScopeGroup, initial.channel.scope)
	})
}

func TestGetChannelFromRequestRejectsGlobalChannelWhenUserSetIntersectionEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	channel := &model.Channel{
		ID:     903,
		Type:   model.ChannelTypeOpenAI,
		Status: model.ChannelStatusEnabled,
		Models: []string{"gpt-5"},
	}
	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			"private-set": {
				"gpt-5": {channel},
			},
		},
		ModelConfig: testModelConfigCache{
			"gpt-5": {Model: "gpt-5", Type: mode.ChatCompletions},
		},
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.ChannelID, 903)

	selected, err := GetChannelFromRequest(
		c,
		mc,
		[]string{},
		"gpt-5",
		mode.ChatCompletions,
	)
	require.Error(t, err)
	require.Nil(t, selected)
	require.Contains(t, err.Error(), "pinned channel 903 not found")
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
			preferChannelKeys: []string{"2"},
			meta: meta.NewMeta(
				ch1,
				mode.Responses,
				"gpt-5",
				model.ModelConfig{},
			),
			migratedChannels: []*scopedChannel{
				newGlobalScopedChannel(ch1),
				newGlobalScopedChannel(ch2),
			},
			failedChannelIDs: map[string]struct{}{},
		}
	}

	t.Run("retry prefers preferred channel when available", func(t *testing.T) {
		t.Parallel()

		state := newRetryState()

		channel, err := getRetryChannel(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.channel.ID)
	})

	t.Run("retry skips preferred channel after it failed", func(t *testing.T) {
		t.Parallel()

		state := newRetryState()

		state.failedChannelIDs = map[string]struct{}{"2": {}}
		channel, err := getRetryChannel(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, 1, channel.channel.ID)
	})

	t.Run(
		"returns exhausted when failed channels consume all retry candidates",
		func(t *testing.T) {
			t.Parallel()

			state := newRetryState()

			state.preferChannelKeys = nil
			state.failedChannelIDs = map[string]struct{}{"1": {}, "2": {}}
			state.ignoreChannelIDs = nil
			state.meta = meta.NewMeta(
				state.migratedChannels[0].channel,
				mode.Responses,
				"gpt-5",
				model.ModelConfig{},
			)

			channel, err := getRetryChannel(context.Background(), state)
			require.ErrorIs(t, err, ErrChannelsExhausted)
			assert.Nil(t, channel)
		},
	)
}

func TestPickMinErrorRateHasPermissionChannel(t *testing.T) {
	t.Parallel()

	current := newGlobalScopedChannel(&model.Channel{ID: 1})
	candidate := newGlobalScopedChannel(&model.Channel{ID: 2})

	t.Run("returns candidate when current is nil", func(t *testing.T) {
		t.Parallel()

		picked := pickMinErrorRateHasPermissionChannel(
			nil,
			0,
			candidate,
			0.2,
		)
		require.NotNil(t, picked)
		assert.Equal(t, 2, picked.channel.ID)
	})

	t.Run("keeps current when candidate error rate is higher", func(t *testing.T) {
		t.Parallel()

		picked := pickMinErrorRateHasPermissionChannel(
			current,
			0.1,
			candidate,
			0.3,
		)
		require.NotNil(t, picked)
		assert.Equal(t, 1, picked.channel.ID)
	})

	t.Run("switches to candidate when candidate error rate is lower", func(t *testing.T) {
		t.Parallel()

		picked := pickMinErrorRateHasPermissionChannel(
			current,
			0.4,
			candidate,
			0.2,
		)
		require.NotNil(t, picked)
		assert.Equal(t, 2, picked.channel.ID)
	})
}

func TestGetRetryChannelFallsBackToLowestErrorRateHasPermissionChannel(t *testing.T) {
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
		meta: meta.NewMeta(
			ch1,
			mode.Responses,
			"gpt-5",
			model.ModelConfig{},
		),
		migratedChannels: []*scopedChannel{
			newGlobalScopedChannel(ch1),
			newGlobalScopedChannel(ch2),
		},
		failedChannelIDs:                     map[string]struct{}{},
		ignoreChannelIDs:                     map[string]struct{}{"1": {}, "2": {}},
		lastMinErrorRateHasPermissionChannel: newGlobalScopedChannel(ch2),
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 2, channel.channel.ID)
	assert.True(t, state.exhausted)
}

func TestInitRetryStateKeepsGroupOnlyRetryScope(t *testing.T) {
	t.Parallel()

	globalChannel := newGlobalScopedChannel(&model.Channel{
		ID:       1,
		Type:     model.ChannelTypeOpenAI,
		Status:   model.ChannelStatusEnabled,
		Priority: 10,
	})
	groupChannel := newGroupScopedChannel("group-1", &model.Channel{
		ID:       2,
		Type:     model.ChannelTypeOpenAI,
		Status:   model.ChannelStatusEnabled,
		Priority: 10,
	})
	requestMeta := meta.NewMeta(
		groupChannel.channel,
		mode.Responses,
		"gpt-5",
		model.ModelConfig{},
	)
	requestMeta.Channel.Scope = model.ChannelScopeGroup
	requestMeta.Channel.GroupID = "group-1"

	state := initRetryState(
		1,
		&initialChannel{
			channel:        groupChannel,
			groupRetryOnly: true,
			migratedChannels: []*scopedChannel{
				groupChannel,
				globalChannel,
			},
		},
		requestMeta,
		&relaycontroller.HandleResult{
			Error: relaymodel.WrapperOpenAIErrorWithMessage(
				"upstream error",
				"upstream_error",
				http.StatusInternalServerError,
			),
		},
		model.Price{},
		time.Now(),
		&model.ModelCaches{},
		"gpt-5",
		RelayController{},
		true,
	)

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, model.ChannelScopeGroup, channel.scope)
	assert.Equal(t, 2, channel.channel.ID)
}

func TestGetPriorityWeight(t *testing.T) {
	t.Parallel()

	channel := newGlobalScopedChannel(&model.Channel{Priority: 10})

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

	channel := newGlobalScopedChannel(&model.Channel{Priority: 10})

	assert.InDelta(t, 1000.0, getPriorityWeight(channel, getChannelErrorRate(nil, "123")), 0.0001)
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

	channel, migratedChannels, err := getChannelWithFallback(
		mc,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Len(t, migratedChannels, 1)
	require.NotNil(t, channel)
	assert.Equal(t, 1, channel.ID)
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
		preferChannelKeys: nil,
		ignoreChannelIDs:  nil,
		meta: meta.NewMeta(
			ch1,
			mode.Responses,
			"gpt-5",
			model.ModelConfig{},
		),
		migratedChannels: []*scopedChannel{newGlobalScopedChannel(ch1)},
		failedChannelIDs: map[string]struct{}{},
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 1, channel.channel.ID)
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
		setTestCacheFollowModelConfig(c, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true},
			},
		})

		assert.Equal(t, []int{11}, getPreferChannelIDs(c))
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
		setTestCacheFollowModelConfig(c, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true},
			},
		})

		assert.Equal(t, []int{11}, getPreferChannelIDs(c))
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
		setTestCacheFollowModelConfig(c, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true},
			},
		})

		assert.Equal(t, []int{33}, getPreferChannelIDs(c))
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
		setTestCacheFollowModelConfig(c, model.ModelConfig{Model: "gpt-5"})

		assert.Nil(t, getPreferChannelIDs(c))
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
		setTestCacheFollowModelConfig(c, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true, "enable_generic_follow": true},
			},
		})

		assert.Equal(t, []int{11, 22}, getPreferChannelIDs(c))
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
		setTestCacheFollowModelConfig(c, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true, "enable_generic_follow": true},
			},
		})

		assert.Equal(
			t,
			[]int{11, 12, 21, 22, 31, 32},
			getPreferChannelIDs(c),
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
		setTestCacheFollowModelConfig(c, model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true, "enable_generic_follow": true},
			},
		})

		assert.Equal(t, []int{31, 32}, getPreferChannelIDs(c))
	})
}

func TestGetPreferChannelKeysSkipsGroupScopeModelConfigPlugins(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		modelName := "gpt-5-scoped-pref"
		groupChannelKey := model.GroupChannelMonitorKey("group-1", 22)
		_, err := model.SaveStoreByScope(&model.StoreV2{
			ID:        model.CacheFollowStoreID(modelName, model.CacheKeyTypeStable),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 22,
			Model:     modelName,
		}, model.ChannelScopeGroup)
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
		c.Set(middleware.Token, model.TokenCache{ID: 7})
		setTestCacheFollowModelConfig(c, model.ModelConfig{
			Model: modelName,
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true, "enable_generic_follow": true},
			},
		})

		require.NotEmpty(t, groupChannelKey)
		require.Nil(t, getPreferChannelKeys(c, modelName, mode.ChatCompletions))
	})
}

func TestGetPreferChannelKeysDefaultsToGlobalScopedStores(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		modelName := "gpt-5-global-scoped-pref"
		storeID := model.CacheFollowStoreID(modelName, model.CacheKeyTypeStable)

		_, err := model.SaveStore(&model.StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 11,
			Model:     modelName,
		})
		require.NoError(t, err)

		_, err = model.SaveStoreByScope(&model.StoreV2{
			ID:        storeID,
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 22,
			Model:     modelName,
		}, model.ChannelScopeGroup)
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
		c.Set(middleware.Token, model.TokenCache{ID: 7})
		setTestCacheFollowModelConfig(c, model.ModelConfig{
			Model: modelName,
			Plugin: map[string]map[string]any{
				"cachefollow": {"enable": true, "enable_generic_follow": true},
			},
		})

		require.NotEmpty(t, model.GroupChannelMonitorKey("group-1", 22))
		require.Equal(t, []string{"11"}, getPreferChannelKeys(c, modelName, mode.ChatCompletions))
	})
}

func setTestCacheFollowModelConfig(c *gin.Context, modelConfig model.ModelConfig) {
	c.Set(middleware.ModelConfig, modelConfig)
	c.Set(middleware.ModelCaches, &model.ModelCaches{
		ModelConfig: testModelConfigCache{
			modelConfig.Model: modelConfig,
		},
	})
}

func withTestStoreDB(t *testing.T, fn func()) {
	t.Helper()

	oldLogDB := model.LogDB
	oldDB := model.DB

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "relay_channel_store_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.StoreV2{},
		&model.GroupChannelStoreV2{},
		&model.GroupChannel{},
		&model.GroupScopeModelConfig{},
	))

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
