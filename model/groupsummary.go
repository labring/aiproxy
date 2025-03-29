package model

import (
	"errors"

	"gorm.io/gorm"
)

// only summary result only requests
type GroupSummary struct {
	ID     int                `gorm:"primaryKey"`
	Unique GroupSummaryUnique `gorm:"embedded"`
	Data   SummaryData        `gorm:"embedded"`
}

type GroupSummaryUnique struct {
	GroupID       string `gorm:"uniqueIndex:idx_groupsummary_unique,priority:1"`
	TokenName     string `gorm:"uniqueIndex:idx_groupsummary_unique,priority:2"`
	Model         string `gorm:"uniqueIndex:idx_groupsummary_unique,priority:3"`
	HourTimestamp int64  `gorm:"uniqueIndex:idx_groupsummary_unique,priority:4,sort:desc"`
}

func (l *GroupSummary) BeforeCreate(_ *gorm.DB) (err error) {
	if l.Unique.Model == "" {
		return errors.New("model is required")
	}
	if l.Unique.HourTimestamp == 0 {
		return errors.New("hour timestamp is required")
	}
	if err := validateHourTimestamp(l.Unique.HourTimestamp); err != nil {
		return err
	}
	return
}

func CreateGroupSummaryIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_group_hour ON summaries (group_id, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_group_token_hour ON summaries (group_id, token_name, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_group_model_hour ON summaries (group_id, model, hour_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertGroupSummary(unique GroupSummaryUnique, data SummaryData) error {
	err := validateHourTimestamp(unique.HourTimestamp)
	if err != nil {
		return err
	}

	for range 3 {
		result := LogDB.
			Model(&GroupSummary{}).
			Where(
				"group_id = ? AND token_name = ? AND model = ? AND hour_timestamp = ?",
				unique.GroupID,
				unique.TokenName,
				unique.Model,
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

		err = createGroupSummary(unique, data)
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createGroupSummary(unique GroupSummaryUnique, data SummaryData) error {
	return LogDB.Create(&GroupSummary{
		Unique: unique,
		Data:   data,
	}).Error
}
