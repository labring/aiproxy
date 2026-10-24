//nolint:testpackage
package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

func TestDecisionsDistributeExtractsPromptCacheKey(t *testing.T) {
	_, c := newRelayModelGlobalTestContext()
	c.Set(
		middleware.Group,
		model.GroupCache{ID: "decisions-request", Status: model.GroupStatusInternal},
	)
	c.Set(middleware.ModelCaches, &model.ModelCaches{
		ModelConfig: testModelConfigCache{
			"public-model": {Model: "public-model", Type: mode.Decisions},
		},
	})
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/decisions",
		strings.NewReader(
			`{"model":"public-model","input":"hello","questions":[{"type":"predicate","instructions":"Is this a greeting?"}],"prompt_cache_key":"decisions-key"}`,
		),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	middleware.NewDistribute(mode.Decisions)(c)

	require.False(t, c.IsAborted())
	require.Equal(t, "public-model", middleware.GetRequestModel(c))
	require.Equal(t, "decisions-key", middleware.GetPromptCacheKey(c))
}

func TestDecisionsCacheFollowUsesActiveScope(t *testing.T) {
	withTestStoreDB(t, func() {
		const (
			groupID   = "decisions-preference"
			modelName = "decision-model"
		)

		mc := model.ModelConfig{
			Model:  modelName,
			Type:   mode.Decisions,
			Plugin: map[string]map[string]any{"cachefollow": {"enable": true}},
		}
		setTestGroupScopeModelConfigs(t, groupID, mc)

		storeID := model.PromptCacheStoreID(modelName, "decisions-key", model.CacheKeyTypeStable)
		for _, scope := range []model.ChannelScope{model.ChannelScopeGlobal, model.ChannelScopeGroup} {
			_, err := model.SaveStoreByScope(&model.StoreV2{
				ID: storeID, GroupID: groupID, TokenID: 8, ChannelID: 31, Model: modelName,
			}, scope)
			require.NoError(t, err)
		}

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(middleware.Group, model.GroupCache{ID: groupID})
		c.Set(middleware.Token, model.TokenCache{ID: 8})
		c.Set(middleware.PromptCacheKey, "decisions-key")
		setTestCacheFollowModelConfig(c, mc)
		require.Equal(t, []string{"31"}, getPreferChannelKeys(c, modelName, mode.Decisions))

		c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeOwn)
		require.Equal(
			t,
			[]string{model.GroupChannelMonitorKey(groupID, 31)},
			getPreferChannelKeys(c, modelName, mode.Decisions),
		)
	})
}
