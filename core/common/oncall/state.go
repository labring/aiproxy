package oncall

import (
	"context"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
)

const (
	// alertCooldown is the time after an alert before we can alert again for the same key
	alertCooldown = 30 * time.Minute
	// errorStateTTL is how long to keep error state in Redis
	errorStateTTL = 1 * time.Hour
	// alertedStateTTL is how long to keep alerted state in Redis
	alertedStateTTL = alertCooldown
)

// Memory storage for fallback
var (
	// errorTimes stores first seen time for each error key
	errorTimes = sync.Map{}
	// alertedTimes stores last alerted time for each key
	alertedTimes = sync.Map{}
)

func init() {
	go cleanupMemory()
}

// cleanupMemory periodically cleans up expired entries from memory
func cleanupMemory() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for now := range ticker.C {
		// Clean up old error times (older than errorStateTTL)
		errorTimes.Range(func(key, value any) bool {
			t, ok := value.(time.Time)
			if !ok || now.Sub(t) > errorStateTTL {
				errorTimes.CompareAndDelete(key, value)
			}
			return true
		})

		// Clean up old alerted times (older than alertCooldown)
		alertedTimes.Range(func(key, value any) bool {
			t, ok := value.(time.Time)
			if !ok || now.Sub(t) > alertCooldown {
				alertedTimes.CompareAndDelete(key, value)
			}
			return true
		})
	}
}

// redisKeyError returns the Redis key for error state
func redisKeyError(key string) string {
	return common.RedisKey("oncall:error:" + key)
}

// redisKeyAlerted returns the Redis key for alerted state
func redisKeyAlerted(key string) string {
	return common.RedisKey("oncall:alerted:" + key)
}

// RecordError records that an error occurred for the given key
// Returns the first seen time (either from existing state or now)
func RecordError(key string) time.Time {
	if !common.RedisEnabled {
		return recordErrorMemory(key)
	}

	t, err := recordErrorRedis(key)
	if err != nil {
		log.Errorf("oncall: failed to record error in Redis: %v, falling back to memory", err)
		return recordErrorMemory(key)
	}

	return t
}

func recordErrorRedis(key string) (time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	redisKey := redisKeyError(key)

	// Try to get existing value first
	val, err := common.RDB.Get(ctx, redisKey).Int64()
	if err == nil {
		return time.Unix(0, val), nil
	}

	// Set new value with NX (only if not exists)
	now := time.Now()
	nowNanos := now.UnixNano()
	set, err := common.RDB.SetNX(ctx, redisKey, nowNanos, errorStateTTL).Result()
	if err != nil {
		return time.Time{}, err
	}

	if set {
		return now, nil
	}

	// Key was set by another instance, get the value
	val, err = common.RDB.Get(ctx, redisKey).Int64()
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(0, val), nil
}

func recordErrorMemory(key string) time.Time {
	now := time.Now()
	actual, loaded := errorTimes.LoadOrStore(key, now)
	if loaded {
		if t, ok := actual.(time.Time); ok {
			return t
		}
		// Invalid type, replace it
		errorTimes.Store(key, now)
	}
	return now
}

// HasError checks if an error state exists for the given key
func HasError(key string) bool {
	if !common.RedisEnabled {
		return hasErrorMemory(key)
	}

	has, err := hasErrorRedis(key)
	if err != nil {
		log.Errorf("oncall: failed to check error state in Redis: %v, falling back to memory", err)
		return hasErrorMemory(key)
	}

	return has
}

func hasErrorRedis(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	exists, err := common.RDB.Exists(ctx, redisKeyError(key)).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}

func hasErrorMemory(key string) bool {
	_, exists := errorTimes.Load(key)
	return exists
}

// ClearError clears the error state for the given key
func ClearError(key string) {
	if common.RedisEnabled {
		if err := clearErrorRedis(key); err != nil {
			log.Errorf("oncall: failed to clear error state in Redis: %v", err)
		}
	}
	clearErrorMemory(key)
}

