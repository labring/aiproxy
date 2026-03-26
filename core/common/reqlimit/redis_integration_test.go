//nolint:testpackage
package reqlimit

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

func TestRedisRateRecordPushAndGet(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForReqLimitTest(t, ctx)
	defer cleanup()

	record := newRedisChannelModelRecord(func() *redis.Client { return redisClient })

	channel := "100"
	model := "gpt-4o"

	normalCount, overCount, secondCount, err := record.PushRequest(
		ctx,
		0,
		time.Minute,
		1,
		channel,
		model,
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), normalCount)
	require.Equal(t, int64(0), overCount)
	require.Equal(t, int64(1), secondCount)

	normalCount, overCount, secondCount, err = record.PushRequest(
		ctx,
		0,
		time.Minute,
		1,
		channel,
		model,
	)
	require.NoError(t, err)
	require.Equal(t, int64(2), normalCount)
	require.Equal(t, int64(0), overCount)
	require.Equal(t, int64(2), secondCount)

	rpm, rps, err := record.GetRequest(ctx, time.Minute, channel, model)
	require.NoError(t, err)
	require.Equal(t, int64(2), rpm)
	require.Equal(t, int64(2), rps)

	tokenRecord := newRedisChannelModelTokensRecord(func() *redis.Client { return redisClient })
	_, _, _, err = tokenRecord.PushRequest(ctx, 0, time.Minute, 40, channel, model)
	require.NoError(t, err)
	_, _, _, err = tokenRecord.PushRequest(ctx, 0, time.Minute, 60, channel, model)
	require.NoError(t, err)
	tpm, tps, err := tokenRecord.GetRequest(ctx, time.Minute, channel, model)
	require.NoError(t, err)
	require.Equal(t, int64(100), tpm)
	require.Equal(t, int64(100), tps)
}

func TestRedisRateRecordWildcardGet(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForReqLimitTest(t, ctx)
	defer cleanup()

	record := newRedisChannelModelRecord(func() *redis.Client { return redisClient })

	require.NoError(
		t,
		seedRedisReqLimitRecord(ctx, redisClient, "channel-model-record:1:model-a", 3, 1),
	)
	require.NoError(
		t,
		seedRedisReqLimitRecord(ctx, redisClient, "channel-model-record:1:model-b", 5, 2),
	)
	require.NoError(
		t,
		seedRedisReqLimitRecord(ctx, redisClient, "channel-model-record:2:model-a", 7, 4),
	)

	totalCount, secondCount, err := record.GetRequest(ctx, time.Minute, "*", "*")
	require.NoError(t, err)
	require.Equal(t, int64(15), totalCount)
	require.Equal(t, int64(7), secondCount)

	totalCount, secondCount, err = record.GetRequest(ctx, time.Minute, "1", "*")
	require.NoError(t, err)
	require.Equal(t, int64(8), totalCount)
	require.Equal(t, int64(3), secondCount)

	totalCount, secondCount, err = record.GetRequest(ctx, time.Minute, "1", "model-a")
	require.NoError(t, err)
	require.Equal(t, int64(3), totalCount)
	require.Equal(t, int64(1), secondCount)

	totalCount, secondCount, err = record.GetRequest(ctx, time.Minute, "1", "model-*")
	require.NoError(t, err)
	require.Equal(t, int64(8), totalCount)
	require.Equal(t, int64(3), secondCount)
}

func TestRedisRateRecordExpirationCleanup(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForReqLimitTest(t, ctx)
	defer cleanup()

	record := newRedisChannelModelRecord(func() *redis.Client { return redisClient })

	require.NoError(
		t,
		seedRedisReqLimitRecordWithOffset(
			ctx,
			redisClient,
			"channel-model-record:9:model-x",
			2,
			1,
			-61,
		),
	)

	totalCount, secondCount, err := record.GetRequest(ctx, time.Minute, "9", "model-x")
	require.NoError(t, err)
	require.Equal(t, int64(0), totalCount)
	require.Equal(t, int64(0), secondCount)

	_, _, _, err = record.PushRequest(ctx, 0, time.Minute, 1, "9", "model-x")
	require.NoError(t, err)
	totalCount, secondCount, err = record.GetRequest(ctx, time.Minute, "9", "model-x")
	require.NoError(t, err)
	require.Equal(t, int64(1), totalCount)
	require.Equal(t, int64(1), secondCount)
}

func TestRedisRateRecordConcurrentPush(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForReqLimitTest(t, ctx)
	defer cleanup()

	record := newRedisChannelModelRecord(func() *redis.Client { return redisClient })

	const (
		numGoroutines        = 50
		requestsPerGoroutine = 20
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()

			for range requestsPerGoroutine {
				_, _, _, err := record.PushRequest(
					ctx,
					0,
					time.Minute,
					1,
					"concurrent-channel",
					"model-a",
				)
				require.NoError(t, err)
			}
		}()
	}

	wg.Wait()

	totalCount, secondCount, err := record.GetRequest(
		ctx,
		time.Minute,
		"concurrent-channel",
		"model-a",
	)
	require.NoError(t, err)
	require.Equal(t, int64(numGoroutines*requestsPerGoroutine), totalCount)
	require.Greater(t, secondCount, int64(0))
}

