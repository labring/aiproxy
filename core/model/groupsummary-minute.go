package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// only summary result only requests
type GroupSummaryMinute struct {
	ID     int                      `gorm:"primaryKey"`
	Unique GroupSummaryMinuteUnique `gorm:"embedded"`
	Data   SummaryData              `gorm:"embedded"`
}

type GroupSummaryMinuteUnique struct {
	GroupID         string `gorm:"size:64;not null;uniqueIndex:idx_groupsummary_minute_unique,priority:1"`
	TokenName       string `gorm:"size:32;not null;uniqueIndex:idx_groupsummary_minute_unique,priority:2"`
	Model           string `gorm:"size:64;not null;uniqueIndex:idx_groupsummary_minute_unique,priority:3"`
	MinuteTimestamp int64  `gorm:"not null;uniqueIndex:idx_groupsummary_minute_unique,priority:4,sort:desc"`
}

func (l *GroupSummaryMinute) BeforeCreate(_ *gorm.DB) (err error) {
	if l.Unique.Model == "" {
		return errors.New("model is required")
	}

	if l.Unique.MinuteTimestamp == 0 {
		return errors.New("minute timestamp is required")
	}

	if err := validateMinuteTimestamp(l.Unique.MinuteTimestamp); err != nil {
		return err
	}

	return err
}

func CreateGroupSummaryMinuteIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_minute_group_minute ON group_summary_minutes (group_id, minute_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_minute_group_token_minute ON group_summary_minutes (group_id, token_name, minute_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_minute_group_model_minute ON group_summary_minutes (group_id, model, minute_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertGroupSummaryMinute(unique GroupSummaryMinuteUnique, data SummaryData) error {
	err := validateMinuteTimestamp(unique.MinuteTimestamp)
	if err != nil {
		return err
	}

	for range 3 {
		result := LogDB.
			Model(&GroupSummaryMinute{}).
			Where(
				"group_id = ? AND token_name = ? AND model = ? AND minute_timestamp = ?",
				unique.GroupID,
				unique.TokenName,
				unique.Model,
				unique.MinuteTimestamp,
			).
			Updates(data.buildUpdateData("group_summary_minutes"))

		err = result.Error
		if err != nil {
			return err
		}

		if result.RowsAffected > 0 {
			return nil
		}

		err = createGroupSummaryMinute(unique, data)
		if err == nil {
			return nil
		}

		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createGroupSummaryMinute(unique GroupSummaryMinuteUnique, data SummaryData) error {
	return LogDB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "group_id"},
				{Name: "token_name"},
				{Name: "model"},
				{Name: "minute_timestamp"},
			},
			DoUpdates: clause.Assignments(data.buildUpdateData("group_summary_minutes")),
		}).
		Create(&GroupSummaryMinute{
			Unique: unique,
			Data:   data,
		}).Error
}

func GetGroupLastRequestTimeMinute(group string) (time.Time, error) {
	if group == "" {
		return time.Time{}, errors.New("group is required")
	}

	var summary GroupSummaryMinute

	err := LogDB.
		Model(&GroupSummaryMinute{}).
		Where("group_id = ?", group).
		Order("minute_timestamp desc").
		First(&summary).Error
	if summary.Unique.MinuteTimestamp == 0 {
		return time.Time{}, nil
	}

	return time.Unix(summary.Unique.MinuteTimestamp, 0), err
}

func GetGroupTokenLastRequestTimeMinute(group, token string) (time.Time, error) {
	var summary GroupSummaryMinute

	err := LogDB.
		Model(&GroupSummaryMinute{}).
		Where("group_id = ? AND token_name = ?", group, token).
		Order("minute_timestamp desc").
		First(&summary).Error
	if summary.Unique.MinuteTimestamp == 0 {
		return time.Time{}, nil
	}

	return time.Unix(summary.Unique.MinuteTimestamp, 0), err
}
