package config_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/stretchr/testify/require"
)

func TestDefaultTrustedProxiesAreValidCIDRs(t *testing.T) {
	for _, cidr := range config.DefaultTrustedProxies {
		_, _, err := net.ParseCIDR(cidr)
		require.NoError(t, err, cidr)
	}
}

func clientIP(t *testing.T, remoteAddr, forwardedFor string) string {
	t.Helper()

	gin.SetMode(gin.TestMode)

	server := gin.New()
	require.NoError(t, server.SetTrustedProxies(config.DefaultTrustedProxies))

	var clientIP string
	server.GET("/", func(c *gin.Context) { clientIP = c.ClientIP() })

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.RemoteAddr = remoteAddr

	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}

	server.ServeHTTP(httptest.NewRecorder(), request)

	return clientIP
}

func TestDefaultTrustedProxiesResolveForwardedForFromPrivateProxy(t *testing.T) {
	// Gateway in the pod network appends the public client IP; the forged
	// leftmost entry is skipped.
	ip := clientIP(t, "10.244.0.7:443", "6.6.6.6, 203.0.113.5")
	require.Equal(t, "203.0.113.5", ip)
}

func TestDefaultTrustedProxiesIgnoreForwardedForFromPublicClient(t *testing.T) {
	// A public direct client cannot pose as a proxy.
	ip := clientIP(t, "203.0.113.9:443", "6.6.6.6")
	require.Equal(t, "203.0.113.9", ip)
}