func clearErrorRedis(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return common.RDB.Del(ctx, redisKeyError(key)).Err()
}

func clearErrorMemory(key string) {
	errorTimes.Delete(key)
}

// HasAlerted checks if we've already alerted for the given key within the cooldown period
func HasAlerted(key string) bool {
	if !common.RedisEnabled {
		return hasAlertedMemory(key)
	}

	has, err := hasAlertedRedis(key)
	if err != nil {
		log.Errorf("oncall: failed to check alerted state in Redis: %v, falling back to memory", err)
		return hasAlertedMemory(key)
	}

	return has
}

func hasAlertedRedis(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	exists, err := common.RDB.Exists(ctx, redisKeyAlerted(key)).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}

func hasAlertedMemory(key string) bool {
	val, exists := alertedTimes.Load(key)
	if !exists {
		return false
	}

	t, ok := val.(time.Time)
	if !ok {
		return false
	}

	// Check if still within cooldown
	return time.Since(t) < alertCooldown
}

// MarkAlerted marks that we've sent an alert for the given key
// Returns false if already alerted (to prevent duplicate alerts)
func MarkAlerted(key string) bool {
	if !common.RedisEnabled {
		return markAlertedMemory(key)
	}

	marked, err := markAlertedRedis(key)
	if err != nil {
		log.Errorf("oncall: failed to mark alerted in Redis: %v, falling back to memory", err)
		return markAlertedMemory(key)
	}

	return marked
}

func markAlertedRedis(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use SetNX to ensure atomicity
	set, err := common.RDB.SetNX(ctx, redisKeyAlerted(key), time.Now().UnixNano(), alertedStateTTL).Result()
	if err != nil {
		return false, err
	}

	return set, nil
}

func markAlertedMemory(key string) bool {
	now := time.Now()

	for {
		val, loaded := alertedTimes.LoadOrStore(key, now)
		if !loaded {
			return true
		}

		t, ok := val.(time.Time)
		if !ok {
			// Invalid type, try to replace
			if alertedTimes.CompareAndSwap(key, val, now) {
				return true
			}
			continue
		}

		if time.Since(t) >= alertCooldown {
			// Cooldown expired, try to update
			if alertedTimes.CompareAndSwap(key, val, now) {
				return true
			}
			continue
		}

		return false
	}
}

// ClearAlerted clears the alerted state for the given key
func ClearAlerted(key string) {
	if common.RedisEnabled {
		if err := clearAlertedRedis(key); err != nil {
			log.Errorf("oncall: failed to clear alerted state in Redis: %v", err)
		}
	}
	clearAlertedMemory(key)
}

func clearAlertedRedis(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return common.RDB.Del(ctx, redisKeyAlerted(key)).Err()
}

func clearAlertedMemory(key string) {
	alertedTimes.Delete(key)
}

// AlertState is a compatibility wrapper that uses package-level functions
// Kept for backward compatibility with existing code
type AlertState struct{}

// NewAlertState creates a new AlertState instance
// The cleanup is handled by the package-level init() function
func NewAlertState() *AlertState {
	return &AlertState{}
}

// RecordError delegates to package-level function
func (s *AlertState) RecordError(key string) time.Time {
	return RecordError(key)
}

// HasError delegates to package-level function
func (s *AlertState) HasError(key string) bool {
	return HasError(key)
}

// ClearError delegates to package-level function
func (s *AlertState) ClearError(key string) {
	ClearError(key)
}

// HasAlerted delegates to package-level function
func (s *AlertState) HasAlerted(key string) bool {
	return HasAlerted(key)
}

// MarkAlerted delegates to package-level function
func (s *AlertState) MarkAlerted(key string) bool {
	return MarkAlerted(key)
}

// ClearAlerted delegates to package-level function
func (s *AlertState) ClearAlerted(key string) {
	ClearAlerted(key)
}
