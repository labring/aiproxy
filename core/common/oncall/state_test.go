package oncall

import (
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
