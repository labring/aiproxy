//nolint:testpackage
package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

func TestGroupChannelPreviewRequestOverridesReachUpstream(t *testing.T) {
	const groupID = "preview-overrides"

	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedis })
	require.NoError(t, model.CacheSetGroup(&model.GroupCache{ID: groupID}))
	t.Cleanup(func() { require.NoError(t, model.CacheDeleteGroup(groupID)) })
	setTestGroupScopeModelConfigs(t, groupID,
		model.ModelConfig{Model: "default-model", Type: mode.ChatCompletions},
		model.ModelConfig{Model: "override-model", Type: mode.ChatCompletions},
	)

	type receivedRequest struct {
		Path          string
		Body          map[string]any
		Authorization string
	}

	var (
		mu       sync.Mutex
		received []receivedRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode upstream request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()

		received = append(
			received,
			receivedRequest{
				Path:          r.URL.Path,
				Body:          body,
				Authorization: r.Header.Get("Authorization"),
			},
		)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(
				`{"id":"test","object":"chat.completion","choices":[{"index":0,"text":"ok","message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			),
		)
	}))
	t.Cleanup(server.Close)

	for _, tt := range []struct {
		name    string
		handler gin.HandlerFunc
		batch   bool
	}{
		{name: "group single", handler: TestGroupChannelPreview},
		{name: "global single", handler: TestGlobalGroupChannelPreview},
		{name: "group batch", handler: TestGroupChannelPreviewAll, batch: true},
		{name: "global batch", handler: TestGlobalGroupChannelPreviewAll, batch: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mu.Lock()
			received = nil
			mu.Unlock()

			payload := map[string]any{
				"group_id":     groupID,
				"name":         "preview",
				"type":         model.ChannelTypeOpenAI,
				"key":          "preview-key",
				"base_url":     server.URL + "/v1",
				"model":        "default-model",
				"models":       []string{"default-model", "override-model"},
				"mode":         mode.Completions,
				"request_body": map[string]any{"prompt": "custom prompt", "max_tokens": 7},
				"model_mapping": map[string]string{
					"default-model":  "upstream-default",
					"override-model": "upstream-override",
				},
				"model_overrides": map[string]any{
					"override-model": map[string]any{
						"mode": mode.ChatCompletions,
						"request_body": map[string]any{
							"messages": []map[string]string{
								{"role": "user", "content": "custom message"},
							},
							"temperature": 0.25,
						},
					},
				},
			}
			body, err := json.Marshal(payload)
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "group", Value: groupID}}
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/?return_success=true",
				bytes.NewReader(body),
			)
			c.Request.Header.Set("Content-Type", "application/json")
			tt.handler(c)
			require.Equal(t, http.StatusOK, recorder.Code)

			var response struct {
				Success bool            `json:"success"`
				Data    json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success, recorder.Body.String())

			want := 1
			if tt.batch {
				want = 2

				var results []struct {
					Success bool   `json:"success"`
					Message string `json:"message"`
					Data    *struct {
						Success  bool   `json:"success"`
						Response string `json:"response"`
						GroupID  string `json:"group_id"`
					} `json:"data"`
				}
				require.NoError(t, json.Unmarshal(response.Data, &results))
				require.Len(t, results, want)

				for _, result := range results {
					require.True(t, result.Success, result.Message)
					require.NotNil(t, result.Data)
					require.True(t, result.Data.Success, result.Data.Response)
					require.Equal(t, groupID, result.Data.GroupID)
				}
			} else {
				var result struct {
					Success bool      `json:"success"`
					GroupID string    `json:"group_id"`
					Mode    mode.Mode `json:"mode"`
				}
				require.NoError(t, json.Unmarshal(response.Data, &result))
				require.True(t, result.Success, recorder.Body.String())
				require.Equal(t, groupID, result.GroupID)
				require.Equal(t, mode.Completions, result.Mode)
			}

			mu.Lock()
			defer mu.Unlock()

			require.Len(t, received, want)

			for _, request := range received {
				require.Equal(t, "Bearer preview-key", request.Authorization)

				if request.Body["model"] == "upstream-default" {
					require.Equal(t, "/v1/completions", request.Path)
					require.Equal(t, "custom prompt", request.Body["prompt"])
					require.EqualValues(t, 7, request.Body["max_tokens"])
				} else {
					require.Equal(t, "upstream-override", request.Body["model"])
					require.Equal(t, "/v1/chat/completions", request.Path)
					require.EqualValues(t, 0.25, request.Body["temperature"])
					require.Equal(
						t,
						[]any{map[string]any{"role": "user", "content": "custom message"}},
						request.Body["messages"],
					)
				}
			}
		})
	}
}

func TestGroupChannelListsRejectInvalidFilters(t *testing.T) {
	for _, handler := range []gin.HandlerFunc{GetGroupChannels, GetGlobalGroupChannels, SearchGroupChannels, SearchGlobalGroupChannels} {
		for _, query := range []string{"?backup_only=invalid", "?backup_only=true&backup_only=false", "?remark=a&remark=b"} {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+query, nil)
			handler(c)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		}
	}
}

func TestResponsesCompactCacheFollowUsesActiveScope(t *testing.T) {
	withTestStoreDB(t, func() {
		const (
			groupID   = "compact-preference"
			modelName = "compact-model"
		)

		mc := model.ModelConfig{
			Model:  modelName,
			Type:   mode.Responses,
			Plugin: map[string]map[string]any{"cachefollow": {"enable": true}},
		}
		setTestGroupScopeModelConfigs(t, groupID, mc)

		storeID := model.PromptCacheStoreID(modelName, "compact-key", model.CacheKeyTypeStable)
		for _, scope := range []model.ChannelScope{model.ChannelScopeGlobal, model.ChannelScopeGroup} {
			_, err := model.SaveStoreByScope(
				&model.StoreV2{
					ID:        storeID,
					GroupID:   groupID,
					TokenID:   8,
					ChannelID: 31,
					Model:     modelName,
				},
				scope,
			)
			require.NoError(t, err)
		}

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(middleware.Group, model.GroupCache{ID: groupID})
		c.Set(middleware.Token, model.TokenCache{ID: 8})
		c.Set(middleware.PromptCacheKey, "compact-key")
		setTestCacheFollowModelConfig(c, mc)
		require.Equal(t, []string{"31"}, getPreferChannelKeys(c, modelName, mode.ResponsesCompact))
		c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeOwn)
		require.Equal(
			t,
			[]string{model.GroupChannelMonitorKey(groupID, 31)},
			getPreferChannelKeys(c, modelName, mode.ResponsesCompact),
		)
		require.NotNil(t, relayController(mode.ResponsesCompact).GetRequestUsage)
	})
}

func TestGroupChannelPatchResponsesContainUpdatedFields(t *testing.T) {
	withTestStoreDB(t, func() {
		for _, handler := range []gin.HandlerFunc{UpdateGroupChannel, UpdateGlobalGroupChannel} {
			channel := &model.GroupChannel{
				GroupID:    "patch-response",
				Type:       model.ChannelTypeOpenAI,
				Name:       "original",
				Key:        "test-key",
				BackupOnly: true,
				Remark:     "old remark",
			}
			require.NoError(t, model.DB.Create(channel).Error)
			t.Cleanup(
				func() { require.NoError(t, model.CacheDeleteGroupChannels(channel.GroupID)) },
			)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{
				{Key: "group", Value: channel.GroupID},
				{Key: "id", Value: strconv.Itoa(channel.ID)},
			}
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPut,
				"/",
				bytes.NewBufferString(`{"name":"updated","remark":"","backup_only":false}`),
			)
			c.Request.Header.Set("Content-Type", "application/json")
			handler(c)
			require.Equal(t, http.StatusOK, recorder.Code)

			var response struct {
				Success bool `json:"success"`
				Data    struct {
					Name       string `json:"name"`
					Remark     string `json:"remark"`
					BackupOnly bool   `json:"backup_only"`
					Key        string `json:"key"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success, recorder.Body.String())
			require.Equal(t, "updated", response.Data.Name)
			require.Empty(t, response.Data.Remark)
			require.False(t, response.Data.BackupOnly)
			require.Equal(t, "test-key", response.Data.Key)
		}
	})
}
