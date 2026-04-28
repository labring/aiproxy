//nolint:testpackage
package monitor

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadWithLocalKeyLockOnlyLoadsOncePerKey(t *testing.T) {
	flushMonitorLocalCache()
	t.Cleanup(flushMonitorLocalCache)

	key := modelChannelErrorRateLocalCacheKey("lock-model")

	var loads atomic.Int32

	started := make(chan struct{})
	release := make(chan struct{})

	loader := func() (map[int64]float64, error) {
		if loads.Add(1) == 1 {
			close(started)
			<-release
			setModelChannelErrorRateLocalUnlocked("lock-model", map[int64]float64{1: 0.5})
		}

		result, ok := getModelChannelErrorRateLocal("lock-model")
		if !ok {
			return nil, errors.New("missing local cache after loader populated it")
		}

		return result, nil
	}

	var wg sync.WaitGroup

	results := make([]map[int64]float64, 2)

	wg.Go(func() {
		result, err := loadWithLocalKeyLock(
			monitorLocalLoadLocker,
			key,
			func() (map[int64]float64, bool) {
				return getModelChannelErrorRateLocal("lock-model")
			},
			loader,
		)
		require.NoError(t, err)

		results[0] = result
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first monitor loader did not start")
	}

	wg.Go(func() {
		result, err := loadWithLocalKeyLock(
			monitorLocalLoadLocker,
			key,
			func() (map[int64]float64, bool) {
				return getModelChannelErrorRateLocal("lock-model")
			},
			loader,
		)
		require.NoError(t, err)

		results[1] = result
	})

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), loads.Load())

	close(release)
	wg.Wait()

	assert.Equal(t, int32(1), loads.Load())
	require.Equal(t, map[int64]float64{1: 0.5}, results[0])
	require.Equal(t, map[int64]float64{1: 0.5}, results[1])
}
