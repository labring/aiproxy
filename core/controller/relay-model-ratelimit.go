package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
)

var modelsLimiter = newTokenRateLimiter(30, 5)

// ModelsRateLimit is a middleware that limits /v1/models requests per token.
func ModelsRateLimit(c *gin.Context) {
	token := middleware.GetToken(c)

	if !modelsLimiter.Allow(token.ID) {
		middleware.ErrorResponse(c, http.StatusTooManyRequests, "rate limit exceeded for models queries")
		c.Abort()

		return
	}

	c.Next()
}
