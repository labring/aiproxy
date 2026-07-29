package monitor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/noop"
)

var _ plugin.Plugin = (*ChannelMonitor)(nil)

type ChannelMonitor struct {
	noop.Noop
}

func NewChannelMonitorPlugin() plugin.Plugin {
	return &ChannelMonitor{}
}

func channelMonitorKey(meta *meta.Meta) string {
	if meta == nil {
		return "0"
	}
	return meta.ChannelMonitorKey()
}

func channelRateKey(meta *meta.Meta) string {
	if meta != nil && meta.Channel.Scope == model.ChannelScopeGroup {
		return strconv.Itoa(meta.Channel.ID)
	}

	return channelMonitorKey(meta)
}

func channelRateScope(meta *meta.Meta) reqlimit.ChannelRateScope {
	if meta != nil && meta.Channel.Scope == model.ChannelScopeGroup {
		return reqlimit.ChannelRateScopeGroup
	}

	return reqlimit.ChannelRateScopeGlobal
}

func isGroupChannelMeta(meta *meta.Meta) bool {
	return meta != nil && meta.Channel.Scope == model.ChannelScopeGroup
}

func observeChannelModelRequestRate(
	meta *meta.Meta,
) (int64, int64, int64) {
	if isGroupChannelMeta(meta) {
		count, secondCount := reqlimit.GetGroupChannelModelRequest(
			context.Background(),
			meta.Channel.GroupID,
			channelRateKey(meta),
			meta.OriginModel,
		)

		return count, 0, secondCount
	}

	return reqlimit.PushScopedChannelModelRequest(
		context.Background(),
		channelRateScope(meta),
		meta.Channel.GroupID,
		channelRateKey(meta),
		meta.OriginModel,
	)
}

func observeChannelModelTokenRate(
	meta *meta.Meta,
	tokens int64,
) (int64, int64, int64) {
	return reqlimit.PushScopedChannelModelTokensRequest(
		context.Background(),
		channelRateScope(meta),
		meta.Channel.GroupID,
		channelRateKey(meta),
		meta.OriginModel,
		tokens,
	)
}

func addChannelMonitorRequest(
	meta *meta.Meta,
	isError, tryBan bool,
	maxErrorRate float64,
) (float64, bool, error) {
	if isGroupChannelMeta(meta) {
		return monitor.AddGroupChannelRequestByChannelKey(
			context.Background(),
			meta.OriginModel,
			meta.ChannelMonitorKey(),
			isError,
			tryBan,
			maxErrorRate,
		)
	}

	return monitor.AddRequestByChannelKey(
		context.Background(),
		meta.OriginModel,
		meta.ChannelMonitorKey(),
		isError,
		tryBan,
		maxErrorRate,
	)
}

func getChannelRequestRate(meta *meta.Meta) (int64, int64) {
	if isGroupChannelMeta(meta) {
		return reqlimit.GetGroupChannelModelRequest(
			context.Background(),
			meta.Channel.GroupID,
			channelRateKey(meta),
			meta.OriginModel,
		)
	}

	return reqlimit.GetChannelModelRequest(
		context.Background(),
		channelMonitorKey(meta),
		meta.OriginModel,
	)
}

func getChannelTokenRate(meta *meta.Meta) (int64, int64) {
	if isGroupChannelMeta(meta) {
		return reqlimit.GetGroupChannelModelTokensRequest(
			context.Background(),
			meta.Channel.GroupID,
			channelRateKey(meta),
			meta.OriginModel,
		)
	}

	return reqlimit.GetChannelModelTokensRequest(
		context.Background(),
		channelMonitorKey(meta),
		meta.OriginModel,
	)
}

var channelNoRetryStatusCodesMap = map[int]struct{}{
	http.StatusBadRequest:                 {},
	http.StatusRequestEntityTooLarge:      {},
	http.StatusUnprocessableEntity:        {},
	http.StatusUnavailableForLegalReasons: {},
}

