//nolint:testpackage
package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaycontroller "github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRetryStateRemainingRelayDelay(t *testing.T) {
	t.Parallel()

	t.Run("uses per-channel failure count plus jitter", func(t *testing.T) {
		t.Parallel()

		state := &retryState{}
		base := time.Unix(100, 0)
		jitter := 400 * time.Millisecond

		state.recordChannelFailure(7, base)

		assert.Equal(t, 1400*time.Millisecond, state.remainingRelayDelay(7, base, jitter))
		assert.Equal(
			t,
			900*time.Millisecond,
			state.remainingRelayDelay(7, base.Add(500*time.Millisecond), jitter),
		)

		state.recordChannelFailure(7, base.Add(2*time.Second))

		assert.Equal(
			t,
			2400*time.Millisecond,
			state.remainingRelayDelay(7, base.Add(2*time.Second), jitter),
		)
		assert.Equal(
			t,
			1150*time.Millisecond,
			state.remainingRelayDelay(7, base.Add(3250*time.Millisecond), jitter),
		)
	})

	t.Run("returns zero after required wait has already elapsed", func(t *testing.T) {
		t.Parallel()

		state := &retryState{}
		base := time.Unix(200, 0)

		state.recordChannelFailure(9, base)

		assert.Zero(
			t,
			state.remainingRelayDelay(9, base.Add(1500*time.Millisecond), 400*time.Millisecond),
		)
		assert.Zero(t, state.remainingRelayDelay(9, base.Add(2*time.Second), 400*time.Millisecond))
	})

	t.Run("tracks each channel independently", func(t *testing.T) {
		t.Parallel()

		state := &retryState{}
		base := time.Unix(300, 0)

		state.recordChannelFailure(1, base)
		state.recordChannelFailure(2, base)
		state.recordChannelFailure(2, base.Add(100*time.Millisecond))

		assert.Equal(
			t,
			500*time.Millisecond,
			state.remainingRelayDelay(1, base.Add(800*time.Millisecond), 300*time.Millisecond),
		)
		assert.Equal(
			t,
			1500*time.Millisecond,
			state.remainingRelayDelay(2, base.Add(900*time.Millisecond), 300*time.Millisecond),
		)
		assert.Zero(t, state.remainingRelayDelay(3, base, 300*time.Millisecond))
	})

	t.Run("caps backoff at five seconds", func(t *testing.T) {
		t.Parallel()

		base := time.Unix(400, 0)
		state := &retryState{}

		for range 20 {
			state.recordChannelFailure(5, base)
		}

		assert.Equal(t, 5*time.Second, state.remainingRelayDelay(5, base, time.Second))
		assert.Zero(t, state.remainingRelayDelay(5, base.Add(5*time.Second), time.Second))
	})
}

func TestCalculateRelayBackoffDelay(t *testing.T) {
	t.Parallel()

	assert.Zero(t, calculateRelayBackoffDelay(0, 500*time.Millisecond))
	assert.Equal(t, time.Second, calculateRelayBackoffDelay(1, -time.Second))
	assert.Equal(t, 1500*time.Millisecond, calculateRelayBackoffDelay(1, 500*time.Millisecond))
	assert.Equal(t, 2*time.Second, calculateRelayBackoffDelay(1, 2*time.Second))
	assert.Equal(t, 2500*time.Millisecond, calculateRelayBackoffDelay(2, 500*time.Millisecond))
	assert.Equal(t, 5*time.Second, calculateRelayBackoffDelay(20, time.Second))
	assert.Equal(t, 2*time.Second, calculateRelayBackoffDelay(1, time.Second))
}

