package router_test

import (
	"testing"

	"github.com/gin-gonic/gin"
	corerouter "github.com/labring/aiproxy/core/router"
	"github.com/stretchr/testify/require"
)

func TestSetRelayRouterRegistersGeminiModelScopedOperationRoutes(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	corerouter.SetRelayRouter(router)

	registered := map[string]bool{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	require.True(t, registered["GET /v1/models/:model/operations/*operation_id"])
	require.True(t, registered["GET /v1beta/models/:model/operations/*operation_id"])
	require.True(t, registered["GET /v1beta/files/*model"])
	require.True(t, registered["POST /v1/responses/compact"])
	require.True(t, registered["POST /v1/alpha/search"])
	require.True(t, registered["POST /v1/decisions"])
}
