package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_group_hour ON group_summaries (group_id, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_group_token_hour ON group_summaries (group_id, token_name, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_groupsummary_group_model_hour ON group_summaries (group_id, model, hour_timestamp DESC)",
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
			Updates(data.buildUpdateData("group_summaries"))
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
	return LogDB.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "group_id"}, {Name: "token_name"}, {Name: "model"}, {Name: "hour_timestamp"}},
			DoUpdates: clause.Assignments(data.buildUpdateData("group_summaries")),
		}).
		Create(&GroupSummary{
			Unique: unique,
			Data:   data,
		}).Error
}

func GetGroupLastRequestTime(group string) (time.Time, error) {
	if group == "" {
		return time.Time{}, errors.New("group is required")
	}
	var summary GroupSummary
	err := LogDB.
		Model(&GroupSummary{}).
		Where("group_id = ?", group).
		Order("hour_timestamp desc").
		First(&summary).Error
	return time.Unix(summary.Unique.HourTimestamp, 0), err
}

func GetGroupTokenLastRequestTime(group string, token string) (time.Time, error) {
	var summary GroupSummary
	err := LogDB.
		Model(&GroupSummary{}).
		Where("group_id = ? AND token_name = ?", group, token).
		Order("hour_timestamp desc").
		First(&summary).Error
	return time.Unix(summary.Unique.HourTimestamp, 0), err
}
