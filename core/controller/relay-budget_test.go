//nolint:testpackage
package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	relaycontroller "github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/require"
)

func TestRetryBudgetStartsWithFirstAttempt(t *testing.T) {
	t.Parallel()

	started := time.Now().Add(-time.Minute)
	times, deadline := getRetryLimits(model.ModelConfig{RetryBudget: new(int64(30))}, 3, 0, started)
	require.Equal(t, -1, times)
	require.Equal(t, started.Add(30*time.Second), deadline)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	bizErr := relaymodel.NewOpenAIError(
		http.StatusBadGateway,
		relaymodel.OpenAIError{Message: "upstream failed"},
	)
	require.True(t, handleRelayResult(c, bizErr, true, times, deadline))
	require.True(t, c.Writer.Written())
}

func TestRetryLoopBudgetAndCount(t *testing.T) {
	t.Setenv("LOG_STORAGE_HOURS", "")

	previous := config.GetLogStorageHours()

	config.SetLogStorageHours(-1)
	t.Cleanup(func() { config.SetLogStorageHours(previous) })

	for _, tt := range []struct {
		name                string
		times               int
		budget              time.Duration
		attemptDuration     time.Duration
		status              int
		wantAttempts        int
		initialBackoff      bool
		cancelDuringBackoff bool
		succeed             bool
	}{
		{name: "count only", times: 2, attemptDuration: time.Second, status: http.StatusBadGateway, wantAttempts: 2},
		{name: "budget only", times: -1, budget: 3 * time.Second, attemptDuration: time.Second, status: http.StatusBadGateway, wantAttempts: 3},
		{name: "count expires first", times: 2, budget: 10 * time.Second, attemptDuration: time.Second, status: http.StatusBadGateway, wantAttempts: 2},
		{name: "budget expires first", times: 10, budget: 2 * time.Second, attemptDuration: time.Second, status: http.StatusBadGateway, wantAttempts: 2},
		{name: "deadline expires during backoff", times: -1, budget: 500 * time.Millisecond, status: http.StatusTooManyRequests, initialBackoff: true},
		{name: "cancellation interrupts backoff", times: 2, status: http.StatusTooManyRequests, initialBackoff: true, cancelDuringBackoff: true},
		{name: "in flight call completes after budget", times: -1, budget: time.Second, attemptDuration: 2 * time.Second, status: http.StatusBadGateway, wantAttempts: 1, succeed: true},
		{name: "permission errors obey count", times: 2, budget: time.Minute, status: http.StatusUnauthorized, wantAttempts: 2},
		{name: "non retryable error stops", times: -1, budget: time.Minute, status: http.StatusBadRequest, wantAttempts: 1},
		{name: "no count or budget", status: http.StatusBadGateway},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequestWithContext(
					ctx,
					http.MethodPost,
					"/",
					strings.NewReader(`{}`),
				)
				c.Set(middleware.Group, model.GroupCache{})
				c.Set(middleware.Token, model.TokenCache{})
				c.Set(middleware.ModelConfig, model.ModelConfig{})
				c.Set(middleware.RequestModel, "retry-budget-test")
				c.Set(middleware.GroupBalance, &middleware.GroupBalanceConsumer{})
				middleware.SetRequestAt(c, time.Now())

				channels := []*model.Channel{
					{ID: 1, Status: model.ChannelStatusEnabled},
					{ID: 2, Status: model.ChannelStatusEnabled, BackupOnly: true},
					{ID: 3, Status: model.ChannelStatusEnabled, BackupOnly: true},
					{ID: 4, Status: model.ChannelStatusEnabled, BackupOnly: true},
				}
				initial := &initialChannel{
					channel:          channels[0],
					migratedChannels: channels,
					preferChannelIDs: []int{1, 2, 3, 4},
				}
				initialError := relaymodel.NewOpenAIError(
					http.StatusBadGateway,
					relaymodel.OpenAIError{Message: "initial failure"},
				)

				state := initRetryState(
					tt.times,
					initial,
					NewMetaByContext(c, channels[0], mode.Responses),
					&relaycontroller.HandleResult{Error: initialError},
					model.Price{},
					time.Now(),
				)
				if tt.budget > 0 {
					state.retryDeadline = time.Now().Add(tt.budget)
				}

				if tt.initialBackoff {
					state.preferChannelIDs = []int{1}
					state.recordChannelFailure(1, time.Now())
				}

				if tt.cancelDuringBackoff {
					go func() {
						time.Sleep(200 * time.Millisecond)
						cancel()
					}()
				}

				attempts := 0
				started := time.Now()
				retryLoop(
					c,
					mode.Responses,
					state,
					func(c *gin.Context, _ *meta.Meta) *relaycontroller.HandleResult {
						attempts++

						if !state.retryDeadline.IsZero() {
							require.True(t, time.Now().Before(state.retryDeadline))
						}

						time.Sleep(tt.attemptDuration)
						require.NoError(t, c.Request.Context().Err())

						if tt.succeed {
							return &relaycontroller.HandleResult{}
						}

						return &relaycontroller.HandleResult{
							Error: relaymodel.NewOpenAIError(
								tt.status,
								relaymodel.OpenAIError{Message: "retry failure"},
							),
						}
					},
				)
				synctest.Wait()
				require.Equal(t, tt.wantAttempts, attempts)
				require.Equal(t, tt.times, state.retryTimes)

				if tt.initialBackoff {
					wantElapsed := tt.budget
					if tt.cancelDuringBackoff {
						wantElapsed = 200 * time.Millisecond
					}

					require.Equal(t, wantElapsed, time.Since(started))
					require.Equal(t, initialError, state.result.Error)
				}

				if tt.succeed {
					require.Nil(t, state.result.Error)
				} else {
					require.True(t, c.Writer.Written())
				}
			})
		})
	}
}

func TestRetryBudgetRequestBinding(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"-1", "181", "1.5"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/",
			strings.NewReader(`{"retry_budget":`+value+`}`),
		)

		var request SaveModelConfigsRequest
		require.Error(t, c.ShouldBindJSON(&request))
		c.Request = httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/",
			strings.NewReader(`{"retry_budget":`+value+`}`),
		)

		var groupRequest SaveGroupModelConfigRequest
		require.Error(t, c.ShouldBindJSON(&groupRequest))
	}

	request := SaveGroupModelConfigRequest{OverrideRetryBudget: true, RetryBudget: 60}
	group := request.ToGroupModelConfig("test")
	require.True(t, group.OverrideRetryBudget)
	require.Equal(t, int64(60), group.RetryBudget)
}
