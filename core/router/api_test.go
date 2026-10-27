package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
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
	SetAPIRouter(router)
	request := httptest.NewRequest(http.MethodGet, "/api/channel/1/models", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "unauthorized")
}

func TestSwaggerDocumentsChannelModels(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	engine := gin.New()
	SetSwaggerRouter(engine)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/doc.json", nil))

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
	require.NoError(t, json.Unmarshal(document.Definitions["controller.ChannelModel"], &channelModel))
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
}
