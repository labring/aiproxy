package model

import (
	"errors"
	"strings"
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
	GroupID       string `gorm:"size:64;not null;uniqueIndex:idx_groupsummary_unique,priority:1"`
	TokenName     string `gorm:"size:32;not null;uniqueIndex:idx_groupsummary_unique,priority:2"`
	Model         string `gorm:"size:64;not null;uniqueIndex:idx_groupsummary_unique,priority:3"`
	HourTimestamp int64  `gorm:"not null;uniqueIndex:idx_groupsummary_unique,priority:4,sort:desc"`
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

	return err
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
			Columns: []clause.Column{
				{Name: "group_id"},
				{Name: "token_name"},
				{Name: "model"},
				{Name: "hour_timestamp"},
			},
			DoUpdates: clause.Assignments(data.buildUpdateData("group_summaries")),
		}).
		Create(&GroupSummary{
			Unique: unique,
			Data:   data,
		}).Error
}

type GroupConsumptionRankingItem struct {
	GroupID      string  `json:"group_id"      gorm:"column:group_id"`
	RequestCount int64   `json:"request_count" gorm:"column:request_count"`
	UsedAmount   float64 `json:"used_amount"   gorm:"column:used_amount"`
	InputTokens  int64   `json:"input_tokens"  gorm:"column:input_tokens"`
	OutputTokens int64   `json:"output_tokens" gorm:"column:output_tokens"`
	TotalTokens  int64   `json:"total_tokens"  gorm:"column:total_tokens"`
}

func normalizeGroupConsumptionRankingOrder(order string) (normalized, clause string) {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "used_amount_asc":
		return "used_amount_asc", "used_amount ASC, request_count DESC, group_id ASC"
	case "request_count_desc":
		return "request_count_desc", "request_count DESC, used_amount DESC, group_id ASC"
	case "request_count_asc":
		return "request_count_asc", "request_count ASC, used_amount DESC, group_id ASC"
	case "total_tokens_desc":
		return "total_tokens_desc", "total_tokens DESC, used_amount DESC, group_id ASC"
	case "total_tokens_asc":
		return "total_tokens_asc", "total_tokens ASC, used_amount DESC, group_id ASC"
	case "group_id_desc":
		return "group_id_desc", "group_id DESC"
	case "group_id_asc":
		return "group_id_asc", "group_id ASC"
	default:
		return "used_amount_desc", "used_amount DESC, request_count DESC, group_id ASC"
	}
}

func buildGroupConsumptionRankingQuery(start, end time.Time) *gorm.DB {
	query := LogDB.Model(&GroupSummary{})

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	return query
}

func GetGroupConsumptionRanking(
	start, end time.Time,
	page, perPage int,
	order string,
) ([]GroupConsumptionRankingItem, int64, string, error) {
	page, perPage = NormalizePageParams(page, perPage)
	normalizedOrder, orderClause := normalizeGroupConsumptionRankingOrder(order)
	limit, offset := toLimitOffset(page, perPage)

	baseQuery := buildGroupConsumptionRankingQuery(start, end)

	var total int64

	err := baseQuery.
		Session(&gorm.Session{}).
		Distinct("group_id").
		Count(&total).Error
	if err != nil {
		return nil, 0, normalizedOrder, err
	}

	items := make([]GroupConsumptionRankingItem, 0, perPage)

	err = baseQuery.
		Session(&gorm.Session{}).
		Select(
			"group_id, " +
				"SUM(request_count) as request_count, " +
				"SUM(used_amount) as used_amount, " +
				"SUM(input_tokens) as input_tokens, " +
				"SUM(output_tokens) as output_tokens, " +
				"SUM(total_tokens) as total_tokens",
		).
		Group("group_id").
		Order(orderClause).
		Offset(offset).
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, normalizedOrder, err
	}

	return items, total, normalizedOrder, nil
}
