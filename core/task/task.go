package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/ipblack"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/oncall"
	"github.com/labring/aiproxy/core/common/trylock"
	"github.com/labring/aiproxy/core/controller"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AutoTestBannedModelsTask 自动测试被禁用的模型
func AutoTestBannedModelsTask(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			controller.AutoTestBannedModels()
		}
	}
}

// DetectIPGroupsTask 检测 IP 使用多个 group 的情况
func DetectIPGroupsTask(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !trylock.Lock("runDetectIPGroups", time.Minute) {
				continue
			}

			detectIPGroups()
		}
	}
}

func detectIPGroups() {
	threshold := config.GetIPGroupsThreshold()
	if threshold < 1 {
		return
	}

	ipGroupList, err := model.GetIPGroups(int(threshold), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		notify.ErrorThrottle("detectIPGroups", time.Minute, "detect IP groups failed", err.Error())
		return
	}

	if len(ipGroupList) == 0 {
		return
	}

	banThreshold := config.GetIPGroupsBanThreshold()
	for ip, groups := range ipGroupList {
		slices.Sort(groups)

		groupsJSON, err := sonic.MarshalString(groups)
		if err != nil {
			notify.ErrorThrottle(
				"detectIPGroupsMarshal",
				time.Minute,
				"marshal IP groups failed",
				err.Error(),
			)

			continue
		}

		if banThreshold >= threshold && len(groups) >= int(banThreshold) {
			rowsAffected, err := model.UpdateGroupsStatus(groups, model.GroupStatusDisabled)
			if err != nil {
				notify.ErrorThrottle(
					"detectIPGroupsBan",
					time.Minute,
					"update groups status failed",
					err.Error(),
				)
			}

			if rowsAffected > 0 {
				notify.Warn(
					fmt.Sprintf(
						"Suspicious activity: IP %s is using %d groups (exceeds ban threshold of %d). IP and all groups have been disabled.",
						ip,
						len(groups),
						banThreshold,
					),
					groupsJSON,
				)
				ipblack.SetIPBlackAnyWay(ip, time.Hour*48)
			}

			continue
		}

		h := sha256.New()
		h.Write(conv.StringToBytes(groupsJSON))
		groupsHash := hex.EncodeToString(h.Sum(nil))
		hashKey := fmt.Sprintf("%s:%s", ip, groupsHash)

		notify.WarnThrottle(
			hashKey,
			time.Hour*3,
			fmt.Sprintf(
				"Potential abuse: IP %s is using %d groups (exceeds threshold of %d)",
				ip,
				len(groups),
				threshold,
			),
			groupsJSON,
		)
	}
}

// UsageAlertTask 用量异常告警任务
func UsageAlertTask(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !trylock.Lock("runUsageAlert", time.Hour) {
				continue
			}

			checkUsageAlert()
		}
	}
}

func checkUsageAlert() {
	threshold := config.GetUsageAlertThreshold()
	if threshold <= 0 {
		return
	}

	// 获取配置的白名单
	whitelist := config.GetUsageAlertWhitelist()

	// 获取前三天平均用量最低阈值
	minAvgThreshold := config.GetUsageAlertMinAvgThreshold()

	alerts, err := model.GetGroupUsageAlert(float64(threshold), float64(minAvgThreshold), whitelist)
	if err != nil {
		notify.ErrorThrottle(
			"usageAlertError",
			time.Minute*5,
			"check usage alert failed",
			err.Error(),
		)

		return
	}

	if len(alerts) == 0 {
		return
	}

	// 计算到明天 0 点的时间，确保每个 group 一天只告警一次
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	lockDuration := tomorrow.Sub(now)

	// 过滤掉当天已经告警过的 group（通过 trylock 判断）
	var validAlerts []model.GroupUsageAlertItem
	for _, alert := range alerts {
		lockKey := "usageAlert:" + alert.GroupID
		// 尝试获取锁，如果获取失败说明当天已经告警过
		if trylock.Lock(lockKey, lockDuration) {
			validAlerts = append(validAlerts, alert)
		}
	}

	if len(validAlerts) == 0 {
		return
	}

	message := formatGroupUsageAlerts(validAlerts)
	notify.Warn(
		fmt.Sprintf("Detected %d groups with abnormal usage", len(validAlerts)),
		message,
	)
}

