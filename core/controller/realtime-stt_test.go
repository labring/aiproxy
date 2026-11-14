package controller_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller"
	"github.com/stretchr/testify/assert"
)

func TestRealtimeSTTBrowserAuth(t *testing.T) {
	for _, tt := range []struct {
		name, protocol, authorization string
		upgrade                       bool
		status                        int
		want                          string
	}{
		{name: "browser key", protocol: "aiproxy-realtime, aiproxy-api-key.fixture", upgrade: true, status: 200, want: "Bearer fixture"},
		{name: "bearer client", authorization: "Bearer header-fixture", upgrade: true, status: 200, want: "Bearer header-fixture"},
		{name: "preserve bearer", protocol: "aiproxy-api-key.other", authorization: "Bearer header-fixture", upgrade: true, status: 200, want: "Bearer header-fixture"},
		{name: "duplicate keys", protocol: "aiproxy-api-key.one, aiproxy-api-key.two", upgrade: true, status: 401},
		{name: "empty key", protocol: "aiproxy-api-key.", upgrade: true, status: 401},
		{name: "not websocket", status: 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.GET("/", controller.RealtimeSTTBrowserAuth, func(c *gin.Context) {
				assert.Equal(t, tt.want, c.GetHeader("Authorization"))
				assert.Equal(t, "aiproxy-realtime", c.GetHeader("Sec-WebSocket-Protocol"))
				c.Status(http.StatusOK)
			})

			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.upgrade {
				request.Header.Set("Connection", "Upgrade")
				request.Header.Set("Upgrade", "websocket")
			}

			request.Header.Set("Authorization", tt.authorization)
			request.Header.Set("Sec-WebSocket-Protocol", tt.protocol)

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, tt.status, recorder.Code)
		})
	}
}
