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

func ChannelHasPermission(relayErr adaptor.Error) bool {
	_, ok := channelNoPermissionStatusCodesMap[relayErr.StatusCode()]
	return !ok
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
	count, overLimitCount, secondCount := reqlimit.PushChannelModelRequest(
		context.Background(),
		strconv.Itoa(meta.Channel.ID),
		meta.OriginModel,
	)
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
	beyondThreshold, banExecution, _err := monitor.AddRequest(
		context.Background(),
		meta.OriginModel,
		int64(meta.Channel.ID),
		true,
		false,
		meta.ModelConfig.WarnErrorRate,
		meta.ModelConfig.MaxErrorRate,
	)
	if _err != nil {
		common.GetLogger(c).Errorf("add request failed: %+v", _err)
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
	case beyondThreshold:
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
) (model.Usage, adaptor.Error) {
	usage, relayErr := do.DoResponse(meta, store, c, resp)

	if usage.TotalTokens > 0 {
		count, overLimitCount, secondCount := reqlimit.PushChannelModelTokensRequest(
			context.Background(),
			strconv.Itoa(meta.Channel.ID),
			meta.OriginModel,
			int64(usage.TotalTokens),
		)
		updateChannelModelTokensRequestRate(c, meta, count+overLimitCount, secondCount)
	}

	if relayErr == nil {
		if _, _, err := monitor.AddRequest(
			context.Background(),
			meta.OriginModel,
			int64(meta.Channel.ID),
			false,
			false,
			meta.ModelConfig.WarnErrorRate,
			meta.ModelConfig.MaxErrorRate,
		); err != nil {
			common.GetLogger(c).Errorf("add request failed: %+v", err)
		}

		return usage, nil
	}

	if !ShouldRetry(relayErr) {
		return usage, relayErr
	}

	handleAdaptorError(meta, c, relayErr)

	return usage, relayErr
}

func handleAdaptorError(meta *meta.Meta, c *gin.Context, relayErr adaptor.Error) {
	hasPermission := ChannelHasPermission(relayErr)

	beyondThreshold, banExecution, err := monitor.AddRequest(
		context.Background(),
		meta.OriginModel,
		int64(meta.Channel.ID),
		true,
		!hasPermission,
		meta.ModelConfig.WarnErrorRate,
		meta.ModelConfig.MaxErrorRate,
	)
	if err != nil {
		common.GetLogger(c).Errorf("add request failed: %+v", err)
	}

	switch {
	case banExecution:
		notifyChannelResponseIssue(c, meta, "autoBanned", "Auto Banned", relayErr, time.Minute*15)
	case beyondThreshold:
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

func notifyChannelResponseIssue(
	c *gin.Context,
	meta *meta.Meta,
	issueType, titleSuffix string,
	err adaptor.Error,
	interval time.Duration,
) {
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
		rpm, rps := reqlimit.GetChannelModelRequest(
			context.Background(),
			strconv.Itoa(meta.Channel.ID),
			meta.OriginModel,
		)
		rate.RPM = rpm
		rate.RPS = rps
		updateChannelModelRequestRate(c, meta, rpm, rps)
	}

	if tpm, ok := meta.Get(MetaChannelModelKeyTPM); ok {
		rate.TPM, _ = tpm.(int64)
		rate.TPS = meta.GetInt64(MetaChannelModelKeyTPS)
	} else {
		tpm, tps := reqlimit.GetChannelModelTokensRequest(
			context.Background(),
			strconv.Itoa(meta.Channel.ID),
			meta.OriginModel,
		)
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