func TestHandleRelayResultDecidesRetryLifecycle(t *testing.T) {
	t.Parallel()

	newContext := func(ctx context.Context) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil)

		return c
	}
	err := relaymodel.NewOpenAIError(http.StatusBadGateway, relaymodel.OpenAIError{
		Message: "upstream unavailable",
	})

	tests := []struct {
		name       string
		bizErr     adaptor.Error
		retry      bool
		retryTimes int
		wantDone   bool
	}{
		{
			name:     "successful request is done",
			wantDone: true,
		},
		{
			name:       "retryable error with budget continues",
			bizErr:     err,
			retry:      true,
			retryTimes: 2,
			wantDone:   false,
		},
		{
			name:       "retry disabled finishes",
			bizErr:     err,
			retryTimes: 2,
			wantDone:   true,
		},
		{
			name:     "zero retry budget finishes",
			bizErr:   err,
			retry:    true,
			wantDone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantDone, handleRelayResult(
				newContext(context.Background()),
				tt.bizErr,
				tt.retry,
				tt.retryTimes,
				time.Time{},
			))
		})
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	assert.True(t, handleRelayResult(newContext(canceled), err, true, 2, time.Time{}))
}

func TestInitRetryStateRecordsInitialFailure(t *testing.T) {
	t.Parallel()

	ch1 := &model.Channel{ID: 1, Status: model.ChannelStatusEnabled}
	ch2 := &model.Channel{ID: 2, Status: model.ChannelStatusEnabled}
	endAt := time.Unix(500, 0)
	initial := &initialChannel{
		channel:          ch1,
		migratedChannels: []*model.Channel{ch1, ch2},
		preferChannelIDs: []int{2},
		ignoreChannelIDs: map[int64]struct{}{9: {}},
	}
	requestMeta := meta.NewMeta(ch1, mode.Responses, "gpt-5", model.ModelConfig{})
	result := &relaycontroller.HandleResult{
		Error: relaymodel.NewOpenAIError(http.StatusTooManyRequests, relaymodel.OpenAIError{
			Message: "rate limited",
		}),
	}

	state := initRetryState(3, initial, requestMeta, result, model.Price{}, endAt)

	assert.Equal(t, 3, state.retryTimes)
	assert.Equal(t, []int{2}, state.preferChannelIDs)
	assert.Equal(t, initial.ignoreChannelIDs, state.ignoreChannelIDs)
	assert.Equal(t, []*model.Channel{ch1, ch2}, state.migratedChannels)
	assert.Contains(t, state.failedChannelIDs, int64(ch1.ID))
	assert.Equal(t, 1, state.channelRetryInfo[ch1.ID].failures)
	assert.Equal(t, endAt, state.channelRetryInfo[ch1.ID].lastEndAt)
	assert.Nil(t, state.designatedChannel)
}

func TestInitRetryStateMarksInitialPermissionFailureAsIgnored(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{ID: 7, Status: model.ChannelStatusEnabled}
	requestMeta := meta.NewMeta(channel, mode.Responses, "gpt-5", model.ModelConfig{})
	result := &relaycontroller.HandleResult{
		Error: relaymodel.NewOpenAIError(http.StatusUnauthorized, relaymodel.OpenAIError{
			Message: "invalid key",
		}),
	}

	state := initRetryState(
		2,
		&initialChannel{channel: channel},
		requestMeta,
		result,
		model.Price{},
		time.Unix(600, 0),
	)

	assert.Contains(t, state.failedChannelIDs, int64(channel.ID))
	assert.Contains(t, state.ignoreChannelIDs, int64(channel.ID))
	assert.Empty(t, state.channelRetryInfo)
}

func TestInitRetryStateTracksDesignatedChannel(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{ID: 8, Status: model.ChannelStatusEnabled}
	requestMeta := meta.NewMeta(channel, mode.Responses, "gpt-5", model.ModelConfig{})
	result := &relaycontroller.HandleResult{
		Error: relaymodel.NewOpenAIError(http.StatusBadGateway, relaymodel.OpenAIError{
			Message: "upstream error",
		}),
	}

	state := initRetryState(
		1,
		&initialChannel{channel: channel, designatedChannel: true},
		requestMeta,
		result,
		model.Price{},
		time.Unix(700, 0),
	)

	assert.Same(t, channel, state.designatedChannel)
}

