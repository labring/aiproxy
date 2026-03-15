//go:build enterprise

package notify

import (
	"context"
	"os"
	"time"

	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/trylock"
	log "github.com/sirupsen/logrus"
)

// EnterpriseNotifier combines Feishu webhook group notifications with
// point-to-point user notifications. It implements the notify.Notifier interface.
type EnterpriseNotifier struct {
	webhookURL string
	p2pClient  *FeishuP2PClient
	std        *notify.StdNotifier
}

var _ notify.Notifier = (*EnterpriseNotifier)(nil)

// Notify sends a notification via stdout logging and, if configured, via Feishu webhook.
func (n *EnterpriseNotifier) Notify(level notify.Level, title, message string) {
	n.std.Notify(level, title, message)

	if n.webhookURL != "" {
		go func() {
			_ = notify.PostToFeiShuv2(context.Background(), level2Color(level), title, message, n.webhookURL)
		}()
	}
}

// NotifyThrottle sends a throttled notification. The notification is suppressed
// if the same key was notified within the expiration window.
func (n *EnterpriseNotifier) NotifyThrottle(
	level notify.Level,
	key string,
	expiration time.Duration,
	title, message string,
) {
	if !trylock.Lock(key, expiration) {
		return
	}

	n.std.Notify(level, title, message)

	if n.webhookURL != "" {
		go func() {
			_ = notify.PostToFeiShuv2(context.Background(), level2Color(level), title, message, n.webhookURL)
		}()
	}
}

// NotifyUser sends a point-to-point card message to a specific Feishu user.
// This is not part of the Notifier interface but is available for direct use.
func (n *EnterpriseNotifier) NotifyUser(openID, title, message string) error {
	if n.p2pClient == nil {
		log.Warn("feishu p2p client not configured, skipping user notification")
		return nil
	}

	return n.p2pClient.SendCardMessage(context.Background(), openID, title, message, notify.FeishuColorGreen)
}

// Init initializes the enterprise notifier by checking environment variables
// and registering it as the default notifier if configured.
func Init() {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	webhookURL := os.Getenv("NOTIFY_FEISHU_WEBHOOK")

	if appID == "" && appSecret == "" && webhookURL == "" {
		log.Info("enterprise notifier: no feishu credentials or webhook configured, skipping")
		return
	}

	n := &EnterpriseNotifier{
		webhookURL: webhookURL,
		std:        &notify.StdNotifier{},
	}

	if appID != "" && appSecret != "" {
		n.p2pClient = NewFeishuP2PClient(appID, appSecret)
		log.Info("enterprise notifier: feishu p2p client initialized")
	} else if appID != "" || appSecret != "" {
		log.Warn("enterprise notifier: both FEISHU_APP_ID and FEISHU_APP_SECRET must be set for P2P notifications")
	}

	if webhookURL != "" {
		log.Info("enterprise notifier: feishu webhook configured")
	}

	notify.SetDefaultNotifier(n)
	log.Info("enterprise notifier: registered as default notifier")
}

// level2Color maps notification levels to Feishu card header colors.
func level2Color(level notify.Level) string {
	switch level {
	case notify.LevelInfo:
		return notify.FeishuColorGreen
	case notify.LevelWarn:
		return notify.FeishuColorOrange
	case notify.LevelError:
		return notify.FeishuColorRed
	default:
		return notify.FeishuColorGreen
	}
}
