//nolint:testpackage
package monitor

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/model"
	modelmonitor "github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymeta "github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/require"
)

type doResponseFunc func(
	meta *relaymeta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error)

func (f doResponseFunc) DoResponse(
	meta *relaymeta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return f(meta, store, c, resp)
}

type doRequestFunc func(
	meta *relaymeta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) (*http.Response, error)

func (f doRequestFunc) DoRequest(
	meta *relaymeta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return f(meta, store, c, req)
}

func TestGetChannelWarnErrorRateUsesChannelValueEvenWhenAutoBalanceDisabled(t *testing.T) {
	meta := &relaymeta.Meta{}
	meta.Channel.WarnErrorRate = 0.42
	meta.Channel.EnabledAutoBalanceCheck = false
	meta.Channel.MaxErrorRate = 0.95

	require.InDelta(t, 0.42, getChannelWarnErrorRate(meta), 0.0001)
	require.InDelta(t, 0.95, getChannelMaxErrorRate(meta), 0.0001)
}

func TestGetChannelWarnErrorRateFallsBackToDefault(t *testing.T) {
	previous := config.GetDefaultWarnNotifyErrorRate()
	config.SetDefaultWarnNotifyErrorRate(previous)
	t.Cleanup(func() {
		config.SetDefaultWarnNotifyErrorRate(previous)
	})

	config.SetDefaultWarnNotifyErrorRate(0.37)

	require.InDelta(t, 0.37, getChannelWarnErrorRate(&relaymeta.Meta{}), 0.0001)
}

func TestGetChannelMaxErrorRateDoesNotDependOnBalanceCheckSwitch(t *testing.T) {
	meta := &relaymeta.Meta{}
	meta.Channel.WarnErrorRate = 0.25
	meta.Channel.MaxErrorRate = 0.88

	require.InDelta(t, 0.88, getChannelMaxErrorRate(meta), 0.0001)

	meta.Channel.EnabledAutoBalanceCheck = true

	require.InDelta(t, 0.88, getChannelMaxErrorRate(meta), 0.0001)
}

func TestShouldTryBanNoPermissionRequiresChannelSwitch(t *testing.T) {
	meta := &relaymeta.Meta{}

	require.False(t, shouldTryBanNoPermission(meta, false))

	meta.Channel.EnabledNoPermissionBan = true

	require.True(t, shouldTryBanNoPermission(meta, false))
	require.False(t, shouldTryBanNoPermission(meta, true))
}

func TestChannelStatusHasPermission(t *testing.T) {
	t.Parallel()

	for _, statusCode := range []int{
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
	} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			t.Parallel()

			require.False(t, ChannelStatusHasPermission(statusCode))
		})
	}

	require.True(t, ChannelStatusHasPermission(http.StatusBadRequest))
}

func TestChannelHasPermissionForForbiddenErrorCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
		want bool
	}{
		{
			name: "request blocked by cyber policy",
			code: "session_blocked_by_cyber_policy",
			want: true,
		},
		{
			name: "channel permission failure",
			code: "insufficient_user_quota",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			relayErr := relaymodel.NewOpenAIError(http.StatusForbidden, relaymodel.OpenAIError{
				Code:    tt.code,
				Message: "forbidden",
				Type:    relaymodel.ErrorTypeUpstream,
			})

			require.Equal(t, tt.want, ChannelHasPermission(relayErr))
		})
	}
}

type countingNotifier struct {
	count atomic.Int64
}

func (n *countingNotifier) Notify(notify.Level, string, string) {
	n.count.Add(1)
}

func (n *countingNotifier) NotifyThrottle(
	notify.Level,
	string,
	time.Duration,
	string,
	string,
) {
	n.count.Add(1)
}

