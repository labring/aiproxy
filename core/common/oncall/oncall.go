package oncall

import (
	"context"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	log "github.com/sirupsen/logrus"
)

const (
	// KeyDBConnectionPrefix is the oncall key prefix for database connection errors
	KeyDBConnectionPrefix = "db_connection_error"
	// DBErrorPersistDuration is how long database errors must persist before triggering urgent alert
	DBErrorPersistDuration = 2 * time.Minute
	// KeyGlobalPhoneCall is the key for global phone call throttling
	// This ensures only one phone call is made even if multiple error sources trigger alerts
	KeyGlobalPhoneCall = "oncall_global_phone_call"
)

// dbConnectionKey returns the oncall key for a specific source
func dbConnectionKey(source string) string {
	return KeyDBConnectionPrefix + ":" + source
}

// AlertDBError triggers an oncall alert for database connection errors
// Call this when a database operation fails with a connection error
// Each source has its own error tracking, but phone calls are globally throttled
func AlertDBError(source string, err error) {
	isConnErr := common.IsDBConnectionError(err)
	log.Debugf(
		"oncall: AlertDBError called, source=%s, isConnectionError=%v, err=%v",
		source,
		isConnErr,
		err,
	)
	if !isConnErr {
		return
	}

	Alert(
		dbConnectionKey(source),
		DBErrorPersistDuration,
		"Database Connection Error",
		source+": "+err.Error(),
	)
}

// ClearDBError clears the database connection error state for a specific source
// Call this when database operations succeed
func ClearDBError(source string) {
	Clear(dbConnectionKey(source))
}

// OnCall handles urgent alerts via Lark (Feishu) API
type OnCall interface {
	// Alert sends an urgent phone call alert to the on-call personnel
	// The alert will only be sent if the error persists for the specified duration
	// key: unique identifier for this alert type (for deduplication)
	// persistDuration: how long the error must persist before alerting
	// title: alert title
	// message: alert message
	Alert(key string, persistDuration time.Duration, title, message string)

	// Clear clears the error state for the given key
	// Call this when the error condition has been resolved
	Clear(key string)
}

var (
	defaultOnCall OnCall = &NoopOnCall{}
	initOnce      sync.Once
)

// NoopOnCall is a no-op implementation of OnCall
type NoopOnCall struct{}

func (n *NoopOnCall) Alert(key string, persistDuration time.Duration, title, message string) {}
func (n *NoopOnCall) Clear(key string)                                                       {}

// LarkOnCall implements OnCall using Lark (Feishu) API
type LarkOnCall struct {
	appID     string
	appSecret string
	openIDs   []string // multiple open IDs for on-call users
	state     *AlertState
	mu        sync.Mutex
	// alerts tracks pending alert goroutines by key
	alerts map[string]context.CancelFunc
}

// Config holds the configuration for LarkOnCall
type Config struct {
	AppID     string
	AppSecret string
	OpenIDs   []string
}

// NewLarkOnCall creates a new LarkOnCall instance
func NewLarkOnCall(cfg Config) *LarkOnCall {
	return &LarkOnCall{
		appID:     cfg.AppID,
		appSecret: cfg.AppSecret,
		openIDs:   cfg.OpenIDs,
		state:     NewAlertState(),
		alerts:    make(map[string]context.CancelFunc),
	}
}

// Init initializes the default on-call handler from environment variables
// Environment variables (configured in config.env.go):
// - ON_CALL_LARK_APP_ID: Lark app ID
// - ON_CALL_LARK_APP_SECRET: Lark app secret
// - ON_CALL_LARK_OPEN_ID: Comma-separated open IDs of on-call users to receive urgent calls
func Init() {
	initOnce.Do(func() {
		appID := config.OnCallLarkAppID
		appSecret := config.OnCallLarkAppSecret
		openIDs := config.OnCallLarkOpenIDs

		if appID == "" || appSecret == "" || len(openIDs) == 0 {
			log.Info("ON_CALL_LARK_* environment variables not fully set, oncall disabled")
			return
		}

		defaultOnCall = NewLarkOnCall(Config{
			AppID:     appID,
			AppSecret: appSecret,
			OpenIDs:   openIDs,
		})
		log.Infof("oncall initialized with Lark, %d on-call users configured", len(openIDs))
	})
}

// SetDefault sets the default on-call handler
func SetDefault(oc OnCall) {
	defaultOnCall = oc
}

// Alert sends an urgent alert using the default on-call handler
func Alert(key string, persistDuration time.Duration, title, message string) {
	log.Debugf("oncall: Alert called, key=%s, persistDuration=%v", key, persistDuration)
	defaultOnCall.Alert(key, persistDuration, title, message)
}

// Clear clears the error state for the given key
func Clear(key string) {
	defaultOnCall.Clear(key)
}

// Alert implements OnCall.Alert
func (l *LarkOnCall) Alert(key string, persistDuration time.Duration, title, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if we've already sent an alert for this key recently
	if l.state.HasAlerted(key) {
		log.Debugf("oncall: skipping alert for key=%s, already alerted recently", key)
		return
	}

	// Check if there's already a pending alert for this key
	if _, exists := l.alerts[key]; exists {
		return
	}

	// Record the first occurrence time
	firstSeen := l.state.RecordError(key)

	// Calculate how long until we should alert
	elapsed := time.Since(firstSeen)
	remaining := persistDuration - elapsed

	if remaining <= 0 {
		// Error has persisted long enough, send alert now
		go l.sendAlert(key, title, message)
		return
	}

	// Schedule alert after remaining duration
	ctx, cancel := context.WithCancel(context.Background())
	l.alerts[key] = cancel

	go func() {
		timer := time.NewTimer(remaining)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			// Alert was cancelled (error cleared)
			return
		case <-timer.C:
			l.mu.Lock()
			delete(l.alerts, key)
			l.mu.Unlock()

			// Check if error still persists
			if l.state.HasError(key) {
				l.sendAlert(key, title, message)
			}
		}
	}()
}

// Clear implements OnCall.Clear
func (l *LarkOnCall) Clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Cancel any pending alert
	if cancel, exists := l.alerts[key]; exists {
		cancel()
		delete(l.alerts, key)
	}

	// Clear the error state
	l.state.ClearError(key)
}

func (l *LarkOnCall) sendAlert(key, title, message string) {
	// Use global throttling for phone calls - even if multiple keys have errors,
	// only one phone call is made within the cooldown period
	if !l.state.MarkAlerted(KeyGlobalPhoneCall) {
		log.Debugf("oncall: skipping phone call for key=%s, global phone call throttled", key)
		return // Already sent a phone call recently
	}

	log.Warnf(
		"oncall: sending urgent alert for key=%s, title=%s, to %d users",
		key,
		title,
		len(l.openIDs),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send message and urgent phone call to each on-call user
	var successCount int
	for _, openID := range l.openIDs {
		// Send message first
		messageID, err := SendMessage(ctx, l.appID, l.appSecret, openID, title, message)
		if err != nil {
			log.Errorf("oncall: failed to send message to %s: %v", openID, err)
			continue
		}

		// Send urgent phone call
		err = SendUrgentPhone(ctx, l.appID, l.appSecret, messageID, openID)
		if err != nil {
			log.Errorf("oncall: failed to send urgent phone call to %s: %v", openID, err)
			// Message was sent successfully, count as partial success
		}

		successCount++
	}

	// If no messages were sent successfully, clear global alerted state so we can retry
	if successCount == 0 {
		log.Error("oncall: failed to send alert to any user, will retry")
		l.state.ClearAlerted(KeyGlobalPhoneCall)
	}
}