func TestRedisRateRecordRateLimitBoundary(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForReqLimitTest(t, ctx)
	defer cleanup()

	record := newRedisChannelModelRecord(func() *redis.Client { return redisClient })

	normalCount, overCount, _, err := record.PushRequest(
		ctx,
		2,
		time.Minute,
		1,
		"limit-channel",
		"model-b",
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), normalCount)
	require.Equal(t, int64(0), overCount)

	normalCount, overCount, _, err = record.PushRequest(
		ctx,
		2,
		time.Minute,
		1,
		"limit-channel",
		"model-b",
	)
	require.NoError(t, err)
	require.Equal(t, int64(2), normalCount)
	require.Equal(t, int64(0), overCount)

	normalCount, overCount, _, err = record.PushRequest(
		ctx,
		2,
		time.Minute,
		1,
		"limit-channel",
		"model-b",
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), normalCount)
	require.Equal(t, int64(0), overCount)

	normalCount, overCount, _, err = record.PushRequest(
		ctx,
		2,
		time.Minute,
		1,
		"limit-channel",
		"model-b",
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), normalCount)
	require.Equal(t, int64(1), overCount)
}

func TestRedisRateRecordTTLAndNaturalExpiry(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForReqLimitTest(t, ctx)
	defer cleanup()

	record := newRedisChannelModelRecord(func() *redis.Client { return redisClient })

	_, _, _, err := record.PushRequest(ctx, 0, 2*time.Second, 1, "ttl-channel", "model-c")
	require.NoError(t, err)

	bucketKey := record.buildBucketKey("ttl-channel", "model-c")
	metaKey := record.buildMetaKey("ttl-channel", "model-c")

	bucketTTL, err := redisClient.TTL(ctx, bucketKey).Result()
	require.NoError(t, err)
	require.Greater(t, bucketTTL, time.Duration(0))
	require.LessOrEqual(t, bucketTTL, 2*time.Second)

	metaTTL, err := redisClient.TTL(ctx, metaKey).Result()
	require.NoError(t, err)
	require.Greater(t, metaTTL, time.Duration(0))
	require.LessOrEqual(t, metaTTL, 2*time.Second)

	require.Eventually(t, func() bool {
		bucketExists, err := redisClient.Exists(ctx, bucketKey).Result()
		require.NoError(t, err)
		metaExists, err := redisClient.Exists(ctx, metaKey).Result()
		require.NoError(t, err)

		return bucketExists == 0 && metaExists == 0
	}, 5*time.Second, 100*time.Millisecond)
}

func TestRedisRateRecordGetDoesNotRefreshTTL(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForReqLimitTest(t, ctx)
	defer cleanup()

	record := newRedisChannelModelRecord(func() *redis.Client { return redisClient })

	_, _, _, err := record.PushRequest(ctx, 0, 2*time.Second, 1, "ttl-read-channel", "model-d")
	require.NoError(t, err)

	bucketKey := record.buildBucketKey("ttl-read-channel", "model-d")
	metaKey := record.buildMetaKey("ttl-read-channel", "model-d")

	time.Sleep(1200 * time.Millisecond)

	_, _, err = record.GetRequest(ctx, 2*time.Second, "ttl-read-channel", "model-d")
	require.NoError(t, err)

	bucketTTL, err := redisClient.TTL(ctx, bucketKey).Result()
	require.NoError(t, err)
	require.Greater(t, bucketTTL, time.Duration(0))
	require.Less(t, bucketTTL, time.Second)

	metaTTL, err := redisClient.TTL(ctx, metaKey).Result()
	require.NoError(t, err)
	require.Greater(t, metaTTL, time.Duration(0))
	require.Less(t, metaTTL, time.Second)

	require.Eventually(t, func() bool {
		bucketExists, err := redisClient.Exists(ctx, bucketKey).Result()
		require.NoError(t, err)
		metaExists, err := redisClient.Exists(ctx, metaKey).Result()
		require.NoError(t, err)

		return bucketExists == 0 && metaExists == 0
	}, 4*time.Second, 100*time.Millisecond)
}

func setupRedisForReqLimitTest(t *testing.T, ctx context.Context) (*redis.Client, func()) {
	t.Helper()

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
		_ = client.Close()
		_ = container.Terminate(ctx)
	}

	return client, cleanup
}

func seedRedisReqLimitRecord(
	ctx context.Context,
	client *redis.Client,
	key string,
	totalCount, secondCount int64,
) error {
	return seedRedisReqLimitRecordWithOffset(ctx, client, key, totalCount, secondCount, 0)
}

func seedRedisReqLimitRecordWithOffset(
	ctx context.Context,
	client *redis.Client,
	key string,
	totalCount, secondCount int64,
	secondOffset int64,
) error {
	now := time.Now().Unix() + secondOffset
	baseKey := common.RedisKey(key)
	bucketKey := baseKey + ":buckets"
	metaKey := baseKey + ":meta"
	previousSecond := now - 1

	pipe := client.Pipeline()
	if totalCount-secondCount > 0 {
		pipe.HSet(
			ctx,
			bucketKey,
			strconv.FormatInt(previousSecond, 10),
			strconv.FormatInt(totalCount-secondCount, 10)+":0",
		)
	}

	if secondCount > 0 {
		pipe.HSet(
			ctx,
			bucketKey,
			strconv.FormatInt(now, 10),
			strconv.FormatInt(secondCount, 10)+":0",
		)
	}

	pipe.HSet(ctx, metaKey,
		"total_normal", totalCount,
		"total_over", 0,
		"last_cleaned_second", now-59,
	)
	pipe.Expire(ctx, bucketKey, time.Minute)
	pipe.Expire(ctx, metaKey, time.Minute)
	_, err := pipe.Exec(ctx)

	return err
}
