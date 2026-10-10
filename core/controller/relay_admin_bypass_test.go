//nolint:testpackage
package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

// Exercise distribution, pinned channel selection, and relay preparation together.
func TestAdminBypassChannelModelCheckRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withTestStoreDB(t, func() {
		for _, tt := range []struct {
			name        string
			env         string
			own         bool
			regular     bool
			noHeader    bool
			configured  bool
			wrongMode   bool
			disabled    bool
			otherGroup  bool
			invalidID   bool
			unsupported bool
			wantStage   string
		}{
			{name: "global unlisted model", env: "true"},
			{name: "global disabled channel", env: "true", disabled: true},
			{name: "global flag disabled", env: "false", wantStage: "distribution"},
			{name: "global flag unset", wantStage: "distribution"},
			{name: "global invalid flag", env: "invalid", wantStage: "distribution"},
			{name: "global ordinary token", env: "true", regular: true, wantStage: "distribution"},
			{name: "global ordinary token cannot pin", env: "true", regular: true, configured: true, wantStage: "selection"},
			{name: "global no pinned header", env: "true", noHeader: true, wantStage: "distribution"},
			{name: "global missing channel", env: "true", invalidID: true, wantStage: "selection"},
			{name: "global unsupported adaptor mode", env: "true", unsupported: true, wantStage: "selection"},
			{name: "global configured mode remains enforced", env: "true", configured: true, wrongMode: true, wantStage: "preparation"},
			{name: "own unlisted model", env: "true", own: true},
			{name: "own listed model without flag", own: true, configured: true},
			{name: "own flag disabled", env: "false", own: true, wantStage: "distribution"},
			{name: "own ordinary token", env: "true", own: true, regular: true, wantStage: "distribution"},
			{name: "own ordinary token listed model", env: "true", own: true, regular: true, configured: true},
			{name: "own no pinned header", env: "true", own: true, noHeader: true, wantStage: "distribution"},
			{name: "own disabled channel", env: "true", own: true, disabled: true, wantStage: "selection"},
			{name: "own other group channel", env: "true", own: true, otherGroup: true, wantStage: "selection"},
			{name: "own missing channel", env: "true", own: true, invalidID: true, wantStage: "selection"},
			{name: "own unsupported adaptor mode", env: "true", own: true, unsupported: true, wantStage: "selection"},
			{name: "own configured mode remains enforced", env: "true", own: true, configured: true, wrongMode: true, wantStage: "preparation"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Cleanup(config.ReloadEnv)
				t.Setenv("ENABLE_ADMIN_BYPASS_CHANNEL_MODEL_CHECK", tt.env)
				t.Setenv("DISABLE_MODEL_CONFIG", "false")
				config.ReloadEnv()
				require.Equal(t, tt.env == "true", config.EnableAdminBypassChannelModelCheck)

				const modelName = "uncatalogued-embedding-model"

				groupID := t.Name()

				requestMode := mode.Embeddings
				if tt.unsupported {
					requestMode = mode.Mode(-1)
				}

				group := model.GroupCache{ID: groupID, Status: model.GroupStatusInternal}
				if tt.regular {
					group.Status = model.GroupStatusEnabled
				}

				channel := &model.Channel{
					ID:     42,
					Type:   model.ChannelTypeOpenAI,
					Status: model.ChannelStatusEnabled,
				}
				if tt.disabled {
					channel.Status = model.ChannelStatusDisabled
				}

				configs := testModelConfigCache{}
				availableModels := map[string][]string{}

				var groupConfigs []model.ModelConfig
				if tt.configured {
					mc := model.NewDefaultModelConfig(modelName)

					mc.Type = mode.Embeddings
					if tt.wrongMode {
						mc.Type = mode.ChatCompletions
					}

					configs[modelName] = mc
					groupConfigs = append(groupConfigs, mc)
					channel.Models = []string{modelName}
					availableModels[model.ChannelDefaultSet] = []string{modelName}
				}

				setTestGroupScopeModelConfigs(t, groupID, groupConfigs...)

				groupMode := middleware.GroupChannelModeGlobal
				if tt.own {
					groupMode = middleware.GroupChannelModeOwn

					owner := groupID
					if tt.otherGroup {
						owner += "-other"
					}

					gc := &model.GroupChannel{
						GroupID: owner,
						Type:    channel.Type,
						Status:  channel.Status,
						Models:  channel.Models,
					}
					require.NoError(t, model.DB.Create(gc).Error)
					channel.ID = gc.ID
				}

				caches := &model.ModelCaches{
					ChannelsByID: map[int]*model.Channel{channel.ID: channel}, ModelConfig: configs,
				}
				recorder := httptest.NewRecorder()
				stage := "distribution"
				router := gin.New()
				router.POST("/v1/embeddings", func(c *gin.Context) {
					c.Set(middleware.Group, group)
					c.Set(middleware.Token, model.TokenCache{})
					c.Set(middleware.ModelCaches, caches)
					c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
					c.Set(middleware.AvailableModels, availableModels)
					c.Set(middleware.GroupChannelMode, groupMode)
					c.Set(middleware.GroupBalance, &middleware.GroupBalanceConsumer{
						CheckBalance: func(float64) bool { return true },
					})
				}, middleware.NewDistribute(requestMode), func(c *gin.Context) {
					stage = "selection"

					initial, err := getInitialChannel(c, middleware.GetRequestModel(c), requestMode)
					if err != nil {
						c.AbortWithStatus(http.StatusForbidden)
						return
					}

					stage = "preparation"

					attempt, err := prepareRelayAttempt(
						c,
						requestMode,
						RelayController{},
						initial.channel,
						caches,
						modelName,
						false,
					)
					if err != nil {
						c.AbortWithStatus(http.StatusForbidden)
						return
					}

					require.Equal(t, modelName, attempt.modelConfig.Model)
					require.Equal(t, requestMode, attempt.modelConfig.Type)
					require.Equal(t, tt.own, initial.channel.isGroupChannel())

					if tt.own {
						require.Equal(t, groupID, attempt.meta.Channel.GroupID)
					}

					stage = ""

					c.Status(http.StatusOK)
				})

				request := httptest.NewRequestWithContext(
					t.Context(),
					http.MethodPost,
					"/v1/embeddings",
					strings.NewReader(`{"model":"`+modelName+`","input":"hello"}`),
				)
				request.Header.Set("Content-Type", "application/json")

				if !tt.noHeader {
					request.Header.Set(AIProxyChannelHeader, strconv.Itoa(channel.ID))

					if tt.invalidID {
						request.Header.Set(AIProxyChannelHeader, "999999")
					}
				}

				router.ServeHTTP(recorder, request)
				require.Equal(t, tt.wantStage, stage, recorder.Body.String())

				if tt.wantStage == "" {
					require.Equal(t, http.StatusOK, recorder.Code)
				} else {
					require.GreaterOrEqual(t, recorder.Code, http.StatusBadRequest)
				}
			})
		}
	})
}
