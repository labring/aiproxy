//nolint:testpackage
package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCollectGroupOverviewMetrics(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForControllerTest(t, ctx)
	defer cleanup()

	withRedisForControllerTest(t, redisClient, func(testCtx *gin.Context) {
		seedGroupMetricFixture(t, testCtx.Request.Context())

		metrics, err := collectGroupOverviewMetrics(testCtx, []string{"g1", "g2"})
		require.NoError(t, err)
		require.Equal(t, map[string]RuntimeRateMetric{
			"g1": {RPM: 6, TPM: 300, RPS: 6, TPS: 300},
			"g2": {RPM: 4, TPM: 200, RPS: 4, TPS: 200},
		}, metrics)
	})
}

func TestCollectGroupTokenMetrics(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForControllerTest(t, ctx)
	defer cleanup()

	withRedisForControllerTest(t, redisClient, func(testCtx *gin.Context) {
		seedGroupMetricFixture(t, testCtx.Request.Context())

		metrics, err := collectGroupTokenMetrics(testCtx, "g1")
		require.NoError(t, err)
		require.Equal(t, map[string]RuntimeRateMetric{
			"t1": {RPM: 3, TPM: 150, RPS: 3, TPS: 150},
			"t2": {RPM: 3, TPM: 150, RPS: 3, TPS: 150},
		}, metrics)
	})
}

func TestCollectGroupModelMetrics(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForControllerTest(t, ctx)
	defer cleanup()

	withRedisForControllerTest(t, redisClient, func(testCtx *gin.Context) {
		seedGroupMetricFixture(t, testCtx.Request.Context())

		metrics, err := collectGroupModelMetrics(testCtx, "g1")
		require.NoError(t, err)
		require.Equal(t, map[string]RuntimeRateMetric{
			"m1": {RPM: 5, TPM: 250, RPS: 5, TPS: 250},
			"m2": {RPM: 1, TPM: 50, RPS: 1, TPS: 50},
		}, metrics)
	})
}

func TestCollectGroupTokennameModelMetrics(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForControllerTest(t, ctx)
	defer cleanup()

	withRedisForControllerTest(t, redisClient, func(testCtx *gin.Context) {
		seedGroupMetricFixture(t, testCtx.Request.Context())

		items, err := collectGroupTokennameModelMetrics(testCtx, "g1")
		require.NoError(t, err)

		sort.Slice(items, func(i, j int) bool {
			if items[i].TokenName != items[j].TokenName {
				return items[i].TokenName < items[j].TokenName
			}
			return items[i].Model < items[j].Model
		})

		require.Equal(t, []GroupTokennameModelMetricsItem{
			{
				Group:             "g1",
				TokenName:         "t1",
				Model:             "m1",
				RuntimeRateMetric: RuntimeRateMetric{RPM: 2, TPM: 100, RPS: 2, TPS: 100},
			},
			{
				Group:             "g1",
				TokenName:         "t1",
				Model:             "m2",
				RuntimeRateMetric: RuntimeRateMetric{RPM: 1, TPM: 50, RPS: 1, TPS: 50},
			},
			{
				Group:             "g1",
				TokenName:         "t2",
				Model:             "m1",
				RuntimeRateMetric: RuntimeRateMetric{RPM: 3, TPM: 150, RPS: 3, TPS: 150},
			},
		}, items)
	})
}

func TestCollectGroupTokenOverviewMetrics(t *testing.T) {
	ctx := context.Background()

	redisClient, cleanup := setupRedisForControllerTest(t, ctx)
	defer cleanup()

	withRedisForControllerTest(t, redisClient, func(testCtx *gin.Context) {
		seedGroupMetricFixture(t, testCtx.Request.Context())

		items, err := collectGroupTokenOverviewMetrics(testCtx, []BatchGroupTokenMetricsRequestItem{
			{Group: "g1", TokenName: "t1"},
			{Group: "g2", TokenName: "t3"},
			{Group: "", TokenName: "ignored"},
		})
		require.NoError(t, err)
		require.Equal(t, []BatchGroupTokenMetricsItem{
			{
				Group:             "g1",
				TokenName:         "t1",
				RuntimeRateMetric: RuntimeRateMetric{RPM: 3, TPM: 150, RPS: 3, TPS: 150},
			},
			{
				Group:             "g2",
				TokenName:         "t3",
				RuntimeRateMetric: RuntimeRateMetric{RPM: 4, TPM: 200, RPS: 4, TPS: 200},
			},
		}, items)
	})
}

func seedGroupMetricFixture(t *testing.T, ctx context.Context) {
	t.Helper()

	for range 2 {
		_, _, _ = reqlimit.PushGroupModelRequest(ctx, "g1", "m1", 0)
		_, _, _ = reqlimit.PushGroupModelTokennameRequest(ctx, "g1", "m1", "t1")
		_, _, _ = reqlimit.PushGroupModelTokensRequest(ctx, "g1", "m1", 0, 50)
		_, _, _ = reqlimit.PushGroupModelTokennameTokensRequest(ctx, "g1", "m1", "t1", 50)
	}

	for range 1 {
		_, _, _ = reqlimit.PushGroupModelRequest(ctx, "g1", "m2", 0)
		_, _, _ = reqlimit.PushGroupModelTokennameRequest(ctx, "g1", "m2", "t1")
		_, _, _ = reqlimit.PushGroupModelTokensRequest(ctx, "g1", "m2", 0, 50)
		_, _, _ = reqlimit.PushGroupModelTokennameTokensRequest(ctx, "g1", "m2", "t1", 50)
	}

	for range 3 {
		_, _, _ = reqlimit.PushGroupModelRequest(ctx, "g1", "m1", 0)
		_, _, _ = reqlimit.PushGroupModelTokennameRequest(ctx, "g1", "m1", "t2")
		_, _, _ = reqlimit.PushGroupModelTokensRequest(ctx, "g1", "m1", 0, 50)
		_, _, _ = reqlimit.PushGroupModelTokennameTokensRequest(ctx, "g1", "m1", "t2", 50)
	}

	for range 4 {
		_, _, _ = reqlimit.PushGroupModelRequest(ctx, "g2", "m3", 0)
		_, _, _ = reqlimit.PushGroupModelTokennameRequest(ctx, "g2", "m3", "t3")
		_, _, _ = reqlimit.PushGroupModelTokensRequest(ctx, "g2", "m3", 0, 50)
		_, _, _ = reqlimit.PushGroupModelTokennameTokensRequest(ctx, "g2", "m3", "t3", 50)
	}
}

func withRedisForControllerTest(t *testing.T, client *redis.Client, fn func(*gin.Context)) {
	t.Helper()

	prevEnabled := common.RedisEnabled
	prevRDB := common.RDB
	common.RedisEnabled = true

	common.RDB = client
	defer func() {
		common.RedisEnabled = prevEnabled
		common.RDB = prevRDB
	}()

	w := httptest.NewRecorder()
	testCtx, _ := gin.CreateTestContext(w)

	reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCtx.Request = httptest.NewRequestWithContext(reqCtx, http.MethodGet, "/monitor", nil)

	fn(testCtx)
}

func setupRedisForControllerTest(t *testing.T, ctx context.Context) (*redis.Client, func()) {
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
				err = redisUnavailableError(r)
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

func redisUnavailableError(v any) error {
	return &redisTestUnavailableError{cause: v}
}

type redisTestUnavailableError struct {
	cause any
}

func (e *redisTestUnavailableError) Error() string {
	return "docker unavailable for redis test"
}
