//nolint:testpackage
package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

func setTestGroupScopeModelConfigs(
	t *testing.T,
	groupID string,
	configs ...model.ModelConfig,
) {
	t.Helper()

	cache := &model.GroupScopeModelConfigsCache{
		GroupID: groupID,
		Models:  make([]string, 0, len(configs)),
		Configs: make(map[string]model.ModelConfig, len(configs)),
		List:    make([]model.GroupScopeModelConfig, 0, len(configs)),
	}
	for _, config := range configs {
		cache.Models = append(cache.Models, config.Model)
		cache.Configs[config.Model] = config
		cache.List = append(cache.List, model.GroupScopeModelConfig{
			GroupID:     groupID,
			ModelConfig: config,
		})
	}

	require.NoError(t, model.CacheSetGroupScopeModelConfigs(cache))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupScopeModelConfig(groupID))
	})
}

type testModelConfigCache map[string]model.ModelConfig

func (c testModelConfigCache) GetModelConfig(modelName string) (model.ModelConfig, bool) {
	config, ok := c[modelName]
	return config, ok
}

func newRelayModelTestContext() (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.Group, model.GroupCache{
		ID:            "group-1",
		Status:        model.GroupStatusEnabled,
		AvailableSets: []string{model.ChannelDefaultSet},
	})

	availableModels := map[string][]string{
		model.ChannelDefaultSet: {"public-model"},
	}

	c.Set(middleware.Token, model.TokenCache{})
	c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
	c.Set(middleware.AvailableModels, availableModels)
	c.Set(middleware.ModelCaches, &model.ModelCaches{
		ModelConfig: testModelConfigCache{},
	})
	c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeOwn)

	return recorder, c
}

func newRelayModelGlobalTestContext() (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.Group, model.GroupCache{
		ID:            "group-1",
		Status:        model.GroupStatusEnabled,
		AvailableSets: []string{model.ChannelDefaultSet},
	})

	availableModels := map[string][]string{
		model.ChannelDefaultSet: {"public-model"},
	}

	c.Set(middleware.Token, model.TokenCache{})
	c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
	c.Set(middleware.AvailableModels, availableModels)
	c.Set(middleware.ModelCaches, &model.ModelCaches{
		ModelConfig: testModelConfigCache{},
	})
	c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeGlobal)

	return recorder, c
}

func TestListModelsIncludesGroupChannelOnlyModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setTestGroupScopeModelConfigs(t, "group-1", model.ModelConfig{
		Model: "public-model",
		Type:  mode.ChatCompletions,
	})

	recorder, c := newRelayModelTestContext()
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/models", nil)

	ListModels(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Object string         `json:"object"`
		Data   []OpenAIModels `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "list", response.Object)
	require.Len(t, response.Data, 1)
	require.Equal(t, "public-model", response.Data[0].ID)
	require.Equal(t, "public-model", response.Data[0].Root)
}

func TestRetrieveModelIncludesGroupChannelOnlyModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setTestGroupScopeModelConfigs(t, "group-1", model.ModelConfig{
		Model: "public-model",
		Type:  mode.ChatCompletions,
	})

	recorder, c := newRelayModelTestContext()
	c.Params = gin.Params{{Key: "model", Value: "public-model"}}
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/models/public-model",
		nil,
	)

	RetrieveModel(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response OpenAIModels
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "public-model", response.ID)
	require.Equal(t, "public-model", response.Root)
}

func TestRetrieveModelReturnsCanonicalModelName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setTestGroupScopeModelConfigs(t, "group-1", model.ModelConfig{
		Model: "Public-Model",
		Type:  mode.ChatCompletions,
	})

	recorder, c := newRelayModelTestContext()
	c.Set(middleware.AvailableModels, map[string][]string{
		model.ChannelDefaultSet: {"Public-Model"},
	})
	c.Params = gin.Params{{Key: "model", Value: "public-model"}}
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/models/public-model",
		nil,
	)

	RetrieveModel(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response OpenAIModels
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "Public-Model", response.ID)
	require.Equal(t, "Public-Model", response.Root)
}

func TestRetrieveModelGroupChannelModeUsesGroupChannelModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setTestGroupScopeModelConfigs(t, "group-1", model.ModelConfig{
		Model: "group-model",
		Type:  mode.ChatCompletions,
	})

	recorder, c := newRelayModelTestContext()
	c.Set(middleware.Token, model.TokenCache{
		Models:             []string{"global-model"},
		GroupChannelModels: []string{"group-model"},
	})
	c.Set(middleware.GroupChannelAvailableSets, []string{model.ChannelDefaultSet})
	c.Set(middleware.GroupChannelAvailableModels, map[string][]string{
		model.ChannelDefaultSet: {"group-model"},
	})
	c.Params = gin.Params{{Key: "model", Value: "group-model"}}
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/models/group-model",
		nil,
	)

	RetrieveModel(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response OpenAIModels
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "group-model", response.ID)
}

func TestRetrieveModelGlobalModeUsesGlobalModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder, c := newRelayModelGlobalTestContext()
	c.Set(middleware.Token, model.TokenCache{
		Models:             []string{"public-model"},
		GroupChannelModels: []string{"group-model"},
	})
	c.Set(middleware.ModelCaches, &model.ModelCaches{
		ModelConfig: testModelConfigCache{
			"public-model": {Model: "public-model", Type: mode.ChatCompletions},
		},
	})
	c.Params = gin.Params{{Key: "model", Value: "group-model"}}
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/models/group-model",
		nil,
	)

	RetrieveModel(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestRetrieveModelDefaultGlobalRequiresGlobalModelConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder, c := newRelayModelGlobalTestContext()
	c.Params = gin.Params{{Key: "model", Value: "public-model"}}
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/models/public-model",
		nil,
	)

	RetrieveModel(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestResolveModelConfigGroupOnlyUsesGroupScopeModelConfig(t *testing.T) {
	t.Parallel()
	setTestGroupScopeModelConfigs(t, "group-resolve", model.ModelConfig{
		Model:      "public-model",
		Type:       mode.ChatCompletions,
		RetryTimes: 5,
		RPM:        60,
		TPM:        120,
	})

	got, ok := middleware.ResolveModelConfig(
		model.GroupCache{
			ID:       "group-resolve",
			RPMRatio: 2,
			TPMRatio: 3,
			ModelConfigs: map[string]model.GroupModelConfig{
				"public-model": {
					Model:              "public-model",
					OverrideRetryTimes: true,
					RetryTimes:         5,
				},
			},
		},
		middleware.GroupChannelModeOwn,
		&model.ModelCaches{
			ModelConfig: testModelConfigCache{
				"public-model": {
					Model:      "public-model",
					Type:       mode.GeminiImage,
					RetryTimes: 9,
				},
			},
		},
		"public-model",
	)

	require.True(t, ok)
	require.Equal(t, "public-model", got.Model)
	require.Equal(t, mode.ChatCompletions, got.Type)
	require.Equal(t, int64(5), got.RetryTimes)
	require.Equal(t, int64(120), got.RPM)
	require.Equal(t, int64(360), got.TPM)
}

func TestGroupChannelTestModelConfigUsesGroupScopeModelConfig(t *testing.T) {
	t.Parallel()
	setTestGroupScopeModelConfigs(t, "group-test-config", model.ModelConfig{
		Model:      "public-model",
		Type:       mode.Anthropic,
		RetryTimes: 5,
		RPM:        60,
		TPM:        120,
	})

	got, err := groupChannelTestModelConfig(
		model.GroupCache{
			ID:       "group-test-config",
			RPMRatio: 2,
			TPMRatio: 3,
			ModelConfigs: map[string]model.GroupModelConfig{
				"public-model": {
					Model:              "public-model",
					OverrideRetryTimes: true,
					RetryTimes:         5,
					OverrideLimit:      true,
					RPM:                60,
					TPM:                120,
				},
			},
		},
		"public-model",
	)

	require.NoError(t, err)
	require.Equal(t, "public-model", got.Model)
	require.Equal(t, mode.Anthropic, got.Type)
	require.Equal(t, int64(5), got.RetryTimes)
	require.Equal(t, int64(120), got.RPM)
	require.Equal(t, int64(360), got.TPM)
	require.Empty(t, got.Plugin)
	require.Empty(t, got.Config)
}

func TestGroupChannelTestableModelsUsesChannelModelsWhenModelConfigDisabled(t *testing.T) {
	oldDisableModelConfig := config.DisableModelConfig
	config.DisableModelConfig = true
	t.Cleanup(func() {
		config.DisableModelConfig = oldDisableModelConfig
	})

	got, err := groupChannelTestableModels("group-disabled-model-config", &model.GroupChannel{
		Models: []string{"z-model", "a-model"},
		ModelMapping: map[string]string{
			"mapped-model": "actual-model",
			"a-model":      "actual-a-model",
		},
	})

	require.NoError(t, err)
	require.True(t, slices.IsSorted(got))
	require.Equal(t, []string{"a-model", "mapped-model", "z-model"}, got)
}

func TestTestRequestModelConfigGuessesTypeWithoutDroppingOverrides(t *testing.T) {
	t.Parallel()

	got := testRequestModelConfig(model.ModelConfig{
		Model:      "claude-sonnet-4-5",
		RetryTimes: 5,
		RPM:        60,
	})

	require.Equal(t, "claude-sonnet-4-5", got.Model)
	require.Equal(t, mode.ChatCompletions, got.Type)
	require.Equal(t, int64(5), got.RetryTimes)
	require.Equal(t, int64(60), got.RPM)
}