// formatGroupUsageAlerts 格式化告警消息
func formatGroupUsageAlerts(alerts []model.GroupUsageAlertItem) string {
	if len(alerts) == 0 {
		return ""
	}

	var result strings.Builder
	for _, alert := range alerts {
		fmt.Fprintf(&result, "GroupID: %s | 3-Day Avg: %.4f | Today: %.4f | Ratio: %.2fx\n",
			alert.GroupID,
			alert.ThreeDayAvgAmount,
			alert.TodayAmount,
			alert.Ratio)
	}

	return result.String()
}

// CleanLogTask 清理日志任务
func CleanLogTask(ctx context.Context) {
	// the interval should not be too large to avoid cleaning too much at once
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !trylock.Lock("runCleanLog", time.Second*3) {
				continue
			}

			optimize := trylock.Lock("runOptimizeLog", time.Hour*24)

			err := model.CleanLog(int(config.GetCleanLogBatchSize()), optimize)
			if err != nil {
				notify.ErrorThrottle(
					"cleanLogError",
					time.Minute*5,
					"clean log failed",
					err.Error(),
				)
			}
		}
	}
}

const (
	asyncUsagePollInterval = time.Second * 10
	asyncUsageBatchSize    = 50
	asyncUsageConcurrency  = 10
	asyncUsageMaxRetry     = 10
)

func AsyncUsagePollTask(ctx context.Context) {
	ticker := time.NewTicker(asyncUsagePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !trylock.Lock("runAsyncUsagePoll", asyncUsagePollInterval-time.Second) {
				continue
			}

			processAsyncUsages(ctx)
		}
	}
}

func processAsyncUsages(ctx context.Context) {
	infos, err := model.GetPendingAsyncUsages(asyncUsageBatchSize)
	if err != nil {
		notify.ErrorThrottle(
			"asyncUsagePoll",
			time.Minute*5,
			"get pending async usages failed",
			err.Error(),
		)

		return
	}

	if len(infos) == 0 {
		return
	}

	log.Debugf(
		"async usage poll: pending=%d batch_size=%d concurrency=%d",
		len(infos),
		asyncUsageBatchSize,
		asyncUsageConcurrency,
	)

	sem := make(chan struct{}, asyncUsageConcurrency)

	var wg sync.WaitGroup

	for _, info := range infos {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case sem <- struct{}{}:
		}

		wg.Add(1)

		go func(info *model.AsyncUsageInfo) {
			defer wg.Done()
			defer func() {
				<-sem
			}()

			processOneAsyncUsage(ctx, info)
		}(info)
	}

	wg.Wait()
}

