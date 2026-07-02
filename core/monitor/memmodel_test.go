//nolint:testpackage
package monitor

import (
	"context"
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
				channels: map[string]*ChannelStats{
					"1": {
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
		errorRate, banExecution := monitor.AddRequest("model-a", 1, true, false, 0)
		if i < minRequestCount-1 {
			require.Zero(t, errorRate)
			require.False(t, banExecution)
		} else {
			require.InDelta(t, 1.0, errorRate, 0.0001)
			require.False(t, banExecution)
		}
	}
}

func TestMemModelMonitorAddRequestReturnsZeroErrorRateWithoutMinimumSamples(t *testing.T) {
	monitor := &MemModelMonitor{
		models: make(map[string]*ModelData),
	}

	for i := range minRequestCount - 1 {
		errorRate, banExecution := monitor.AddRequest("model-a", 1, true, false, 0)
		require.Zero(
			t,
			errorRate,
			"request %d should not return an error rate before the minimum sample size",
			i,
		)
		require.False(t, banExecution, "request %d should not trigger ban", i)
	}
}

func TestMemModelMonitorAddRequestReturnsCurrentErrorRateAndBans(t *testing.T) {
	monitor := &MemModelMonitor{
		models: make(map[string]*ModelData),
	}

	for i := range minRequestCount {
		errorRate, banExecution := monitor.AddRequest("model-ban", 1, true, false, 0.8)
		if i < minRequestCount-1 {
			require.InDelta(t, 0, errorRate, 0.0001)
			require.False(t, banExecution)
		} else {
			require.InDelta(t, 1.0, errorRate, 0.0001)
			require.True(t, banExecution)
		}
	}

	errorRate, banExecution := monitor.AddRequest("model-ban", 1, true, false, 0.8)
	require.InDelta(t, 1.0, errorRate, 0.0001)
	require.False(t, banExecution)
}

func TestMemModelMonitorAddRequestBansNoPermissionWithoutMaxErrorRate(t *testing.T) {
	monitor := &MemModelMonitor{
		models: make(map[string]*ModelData),
	}

	errorRate, banExecution := monitor.AddRequest("model-no-permission", 1, true, true, 0)
	require.Zero(t, errorRate)
	require.True(t, banExecution)
}

func TestMemModelMonitorGetChannelModelErrorRate(t *testing.T) {
	monitor := &MemModelMonitor{
		models: make(map[string]*ModelData),
	}

	for i := range minRequestCount {
		_, _ = monitor.AddRequest("model-single-rate", 42, i < 5, false, 0)
	}

	rate, err := monitor.GetChannelModelErrorRate(context.Background(), "model-single-rate", 42)
	require.NoError(t, err)
	require.InDelta(t, 0.5, rate, 0.01)

	rate, err = monitor.GetChannelModelErrorRate(context.Background(), "model-single-rate", 404)
	require.NoError(t, err)
	require.Zero(t, rate)
}

func TestMemModelMonitorGroupChannelKeyIsolation(t *testing.T) {
	monitor := &MemModelMonitor{
		models: make(map[string]*ModelData),
	}

	groupKey := "group_channel:group-1:42"
	for range minRequestCount {
		_, _ = monitor.AddRequestByChannelKey("model-group", groupKey, true, false, 0)
	}

	stringRates, err := monitor.GetModelChannelErrorRateByKey(context.Background(), "model-group")
	require.NoError(t, err)
	require.Contains(t, stringRates, groupKey)

	intRates, err := monitor.GetModelChannelErrorRate(context.Background(), "model-group")
	require.NoError(t, err)
	require.Empty(t, intRates)

	banned, err := monitor.GetBannedChannelsMapWithModelByKey(context.Background(), "model-group")
	require.NoError(t, err)
	require.Empty(t, banned)

	_, _ = monitor.AddRequestByChannelKey("model-group", groupKey, true, true, 0)
	banned, err = monitor.GetBannedChannelsMapWithModelByKey(context.Background(), "model-group")
	require.NoError(t, err)
	require.Contains(t, banned, groupKey)

	require.NoError(t, monitor.ClearChannelAllModelErrorsByKey(context.Background(), groupKey))
	rate, err := monitor.GetChannelModelErrorRateByKey(
		context.Background(),
		"model-group",
		groupKey,
	)
	require.NoError(t, err)
	require.Zero(t, rate)
}

func TestGroupChannelMonitorNamespaceIsolatedFromGlobalMonitor(t *testing.T) {
	oldGlobalMonitor := memModelMonitor
	oldGroupChannelMonitor := memGroupChannelModelMonitor
	memModelMonitor = &MemModelMonitor{models: make(map[string]*ModelData)}
	memGroupChannelModelMonitor = &MemModelMonitor{models: make(map[string]*ModelData)}
	t.Cleanup(func() {
		memModelMonitor = oldGlobalMonitor
		memGroupChannelModelMonitor = oldGroupChannelMonitor
	})

	ctx := context.Background()

	groupKey := "group_channel:group-1:42"
	for range minRequestCount {
		_, _, err := AddGroupChannelRequestByChannelKey(
			ctx,
			"model-group-scope",
			groupKey,
			true,
			false,
			0,
		)
		require.NoError(t, err)
	}

	globalModelRates, err := GetModelsErrorRate(ctx)
	require.NoError(t, err)
	require.Empty(t, globalModelRates)

	globalStringRates, err := GetModelChannelErrorRateByKey(ctx, "model-group-scope")
	require.NoError(t, err)
	require.Empty(t, globalStringRates)

	groupStringRates, err := GetGroupChannelModelErrorRateByKey(ctx, "model-group-scope")
	require.NoError(t, err)
	require.Contains(t, groupStringRates, groupKey)

	_, _, err = AddGroupChannelRequestByChannelKey(
		ctx,
		"model-group-scope",
		groupKey,
		true,
		true,
		0,
	)
	require.NoError(t, err)

	globalBanned, err := GetBannedChannelKeysMapWithModel(ctx, "model-group-scope")
	require.NoError(t, err)
	require.Empty(t, globalBanned)

	groupBanned, err := GetGroupChannelBannedChannelKeysMapWithModel(ctx, "model-group-scope")
	require.NoError(t, err)
	require.Contains(t, groupBanned, groupKey)

	require.NoError(t, ClearGroupChannelAllModelErrorsByKey(ctx, groupKey))
	groupRate, err := GetGroupChannelChannelModelErrorRateByKey(ctx, "model-group-scope", groupKey)
	require.NoError(t, err)
	require.Zero(t, groupRate)
}
