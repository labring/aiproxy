//nolint:testpackage
package monitor

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisMonitorAddRequestAndGetRates(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	for i := range minRequestCount {
		isError := i < 10
		errorRate, banExecution, err := monitor.AddRequest(
			ctx,
			"model-a",
			101,
			isError,
			false,
			0,
		)
		require.NoError(t, err)

		if i < minRequestCount-1 {
			require.Zero(t, errorRate)
			require.False(t, banExecution)
		} else {
			require.InDelta(t, 0.5, errorRate, 0.01)
			require.False(t, banExecution)
		}
	}

	modelRates, err := monitor.GetModelsErrorRate(ctx)
	require.NoError(t, err)
	require.Contains(t, modelRates, "model-a")
	require.InDelta(t, 0.5, modelRates["model-a"], 0.01)

	channelRates, err := monitor.GetModelChannelErrorRate(ctx, "model-a")
	require.NoError(t, err)
	require.Contains(t, channelRates, int64(101))
	require.InDelta(t, 0.5, channelRates[101], 0.01)

	modelByChannel, err := monitor.GetChannelModelErrorRates(ctx, 101)
	require.NoError(t, err)
	require.Contains(t, modelByChannel, "model-a")
	require.InDelta(t, 0.5, modelByChannel["model-a"], 0.01)

	allRates, err := monitor.GetAllChannelModelErrorRates(ctx)
	require.NoError(t, err)
	require.Contains(t, allRates, int64(101))
	require.Contains(t, allRates[101], "model-a")
	require.InDelta(t, 0.5, allRates[101]["model-a"], 0.01)
}

func TestRedisMonitorAddRequestReturnsZeroErrorRateWithoutMinimumSamples(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	for i := range minRequestCount - 1 {
		errorRate, banExecution, err := monitor.AddRequest(
			ctx,
			"model-no-auto-balance",
			404,
			true,
			false,
			0,
		)
		require.NoError(t, err)
		require.Zero(
			t,
			errorRate,
			"request %d should not return an error rate before the minimum sample size",
			i,
		)
		require.False(t, banExecution, "request %d should not trigger ban", i)
	}
}

func TestRedisMonitorBanAndBannedQuery(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	for range minRequestCount {
		errorRate, banExecution, err := monitor.AddRequest(ctx, "model-ban", 202, true, false, 0.8)
		require.NoError(t, err)

		if errorRate > 0 {
			require.InDelta(t, 1.0, errorRate, 0.01)
			require.True(t, banExecution)
		}
	}

	errorRate, banExecution, err := monitor.AddRequest(
		ctx,
		"model-ban",
		202,
		true,
		false,
		0.8,
	)
	require.NoError(t, err)
	require.InDelta(t, 1.0, errorRate, 0.01)
	require.False(t, banExecution)

	bannedChannels, err := monitor.GetBannedChannelsWithModel(ctx, "model-ban")
	require.NoError(t, err)
	require.Contains(t, bannedChannels, int64(202))

	bannedMap, err := monitor.GetBannedChannelsMapWithModel(ctx, "model-ban")
	require.NoError(t, err)

	_, ok := bannedMap[202]
	require.True(t, ok)
}

func TestRedisMonitorBansNoPermissionWithoutMaxErrorRate(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	errorRate, banExecution, err := monitor.AddRequest(
		ctx,
		"model-no-permission",
		212,
		true,
		true,
		0,
	)
	require.NoError(t, err)
	require.Zero(t, errorRate)
	require.True(t, banExecution)

	bannedChannels, err := monitor.GetBannedChannelsWithModel(ctx, "model-no-permission")
	require.NoError(t, err)
	require.Contains(t, bannedChannels, int64(212))
}

