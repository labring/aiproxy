//nolint:testpackage
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestMergeGroupChannelModelsBySetIncludesModelMappingKeys(t *testing.T) {
	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: "group-mapped",
		Models:  []string{"public-model"},
		Configs: map[string]model.ModelConfig{
			"public-model": {Model: "public-model"},
		},
		List: []model.GroupScopeModelConfig{
			{GroupID: "group-mapped", ModelConfig: model.ModelConfig{Model: "public-model"}},
		},
	}))
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: "group-mapped",
		Channels: []*model.GroupChannel{
			{
				ID:           1,
				GroupID:      "group-mapped",
				Status:       model.ChannelStatusEnabled,
				Models:       []string{"provider-model"},
				ModelMapping: map[string]string{"public-model": "provider-model"},
				Sets:         []string{"default"},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig("group-mapped"))
		require.NoError(t, model.CacheDeleteGroupChannels("group-mapped"))
	})

	modelsBySet := mergeGroupChannelModelsBySet(
		model.GroupCache{
			ID:            "group-mapped",
			Status:        model.GroupStatusEnabled,
			AvailableSets: []string{"default"},
		},
		[]string{"default"},
		GroupChannelModeOwn,
		map[string][]string{},
	)

	require.ElementsMatch(t, []string{"public-model"}, modelsBySet["default"])

	require.Equal(
		t,
		"public-model",
		model.FindTokenModel(
			model.TokenCache{},
			"public-model",
			[]string{"default"},
			modelsBySet,
		),
	)
}

func TestMergeGroupChannelModelsBySetUsesEffectiveSets(t *testing.T) {
	groupID := "group-set-intersection"
	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: groupID,
		Models:  []string{"default-model", "beta-model"},
		Configs: map[string]model.ModelConfig{
			"default-model": {Model: "default-model"},
			"beta-model":    {Model: "beta-model"},
		},
		List: []model.GroupScopeModelConfig{
			{GroupID: groupID, ModelConfig: model.ModelConfig{Model: "default-model"}},
			{GroupID: groupID, ModelConfig: model.ModelConfig{Model: "beta-model"}},
		},
	}))
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: groupID,
		Channels: []*model.GroupChannel{
			{
				ID:      1,
				GroupID: groupID,
				Status:  model.ChannelStatusEnabled,
				Models:  []string{"default-model"},
			},
			{
				ID:      2,
				GroupID: groupID,
				Status:  model.ChannelStatusEnabled,
				Models:  []string{"beta-model"},
				Sets:    []string{"beta"},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig(groupID))
		require.NoError(t, model.CacheDeleteGroupChannels(groupID))
	})

	modelsBySet := mergeGroupChannelModelsBySet(
		model.GroupCache{
			ID:            groupID,
			Status:        model.GroupStatusEnabled,
			AvailableSets: []string{model.ChannelDefaultSet, "beta"},
		},
		[]string{"beta"},
		GroupChannelModeOwn,
		map[string][]string{},
	)

	require.NotContains(t, modelsBySet, model.ChannelDefaultSet)
	require.ElementsMatch(t, []string{"beta-model"}, modelsBySet["beta"])
}

func TestMergeGroupChannelModelsBySetUsesChannelModelsWhenModelConfigDisabled(t *testing.T) {
	oldDisableModelConfig := config.DisableModelConfig
	config.DisableModelConfig = true
	t.Cleanup(func() {
		config.DisableModelConfig = oldDisableModelConfig
	})

	groupID := "group-disabled-model-config"
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: groupID,
		Channels: []*model.GroupChannel{
			{
				ID:      1,
				GroupID: groupID,
				Status:  model.ChannelStatusEnabled,
				Models:  []string{"dynamic-model"},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupChannels(groupID))
	})

	modelsBySet := mergeGroupChannelModelsBySet(
		model.GroupCache{
			ID:            groupID,
			Status:        model.GroupStatusEnabled,
			AvailableSets: []string{model.ChannelDefaultSet},
		},
		[]string{model.ChannelDefaultSet},
		GroupChannelModeOwn,
		map[string][]string{},
	)

	require.ElementsMatch(t, []string{"dynamic-model"}, modelsBySet[model.ChannelDefaultSet])
}

func TestMergeGroupChannelModelsBySetGlobalModeUsesEffectiveSets(t *testing.T) {
	global := map[string][]string{
		model.ChannelDefaultSet: {"default-model"},
		"beta":                  {"beta-model"},
	}

	modelsBySet := mergeGroupChannelModelsBySet(
		model.GroupCache{ID: "group-global-sets", Status: model.GroupStatusEnabled},
		[]string{"beta"},
		GroupChannelModeGlobal,
		global,
	)

	require.Equal(t, map[string][]string{"beta": {"beta-model"}}, modelsBySet)
}