func processOneAsyncUsage(ctx context.Context, info *model.AsyncUsageInfo) {
	log.Debugf(
		"async usage poll: start id=%d request_id=%s upstream_id=%s mode=%d model=%s channel_id=%d retry=%d next_poll_at=%s",
		info.ID,
		info.RequestID,
		info.UpstreamID,
		info.Mode,
		info.Model,
		info.ChannelID,
		info.RetryCount,
		info.NextPollAt.Format(time.RFC3339),
	)

	channel, err := model.GetChannelByID(info.ChannelID)
	if err != nil {
		log.Debugf(
			"async usage poll: fail id=%d request_id=%s reason=channel_not_found err=%v",
			info.ID,
			info.RequestID,
			err,
		)
		markAsyncUsageFailed(info, "channel not found: "+err.Error())

		return
	}

	a, ok := adaptors.GetAdaptor(channel.Type)
	if !ok {
		log.Debugf(
			"async usage poll: fail id=%d request_id=%s reason=adaptor_not_found channel_type=%d",
			info.ID,
			info.RequestID,
			channel.Type,
		)
		markAsyncUsageFailed(
			info,
			fmt.Sprintf("adaptor not found for channel type %d", channel.Type),
		)

		return
	}

	fetcher, ok := a.(adaptor.AsyncUsageFetcher)
	if !ok {
		log.Debugf(
			"async usage poll: fail id=%d request_id=%s reason=fetcher_not_supported channel_type=%d",
			info.ID,
			info.RequestID,
			channel.Type,
		)
		markAsyncUsageFailed(info, "adaptor does not support async usage fetching")

		return
	}

	usage, completed, err := fetcher.FetchAsyncUsage(ctx, channel, info)
	if err != nil {
		if completed {
			log.Debugf(
				"async usage poll: fail id=%d request_id=%s upstream_id=%s reason=upstream_final_error err=%v",
				info.ID,
				info.RequestID,
				info.UpstreamID,
				err,
			)
			markAsyncUsageFailed(info, err.Error())

			return
		}

		scheduleAsyncUsageRetry(info, err)

		return
	}

	if !completed {
		log.Debugf(
			"async usage poll: pending id=%d request_id=%s upstream_id=%s",
			info.ID,
			info.RequestID,
			info.UpstreamID,
		)
		touchAsyncUsagePollCursor(info)

		return
	}

	if err := completeAsyncUsage(ctx, info, usage); err != nil {
		log.Debugf(
			"async usage poll: complete_error id=%d request_id=%s upstream_id=%s err=%v",
			info.ID,
			info.RequestID,
			info.UpstreamID,
			err,
		)
		scheduleAsyncUsageRetry(info, fmt.Errorf("complete failed: %w", err))

		return
	}

	log.Debugf(
		"async usage poll: completed id=%d request_id=%s upstream_id=%s input=%d output=%d total=%d",
		info.ID,
		info.RequestID,
		info.UpstreamID,
		usage.InputTokens,
		usage.OutputTokens,
		usage.TotalTokens,
	)
}

func completeAsyncUsage(ctx context.Context, info *model.AsyncUsageInfo, usage model.Usage) error {
	price := info.Price
	price.PerRequestPrice = 0

	amount := consume.CalculateAmountDetail(
		http.StatusOK,
		usage,
		price,
		info.ServiceTier,
	)

	if amount.UsedAmount > 0 && !info.BalanceConsumed {
		if err := consumeAsyncUsageGroupBalance(ctx, info, amount.UsedAmount); err != nil {
			notify.ErrorThrottle(
				"asyncUsageConsumeBalance",
				time.Minute*5,
				"consume async usage balance failed",
				err.Error(),
			)
			recordAsyncUsageConsumeError(info, amount.UsedAmount, err)

			return fmt.Errorf("consume async usage balance: %w", err)
		}

		info.BalanceConsumed = true
		if err := model.UpdateAsyncUsageInfo(info); err != nil {
			return fmt.Errorf("update async usage balance consumed: %w", err)
		}
	}

	if err := model.UpdateLogUsageByRequestID(info.RequestID, usage, amount); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			notify.ErrorThrottle(
				"asyncUsageUpdateLog",
				time.Minute*5,
				"update async usage log failed",
				err.Error(),
			)

			return fmt.Errorf("update async usage log: %w", err)
		}
	}

	model.BatchUpdateSummaryOnlyUsage(
		time.Now(),
		info.RequestAt,
		info.GroupID,
		info.ChannelID,
		info.Model,
		info.TokenID,
		info.TokenName,
		usage,
		amount,
		info.ServiceTier,
		model.IsClaudeLongContextSummary(info.Model, usage),
	)

	info.Status = model.AsyncUsageStatusCompleted
	info.Usage = usage
	info.Amount = amount
	info.Error = ""

	if err := model.UpdateAsyncUsageInfo(info); err != nil {
		return fmt.Errorf("update async usage info: %w", err)
	}

	return nil
}