func TestRedisMonitorConcurrentAddRequest(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	const (
		numGoroutines        = 40
		requestsPerGoroutine = 10
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()

			for j := range requestsPerGoroutine {
				_, _, err := monitor.AddRequest(
					ctx,
					"model-concurrent",
					303,
					(id+j)%4 == 0,
					false,
					0,
				)
				require.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	rates, err := monitor.GetModelChannelErrorRate(ctx, "model-concurrent")
	require.NoError(t, err)
	require.Contains(t, rates, int64(303))
	require.Greater(t, rates[303], 0.0)
}

func TestRedisMonitorCleansExpiredSlicesWithCachedTotals(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	statsKey := buildStatsKey("model-expired", "404")
	currentSlice := time.Now().UnixMilli() / 10000
	expiredSlice := currentSlice - maxSliceCount - 1
	validSlice := currentSlice

	err := redisClient.HSet(ctx, statsKey,
		strconv.FormatInt(expiredSlice, 10), "15:15",
		strconv.FormatInt(validSlice, 10), "20:10",
		"__meta_total_req", 35,
		"__meta_total_err", 25,
		"__meta_last_cleaned_slice", expiredSlice,
	).Err()
	require.NoError(t, err)
	require.NoError(
		t,
		redisClient.PExpire(ctx, statsKey, time.Duration(maxSliceCount*10)*time.Second).Err(),
	)

	rates, err := monitor.GetModelChannelErrorRate(ctx, "model-expired")
	require.NoError(t, err)
	require.Contains(t, rates, int64(404))
	require.InDelta(t, 0.5, rates[404], 0.01)

	exists, err := redisClient.HExists(ctx, statsKey, strconv.FormatInt(expiredSlice, 10)).Result()
	require.NoError(t, err)
	require.False(t, exists)
}

func TestRedisMonitorGetModelChannelErrorRateUsesLocalCache(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	for i := range minRequestCount {
		_, _, err := monitor.AddRequest(ctx, "model-local-rate", 505, i < 5, false, 0)
		require.NoError(t, err)
	}

	rates, err := monitor.GetModelChannelErrorRate(ctx, "model-local-rate")
	require.NoError(t, err)
	require.Contains(t, rates, int64(505))

	require.NoError(t, redisClient.Del(ctx, buildStatsKey("model-local-rate", "505")).Err())

	rates, err = monitor.GetModelChannelErrorRate(ctx, "model-local-rate")
	require.NoError(t, err)
	require.Contains(t, rates, int64(505))
}

func TestRedisMonitorGetModelChannelErrorRateKeepsLocalCacheUntilTTL(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	for i := range minRequestCount {
		_, _, err := monitor.AddRequest(
			ctx,
			"model-local-rate-invalidate",
			606,
			i < 5,
			false,
			0,
		)
		require.NoError(t, err)
	}

	rates, err := monitor.GetModelChannelErrorRate(ctx, "model-local-rate-invalidate")
	require.NoError(t, err)
	require.Contains(t, rates, int64(606))
	require.InDelta(t, 0.25, rates[606], 0.01)

	for i := range minRequestCount {
		_, _, err = monitor.AddRequest(
			ctx,
			"model-local-rate-invalidate",
			606,
			i < 10,
			false,
			0,
		)
		require.NoError(t, err)
	}

	require.NoError(
		t,
		redisClient.Del(ctx, buildStatsKey("model-local-rate-invalidate", "606")).Err(),
	)

	rates, err = monitor.GetModelChannelErrorRate(ctx, "model-local-rate-invalidate")
	require.NoError(t, err)
	require.Contains(t, rates, int64(606))
	require.InDelta(t, 0.25, rates[606], 0.01)

	time.Sleep(monitorLocalTTL + 150*time.Millisecond)

	rates, err = monitor.GetModelChannelErrorRate(ctx, "model-local-rate-invalidate")
	require.NoError(t, err)
	require.NotContains(t, rates, int64(606))
}

func TestRedisMonitorGetBannedChannelsMapWithModelUsesLocalCache(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	for range minRequestCount + 1 {
		_, _, err := monitor.AddRequest(ctx, "model-local-banned", 707, true, false, 0.8)
		require.NoError(t, err)
	}

	banned, err := monitor.GetBannedChannelsMapWithModel(ctx, "model-local-banned")
	require.NoError(t, err)
	require.Contains(t, banned, int64(707))

	require.NoError(
		t,
		redisClient.Del(ctx, common.RedisKey("model:model-local-banned:channel:707:banned")).Err(),
	)

	banned, err = monitor.GetBannedChannelsMapWithModel(ctx, "model-local-banned")
	require.NoError(t, err)
	require.Contains(t, banned, int64(707))
}

func TestRedisMonitorGetBannedChannelsMapWithModelInvalidatesLocalCacheOnClear(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	for range minRequestCount + 1 {
		_, _, err := monitor.AddRequest(ctx, "model-local-banned-clear", 808, true, false, 0.8)
		require.NoError(t, err)
	}

	banned, err := monitor.GetBannedChannelsMapWithModel(ctx, "model-local-banned-clear")
	require.NoError(t, err)
	require.Contains(t, banned, int64(808))

	require.NoError(t, monitor.ClearChannelModelErrors(ctx, "model-local-banned-clear", 808))

	banned, err = monitor.GetBannedChannelsMapWithModel(ctx, "model-local-banned-clear")
	require.NoError(t, err)
	require.NotContains(t, banned, int64(808))
}

func TestRedisMonitorGetBannedChannelsMapWithModelKeepsLocalCacheUntilTTL(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	monitor := newTestRedisModelMonitor(redisClient)

	for range minRequestCount + 1 {
		_, _, err := monitor.AddRequest(ctx, "model-local-banned-stale", 909, true, false, 0.8)
		require.NoError(t, err)
	}

	banned, err := monitor.GetBannedChannelsMapWithModel(ctx, "model-local-banned-stale")
	require.NoError(t, err)
	require.Contains(t, banned, int64(909))

	_, _, err = monitor.AddRequest(ctx, "model-local-banned-stale", 909, true, false, 0.8)
	require.NoError(t, err)

	require.NoError(
		t,
		redisClient.Del(ctx, common.RedisKey("model:model-local-banned-stale:channel:909:banned")).
			Err(),
	)

	banned, err = monitor.GetBannedChannelsMapWithModel(ctx, "model-local-banned-stale")
	require.NoError(t, err)
	require.Contains(t, banned, int64(909))

	time.Sleep(monitorLocalTTL + 150*time.Millisecond)

	banned, err = monitor.GetBannedChannelsMapWithModel(ctx, "model-local-banned-stale")
	require.NoError(t, err)
	require.NotContains(t, banned, int64(909))
}

func newTestRedisModelMonitor(client *redis.Client) *redisModelMonitor {
	return newRedisModelMonitor(func() *redis.Client {
		return client
	})
}

func setupRedisForMonitorTest(t *testing.T, ctx context.Context) (*redis.Client, func()) {
	t.Helper()

	flushMonitorLocalCache()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}

	var (
		container testcontainers.Container
		err       error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("docker unavailable: %v", r)
			}
		}()

		container, err = testcontainers.GenericContainer(
			ctx,
			testcontainers.GenericContainerRequest{
				ContainerRequest: req,
				Started:          true,
			},
		)
	}()

	if err != nil {
		t.Skipf("skipping redis integration test: %v", err)
	}

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + mappedPort.Port(),
		DB:   0,
	})
	require.NoError(t, client.Ping(ctx).Err())

	cleanup := func() {
		flushMonitorLocalCache()

		_ = client.Close()
		_ = container.Terminate(ctx)
	}

	return client, cleanup
}
