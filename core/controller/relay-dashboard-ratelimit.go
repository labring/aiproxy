package controller

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"golang.org/x/time/rate"
)

type tokenLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64 // Unix nano timestamp
}

var dashboardLimiters sync.Map

const (
	dashboardRPM   = 10
	dashboardBurst = 3
	limiterTTL     = 10 * time.Minute
)

func init() {
	go cleanupDashboardLimiters()
}

func cleanupDashboardLimiters() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		threshold := time.Now().Add(-limiterTTL).UnixNano()
		dashboardLimiters.Range(func(key, value any) bool {
			entry := value.(*tokenLimiterEntry)
			if entry.lastSeen.Load() < threshold {
				dashboardLimiters.Delete(key)
			}

			return true
		})
	}
}

func getDashboardLimiter(tokenID int) *rate.Limiter {
	entry := &tokenLimiterEntry{
		limiter: rate.NewLimiter(rate.Limit(float64(dashboardRPM)/60.0), dashboardBurst),
	}
	entry.lastSeen.Store(time.Now().UnixNano())

	if actual, loaded := dashboardLimiters.LoadOrStore(tokenID, entry); loaded {
		existing := actual.(*tokenLimiterEntry)
		existing.lastSeen.Store(time.Now().UnixNano())

		return existing.limiter
	}

	return entry.limiter
}

// DashboardRateLimit is a middleware that limits dashboard log query requests per token.
func DashboardRateLimit(c *gin.Context) {
	token := middleware.GetToken(c)
	limiter := getDashboardLimiter(token.ID)

	if !limiter.Allow() {
		middleware.ErrorResponse(c, http.StatusTooManyRequests, "rate limit exceeded for dashboard queries")
		c.Abort()

		return
	}

	c.Next()
}
