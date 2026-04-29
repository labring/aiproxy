package trylock_test

import (
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common/trylock"
)

func TestMemLock(t *testing.T) {
	if !trylock.MemLock("", time.Second) {
		t.Error("Expected true, Got false")
	}

	if trylock.MemLock("", time.Second) {
		t.Error("Expected false, Got true")
	}

	if trylock.MemLock("", time.Second) {
		t.Error("Expected false, Got true")
	}

	time.Sleep(time.Second)

	if !trylock.MemLock("", time.Second) {
		t.Error("Expected true, Got false")
	}
}

func TestMemLockPanicsOnTypeMismatch(t *testing.T) {
	trylock.InjectMemLockValueForTest("panic-key", "bad")
	t.Cleanup(func() {
		trylock.ResetMemLockValueForTest("panic-key")
	})

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on mem lock type mismatch")
		}
	}()

	trylock.MemLock("panic-key", time.Second)
}
