//go:build enterprise

package quota

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/trylock"
	"github.com/labring/aiproxy/core/enterprise/models"
	enterprisenotify "github.com/labring/aiproxy/core/enterprise/notify"
	"github.com/labring/aiproxy/core/model"
	log "github.com/sirupsen/logrus"
)

const optionKeyQuotaNotifConfig = "QuotaNotifConfig"

// NotifConfig holds the notification templates for each quota tier transition.
type NotifConfig struct {
	Enabled      bool   `json:"enabled"`
	Tier2Title   string `json:"tier2_title"`
	Tier2Body    string `json:"tier2_body"`
	Tier3Title   string `json:"tier3_title"`
	Tier3Body    string `json:"tier3_body"`
	ExhaustTitle string `json:"exhaust_title"`
	ExhaustBody  string `json:"exhaust_body"`

	// Admin webhook alert: notify admin group when any user reaches a threshold
	AdminAlertEnabled   bool    `json:"admin_alert_enabled"`
	AdminAlertThreshold float64 `json:"admin_alert_threshold"` // 0.0-1.0, e.g. 0.8 = 80%
	AdminAlertTitle     string  `json:"admin_alert_title"`
	AdminAlertBody      string  `json:"admin_alert_body"`
}

// DefaultNotifConfig is the default Chinese notification template.
var DefaultNotifConfig = NotifConfig{
	Enabled:             false,
	Tier2Title:          "AI 用量提醒",
	Tier2Body:           "您好 {name}，您本{period_type}的 AI 用量已达 {usage_pct}（阈值 {tier_threshold}，周期额度 {period_quota}），已进入二级限速，RPM/TPM 有所降低，请注意控制用量。",
	Tier3Title:          "AI 用量紧张提醒",
	Tier3Body:           "您好 {name}，您本{period_type}的 AI 用量已达 {usage_pct}（阈值 {tier_threshold}，周期额度 {period_quota}），已进入三级限速，请控制用量以避免服务中断。",
	ExhaustTitle:        "AI 用量已耗尽",
	ExhaustBody:         "您好 {name}，您本{period_type}的 AI 用量已耗尽（周期额度 {period_quota}），所有请求将被拒绝，请联系管理员或等待下一周期重置。",
	AdminAlertEnabled:   false,
	AdminAlertThreshold: 0.8,
	AdminAlertTitle:     "成员额度用量告警",
	AdminAlertBody:      "{name} 本{period_type}的 AI 用量已达 {usage_pct}（告警阈值 {admin_threshold}，周期额度 {period_quota}），请关注。",
}

// cachedNotifConfig holds the in-memory config to avoid a DB read on every
// notification check. Updated on write by SetNotifConfig.
var cachedNotifConfig atomic.Pointer[NotifConfig]

// GetNotifConfig returns the notification config, using an in-memory cache to
// avoid per-request DB reads. Falls back to DefaultNotifConfig if not set.
func GetNotifConfig() NotifConfig {
	if p := cachedNotifConfig.Load(); p != nil {
		return *p
	}

	var opt model.Option
	if err := model.DB.Where("key = ?", optionKeyQuotaNotifConfig).First(&opt).Error; err != nil {
		return DefaultNotifConfig
	}

	var cfg NotifConfig
	if err := sonic.UnmarshalString(opt.Value, &cfg); err != nil {
		return DefaultNotifConfig
	}

	cachedNotifConfig.Store(&cfg)

	return cfg
}

// SetNotifConfig persists the notification config and updates the in-memory cache.
func SetNotifConfig(cfg NotifConfig) error {
	data, err := sonic.MarshalString(cfg)
	if err != nil {
		return err
	}

	if err := model.DB.Save(&model.Option{Key: optionKeyQuotaNotifConfig, Value: data}).Error; err != nil {
		return err
	}

	cachedNotifConfig.Store(&cfg)

	return nil
}

// NotifConfigResponse wraps NotifConfig with runtime P2P availability status.
type NotifConfigResponse struct {
	NotifConfig
	P2PAvailable bool `json:"p2p_available"`
}

// RenderTemplate replaces {key} placeholders with corresponding values.
func RenderTemplate(tmpl string, vars map[string]string) string {
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, "{"+k+"}", v)
	}

	return strings.NewReplacer(pairs...).Replace(tmpl)
}

// notifDedupKey returns the Redis/memory dedup key for a given notification.
func notifDedupKey(openID string, tier int, periodType string) string {
	return fmt.Sprintf("enterprise:quota_notif:%s:%d:%s", openID, tier, periodKey(periodType))
}

// periodKey returns a string representing the current period window.
// daily → "2026-03-21", weekly → "2026-W12", monthly → "2026-03"
func periodKey(periodType string) string {
	now := time.Now()
	switch periodType {
	case model.PeriodTypeDaily:
		return now.Format("2006-01-02")
	case model.PeriodTypeWeekly:
		_, week := now.ISOWeek()
		return fmt.Sprintf("%d-W%02d", now.Year(), week)
	default: // monthly
		return now.Format("2006-01")
	}
}

