package common_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadWithKeyLockLoadsOncePerKey(t *testing.T) {
	locker := common.NewKeyedLocker()
	cache := struct {
		sync.Mutex
		value int
		ok    bool
	}{}

	var loads atomic.Int32

	started := make(chan struct{})
	release := make(chan struct{})

	load := func() (int, error) {
		if loads.Add(1) == 1 {
			close(started)
			<-release
			cache.Lock()
			cache.value = 42
			cache.ok = true
			cache.Unlock()
		}

		cache.Lock()
		defer cache.Unlock()

		return cache.value, nil
	}

	getCached := func() (int, bool) {
		cache.Lock()
		defer cache.Unlock()
		return cache.value, cache.ok
	}

	var wg sync.WaitGroup

	results := make([]int, 2)

	wg.Go(func() {
		value, err := common.LoadWithKeyLock(locker, "same-key", getCached, load)
		require.NoError(t, err)

		results[0] = value
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first keyed load did not start")
	}

	wg.Go(func() {
		value, err := common.LoadWithKeyLock(locker, "same-key", getCached, load)
		require.NoError(t, err)

		results[1] = value
	})

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), loads.Load())

	close(release)
	wg.Wait()

	assert.Equal(t, int32(1), loads.Load())
	assert.Equal(t, []int{42, 42}, results)
}
