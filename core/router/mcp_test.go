package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	aiproxyrouter "github.com/labring/aiproxy/core/router"
	"github.com/stretchr/testify/require"
)

// The public MCP test endpoint must only be reachable through the admin API
// route; the unauthenticated root mount was removed.
func TestPublicMCPTestRouteOnlyExposedViaAdminAPI(t *testing.T) {
	previousAdminKey := config.AdminKey
	config.AdminKey = ""
	t.Cleanup(func() { config.AdminKey = previousAdminKey })

	previousMode := gin.Mode()

	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	engine := gin.New()
	aiproxyrouter.SetMCPRouter(engine)
	aiproxyrouter.SetAPIRouter(engine)

	rootResponse := httptest.NewRecorder()
	engine.ServeHTTP(
		rootResponse,
		httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/test-publicmcp/default/1/sse",
			nil,
		),
	)
	require.Equal(t, http.StatusNotFound, rootResponse.Code)

	apiResponse := httptest.NewRecorder()
	engine.ServeHTTP(
		apiResponse,
		httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/test-publicmcp/default/1/sse",
			nil,
		),
	)
	require.Equal(t, http.StatusUnauthorized, apiResponse.Code)
}
