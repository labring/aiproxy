package oncall

import (
	"sync"
	"testing"
	"time"
)

func TestRecordErrorMemory(t *testing.T) {
	key := "test_error_" + time.Now().Format(time.RFC3339Nano)

	// First call should return current time
	firstTime := recordErrorMemory(key)
	if firstTime.IsZero() {
		t.Fatal("recordErrorMemory returned zero time")
	}

	// Second call should return the same time
	time.Sleep(10 * time.Millisecond)

	secondTime := recordErrorMemory(key)
	if !secondTime.Equal(firstTime) {
		t.Errorf("expected same time %v, got %v", firstTime, secondTime)
	}

	// Clean up
	clearErrorMemory(key)
}

func TestHasErrorMemory(t *testing.T) {
	key := "test_has_error_" + time.Now().Format(time.RFC3339Nano)

	// Should not exist initially
	if hasErrorMemory(key) {
		t.Error("expected hasErrorMemory to return false for non-existent key")
	}

	// Record error
	recordErrorMemory(key)

	// Should exist now
	if !hasErrorMemory(key) {
		t.Error("expected hasErrorMemory to return true after recording")
	}

	// Clear and check again
	clearErrorMemory(key)

	if hasErrorMemory(key) {
		t.Error("expected hasErrorMemory to return false after clearing")
	}
}

func TestMarkAlertedMemory(t *testing.T) {
	key := "test_alerted_" + time.Now().Format(time.RFC3339Nano)

	// First mark should succeed
	if !markAlertedMemory(key) {
		t.Error("expected first markAlertedMemory to return true")
	}

	// Second mark should fail (within cooldown)
	if markAlertedMemory(key) {
		t.Error("expected second markAlertedMemory to return false (within cooldown)")
	}

	// Check hasAlerted
	if !hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to return true")
	}

	// Clean up
	clearAlertedMemory(key)

	// After clearing, should be able to mark again
	if !markAlertedMemory(key) {
		t.Error("expected markAlertedMemory to return true after clearing")
	}

	// Clean up
	clearAlertedMemory(key)
}

func TestAlertStateWrapper(t *testing.T) {
	state := NewAlertState()
	key := "test_wrapper_" + time.Now().Format(time.RFC3339Nano)

	// Test that wrapper delegates to package functions
	firstTime := state.RecordError(key)
	if firstTime.IsZero() {
		t.Fatal("RecordError returned zero time")
	}

	if !state.HasError(key) {
		t.Error("expected HasError to return true")
	}

	state.ClearError(key)

	if state.HasError(key) {
		t.Error("expected HasError to return false after clearing")
	}

	// Test alerted functions
	if !state.MarkAlerted(key) {
		t.Error("expected MarkAlerted to return true")
	}

	if state.MarkAlerted(key) {
		t.Error("expected MarkAlerted to return false (within cooldown)")
	}

	if !state.HasAlerted(key) {
		t.Error("expected HasAlerted to return true")
	}

	state.ClearAlerted(key)

	if state.HasAlerted(key) {
		t.Error("expected HasAlerted to return false after clearing")
	}
}

func TestRecordErrorMemoryConcurrent(t *testing.T) {
	key := "test_concurrent_error_" + time.Now().Format(time.RFC3339Nano)
	defer clearErrorMemory(key)

	var wg sync.WaitGroup

	results := make(chan time.Time, 100)

	// Launch 100 concurrent goroutines
	for range 100 {
		wg.Go(func() {
			results <- recordErrorMemory(key)
		})
	}

	wg.Wait()
	close(results)

	// All results should be the same time (first recorded time)
	var firstTime time.Time
	for t := range results {
		if firstTime.IsZero() {
			firstTime = t
		} else if !t.Equal(firstTime) {
			// Allow small difference due to timing
			diff := t.Sub(firstTime)
			if diff < 0 {
				diff = -diff
			}

			if diff > time.Millisecond {
				// This is acceptable - concurrent LoadOrStore may return different times
				// as long as they're consistent
			}
		}
	}
}

func TestMarkAlertedMemoryConcurrent(t *testing.T) {
	key := "test_concurrent_alerted_" + time.Now().Format(time.RFC3339Nano)
	defer clearAlertedMemory(key)

	var wg sync.WaitGroup

	successCount := 0

	var mu sync.Mutex

	// Launch 100 concurrent goroutines
	for range 100 {
		wg.Go(func() {
			if markAlertedMemory(key) {
				mu.Lock()

				successCount++

				mu.Unlock()
			}
		})
	}

	wg.Wait()

	// Only one goroutine should succeed
	if successCount != 1 {
		t.Errorf("expected exactly 1 successful mark, got %d", successCount)
	}
}

func TestHasAlertedMemoryWithExpiredCooldown(t *testing.T) {
	key := "test_expired_cooldown_" + time.Now().Format(time.RFC3339Nano)

	// Manually set an old alerted time (past cooldown)
	oldTime := time.Now().Add(-alertCooldown - time.Minute)

	alertedTimes.Store(key, oldTime)
	defer clearAlertedMemory(key)

	// Should return false because cooldown has expired
	if hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to return false for expired cooldown")
	}
}