func ShouldRetry(relayErr adaptor.Error) bool {
	_, ok := channelNoRetryStatusCodesMap[relayErr.StatusCode()]
	return !ok
}

var channelNoPermissionStatusCodesMap = map[int]struct{}{
	http.StatusUnauthorized:    {},
	http.StatusPaymentRequired: {},
	http.StatusForbidden:       {},
	http.StatusNotFound:        {},
}

var forbiddenHasPermissionErrorCodesMap = map[string]struct{}{
	"session_blocked_by_cyber_policy": {},
}

func ChannelStatusHasPermission(statusCode int) bool {
	_, ok := channelNoPermissionStatusCodesMap[statusCode]
	return !ok
}

func ChannelHasPermission(relayErr adaptor.Error) bool {
	if ChannelStatusHasPermission(relayErr.StatusCode()) {
		return true
	}

	if relayErr.StatusCode() != http.StatusForbidden {
		return false
	}

	provider, ok := relayErr.(adaptor.ErrorCodeProvider)
	if !ok {
		return false
	}

	errorCode, ok := provider.ErrorCode().(string)
	if !ok {
		return false
	}

	_, ok = forbiddenHasPermissionErrorCodesMap[errorCode]

	return ok
}

func getRequestDuration(meta *meta.Meta) time.Duration {
	requestAt, ok := meta.Get("requestAt")
	if !ok {
		return 0
	}

	requestAtTime, ok := requestAt.(time.Time)
	if !ok {
		return 0
	}

	return common.TruncateDuration(time.Since(requestAtTime))
}

func (m *ChannelMonitor) DoRequest(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
	do adaptor.DoRequest,
) (*http.Response, error) {
	count, overLimitCount, secondCount := observeChannelModelRequestRate(meta)
	updateChannelModelRequestRate(c, meta, count+overLimitCount, secondCount)

	requestAt := time.Now()
	meta.Set("requestAt", requestAt)

	resp, err := do.DoRequest(meta, store, c, req)

	requestCost := common.TruncateDuration(time.Since(requestAt))
	log := common.GetLogger(c)
	log.Data["req_cost"] = requestCost.String()

	if err == nil {
		return resp, nil
	}

	var adaptorErr adaptor.Error

	ok := errors.As(err, &adaptorErr)
	if ok {
		if !ShouldRetry(adaptorErr) {
			return resp, err
		}

		handleAdaptorError(meta, c, adaptorErr)
	} else {
		handleDoRequestError(meta, c, err, requestCost)
	}

	return resp, err
}

func handleDoRequestError(meta *meta.Meta, c *gin.Context, err error, requestCost time.Duration) {
	warnErrorRate := getChannelWarnErrorRate(meta)
	maxErrorRate := getChannelMaxErrorRate(meta)

	errorRate, banExecution, _err := addChannelMonitorRequest(meta, true, false, maxErrorRate)
	if _err != nil {
		common.GetLogger(c).Errorf("add request failed: %+v", _err)
	}

	if isGroupChannelMeta(meta) {
		common.GetLogger(c).WithError(err).Warn("group channel request failed")
		return
	}

	switch {
	case banExecution:
		notifyChannelRequestIssue(
			meta,
			"autoBanned",
			"Auto Banned",
			err,
			requestCost,
			time.Minute*15,
		)
	case shouldNotifyErrorRate(warnErrorRate, errorRate):
		notifyChannelRequestIssue(
			meta,
			"beyondThreshold",
			"Error Rate Beyond Threshold",
			err,
			requestCost,
			time.Minute*15,
		)
	}
}

