package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
)

var dashboardLimiter = newTokenRateLimiter(10, 3)

// DashboardRateLimit is a middleware that limits dashboard log query requests per token.
func DashboardRateLimit(c *gin.Context) {
	token := middleware.GetToken(c)

	if !dashboardLimiter.Allow(token.ID) {
		middleware.ErrorResponse(c, http.StatusTooManyRequests, "rate limit exceeded for dashboard queries")
		c.Abort()

		return
	}

	c.Next()
}
