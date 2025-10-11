package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/ipblack"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/trylock"
	"github.com/labring/aiproxy/core/controller"
	"github.com/labring/aiproxy/core/model"
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

	var result string
	for _, alert := range alerts {
		result += fmt.Sprintf(
			"GroupID: %s | 3-Day Avg: %.4f | Today: %.4f | Ratio: %.2fx\n",
			alert.GroupID,
			alert.ThreeDayAvgAmount,
			alert.TodayAmount,
			alert.Ratio,
		)
	}

	return result
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
			if !trylock.Lock("runCleanLog", time.Second*5) {
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
