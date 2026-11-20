package middleware

import (
	"crypto/subtle"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
)

// GatewaySecretHeader is set by the trusted gateway on every forwarded request.
const GatewaySecretHeader = "X-Gateway-Secret"

// GuardForwardedHeaders keeps spoofed X-Forwarded-For / X-Real-IP values away
// from gin's ClientIP when GATEWAY_SECRET is configured. Requests without the
// secret lose both headers, so the client IP falls back to the TCP peer
// address; requests with the secret keep only the rightmost X-Forwarded-For
// entry, the one the gateway appended, instead of the client-controlled
// leftmost entries.
func GuardForwardedHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.GatewaySecret == "" {
			c.Next()
			return
		}

		secret := c.Request.Header.Get(GatewaySecretHeader)
		if subtle.ConstantTimeCompare([]byte(secret), []byte(config.GatewaySecret)) != 1 {
			c.Request.Header.Del("X-Forwarded-For")
			c.Request.Header.Del("X-Real-IP")
			c.Next()
			return
		}

		forwarded := strings.Join(c.Request.Header.Values("X-Forwarded-For"), ",")
		if forwarded == "" {
			c.Next()
			return
		}

		parts := strings.Split(forwarded, ",")

		clientIP := strings.TrimSpace(parts[len(parts)-1])
		if net.ParseIP(clientIP) == nil {
			c.Request.Header.Del("X-Forwarded-For")
			c.Request.Header.Del("X-Real-IP")
			c.Next()
			return
		}

		c.Request.Header.Set("X-Forwarded-For", clientIP)
		c.Request.Header.Set("X-Real-IP", clientIP)

		c.Next()
	}
}