func TestHandleRetryResultUpdatesAutomaticRetryState(t *testing.T) {
	t.Parallel()

	newContext := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)

		return c
	}
	newState := func(err adaptor.Error) *retryState {
		return &retryState{
			retryTimes: 2,
			result:     &relaycontroller.HandleResult{Error: err},
		}
	}
	permissionError := relaymodel.NewOpenAIError(http.StatusBadGateway, relaymodel.OpenAIError{
		Message: "upstream unavailable",
	})
	noPermissionError := relaymodel.NewOpenAIError(http.StatusUnauthorized, relaymodel.OpenAIError{
		Message: "invalid key",
	})
	channel := &model.Channel{ID: 11, Status: model.ChannelStatusEnabled}

	t.Run("permissioned failure keeps retrying without hard filtering", func(t *testing.T) {
		t.Parallel()

		state := newState(permissionError)

		done := handleRetryResult(newContext(), true, channel, state)

		assert.False(t, done)
		assert.Equal(t, 2, state.retryTimes)
		assert.Empty(t, state.ignoreChannelIDs)
	})

	t.Run("permission failure is hard filtered within retry limit", func(t *testing.T) {
		t.Parallel()

		state := newState(noPermissionError)

		done := handleRetryResult(newContext(), true, channel, state)

		assert.False(t, done)
		assert.Equal(t, 2, state.retryTimes)
		assert.Contains(t, state.ignoreChannelIDs, int64(channel.ID))
	})

	t.Run("retry disabled finishes immediately", func(t *testing.T) {
		t.Parallel()

		state := newState(permissionError)

		done := handleRetryResult(newContext(), false, channel, state)

		assert.True(t, done)
	})

	t.Run("nil result error finishes immediately", func(t *testing.T) {
		t.Parallel()

		state := newState(nil)

		done := handleRetryResult(newContext(), true, channel, state)

		assert.True(t, done)
	})
}

func TestHandleRetryResultKeepsDesignatedChannelSemantics(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	channel := &model.Channel{ID: 12, Status: model.ChannelStatusEnabled}

	permissionedState := &retryState{
		designatedChannel: channel,
		result: &relaycontroller.HandleResult{
			Error: relaymodel.NewOpenAIError(http.StatusBadGateway, relaymodel.OpenAIError{}),
		},
	}
	assert.False(t, handleRetryResult(c, true, channel, permissionedState))

	noPermissionState := &retryState{
		designatedChannel: channel,
		result: &relaycontroller.HandleResult{
			Error: relaymodel.NewOpenAIError(http.StatusForbidden, relaymodel.OpenAIError{}),
		},
	}
	assert.True(t, handleRetryResult(c, true, channel, noPermissionState))
	assert.Empty(t, noPermissionState.ignoreChannelIDs)
}

func TestHandleRetryResultStopsOnCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil)

	cancel()

	state := &retryState{
		result: &relaycontroller.HandleResult{
			Error: relaymodel.NewOpenAIError(http.StatusBadGateway, relaymodel.OpenAIError{}),
		},
	}

	assert.True(t, handleRetryResult(c, true, &model.Channel{ID: 13}, state))
}

func TestRelayControllerVideoModesValidateRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode mode.Mode
		want ValidateRequest
	}{
		{
			name: "video generation jobs",
			mode: mode.VideoGenerationsJobs,
			want: relaycontroller.ValidateVideoGenerationJobRequest,
		},
		{
			name: "videos",
			mode: mode.Videos,
			want: relaycontroller.ValidateVideosRequest,
		},
		{
			name: "videos remix",
			mode: mode.VideosRemix,
			want: relaycontroller.ValidateVideosRequest,
		},
		{
			name: "gemini video",
			mode: mode.GeminiVideo,
			want: relaycontroller.ValidateGeminiVideoRequest,
		},
		{
			name: "ali native video",
			mode: mode.AliVideo,
			want: relaycontroller.ValidateAliVideoRequest,
		},
		{
			name: "doubao native video",
			mode: mode.DoubaoVideo,
			want: relaycontroller.ValidateDoubaoVideoRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc := relayController(tt.mode)
			require.NotNil(t, rc.ValidateRequest)
			require.Equal(
				t,
				reflect.ValueOf(tt.want).Pointer(),
				reflect.ValueOf(rc.ValidateRequest).Pointer(),
			)
		})
	}
}

