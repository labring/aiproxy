//nolint:testpackage
package monitor

import (
	"context"
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

	prevEnabled := common.RedisEnabled
	prevRDB := common.RDB
	common.RedisEnabled = true

	common.RDB = redisClient
	defer func() {
		common.RedisEnabled = prevEnabled
		common.RDB = prevRDB
	}()

	for i := range minRequestCount {
		isError := i < 10
		beyondThreshold, banExecution, err := AddRequest(
			ctx,
			"model-a",
			101,
			isError,
			false,
			0.3,
			0,
		)
		require.NoError(t, err)

		if i < minRequestCount-1 {
			require.False(t, beyondThreshold)
			require.False(t, banExecution)
		} else {
			require.True(t, beyondThreshold)
			require.False(t, banExecution)
		}
	}

	modelRates, err := GetModelsErrorRate(ctx)
	require.NoError(t, err)
	require.Contains(t, modelRates, "model-a")
	require.InDelta(t, 0.5, modelRates["model-a"], 0.01)

	channelRates, err := GetModelChannelErrorRate(ctx, "model-a")
	require.NoError(t, err)
	require.Contains(t, channelRates, int64(101))
	require.InDelta(t, 0.5, channelRates[101], 0.01)

	modelByChannel, err := GetChannelModelErrorRates(ctx, 101)
	require.NoError(t, err)
	require.Contains(t, modelByChannel, "model-a")
	require.InDelta(t, 0.5, modelByChannel["model-a"], 0.01)

	allRates, err := GetAllChannelModelErrorRates(ctx)
	require.NoError(t, err)
	require.Contains(t, allRates, int64(101))
	require.Contains(t, allRates[101], "model-a")
	require.InDelta(t, 0.5, allRates[101]["model-a"], 0.01)
}

func TestRedisMonitorBanAndBannedQuery(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	prevEnabled := common.RedisEnabled
	prevRDB := common.RDB
	common.RedisEnabled = true

	common.RDB = redisClient
	defer func() {
		common.RedisEnabled = prevEnabled
		common.RDB = prevRDB
	}()

	for range minRequestCount {
		_, _, err := AddRequest(ctx, "model-ban", 202, true, false, 0.3, 0.8)
		require.NoError(t, err)
	}

	beyondThreshold, banExecution, err := AddRequest(ctx, "model-ban", 202, true, false, 0.3, 0.8)
	require.NoError(t, err)
	require.True(t, beyondThreshold)
	require.False(t, banExecution)

	bannedChannels, err := GetBannedChannelsWithModel(ctx, "model-ban")
	require.NoError(t, err)
	require.Contains(t, bannedChannels, int64(202))

	bannedMap, err := GetBannedChannelsMapWithModel(ctx, "model-ban")
	require.NoError(t, err)

	_, ok := bannedMap[202]
	require.True(t, ok)
}

func TestRedisMonitorConcurrentAddRequest(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	prevEnabled := common.RedisEnabled
	prevRDB := common.RDB
	common.RedisEnabled = true

	common.RDB = redisClient
	defer func() {
		common.RedisEnabled = prevEnabled
		common.RDB = prevRDB
	}()

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
				_, _, err := AddRequest(ctx, "model-concurrent", 303, (id+j)%4 == 0, false, 0.3, 0)
				require.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	rates, err := GetModelChannelErrorRate(ctx, "model-concurrent")
	require.NoError(t, err)
	require.Contains(t, rates, int64(303))
	require.Greater(t, rates[303], 0.0)
}

func TestRedisMonitorCleansExpiredSlicesWithCachedTotals(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForMonitorTest(t, ctx)
	defer cleanup()

	prevEnabled := common.RedisEnabled
	prevRDB := common.RDB
	common.RedisEnabled = true

	common.RDB = redisClient
	defer func() {
		common.RedisEnabled = prevEnabled
		common.RDB = prevRDB
	}()

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

	rates, err := GetModelChannelErrorRate(ctx, "model-expired")
	require.NoError(t, err)
	require.Contains(t, rates, int64(404))
	require.InDelta(t, 0.5, rates[404], 0.01)

	exists, err := redisClient.HExists(ctx, statsKey, strconv.FormatInt(expiredSlice, 10)).Result()
	require.NoError(t, err)
	require.False(t, exists)
}

func setupRedisForMonitorTest(t *testing.T, ctx context.Context) (*redis.Client, func()) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

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
		_ = client.Close()
		_ = container.Terminate(ctx)
	}

	return client, cleanup
}