// periodTTL returns the dedup lock TTL for the given period type.
func periodTTL(periodType string) time.Duration {
	switch periodType {
	case model.PeriodTypeDaily:
		return 26 * time.Hour
	case model.PeriodTypeWeekly:
		return 8 * 24 * time.Hour
	default: // monthly
		return 32 * 24 * time.Hour
	}
}

// MaybeNotifyUser sends a quota tier change notification to the user if:
// 1. Notifications are enabled, 2. The user has not been notified in this period for this tier.
// tier: 2 = tier2 throttle, 3 = tier3 throttle, 4 = exhausted.
// This function must be called in a goroutine (non-blocking from the caller's perspective).
func MaybeNotifyUser(
	openID, userName, periodType string,
	tier int,
	usageRatio float64,
	periodQuota float64,
	tierThreshold float64,
) {
	// Cheapest checks first: avoid lock and DB access when not needed.
	n := enterprisenotify.GetEnterpriseNotifier()
	if n == nil {
		return
	}

	cfg := GetNotifConfig() // in-memory after first load
	if !cfg.Enabled {
		return
	}

	// Dedup: only notify once per period per tier.
	if !trylock.Lock(notifDedupKey(openID, tier, periodType), periodTTL(periodType)) {
		return
	}

	vars := map[string]string{
		"name":           userName,
		"usage_pct":      fmt.Sprintf("%.1f%%", usageRatio*100),
		"period_quota":   fmt.Sprintf("¥%.2f", periodQuota),
		"period_type":    periodTypeLabel(periodType),
		"tier_threshold": fmt.Sprintf("%.0f%%", tierThreshold*100),
	}

	var title, body, color string

	switch tier {
	case 2:
		title = RenderTemplate(cfg.Tier2Title, vars)
		body = RenderTemplate(cfg.Tier2Body, vars)
		color = notify.FeishuColorOrange
	case 3:
		title = RenderTemplate(cfg.Tier3Title, vars)
		body = RenderTemplate(cfg.Tier3Body, vars)
		color = notify.FeishuColorRed
	default: // 4 = exhausted
		title = RenderTemplate(cfg.ExhaustTitle, vars)
		body = RenderTemplate(cfg.ExhaustBody, vars)
		color = notify.FeishuColorRed
	}

	record := models.QuotaAlertHistory{
		OpenID:      openID,
		UserName:    userName,
		Tier:        tier,
		UsageRatio:  usageRatio,
		PeriodQuota: periodQuota,
		PeriodType:  periodType,
		Title:       title,
		Body:        body,
	}

	if err := n.NotifyUser(openID, title, body, color); err != nil {
		log.WithError(err).WithField("open_id", openID).Warn("quota tier notification failed")
		record.Status = "failed"
		record.Error = err.Error()
	} else {
		record.Status = "sent"
	}

	if err := model.DB.Create(&record).Error; err != nil {
		log.WithError(err).Warn("failed to record quota alert history")
	}
}

// periodTypeLabel returns the Chinese label for a period type.
func periodTypeLabel(periodType string) string {
	switch periodType {
	case model.PeriodTypeDaily:
		return "日"
	case model.PeriodTypeWeekly:
		return "周"
	default:
		return "月"
	}
}

// adminNotifDedupKey returns the dedup key for admin webhook alerts.
func adminNotifDedupKey(openID, periodType string) string {
	return fmt.Sprintf("enterprise:quota_admin_alert:%s:%s", openID, periodKey(periodType))
}

// MaybeNotifyAdmin sends a webhook group notification to admins when a user's
// usage ratio exceeds the configured admin alert threshold.
// Deduplication: each user triggers at most one admin alert per quota period.
// This function must be called in a goroutine.
func MaybeNotifyAdmin(
	openID, userName, periodType string,
	usageRatio float64,
	periodQuota float64,
) {
	cfg := GetNotifConfig()
	if !cfg.AdminAlertEnabled || cfg.AdminAlertThreshold <= 0 {
		return
	}

	if usageRatio < cfg.AdminAlertThreshold {
		return
	}

	n := enterprisenotify.GetEnterpriseNotifier()
	if n == nil {
		return
	}

	if !trylock.Lock(adminNotifDedupKey(openID, periodType), periodTTL(periodType)) {
		return
	}

	vars := map[string]string{
		"name":            userName,
		"usage_pct":       fmt.Sprintf("%.1f%%", usageRatio*100),
		"period_quota":    fmt.Sprintf("¥%.2f", periodQuota),
		"period_type":     periodTypeLabel(periodType),
		"admin_threshold": fmt.Sprintf("%.0f%%", cfg.AdminAlertThreshold*100),
	}

	title := RenderTemplate(cfg.AdminAlertTitle, vars)
	body := RenderTemplate(cfg.AdminAlertBody, vars)

	// Send via webhook (group notification), not P2P
	n.Notify(notify.LevelWarn, title, body)

	record := models.QuotaAlertHistory{
		OpenID:      openID,
		UserName:    userName,
		Tier:        0, // 0 = admin alert (not a user tier notification)
		UsageRatio:  usageRatio,
		PeriodQuota: periodQuota,
		PeriodType:  periodType,
		Title:       title,
		Body:        body,
		Status:      "sent",
	}
	if err := model.DB.Create(&record).Error; err != nil {
		log.WithError(err).Warn("failed to record admin quota alert history")
	}
}
