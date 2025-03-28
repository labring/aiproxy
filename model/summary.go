package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// only summary result only requests
type Summary struct {
	ID     int           `gorm:"primaryKey"`
	Unique SummaryUnique `gorm:"embedded"`
	Data   SummaryData   `gorm:"embedded"`
}

type SummaryUnique struct {
	GroupID       string
	Model         string
	TokenName     string
	ChannelID     int
	HourTimestamp int64
}

type SummaryData struct {
	RequestCount   int64   `json:"request_count"`
	UsedAmount     float64 `json:"used_amount"`
	ExceptionCount int64   `json:"exception_count"`
	Usage          Usage   `gorm:"embedded"        json:"usage,omitempty"`
}

func (l *Summary) BeforeCreate(_ *gorm.DB) (err error) {
	if l.Unique.Model == "" {
		return errors.New("model is required")
	}
	if l.Unique.ChannelID == 0 {
		return errors.New("channel id is required")
	}
	if l.Unique.HourTimestamp == 0 {
		return errors.New("hour timestamp is required")
	}
	if err := validateHourTimestamp(l.Unique.HourTimestamp); err != nil {
		return err
	}
	return
}

var hourTimestampDivisor = int64(time.Hour.Seconds())

func validateHourTimestamp(hourTimestamp int64) error {
	if hourTimestamp%hourTimestampDivisor != 0 {
		return errors.New("hour timestamp must be an exact hour")
	}
	return nil
}

func CreateSummaryIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_summary_unique_group_token_model_channel_hour ON summaries (group_id, token_name, model, channel_id, hour_timestamp DESC)",

		"CREATE INDEX IF NOT EXISTS idx_summary_group_hour ON summaries (group_id, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_summary_group_token_hour ON summaries (group_id, token_name, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_summary_group_model_hour ON summaries (group_id, model, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_summary_group_token_model_hour ON summaries (group_id, token_name, model, hour_timestamp DESC)",

		"CREATE INDEX IF NOT EXISTS idx_summary_model_hour ON summaries (model, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_summary_channel_hour ON summaries (channel_id, hour_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpdateSummary(unique SummaryUnique, data SummaryData) error {
	err := validateHourTimestamp(unique.HourTimestamp)
	if err != nil {
		return err
	}

	for range 3 {
		result := LogDB.
			Model(&Summary{}).
			Where(
				"group_id = ? AND token_name = ? AND model = ? AND channel_id = ? AND hour_timestamp = ?",
				unique.GroupID,
				unique.TokenName,
				unique.Model,
				unique.ChannelID,
				unique.HourTimestamp,
			).
			Updates(map[string]interface{}{
				"request_count":         gorm.Expr("request_count + ?", data.RequestCount),
				"used_amount":           gorm.Expr("used_amount + ?", data.UsedAmount),
				"exception_count":       gorm.Expr("exception_count + ?", data.ExceptionCount),
				"input_tokens":          gorm.Expr("input_tokens + ?", data.Usage.InputTokens),
				"output_tokens":         gorm.Expr("output_tokens + ?", data.Usage.OutputTokens),
				"total_tokens":          gorm.Expr("total_tokens + ?", data.Usage.TotalTokens),
				"cached_tokens":         gorm.Expr("cached_tokens + ?", data.Usage.CachedTokens),
				"cache_creation_tokens": gorm.Expr("cache_creation_tokens + ?", data.Usage.CacheCreationTokens),
			})
		err = result.Error
		if err != nil {
			return err
		}
		if result.RowsAffected > 0 {
			return nil
		}

		err = createSummary(unique, data)
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createSummary(unique SummaryUnique, data SummaryData) error {
	return LogDB.Create(&Summary{
		Unique: unique,
		Data:   data,
	}).Error
}