func TestExternalTokenGroupChannelModeIgnoresCallerHeader(t *testing.T) {
	require.Equal(
		t,
		GroupChannelModeOwn,
		getTokenGroupChannelMode(model.TokenCache{Scope: model.ChannelScopeGroup}),
	)
	require.Equal(
		t,
		GroupChannelModeGlobal,
		getTokenGroupChannelMode(model.TokenCache{Scope: model.ChannelScopeGlobal}),
	)
	require.Equal(t, GroupChannelModeGlobal, getTokenGroupChannelMode(model.TokenCache{}))
}

func TestGroupChannelModelLimitUsesGroupChannelCounters(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	groupID := "group-channel-limit-isolated"
	modelName := "group-channel-limit-model"
	tokenName := "group-channel-limit-token"
	channelID := 77

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

	err := CheckGroupModelRPMAndTPM(
		c,
		model.GroupCache{ID: groupID, Status: model.GroupStatusEnabled},
		model.ModelConfig{
			Model: modelName,
			RPM:   100,
			TPM:   100,
		},
		tokenName,
		model.ChannelScopeGroup,
		channelID,
	)
	require.NoError(t, err)

	globalRPM, _ := reqlimit.GetGroupModelRequest(t.Context(), groupID, modelName)
	require.Zero(t, globalRPM)
	globalTPM, _ := reqlimit.GetGroupModelTokensRequest(t.Context(), groupID, modelName)
	require.Zero(t, globalTPM)
	globalTokenRPM, _ := reqlimit.GetGroupModelTokennameRequest(
		t.Context(),
		groupID,
		modelName,
		tokenName,
	)
	require.Zero(t, globalTokenRPM)

	channelRPM, _ := reqlimit.GetChannelModelRequest(
		t.Context(),
		strconv.Itoa(channelID),
		modelName,
	)
	require.Zero(t, channelRPM)

	groupChannelRPM, _ := reqlimit.GetGroupChannelModelRequest(
		t.Context(),
		groupID,
		strconv.Itoa(channelID),
		modelName,
	)
	require.Equal(t, int64(1), groupChannelRPM)
}

func TestParseGroupChannelModeDefaultsToGlobal(t *testing.T) {
	t.Parallel()

	require.Equal(t, GroupChannelModeGlobal, parseGroupChannelMode(""))
	require.Equal(t, GroupChannelModeGlobal, parseGroupChannelMode("mixed"))
	require.Equal(t, GroupChannelModeGlobal, parseGroupChannelMode("unknown"))
	require.Equal(t, GroupChannelModeGlobal, parseGroupChannelMode("global"))
	require.Equal(t, GroupChannelModeOwn, parseGroupChannelMode("own"))
	require.Equal(t, GroupChannelModeOwn, parseGroupChannelMode("group-only"))
	require.Equal(t, GroupChannelModeOwn, parseGroupChannelMode("group_only"))
	require.Equal(t, GroupChannelModeOwn, parseGroupChannelMode("group"))
}

func TestGetTokenGroupChannelModeInternalDefaultsToGlobalAndHonorsHeader(t *testing.T) {
	t.Parallel()

	require.Equal(t, GroupChannelModeGlobal, getInternalTokenGroupChannelMode("", ""))
	require.Equal(t, GroupChannelModeGlobal, getInternalTokenGroupChannelMode("group", ""))
	require.Equal(t, GroupChannelModeGlobal, getInternalTokenGroupChannelMode("", "group-1"))
	require.Equal(t, GroupChannelModeGlobal, getInternalTokenGroupChannelMode("global", "group-1"))
	require.Equal(t, GroupChannelModeOwn, getInternalTokenGroupChannelMode("group", "group-1"))
	require.Equal(t, GroupChannelModeOwn, getInternalTokenGroupChannelMode("own", "group-1"))
}

func TestMergeGroupChannelModelsBySetAllowsInternalGroupInOwnMode(t *testing.T) {
	groupID := "internal-group-channel-mode"
	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: groupID,
		Models:  []string{"group-scope-model"},
		Configs: map[string]model.ModelConfig{
			"group-scope-model": {Model: "group-scope-model"},
		},
		List: []model.GroupScopeModelConfig{
			{GroupID: groupID, ModelConfig: model.ModelConfig{Model: "group-scope-model"}},
		},
	}))
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: groupID,
		Channels: []*model.GroupChannel{
			{
				ID:      1,
				GroupID: groupID,
				Status:  model.ChannelStatusEnabled,
				Models:  []string{"group-scope-model"},
				Sets:    []string{model.ChannelDefaultSet},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig(groupID))
		require.NoError(t, model.CacheDeleteGroupChannels(groupID))
	})

	modelsBySet := mergeGroupChannelModelsBySet(
		model.GroupCache{
			ID:            groupID,
			Status:        model.GroupStatusInternal,
			AvailableSets: []string{model.ChannelDefaultSet},
		},
		[]string{model.ChannelDefaultSet},
		GroupChannelModeOwn,
		map[string][]string{model.ChannelDefaultSet: {"global-model"}},
	)

	require.ElementsMatch(t, []string{"group-scope-model"}, modelsBySet[model.ChannelDefaultSet])
}