func TestSaveAsyncUsageInfoDoesNotStoreInitialUsage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AsyncUsageInfo{}))

	oldLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = oldLogDB
	})

	m := meta.NewMeta(
		&model.Channel{ID: 11, BaseURL: "https://example.com"},
		mode.Videos,
		"test-video-model",
		model.ModelConfig{},
		meta.WithRequestID("request-async-1"),
		meta.WithRequestUsage(model.Usage{
			OutputTokens: 9,
			TotalTokens:  9,
		}),
		meta.WithRequestUsageContext(model.UsageContext{
			ServiceTier: "priority",
		}),
		meta.WithGroup(model.GroupCache{ID: "group-1"}),
		meta.WithToken(model.TokenCache{ID: 22, Name: "token-1"}),
	)

	saveAsyncUsageInfo(m, model.Price{}, &relaycontroller.HandleResult{
		UpstreamID: "video-123",
		Usage: model.Usage{
			OutputTokens: 99,
			TotalTokens:  99,
		},
	})

	var captured model.AsyncUsageInfo
	require.NoError(t, db.Where("upstream_id = ?", "video-123").First(&captured).Error)
	require.Zero(t, captured.Usage.OutputTokens)
	require.Zero(t, captured.Usage.TotalTokens)
	require.Equal(t, "priority", captured.UsageContext.ServiceTier)
}

func TestBuildRequestDetailForLogSkipsRequestBodyForUpstreamOnlyStatuses(t *testing.T) {
	t.Parallel()

	bodyDetail := &relaycontroller.BodyDetail{
		RequestBody:  `{"prompt":"secret"}`,
		ResponseBody: `{"error":"upstream"}`,
	}

	for _, statusCode := range []int{
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusTooManyRequests,
	} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			t.Parallel()

			detail := buildRequestDetailForLog(bodyDetail, model.ModelConfig{}, statusCode, false)

			require.NotNil(t, detail)
			assert.Empty(t, detail.RequestBody)
			assert.Equal(t, `{"error":"upstream"}`, detail.ResponseBody)
		})
	}
}

func TestBuildRequestDetailForLogKeepsRequestBodyWhenForced(t *testing.T) {
	t.Parallel()

	detail := buildRequestDetailForLog(
		&relaycontroller.BodyDetail{
			RequestBody:  `{"prompt":"secret"}`,
			ResponseBody: `{"error":"limited"}`,
		},
		model.ModelConfig{},
		http.StatusTooManyRequests,
		true,
	)

	require.NotNil(t, detail)
	assert.Equal(t, `{"prompt":"secret"}`, detail.RequestBody)
	assert.Equal(t, `{"error":"limited"}`, detail.ResponseBody)
}

func TestBuildRequestDetailForLogKeepsRequestBodyForClientPayloadErrors(t *testing.T) {
	t.Parallel()

	detail := buildRequestDetailForLog(
		&relaycontroller.BodyDetail{
			RequestBody:  `{"prompt":"secret"}`,
			ResponseBody: `{"error":"bad request"}`,
		},
		model.ModelConfig{},
		http.StatusBadRequest,
		false,
	)

	require.NotNil(t, detail)
	assert.Equal(t, `{"prompt":"secret"}`, detail.RequestBody)
	assert.Equal(t, `{"error":"bad request"}`, detail.ResponseBody)
}

func TestBuildRequestDetailForLogDropsInvalidUTF8Bodies(t *testing.T) {
	t.Parallel()

	detail := buildRequestDetailForLog(
		&relaycontroller.BodyDetail{
			RequestBody:  string([]byte{0xff, 0xfe}),
			ResponseBody: string([]byte{'o', 'k', 0xff}),
		},
		model.ModelConfig{},
		http.StatusBadRequest,
		false,
	)

	require.NotNil(t, detail)
	assert.Empty(t, detail.RequestBody)
	assert.Empty(t, detail.ResponseBody)
}
