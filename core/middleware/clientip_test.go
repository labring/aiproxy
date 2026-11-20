//nolint:testpackage
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/stretchr/testify/require"
)

func requestWithClientIP(t *testing.T, remoteAddr string, build func(*http.Request)) string {
	t.Helper()

	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	c.Request.RemoteAddr = remoteAddr
	build(c.Request)

	GuardForwardedHeaders()(c)

	return c.ClientIP()
}

func TestGuardForwardedHeadersSecretUnsetKeepsHeaders(t *testing.T) {
	config.GatewaySecret = ""
	t.Cleanup(func() { config.GatewaySecret = "" })

	// Trust-all default picks the leftmost, client-controlled entry.
	ip := requestWithClientIP(t, "10.0.0.5:1234", func(req *http.Request) {
		req.Header.Set("X-Forwarded-For", "6.6.6.6, 7.7.7.7")
	})
	require.Equal(t, "6.6.6.6", ip)
}

func TestGuardForwardedHeadersRejectsWrongSecret(t *testing.T) {
	config.GatewaySecret = "s3cret"
	t.Cleanup(func() { config.GatewaySecret = "" })

	for _, secret := range []string{"", "wrong"} {
		ip := requestWithClientIP(t, "203.0.113.9:1234", func(req *http.Request) {
			req.Header.Set("X-Gateway-Secret", secret)
			req.Header.Set("X-Forwarded-For", "6.6.6.6, 7.7.7.7")
			req.Header.Set("X-Real-IP", "6.6.6.6")
		})
		require.Equal(t, "203.0.113.9", ip, "secret %q", secret)
	}
}

func TestGuardForwardedHeadersKeepsGatewayAppendedIP(t *testing.T) {
	config.GatewaySecret = "s3cret"
	t.Cleanup(func() { config.GatewaySecret = "" })

	ip := requestWithClientIP(t, "10.0.0.5:1234", func(req *http.Request) {
		req.Header.Set("X-Gateway-Secret", "s3cret")
		req.Header.Set("X-Forwarded-For", "6.6.6.6, 7.7.7.7")
	})
	require.Equal(t, "7.7.7.7", ip)
}

func TestGuardForwardedHeadersMultipleHeaderLines(t *testing.T) {
	config.GatewaySecret = "s3cret"
	t.Cleanup(func() { config.GatewaySecret = "" })

	ip := requestWithClientIP(t, "10.0.0.5:1234", func(req *http.Request) {
		req.Header.Set("X-Gateway-Secret", "s3cret")
		req.Header.Add("X-Forwarded-For", "6.6.6.6")
		req.Header.Add("X-Forwarded-For", "8.8.4.4")
	})
	require.Equal(t, "8.8.4.4", ip)
}

func TestGuardForwardedHeadersDropsMalformedForwardedFor(t *testing.T) {
	config.GatewaySecret = "s3cret"
	t.Cleanup(func() { config.GatewaySecret = "" })

	ip := requestWithClientIP(t, "10.0.0.5:1234", func(req *http.Request) {
		req.Header.Set("X-Gateway-Secret", "s3cret")
		req.Header.Set("X-Forwarded-For", "7.7.7.7, not-an-ip")
		req.Header.Set("X-Real-IP", "6.6.6.6")
	})
	require.Equal(t, "10.0.0.5", ip)
}

func TestGuardForwardedHeadersKeepsXRealIPWhenForwardedForAbsent(t *testing.T) {
	config.GatewaySecret = "s3cret"
	t.Cleanup(func() { config.GatewaySecret = "" })

	ip := requestWithClientIP(t, "10.0.0.5:1234", func(req *http.Request) {
		req.Header.Set("X-Gateway-Secret", "s3cret")
		req.Header.Set("X-Real-IP", "7.7.7.7")
	})
	require.Equal(t, "7.7.7.7", ip)
}