func TestMergeGroupChannelModelsBySetInternalOwnModeUsesDefaultSet(t *testing.T) {
	groupID := "internal-group-channel-private-set"
	require.NoError(t, model.CacheSetGroupScopeModelConfigs(&model.GroupScopeModelConfigsCache{
		GroupID: groupID,
		Models:  []string{"group-private-model"},
		Configs: map[string]model.ModelConfig{
			"group-private-model": {Model: "group-private-model"},
		},
		List: []model.GroupScopeModelConfig{
			{GroupID: groupID, ModelConfig: model.ModelConfig{Model: "group-private-model"}},
		},
	}))
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: groupID,
		Channels: []*model.GroupChannel{
			{
				ID:      1,
				GroupID: groupID,
				Status:  model.ChannelStatusEnabled,
				Models:  []string{"group-private-model"},
				Sets:    []string{"private-set"},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig(groupID))
		require.NoError(t, model.CacheDeleteGroupChannels(groupID))
	})

	modelsBySet := mergeGroupChannelModelsBySet(
		model.GroupCache{
			ID:            groupID,
			Status:        model.GroupStatusInternal,
			AvailableSets: []string{model.ChannelDefaultSet},
		},
		[]string{model.ChannelDefaultSet},
		GroupChannelModeOwn,
		map[string][]string{model.ChannelDefaultSet: {"global-model"}},
	)

	require.Empty(t, modelsBySet)
}

func TestResolveRequestGroupChannelAvailableSetsInternalDefaultsToGroupSets(t *testing.T) {
	groupID := "internal-expand-group-channel-set"
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: groupID,
		Channels: []*model.GroupChannel{
			{
				ID:      1,
				GroupID: groupID,
				Status:  model.ChannelStatusEnabled,
				Sets:    []string{"private-set"},
			},
			{
				ID:      2,
				GroupID: groupID,
				Status:  model.ChannelStatusDisabled,
				Sets:    []string{"disabled-set"},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupChannels(groupID))
	})

	sets := resolveRequestGroupChannelAvailableSets(
		model.GroupCache{ID: groupID, Status: model.GroupStatusInternal},
		model.TokenCache{},
		GroupChannelModeOwn,
	)

	require.Equal(t, []string{model.ChannelDefaultSet}, sets)
}

func TestResolveRequestGroupChannelAvailableSetsAppliesTokenSets(t *testing.T) {
	groupID := "token-filter-group-channel-set"
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: groupID,
		Channels: []*model.GroupChannel{
			{
				ID:      1,
				GroupID: groupID,
				Status:  model.ChannelStatusEnabled,
				Sets:    []string{"private-set"},
			},
			{
				ID:      2,
				GroupID: groupID,
				Status:  model.ChannelStatusEnabled,
				Sets:    []string{"other-set"},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupChannels(groupID))
	})

	sets := resolveRequestGroupChannelAvailableSets(
		model.GroupCache{ID: groupID, Status: model.GroupStatusEnabled},
		model.TokenCache{GroupChannelSets: []string{"private-set"}},
		GroupChannelModeOwn,
	)

	require.Equal(t, []string{"private-set"}, sets)
}

func TestMergeGroupChannelModelsBySetGlobalModeReturnsGlobalOnly(t *testing.T) {
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: "group-global-mode",
		Channels: []*model.GroupChannel{
			{
				ID:      1,
				GroupID: "group-global-mode",
				Status:  model.ChannelStatusEnabled,
				Models:  []string{"group-model"},
				Sets:    []string{"default"},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupChannels("group-global-mode"))
	})

	global := map[string][]string{
		model.ChannelDefaultSet: {"global-model"},
	}
	modelsBySet := mergeGroupChannelModelsBySet(
		model.GroupCache{
			ID:            "group-global-mode",
			Status:        model.GroupStatusEnabled,
			AvailableSets: []string{model.ChannelDefaultSet},
		},
		[]string{model.ChannelDefaultSet},
		GroupChannelModeGlobal,
		global,
	)

	require.Equal(t, global, modelsBySet)
}