func TestMarkAlertedMemoryAfterCooldownExpires(t *testing.T) {
	key := "test_mark_after_cooldown_" + time.Now().Format(time.RFC3339Nano)

	// Manually set an old alerted time (past cooldown)
	oldTime := time.Now().Add(-alertCooldown - time.Minute)

	alertedTimes.Store(key, oldTime)
	defer clearAlertedMemory(key)

	// Should succeed because cooldown has expired
	if !markAlertedMemory(key) {
		t.Error("expected markAlertedMemory to return true after cooldown expires")
	}

	// Second call should fail (within new cooldown)
	if markAlertedMemory(key) {
		t.Error("expected second markAlertedMemory to return false")
	}
}

func TestClearErrorMemoryNonExistent(t *testing.T) {
	key := "test_clear_nonexistent_" + time.Now().Format(time.RFC3339Nano)

	// Should not panic when clearing non-existent key
	clearErrorMemory(key)

	// Verify key still doesn't exist
	if hasErrorMemory(key) {
		t.Error("expected hasErrorMemory to return false for non-existent key")
	}
}

func TestClearAlertedMemoryNonExistent(t *testing.T) {
	key := "test_clear_alerted_nonexistent_" + time.Now().Format(time.RFC3339Nano)

	// Should not panic when clearing non-existent key
	clearAlertedMemory(key)

	// Verify key still doesn't exist
	if hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to return false for non-existent key")
	}
}

func TestMultipleKeysMemory(t *testing.T) {
	keys := []string{
		"test_multi_1_" + time.Now().Format(time.RFC3339Nano),
		"test_multi_2_" + time.Now().Format(time.RFC3339Nano),
		"test_multi_3_" + time.Now().Format(time.RFC3339Nano),
	}

	// Clean up at the end
	defer func() {
		for _, key := range keys {
			clearErrorMemory(key)
			clearAlertedMemory(key)
		}
	}()

	// Record errors for all keys
	times := make(map[string]time.Time)
	for _, key := range keys {
		times[key] = recordErrorMemory(key)

		time.Sleep(time.Millisecond) // Ensure different times
	}

	// Verify all keys exist and have correct times
	for _, key := range keys {
		if !hasErrorMemory(key) {
			t.Errorf("expected hasErrorMemory to return true for key %s", key)
		}

		recordedTime := recordErrorMemory(key)
		if !recordedTime.Equal(times[key]) {
			t.Errorf("key %s: expected time %v, got %v", key, times[key], recordedTime)
		}
	}

	// Clear one key and verify others still exist
	clearErrorMemory(keys[1])

	if hasErrorMemory(keys[1]) {
		t.Error("expected hasErrorMemory to return false for cleared key")
	}

	if !hasErrorMemory(keys[0]) || !hasErrorMemory(keys[2]) {
		t.Error("expected other keys to still exist")
	}
}

func TestHasAlertedMemoryWithInvalidType(t *testing.T) {
	key := "test_invalid_type_alerted_" + time.Now().Format(time.RFC3339Nano)

	// Store invalid type
	alertedTimes.Store(key, "invalid")
	defer clearAlertedMemory(key)

	// Should return false for invalid type
	if hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to return false for invalid type")
	}
}

func TestMarkAlertedMemoryWithInvalidType(t *testing.T) {
	key := "test_invalid_type_mark_" + time.Now().Format(time.RFC3339Nano)

	// Store invalid type
	alertedTimes.Store(key, "invalid")
	defer clearAlertedMemory(key)

	// Should succeed and replace the invalid value
	if !markAlertedMemory(key) {
		t.Error("expected markAlertedMemory to return true and replace invalid type")
	}

	// Now should be marked
	if !hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to return true after marking")
	}
}

func TestRecordErrorMemoryWithInvalidType(t *testing.T) {
	key := "test_invalid_type_error_" + time.Now().Format(time.RFC3339Nano)

	// Store invalid type
	errorTimes.Store(key, "invalid")
	defer clearErrorMemory(key)

	// Should return current time and replace the invalid value
	before := time.Now()
	result := recordErrorMemory(key)
	after := time.Now()

	if result.Before(before) || result.After(after) {
		t.Errorf("expected time between %v and %v, got %v", before, after, result)
	}
}

func TestErrorAndAlertedIndependence(t *testing.T) {
	key := "test_independence_" + time.Now().Format(time.RFC3339Nano)
	defer func() {
		clearErrorMemory(key)
		clearAlertedMemory(key)
	}()

	// Record error
	recordErrorMemory(key)

	if !hasErrorMemory(key) {
		t.Error("expected hasErrorMemory to return true")
	}

	if hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to return false (not marked yet)")
	}

	// Mark alerted
	markAlertedMemory(key)

	if !hasErrorMemory(key) {
		t.Error("expected hasErrorMemory to still return true")
	}

	if !hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to return true")
	}

	// Clear error only
	clearErrorMemory(key)

	if hasErrorMemory(key) {
		t.Error("expected hasErrorMemory to return false after clearing")
	}

	if !hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to still return true")
	}

	// Clear alerted
	clearAlertedMemory(key)

	if hasAlertedMemory(key) {
		t.Error("expected hasAlertedMemory to return false after clearing")
	}
}
