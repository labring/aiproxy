//nolint:testpackage
package monitor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeWindowStatsMaintainsRollingTotals(t *testing.T) {
	stats := NewTimeWindowStats()
	base := time.Now().Truncate(timeWindow)

	stats.AddRequest(base.Add(-11*timeWindow), false)
	stats.AddRequest(base.Add(-2*timeWindow), false)
	stats.AddRequest(base.Add(-2*timeWindow), true)
	stats.AddRequest(base, false)

	req, err := stats.GetStats()
	require.Equal(t, 4, req)
	require.Equal(t, 1, err)

	req, err = stats.GetStats()
	require.Equal(t, 4, req)
	require.Equal(t, 1, err)
}

func TestTimeWindowStatsHasValidSlicesAfterExpiry(t *testing.T) {
	stats := &TimeWindowStats{
		slices: []*timeSlice{
			{windowStart: time.Now().Add(-13 * timeWindow), requests: 2, errors: 1},
		},
	}

	require.False(t, stats.HasValidSlices())

	req, err := stats.GetStats()
	require.Equal(t, 0, req)
	require.Equal(t, 0, err)
}

func TestTimeWindowStatsConcurrentAddRequest(t *testing.T) {
	stats := NewTimeWindowStats()

	const (
		numGoroutines        = 40
		requestsPerGoroutine = 25
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()

			for j := range requestsPerGoroutine {
				stats.AddRequest(time.Now(), (id+j)%3 == 0)
			}
		}(i)
	}

	wg.Wait()

	req, err := stats.GetStats()
	require.Equal(t, numGoroutines*requestsPerGoroutine, req)
	require.Greater(t, err, 0)
}

func TestMemModelMonitorCleanupExpiredData(t *testing.T) {
	monitor := &MemModelMonitor{
		models: map[string]*ModelData{
			"model-a": {
				channels: map[int64]*ChannelStats{
					1: {
						timeWindows: &TimeWindowStats{
							slices: []*timeSlice{
								{windowStart: time.Now().Add(-13 * timeWindow), requests: 1},
							},
						},
					},
				},
				totalStats: &TimeWindowStats{
					slices: []*timeSlice{
						{windowStart: time.Now().Add(-13 * timeWindow), requests: 1},
					},
				},
			},
		},
	}

	monitor.cleanupExpiredData()

	require.Empty(t, monitor.models)
}

func TestMemModelMonitorAddRequestAndThreshold(t *testing.T) {
	monitor := &MemModelMonitor{
		models: make(map[string]*ModelData),
	}

	for i := range minRequestCount {
		beyondThreshold, banExecution := monitor.AddRequest("model-a", 1, true, false, 0.3, 0)
		if i < minRequestCount-1 {
			require.False(t, beyondThreshold)
			require.False(t, banExecution)
		} else {
			require.True(t, beyondThreshold)
			require.False(t, banExecution)
		}
	}
}