func notifyChannelRequestIssue(
	meta *meta.Meta,
	issueType, titleSuffix string,
	err error,
	requestCost time.Duration,
	interval time.Duration,
) {
	if isGroupChannelMeta(meta) {
		return
	}

	var notifyFunc func(title, message string)

	lockKey := fmt.Sprintf(
		"%s:%d:%s:%s",
		issueType,
		meta.Channel.ID,
		meta.OriginModel,
		issueType,
	)
	switch issueType {
	case "beyondThreshold":
		notifyFunc = func(title, message string) {
			notify.WarnThrottle(lockKey, interval, title, message)
		}
	default:
		notifyFunc = func(title, message string) {
			notify.ErrorThrottle(lockKey, interval, title, message)
		}
	}

	message := fmt.Sprintf(
		"channel: %s (type: %d, type name: %s, id: %d)\nmodel: %s\nmode: %s\nerror: %s\nrequest id: %s\ntime cost: %s",
		meta.Channel.Name,
		meta.Channel.Type,
		meta.Channel.Type.String(),
		meta.Channel.ID,
		meta.OriginModel,
		meta.Mode,
		err.Error(),
		meta.RequestID,
		requestCost.String(),
	)

	notifyFunc(
		fmt.Sprintf("%s `%s` %s", meta.Channel.Name, meta.OriginModel, titleSuffix),
		message,
	)
}

func (m *ChannelMonitor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
) (adaptor.DoResponseResult, adaptor.Error) {
	result, relayErr := do.DoResponse(meta, store, c, resp)

	if result.Usage.TotalTokens > 0 {
		count, overLimitCount, secondCount := observeChannelModelTokenRate(
			meta,
			int64(result.Usage.TotalTokens),
		)
		updateChannelModelTokensRequestRate(c, meta, count+overLimitCount, secondCount)
	}

	if relayErr == nil {
		maxErrorRate := getChannelMaxErrorRate(meta)
		if _, _, err := addChannelMonitorRequest(meta, false, false, maxErrorRate); err != nil {
			common.GetLogger(c).Errorf("add request failed: %+v", err)
		}

		return result, nil
	}

	if !ShouldRetry(relayErr) {
		return result, relayErr
	}

	handleAdaptorError(meta, c, relayErr)

	return result, relayErr
}

func handleAdaptorError(meta *meta.Meta, c *gin.Context, relayErr adaptor.Error) {
	hasPermission := ChannelHasPermission(relayErr)
	warnErrorRate := getChannelWarnErrorRate(meta)
	maxErrorRate := getChannelMaxErrorRate(meta)
	tryBanNoPermission := shouldTryBanNoPermission(meta, hasPermission)

	errorRate, banExecution, err := addChannelMonitorRequest(
		meta,
		true,
		tryBanNoPermission,
		maxErrorRate,
	)
	if err != nil {
		common.GetLogger(c).Errorf("add request failed: %+v", err)
	}

	if isGroupChannelMeta(meta) {
		common.GetLogger(c).WithError(relayErr).Warn("group channel response failed")
		return
	}

	switch {
	case banExecution:
		notifyChannelResponseIssue(c, meta, "autoBanned", "Auto Banned", relayErr, time.Minute*15)
	case shouldNotifyErrorRate(warnErrorRate, errorRate):
		notifyChannelResponseIssue(
			c,
			meta,
			"beyondThreshold",
			"Error Rate Beyond Threshold",
			relayErr,
			time.Minute*15,
		)
	case !hasPermission:
		notifyChannelResponseIssue(
			c,
			meta,
			"channelHasPermission",
			"No Permission",
			relayErr,
			time.Minute*15,
		)
	}
}

func getChannelWarnErrorRate(meta *meta.Meta) float64 {
	if meta != nil && meta.Channel.WarnErrorRate > 0 {
		return meta.Channel.WarnErrorRate
	}

	return config.GetDefaultWarnNotifyErrorRate()
}

func getChannelMaxErrorRate(meta *meta.Meta) float64 {
	if meta == nil {
		return 0
	}

	return meta.Channel.MaxErrorRate
}

func shouldTryBanNoPermission(meta *meta.Meta, hasPermission bool) bool {
	return meta != nil && meta.Channel.EnabledNoPermissionBan && !hasPermission
}

func shouldNotifyErrorRate(warnErrorRate, errorRate float64) bool {
	return warnErrorRate > 0 && errorRate >= warnErrorRate
}