func TestChannelMonitorSkipsNotifyForGroupChannelErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	notifier := &countingNotifier{}
	notify.SetDefaultNotifier(notifier)
	t.Cleanup(func() {
		notify.SetDefaultNotifier(&notify.StdNotifier{})
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	common.SetLogger(c.Request, common.NewLogger())

	meta := relaymeta.NewMeta(nil, 0, "group-channel-error-model", model.ModelConfig{})
	meta.Channel.Scope = model.ChannelScopeGroup
	meta.Channel.GroupID = "group-channel-error"
	meta.Channel.ID = 91

	monitor := &ChannelMonitor{}
	result, relayErr := monitor.DoResponse(
		meta,
		nil,
		c,
		nil,
		doResponseFunc(func(
			_ *relaymeta.Meta,
			_ adaptor.Store,
			_ *gin.Context,
			_ *http.Response,
		) (adaptor.DoResponseResult, adaptor.Error) {
			return adaptor.DoResponseResult{}, adaptor.NewError(
				http.StatusInternalServerError,
				"upstream failed",
			)
		}),
	)

	require.Zero(t, result.Usage.TotalTokens)
	require.NotNil(t, relayErr)
	require.Zero(t, notifier.count.Load())
}

func TestGroupMonitorDoResponseUsesGroupChannelCounters(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	groupID := "group-monitor-channel-isolated"
	modelName := "group-monitor-channel-model"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	common.SetLogger(c.Request, common.NewLogger())

	meta := relaymeta.NewMeta(nil, 0, modelName, model.ModelConfig{})
	meta.Group = model.GroupCache{ID: groupID, Status: model.GroupStatusEnabled}
	meta.Channel.Scope = model.ChannelScopeGroup
	meta.Channel.GroupID = groupID
	meta.Channel.ID = 88

	groupMonitor := &GroupMonitor{}
	channelMonitor := &ChannelMonitor{}
	result, relayErr := groupMonitor.DoResponse(
		meta,
		nil,
		c,
		nil,
		doResponseFunc(func(
			_ *relaymeta.Meta,
			_ adaptor.Store,
			_ *gin.Context,
			_ *http.Response,
		) (adaptor.DoResponseResult, adaptor.Error) {
			return channelMonitor.DoResponse(
				meta,
				nil,
				c,
				nil,
				doResponseFunc(func(
					_ *relaymeta.Meta,
					_ adaptor.Store,
					_ *gin.Context,
					_ *http.Response,
				) (adaptor.DoResponseResult, adaptor.Error) {
					return adaptor.DoResponseResult{
						Usage: model.Usage{TotalTokens: 23},
					}, nil
				}),
			)
		}),
	)
	require.Nil(t, relayErr)
	require.Equal(t, model.Usage{TotalTokens: 23}, result.Usage)

	globalTPM, _ := reqlimit.GetGroupModelTokensRequest(t.Context(), groupID, modelName)
	require.Zero(t, globalTPM)

	groupChannelTPM, _ := reqlimit.GetGroupChannelModelTokensRequest(
		t.Context(),
		groupID,
		"88",
		modelName,
	)
	require.Equal(t, int64(23), groupChannelTPM)

	channelTPM, _ := reqlimit.GetChannelModelTokensRequest(
		t.Context(),
		meta.ChannelMonitorKey(),
		modelName,
	)
	require.Zero(t, channelTPM)

	log := common.GetLogger(c)
	require.Equal(t, int64(23), log.Data["ch_tpm"])
	require.Equal(t, int64(23), log.Data["ch_tps"])
	require.Equal(t, "23", log.Data["group_channel_tpm"])
	require.Equal(t, "23", log.Data["group_channel_tps"])
}

func TestChannelMonitorUsesGroupChannelRealtimeNamespace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := "group-channel-monitor"
	modelName := "group-channel-monitor-model"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	common.SetLogger(c.Request, common.NewLogger())

	meta := relaymeta.NewMeta(nil, 0, modelName, model.ModelConfig{})
	meta.Group = model.GroupCache{ID: groupID, Status: model.GroupStatusEnabled}
	meta.Channel.Scope = model.ChannelScopeGroup
	meta.Channel.GroupID = groupID
	meta.Channel.ID = 89

	_, _, _ = reqlimit.PushGroupChannelModelRequest(
		t.Context(),
		groupID,
		strconv.Itoa(meta.Channel.ID),
		modelName,
	)

	monitor := &ChannelMonitor{}
	resp, err := monitor.DoRequest(
		meta,
		nil,
		c,
		c.Request,
		doRequestFunc(func(
			_ *relaymeta.Meta,
			_ adaptor.Store,
			_ *gin.Context,
			_ *http.Request,
		) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		}),
	)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	groupChannelRPM, _ := reqlimit.GetGroupChannelModelRequest(
		t.Context(),
		groupID,
		strconv.Itoa(meta.Channel.ID),
		modelName,
	)
	require.Equal(t, int64(1), groupChannelRPM)

	prefixedGroupChannelRPM, _ := reqlimit.GetGroupChannelModelRequest(
		t.Context(),
		groupID,
		meta.ChannelMonitorKey(),
		modelName,
	)
	require.Zero(t, prefixedGroupChannelRPM)

	channelRPM, _ := reqlimit.GetChannelModelRequest(
		t.Context(),
		meta.ChannelMonitorKey(),
		modelName,
	)
	require.Zero(t, channelRPM)

	_, _, err = modelmonitor.AddGroupChannelRequestByChannelKey(
		t.Context(),
		modelName,
		meta.ChannelMonitorKey(),
		true,
		true,
		0,
	)
	require.NoError(t, err)

	globalBanned, err := modelmonitor.GetBannedChannelKeysMapWithModel(t.Context(), modelName)
	require.NoError(t, err)
	require.Empty(t, globalBanned)

	groupBanned, err := modelmonitor.GetGroupChannelBannedChannelKeysMapWithModel(
		t.Context(),
		modelName,
	)
	require.NoError(t, err)
	require.Contains(t, groupBanned, meta.ChannelMonitorKey())
}

func TestChannelMonitorReadsGroupChannelRetryCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := "group-channel-monitor-retry"
	modelName := "group-channel-monitor-retry-model"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	common.SetLogger(c.Request, common.NewLogger())

	meta := relaymeta.NewMeta(nil, 0, modelName, model.ModelConfig{})
	meta.Group = model.GroupCache{ID: groupID, Status: model.GroupStatusEnabled}
	meta.Channel.Scope = model.ChannelScopeGroup
	meta.Channel.GroupID = groupID
	meta.Channel.ID = 90
	meta.RetryAt = time.Now()

	_, _, _ = reqlimit.PushGroupChannelModelRequest(
		t.Context(),
		groupID,
		strconv.Itoa(meta.Channel.ID),
		modelName,
	)

	monitor := &ChannelMonitor{}
	resp, err := monitor.DoRequest(
		meta,
		nil,
		c,
		c.Request,
		doRequestFunc(func(
			_ *relaymeta.Meta,
			_ adaptor.Store,
			_ *gin.Context,
			_ *http.Request,
		) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		}),
	)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	groupChannelRPM, _ := reqlimit.GetGroupChannelModelRequest(
		t.Context(),
		groupID,
		strconv.Itoa(meta.Channel.ID),
		modelName,
	)
	require.Equal(t, int64(1), groupChannelRPM)
}
