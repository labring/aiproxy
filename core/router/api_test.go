package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/model"
	aiproxyrouter "github.com/labring/aiproxy/core/router"
	"github.com/stretchr/testify/require"
)

func TestChannelModelsRouteRequiresAdminAuth(t *testing.T) {
	previousAdminKey := config.AdminKey
	config.AdminKey = ""
	t.Cleanup(func() { config.AdminKey = previousAdminKey })

	previousMode := gin.Mode()

	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	router := gin.New()
	aiproxyrouter.SetAPIRouter(router)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/channel/1/models",
		nil,
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "unauthorized")
}

func TestGroupChannelModelsRoute(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "group-channels.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.GroupChannel{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)

	previousDB, previousAdminKey, previousMode := model.DB, config.AdminKey, gin.Mode()
	model.DB = db
	config.AdminKey = "catalog-test-admin"

	gin.SetMode(gin.TestMode)
	t.Cleanup(func() {
		model.DB = previousDB
		config.AdminKey = previousAdminKey

		gin.SetMode(previousMode)
		require.NoError(t, sqlDB.Close())
	})

	const providerKey = "catalog-test-provider"

	var requests atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)

		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" ||
			r.Header.Get("Authorization") != "Bearer "+providerKey {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_, _ = w.Write([]byte(`{"data":[{"id":"upstream-only-model"}]}`))
	}))
	t.Cleanup(upstream.Close)

	channel := &model.GroupChannel{
		GroupID: "fastgpt:tmb:catalog-owner",
		Type:    model.ChannelTypeOpenAI,
		Key:     providerKey,
		BaseURL: upstream.URL + "/v1",
		Models:  []string{"configured-model"},
		ModelMapping: map[string]string{
			"configured-model": "provider/configured-model",
		},
		Name:   "catalog test",
		Status: model.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(channel).Error)

	engine := gin.New()
	aiproxyrouter.SetAPIRouter(engine)

	channelPath := func(group string) string {
		return "/api/group/" + group + "/channel/" + strconv.Itoa(channel.ID)
	}
	get := func(path, adminKey string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if adminKey != "" {
			request.Header.Set("Authorization", "Bearer "+adminKey)
		}

		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		return response
	}

	ownerPath := channelPath("fastgpt%3Atmb%3Acatalog-owner") + "/models"
	foreignPath := channelPath("other-member") + "/models"

	require.Equal(t, http.StatusUnauthorized, get(ownerPath, "").Code)
	require.Equal(t, http.StatusNotFound, get(foreignPath, config.AdminKey).Code)
	require.Zero(t, requests.Load())

	response := get(ownerPath, config.AdminKey)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	var result struct {
		Success bool `json:"success"`
		Data    []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success)
	require.Len(t, result.Data, 1)
	require.Equal(t, "upstream-only-model", result.Data[0].ID)
	require.NotContains(t, response.Body.String(), providerKey)
	require.EqualValues(t, 1, requests.Load())
}

func TestSwaggerDocumentsChannelModels(t *testing.T) {
	previousMode := gin.Mode()

	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	engine := gin.New()
	aiproxyrouter.SetSwaggerRouter(engine)

	response := httptest.NewRecorder()
	engine.ServeHTTP(
		response,
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/doc.json", nil),
	)

	require.Equal(t, http.StatusOK, response.Code)

	var document struct {
		Definitions map[string]json.RawMessage `json:"definitions"`
		Paths       map[string]struct {
			Get struct {
				Security  []map[string][]string      `json:"security"`
				Responses map[string]json.RawMessage `json:"responses"`
			} `json:"get"`
		} `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &document))

	var channelModel struct {
		Required []string `json:"required"`
	}
	require.NoError(
		t,
		json.Unmarshal(document.Definitions["controller.ChannelModel"], &channelModel),
	)
	require.ElementsMatch(t, []string{"id"}, channelModel.Required)

	path, ok := document.Paths["/api/channel/{id}/models"]
	require.True(t, ok)
	require.Equal(t, []map[string][]string{{"ApiKeyAuth": {}}}, path.Get.Security)

	for _, status := range []string{"200", "400", "401", "404", "422", "500", "502", "504"} {
		require.Contains(t, path.Get.Responses, status)
	}

	require.Contains(
		t,
		string(path.Get.Responses["200"]),
		"#/definitions/middleware.APIResponse",
	)

	groupPath, ok := document.Paths["/api/group/{group}/channel/{id}/models"]
	require.True(t, ok)
	require.Equal(t, path.Get.Security, groupPath.Get.Security)

	for _, status := range []string{"200", "400", "401", "404", "500", "501", "502", "504"} {
		require.Contains(t, groupPath.Get.Responses, status)
	}
}
