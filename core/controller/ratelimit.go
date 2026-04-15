package controller

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

const limiterTTL = 10 * time.Minute

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

type tokenRateLimiter struct {
	limiters sync.Map
	rpm      int
	burst    int
}

func newTokenRateLimiter(rpm, burst int) *tokenRateLimiter {
	rl := &tokenRateLimiter{rpm: rpm, burst: burst}
	go rl.cleanup()

	return rl
}

func (rl *tokenRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		threshold := time.Now().Add(-limiterTTL).UnixNano()
		rl.limiters.Range(func(key, value any) bool {
			entry := value.(*limiterEntry)
			if entry.lastSeen.Load() < threshold {
				rl.limiters.Delete(key)
			}

			return true
		})
	}
}

func (rl *tokenRateLimiter) Allow(tokenID int) bool {
	now := time.Now().UnixNano()

	if actual, ok := rl.limiters.Load(tokenID); ok {
		entry := actual.(*limiterEntry)
		entry.lastSeen.Store(now)

		return entry.limiter.Allow()
	}

	entry := &limiterEntry{
		limiter: rate.NewLimiter(rate.Limit(float64(rl.rpm)/60.0), rl.burst),
	}
	entry.lastSeen.Store(now)

	if actual, loaded := rl.limiters.LoadOrStore(tokenID, entry); loaded {
		existing := actual.(*limiterEntry)
		existing.lastSeen.Store(now)

		return existing.limiter.Allow()
	}

	return entry.limiter.Allow()
}