func scheduleAsyncUsageRetry(info *model.AsyncUsageInfo, err error) {
	info.RetryCount++
	info.Error = err.Error()
	info.NextPollAt = time.Now().Add(model.AsyncUsageBackoffDelay(info.RetryCount))

	if info.RetryCount >= asyncUsageMaxRetry {
		log.Debugf(
			"async usage poll: fail id=%d request_id=%s upstream_id=%s reason=max_retry retry=%d next_poll_at=%s err=%v",
			info.ID,
			info.RequestID,
			info.UpstreamID,
			info.RetryCount,
			info.NextPollAt.Format(time.RFC3339),
			err,
		)
		markAsyncUsageFailed(info, "max retry exceeded: "+err.Error())

		return
	}

	log.Debugf(
		"async usage poll: retry id=%d request_id=%s upstream_id=%s retry=%d next_poll_at=%s err=%v",
		info.ID,
		info.RequestID,
		info.UpstreamID,
		info.RetryCount,
		info.NextPollAt.Format(time.RFC3339),
		err,
	)

	if updateErr := model.UpdateAsyncUsageInfo(info); updateErr != nil {
		notify.ErrorThrottle(
			"asyncUsageUpdateRetry",
			time.Minute*5,
			"update async usage retry failed",
			updateErr.Error(),
		)
	}
}

func touchAsyncUsagePollCursor(info *model.AsyncUsageInfo) {
	info.Error = ""
	info.NextPollAt = time.Now().Add(model.AsyncUsageDefaultPollDelay)

	if err := model.UpdateAsyncUsageInfo(info); err != nil {
		notify.ErrorThrottle(
			"asyncUsageTouchPending",
			time.Minute*5,
			"touch pending async usage failed",
			err.Error(),
		)
	}
}

func consumeAsyncUsageGroupBalance(
	ctx context.Context,
	info *model.AsyncUsageInfo,
	amount float64,
) error {
	if balance.Default == nil || info.GroupID == "" {
		return nil
	}

	group, err := model.CacheGetGroup(info.GroupID)
	if err != nil {
		return fmt.Errorf("get group: %w", err)
	}

	_, consumer, err := balance.Default.GetGroupRemainBalance(ctx, *group)
	if err != nil {
		return fmt.Errorf("get group balance: %w", err)
	}

	if consumer == nil {
		return nil
	}

	_, err = consumer.PostGroupConsume(ctx, info.TokenName, amount)

	return err
}

func recordAsyncUsageConsumeError(
	info *model.AsyncUsageInfo,
	amount float64,
	err error,
) {
	if err := model.CreateConsumeError(
		info.RequestID,
		info.RequestAt,
		info.GroupID,
		info.TokenName,
		info.Model,
		err.Error(),
		amount,
		info.TokenID,
	); err != nil {
		log.Error("failed to create async usage consume error: " + err.Error())
	}
}

func markAsyncUsageFailed(info *model.AsyncUsageInfo, errMsg string) {
	info.Status = model.AsyncUsageStatusFailed
	info.Error = errMsg

	if err := model.IgnoreNotFound(
		model.UpdateLogAsyncUsageStatusByRequestID(info.RequestID, model.AsyncUsageStatusFailed),
	); err != nil {
		notify.ErrorThrottle(
			"asyncUsageUpdateLogStatus",
			time.Minute*5,
			"update async usage log status failed",
			err.Error(),
		)
	}

	if err := model.UpdateAsyncUsageInfo(info); err != nil {
		notify.ErrorThrottle(
			"asyncUsageMarkFailed",
			time.Minute*5,
			"mark async usage failed",
			err.Error(),
		)
	}
}

const (
	// KeyRedisConnection is the oncall key prefix for redis connection errors
	KeyRedisConnection = "redis_connection_error"
	// RedisHealthCheckInterval is how often to ping redis
	RedisHealthCheckInterval = 30 * time.Second
	// RedisErrorPersistDuration is how long redis errors must persist before triggering urgent alert
	RedisErrorPersistDuration = 2 * time.Minute
)

// RedisHealthCheckTask monitors Redis connection health
func RedisHealthCheckTask(ctx context.Context) {
	ticker := time.NewTicker(RedisHealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkRedisHealth()
		}
	}
}

func checkRedisHealth() {
	if !common.RedisEnabled {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := common.RDB.Ping(ctx).Result()
	if err != nil {
		oncall.Alert(
			KeyRedisConnection,
			RedisErrorPersistDuration,
			"Redis Connection Error",
			"Redis ping failed: "+err.Error(),
		)

		return
	}

	// Clear error state if ping succeeds
	oncall.Clear(KeyRedisConnection)
}