func notifyChannelResponseIssue(
	c *gin.Context,
	meta *meta.Meta,
	issueType, titleSuffix string,
	err adaptor.Error,
	interval time.Duration,
) {
	if isGroupChannelMeta(meta) {
		return
	}

	var notifyFunc func(title, message string)

	lockKey := fmt.Sprintf(
		"%s:%d:%s:%s:%d",
		issueType,
		meta.Channel.ID,
		meta.OriginModel,
		issueType,
		err.StatusCode(),
	)
	switch issueType {
	case "beyondThreshold", "requestRateLimitExceeded":
		notifyFunc = func(title, message string) {
			notify.WarnThrottle(lockKey, interval, title, message)
		}
	default:
		notifyFunc = func(title, message string) {
			notify.ErrorThrottle(lockKey, interval, title, message)
		}
	}

	respBody, _ := err.MarshalJSON()

	message := fmt.Sprintf(
		"channel: %s (type: %d, type name: %s, id: %d)\nmodel: %s\nmode: %s\nstatus code: %d\ndetail: %s\nrequest id: %s\ntime cost: %s",
		meta.Channel.Name,
		meta.Channel.Type,
		meta.Channel.Type.String(),
		meta.Channel.ID,
		meta.OriginModel,
		meta.Mode,
		err.StatusCode(),
		conv.BytesToString(respBody),
		meta.RequestID,
		getRequestDuration(meta).String(),
	)
	if err.StatusCode() == http.StatusTooManyRequests {
		rate := GetChannelModelRequestRate(c, meta)
		message += fmt.Sprintf(
			"\nrpm: %d\nrps: %d\ntpm: %d\ntps: %d",
			rate.RPM,
			rate.RPS,
			rate.TPM,
			rate.TPS,
		)
	}

	notifyFunc(
		fmt.Sprintf("%s `%s` %s", meta.Channel.Name, meta.OriginModel, titleSuffix),
		message,
	)
}

const (
	MetaChannelModelKeyRPM = "channel_model_rpm"
	MetaChannelModelKeyRPS = "channel_model_rps"
	MetaChannelModelKeyTPM = "channel_model_tpm"
	MetaChannelModelKeyTPS = "channel_model_tps"
)

type RequestRate struct {
	RPM int64
	RPS int64
	TPM int64
	TPS int64
}

func GetChannelModelRequestRate(c *gin.Context, meta *meta.Meta) RequestRate {
	rate := RequestRate{}

	if rpm, ok := meta.Get(MetaChannelModelKeyRPM); ok {
		rate.RPM, _ = rpm.(int64)
		rate.RPS = meta.GetInt64(MetaChannelModelKeyRPS)
	} else {
		rpm, rps := getChannelRequestRate(meta)
		rate.RPM = rpm
		rate.RPS = rps
		updateChannelModelRequestRate(c, meta, rpm, rps)
	}

	if tpm, ok := meta.Get(MetaChannelModelKeyTPM); ok {
		rate.TPM, _ = tpm.(int64)
		rate.TPS = meta.GetInt64(MetaChannelModelKeyTPS)
	} else {
		tpm, tps := getChannelTokenRate(meta)
		rate.TPM = tpm
		rate.TPS = tps
		updateChannelModelTokensRequestRate(c, meta, tpm, tps)
	}

	return rate
}

func updateChannelModelRequestRate(c *gin.Context, meta *meta.Meta, rpm, rps int64) {
	meta.Set(MetaChannelModelKeyRPM, rpm)
	meta.Set(MetaChannelModelKeyRPS, rps)

	log := common.GetLogger(c)
	log.Data["ch_rpm"] = rpm
	log.Data["ch_rps"] = rps
}

func updateChannelModelTokensRequestRate(c *gin.Context, meta *meta.Meta, tpm, tps int64) {
	meta.Set(MetaChannelModelKeyTPM, tpm)
	meta.Set(MetaChannelModelKeyTPS, tps)

	log := common.GetLogger(c)
	log.Data["ch_tpm"] = tpm
	log.Data["ch_tps"] = tps
}
