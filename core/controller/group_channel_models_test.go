//nolint:testpackage
package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestLoadGroupChannelModelsBySetUsesGroupScopedChannels(t *testing.T) {
	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "group_channel_models.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Group{},
		&model.GroupModelConfig{},
		&model.GroupChannel{},
		&model.GroupScopeModelConfig{},
	))

	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedisEnabled

		require.NoError(t, model.CacheDeleteGroup("group-1"))
		require.NoError(t, model.CacheDeleteGroupChannels("group-1"))
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig("group-1"))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, db.Create(&model.Group{
		ID:            "group-1",
		AvailableSets: []string{"global-only"},
	}).Error)
	require.NoError(t, db.Create(&[]model.GroupChannel{
		{
			ID:      7,
			GroupID: "group-1",
			Name:    "default-channel",
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Sets:    []string{model.ChannelDefaultSet},
		},
		{
			ID:      8,
			GroupID: "group-1",
			Name:    "beta-channel",
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Sets:    []string{"beta"},
		},
		{
			ID:      9,
			GroupID: "group-1",
			Name:    "missing-config-channel",
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"missing-config"},
			Sets:    []string{"beta"},
		},
		{
			ID:      10,
			GroupID: "group-2",
			Name:    "other-group-channel",
			Type:    model.ChannelTypeOpenAI,
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"gpt-5"},
			Sets:    []string{"beta"},
		},
	}).Error)
	require.NoError(t, model.SaveGroupScopeModelConfig(model.GroupScopeModelConfig{
		GroupID: "group-1",
		ModelConfig: model.ModelConfig{
			Model: "gpt-5",
		},
	}))

	configsBySet, channelsByModelSet, err := loadGroupChannelModelsBySet("group-1")
	require.NoError(t, err)

	require.Len(t, configsBySet[model.ChannelDefaultSet], 1)
	require.Equal(t, "gpt-5", configsBySet[model.ChannelDefaultSet][0].Model)
	require.Len(t, configsBySet["beta"], 1)
	require.Equal(t, "gpt-5", configsBySet["beta"][0].Model)

	require.Len(t, channelsByModelSet["gpt-5"][model.ChannelDefaultSet], 1)
	require.Equal(t, 7, channelsByModelSet["gpt-5"][model.ChannelDefaultSet][0].ID)
	require.Len(t, channelsByModelSet["gpt-5"]["beta"], 1)
	require.Equal(t, 8, channelsByModelSet["gpt-5"]["beta"][0].ID)
	require.NotContains(t, channelsByModelSet, "missing-config")
}

func TestGetGroupChannelDashboardModelsUsesEnabledGroupChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "group_channel_dashboard_models.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Group{},
		&model.GroupModelConfig{},
		&model.GroupChannel{},
		&model.GroupScopeModelConfig{},
	))

	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedisEnabled

		require.NoError(t, model.CacheDeleteGroup("group-dashboard-models"))
		require.NoError(t, model.CacheDeleteGroupChannels("group-dashboard-models"))
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig("group-dashboard-models"))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	groupID := "group-dashboard-models"
	require.NoError(t, db.Create(&model.Group{ID: groupID}).Error)
	require.NoError(t, db.Create(&model.GroupChannel{
		ID:      11,
		GroupID: groupID,
		Name:    "enabled-channel",
		Type:    model.ChannelTypeOpenAI,
		Status:  model.ChannelStatusEnabled,
		Models:  []string{"enabled-model"},
		Sets:    []string{model.ChannelDefaultSet},
	}).Error)
	require.NoError(t, model.SaveGroupScopeModelConfigs(groupID, []model.GroupScopeModelConfig{
		{
			GroupID: groupID,
			ModelConfig: model.ModelConfig{
				Model: "enabled-model",
			},
		},
		{
			GroupID: groupID,
			ModelConfig: model.ModelConfig{
				Model: "scope-only-model",
			},
		},
	}))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "group", Value: groupID}}
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/api/group/"+groupID+"/channel-dashboard/models",
		nil,
	)

	GetGroupChannelDashboardModels(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response middleware.APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))

	data, ok := response.Data.([]any)
	require.True(t, ok)
	require.Len(t, data, 1)

	item, ok := data[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "enabled-model", item["model"])
}
