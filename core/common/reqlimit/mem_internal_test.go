package reqlimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInMemoryRecordDropsExpiredMinuteWindowWithoutRealWait(t *testing.T) {
	rl := &InMemoryRecord{}

	e := rl.getEntry([]string{"group1", "model1"})

	nowSecond := time.Now().Unix()

	e.Lock()
	e.windows[nowSecond-61] = &windowCounts{normal: 3}
	e.windows[nowSecond-1] = &windowCounts{normal: 2}
	e.aggregateInitialized = false
	e.Unlock()

	totalCount, secondCount := rl.GetRequest(time.Minute, "group1", "model1")
	require.Equal(t, int64(2), totalCount)
	require.Equal(t, int64(0), secondCount)

	e.Lock()
	_, oldExists := e.windows[nowSecond-61]
	_, recentExists := e.windows[nowSecond-1]
	e.Unlock()
	require.False(t, oldExists)
	require.True(t, recentExists)
}

func TestInMemoryRecordCleanupInactiveEntries(t *testing.T) {
	rl := &InMemoryRecord{}

	e := rl.getEntry([]string{"group1", "model1"})
	e.lastAccess.Store(time.Now().Add(-time.Second))

	go rl.cleanupInactiveEntries(10*time.Millisecond, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := rl.entries.Load("group1:model1")
		return !ok
	}, 300*time.Millisecond, 10*time.Millisecond)
}
