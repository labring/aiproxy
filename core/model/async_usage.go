package model

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type AsyncUsageStatus int

const (
	AsyncUsageStatusNone AsyncUsageStatus = iota
	AsyncUsageStatusPending
	AsyncUsageStatusCompleted
	AsyncUsageStatusFailed
)

const (
	AsyncUsageDefaultPollDelay = 3 * time.Second
	AsyncUsageMaxPollDelay     = 3 * time.Minute
)

var asyncUsageSchemaCache sync.Map

type AsyncUsageInfo struct {
	ID                          int              `gorm:"primaryKey"              json:"id"`
	RequestID                   string           `gorm:"type:char(16);index"     json:"request_id"`
	RequestAt                   time.Time        `                               json:"request_at"`
	Mode                        int              `gorm:"index"                   json:"mode"`
	Model                       string           `gorm:"size:128"                json:"model"`
	ChannelID                   int              `gorm:"index"                   json:"channel_id"`
	BaseURL                     string           `gorm:"type:text"               json:"base_url,omitempty"`
	GroupID                     string           `gorm:"size:64;index"           json:"group_id"`
	TokenID                     int              `gorm:"index"                   json:"token_id"`
	TokenName                   string           `gorm:"size:128"                json:"token_name,omitempty"`
	Price                       Price            `gorm:"embedded"                json:"price"`
	UpstreamID                  string           `gorm:"type:varchar(256);index" json:"upstream_id"`
	Status                      AsyncUsageStatus `gorm:"index;default:1"         json:"status"`
	Usage                       Usage            `gorm:"embedded"                json:"usage"`
	UsageContext                UsageContext     `gorm:"embedded"                json:"usage_context,omitempty"`
	DisableResolutionFuzzyMatch bool             `                               json:"disable_resolution_fuzzy_match,omitempty"`
	Amount                      Amount           `gorm:"embedded"                json:"amount,omitempty"`
	Error                       string           `gorm:"type:text"               json:"error,omitempty"`
	RetryCount                  int              `                               json:"retry_count"`
	BalanceConsumed             bool             `                               json:"balance_consumed"`
	ProcessingToken             string           `gorm:"size:64;index"           json:"-"`
	NextPollAt                  time.Time        `gorm:"index"                   json:"next_poll_at"`
	CreatedAt                   time.Time        `                               json:"created_at"`
	UpdatedAt                   time.Time        `                               json:"updated_at"`
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

func TryClaimAsyncUsageInfo(
	info *AsyncUsageInfo,
	token string,
	leaseUntil time.Time,
	now time.Time,
) (bool, error) {
	if info == nil || info.ID == 0 || token == "" {
		return false, nil
	}

	tx := LogDB.
		Model(&AsyncUsageInfo{}).
		Where("id = ? AND status = ?", info.ID, int(AsyncUsageStatusPending)).
		Where(
			LogDB.
				Where("next_poll_at <= ?", now).
				Or("next_poll_at IS NULL"),
		).
		Updates(map[string]any{
			"processing_token": token,
			"next_poll_at":     leaseUntil,
			"updated_at":       now,
		})
	if tx.Error != nil {
		return false, tx.Error
	}

	if tx.RowsAffected == 0 {
		return false, nil
	}

	info.ProcessingToken = token
	info.NextPollAt = leaseUntil
	info.UpdatedAt = now

	return true, nil
}

func RenewAsyncUsageClaim(
	id int,
	token string,
	leaseUntil time.Time,
) (bool, error) {
	if id == 0 || token == "" {
		return false, nil
	}

	now := time.Now()

	tx := LogDB.
		Model(&AsyncUsageInfo{}).
		Where("id = ? AND status = ? AND processing_token = ?", id, int(AsyncUsageStatusPending), token).
		Updates(map[string]any{
			"next_poll_at": leaseUntil,
			"updated_at":   now,
		})
	if tx.Error != nil {
		return false, tx.Error
	}

	return tx.RowsAffected > 0, nil
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

func MarkAsyncUsageBalanceConsumed(info *AsyncUsageInfo) error {
	return updateClaimedAsyncUsageInfo(info, map[string]any{
		"balance_consumed": true,
	})
}

func ClaimedAsyncUsageInfoExists(info *AsyncUsageInfo) (bool, error) {
	if info == nil || info.ID == 0 || info.ProcessingToken == "" {
		return false, nil
	}

	var count int64

	err := LogDB.
		Model(&AsyncUsageInfo{}).
		Where("id = ? AND status = ? AND processing_token = ?",
			info.ID,
			int(AsyncUsageStatusPending),
			info.ProcessingToken,
		).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func RetryClaimedAsyncUsageInfo(info *AsyncUsageInfo) error {
	return updateClaimedAsyncUsageInfo(info, map[string]any{
		"retry_count":      info.RetryCount,
		"error":            info.Error,
		"next_poll_at":     info.NextPollAt,
		"processing_token": "",
	})
}

func TouchClaimedAsyncUsageInfo(info *AsyncUsageInfo) error {
	return updateClaimedAsyncUsageInfo(info, map[string]any{
		"error":            "",
		"next_poll_at":     info.NextPollAt,
		"processing_token": "",
	})
}

func FailClaimedAsyncUsageInfo(info *AsyncUsageInfo) (bool, error) {
	tx := LogDB.
		Model(&AsyncUsageInfo{}).
		Where("id = ? AND processing_token = ?", info.ID, info.ProcessingToken).
		Updates(map[string]any{
			"status":           int(AsyncUsageStatusFailed),
			"error":            info.Error,
			"processing_token": "",
			"updated_at":       time.Now(),
		})
	if tx.Error != nil {
		return false, tx.Error
	}

	return tx.RowsAffected > 0, nil
}

func SaveClaimedAsyncUsageResult(
	info *AsyncUsageInfo,
	usage Usage,
	usageContext UsageContext,
	amount Amount,
) (bool, error) {
	now := time.Now()
	updatesModel := &AsyncUsageInfo{
		Usage:           usage,
		UsageContext:    usageContext,
		Amount:          amount,
		Error:           "",
		BalanceConsumed: info.BalanceConsumed,
		UpdatedAt:       now,
	}

	updates, err := asyncUsageUpdateValues(
		updatesModel,
		"Usage",
		"UsageContext",
		"Amount",
		"Error",
		"BalanceConsumed",
		"UpdatedAt",
	)
	if err != nil {
		return false, err
	}

	tx := LogDB.
		Model(&AsyncUsageInfo{}).
		Where("id = ? AND processing_token = ?", info.ID, info.ProcessingToken).
		Updates(updates)
	if tx.Error != nil {
		return false, tx.Error
	}

	return tx.RowsAffected > 0, nil
}

func CompleteClaimedAsyncUsageInfo(info *AsyncUsageInfo) (bool, error) {
	now := time.Now()
	updatesModel := &AsyncUsageInfo{
		Status:          AsyncUsageStatusCompleted,
		Error:           "",
		ProcessingToken: "",
		UpdatedAt:       now,
	}

	updates, err := asyncUsageUpdateValues(
		updatesModel,
		"Status",
		"Error",
		"ProcessingToken",
		"UpdatedAt",
	)
	if err != nil {
		return false, err
	}

	tx := LogDB.
		Model(&AsyncUsageInfo{}).
		Where("id = ? AND processing_token = ?", info.ID, info.ProcessingToken).
		Updates(updates)
	if tx.Error != nil {
		return false, tx.Error
	}

	return tx.RowsAffected > 0, nil
}

func asyncUsageUpdateValues(
	info *AsyncUsageInfo,
	names ...string,
) (map[string]any, error) {
	if info == nil {
		return nil, errors.New("async usage info is nil")
	}

	var namer schema.Namer = schema.NamingStrategy{IdentifierMaxLength: 64}
	if LogDB != nil && LogDB.NamingStrategy != nil {
		namer = LogDB.NamingStrategy
	}

	s, err := schema.Parse(&AsyncUsageInfo{}, &asyncUsageSchemaCache, namer)
	if err != nil {
		return nil, fmt.Errorf("parse async usage schema: %w", err)
	}

	values := make(map[string]any)
	reflectValue := reflect.ValueOf(info)
	ctx := context.Background()
	selected := make(map[string]struct{}, len(names))
	seen := make(map[string]struct{}, len(names))

	for _, name := range names {
		selected[name] = struct{}{}
	}

	for _, field := range s.Fields {
		if !field.Updatable || field.DBName == "" || len(field.BindNames) == 0 {
			continue
		}

		topName := field.BindNames[0]
		if _, ok := selected[topName]; !ok {
			continue
		}

		value, _ := field.ValueOf(ctx, reflectValue)
		values[field.DBName] = value
		seen[topName] = struct{}{}
	}

	for _, name := range names {
		if _, ok := seen[name]; !ok {
			return nil, fmt.Errorf("async usage field %q not found", name)
		}
	}

	return values, nil
}

func updateClaimedAsyncUsageInfo(
	info *AsyncUsageInfo,
	updates map[string]any,
) error {
	if info == nil || info.ProcessingToken == "" {
		return NotFoundError("async usage claim")
	}

	updates["updated_at"] = time.Now()

	tx := LogDB.
		Model(&AsyncUsageInfo{}).
		Where("id = ? AND processing_token = ?", info.ID, info.ProcessingToken).
		Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}

	if tx.RowsAffected == 0 {
		return NotFoundError("async usage claim")
	}

	return nil
}

func UpdateLogUsageByRequestID(
	requestID string,
	usage Usage,
	usageContext UsageContext,
	price Price,
	amount Amount,
) error {
	var logEntry Log
	if err := LogDB.Where("request_id = ?", requestID).First(&logEntry).Error; err != nil {
		return err
	}

	logEntry.Usage = usage
	logEntry.UsageContext = usageContext
	logEntry.Price = price
	logEntry.Amount = amount
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

func UpdateLogAsyncUsageFailedByRequestID(requestID, message string) error {
	if requestID == "" {
		return nil
	}

	tx := LogDB.
		Model(&Log{}).
		Where("request_id = ?", requestID).
		Updates(map[string]any{
			"async_usage_status": AsyncUsageStatusFailed,
			"content":            message,
		})
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
