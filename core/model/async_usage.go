package model

import (
	"time"

	"gorm.io/gorm"
)

type AsyncUsageStatus int

const (
	AsyncUsageStatusNone AsyncUsageStatus = iota
	AsyncUsageStatusPending
	AsyncUsageStatusCompleted
	AsyncUsageStatusFailed
)

const (
	AsyncUsageDefaultPollDelay = 10 * time.Second
	AsyncUsageMaxPollDelay     = 3 * time.Minute
)

type AsyncUsageInfo struct {
	ID              int              `gorm:"primaryKey"                    json:"id"`
	RequestID       string           `gorm:"type:char(16);index"           json:"request_id"`
	RequestAt       time.Time        `                                     json:"request_at"`
	Mode            int              `gorm:"index"                         json:"mode"`
	Model           string           `gorm:"size:128"                      json:"model"`
	ChannelID       int              `gorm:"index"                         json:"channel_id"`
	BaseURL         string           `gorm:"type:text"                     json:"base_url,omitempty"`
	GroupID         string           `gorm:"size:64;index"                 json:"group_id"`
	TokenID         int              `gorm:"index"                         json:"token_id"`
	TokenName       string           `gorm:"size:128"                      json:"token_name,omitempty"`
	Price           Price            `gorm:"serializer:fastjson;type:text" json:"price"`
	ServiceTier     string           `gorm:"size:16"                       json:"service_tier,omitempty"`
	UpstreamID      string           `gorm:"type:varchar(256);index"       json:"upstream_id"`
	Status          AsyncUsageStatus `gorm:"index;default:1"               json:"status"`
	Usage           Usage            `gorm:"serializer:fastjson;type:text" json:"usage"`
	Amount          Amount           `gorm:"embedded"                      json:"amount,omitempty"`
	Error           string           `gorm:"type:text"                     json:"error,omitempty"`
	RetryCount      int              `                                     json:"retry_count"`
	DownstreamDone  bool             `                                     json:"downstream_done"`
	BalanceConsumed bool             `                                     json:"balance_consumed"`
	NextPollAt      time.Time        `gorm:"index"                         json:"next_poll_at"`
	CreatedAt       time.Time        `                                     json:"created_at"`
	UpdatedAt       time.Time        `                                     json:"updated_at"`
}

func CreateAsyncUsageInfo(info *AsyncUsageInfo) error {
	info.Status = AsyncUsageStatusPending
	info.CreatedAt = time.Now()

	info.UpdatedAt = info.CreatedAt
	if info.NextPollAt.IsZero() {
		info.NextPollAt = info.CreatedAt.Add(AsyncUsageDefaultPollDelay)
	}

	return LogDB.Create(info).Error
}

func GetPendingAsyncUsages(limit int) ([]*AsyncUsageInfo, error) {
	return GetPendingAsyncUsagesDue(limit, time.Now())
}

func GetPendingAsyncUsagesDue(
	limit int,
	now time.Time,
) ([]*AsyncUsageInfo, error) {
	var infos []*AsyncUsageInfo

	err := LogDB.
		Where("status = ?", int(AsyncUsageStatusPending)).
		Where(
			LogDB.
				Where("next_poll_at <= ?", now).
				Or("next_poll_at IS NULL"),
		).
		Order("next_poll_at ASC, updated_at ASC, created_at ASC").
		Limit(limit).
		Find(&infos).Error

	return infos, err
}

func AsyncUsageBackoffDelay(
	retryCount int,
) time.Duration {
	if retryCount <= 1 {
		return AsyncUsageDefaultPollDelay
	}

	delay := AsyncUsageDefaultPollDelay
	for range retryCount - 1 {
		delay *= 2
		if delay >= AsyncUsageMaxPollDelay {
			return AsyncUsageMaxPollDelay
		}
	}

	return delay
}

func UpdateAsyncUsageInfo(info *AsyncUsageInfo) error {
	info.UpdatedAt = time.Now()
	return LogDB.Save(info).Error
}

func UpdateLogUsageByRequestID(
	requestID string,
	usage Usage,
	amount Amount,
) error {
	var logEntry Log
	if err := LogDB.Where("request_id = ?", requestID).First(&logEntry).Error; err != nil {
		return err
	}

	logEntry.Usage = usage
	logEntry.Amount.Add(amount)
	logEntry.AsyncUsageStatus = AsyncUsageStatusCompleted

	return LogDB.Save(&logEntry).Error
}

func UpdateLogAsyncUsageStatusByRequestID(
	requestID string,
	status AsyncUsageStatus,
) error {
	if requestID == "" {
		return nil
	}

	tx := LogDB.
		Model(&Log{}).
		Where("request_id = ?", requestID).
		Update("async_usage_status", status)
	if tx.Error != nil {
		return tx.Error
	}

	if tx.RowsAffected == 0 {
		return NotFoundError("log")
	}

	return nil
}

func CleanupFinishedAsyncUsages(olderThan time.Duration, batchSize int) error {
	if batchSize <= 0 {
		batchSize = defaultCleanLogBatchSize
	}

	cutoff := time.Now().Add(-olderThan)

	subQuery := LogDB.
		Model(&AsyncUsageInfo{}).
		Where(
			"status IN (?) AND updated_at < ?",
			[]AsyncUsageStatus{AsyncUsageStatusCompleted, AsyncUsageStatusFailed},
			cutoff,
		).
		Limit(batchSize).
		Select("id")

	return LogDB.
		Session(&gorm.Session{SkipDefaultTransaction: true}).
		Where("id IN (?)", subQuery).
		Delete(&AsyncUsageInfo{}).Error
}
