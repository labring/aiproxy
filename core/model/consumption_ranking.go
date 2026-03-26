package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type ConsumptionRankingType string

const (
	ConsumptionRankingTypeGroup   ConsumptionRankingType = "group"
	ConsumptionRankingTypeChannel ConsumptionRankingType = "channel"
	ConsumptionRankingTypeModel   ConsumptionRankingType = "model"
)

type ConsumptionRankingItem struct {
	GroupID      string  `json:"group_id,omitempty"   gorm:"column:group_id"`
	ChannelID    int     `json:"channel_id,omitempty" gorm:"column:channel_id"`
	Model        string  `json:"model,omitempty"      gorm:"column:model"`
	RequestCount int64   `json:"request_count"        gorm:"column:request_count"`
	UsedAmount   float64 `json:"used_amount"          gorm:"column:used_amount"`
	InputTokens  int64   `json:"input_tokens"         gorm:"column:input_tokens"`
	OutputTokens int64   `json:"output_tokens"        gorm:"column:output_tokens"`
	TotalTokens  int64   `json:"total_tokens"         gorm:"column:total_tokens"`
}

func normalizeConsumptionRankingOrder(order, dimension string) (normalized, clause string) {
	order = strings.ToLower(strings.TrimSpace(order))
	dimensionOrderDesc := dimension + "_desc"
	dimensionOrderAsc := dimension + "_asc"

	switch order {
	case "used_amount_asc":
		return "used_amount_asc", "used_amount ASC, request_count DESC, " + dimension + " ASC"
	case "request_count_desc":
		return "request_count_desc", "request_count DESC, used_amount DESC, " + dimension + " ASC"
	case "request_count_asc":
		return "request_count_asc", "request_count ASC, used_amount DESC, " + dimension + " ASC"
	case "total_tokens_desc":
		return "total_tokens_desc", "total_tokens DESC, used_amount DESC, " + dimension + " ASC"
	case "total_tokens_asc":
		return "total_tokens_asc", "total_tokens ASC, used_amount DESC, " + dimension + " ASC"
	case dimensionOrderDesc:
		return dimensionOrderDesc, dimension + " DESC"
	case dimensionOrderAsc:
		return dimensionOrderAsc, dimension + " ASC"
	default:
		return "used_amount_desc", "used_amount DESC, request_count DESC, " + dimension + " ASC"
	}
}

//nolint:unparam
func buildConsumptionRankingTimeQuery(
	tx *gorm.DB,
	timestampColumn string,
	start, end time.Time,
) *gorm.DB {
	switch {
	case !start.IsZero() && !end.IsZero():
		tx = tx.Where(timestampColumn+" BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		tx = tx.Where(timestampColumn+" >= ?", start.Unix())
	case !end.IsZero():
		tx = tx.Where(timestampColumn+" <= ?", end.Unix())
	}

	return tx
}

func GetConsumptionRanking(
	rankingType ConsumptionRankingType,
	start, end time.Time,
	page, perPage int,
	order string,
) ([]ConsumptionRankingItem, int64, string, error) {
	page, perPage = NormalizePageParams(page, perPage)
	limit, offset := toLimitOffset(page, perPage)

	var (
		baseQuery   *gorm.DB
		groupField  string
		selectField string
	)

	switch rankingType {
	case ConsumptionRankingTypeChannel:
		groupField = "channel_id"
		selectField = "channel_id"
		baseQuery = buildConsumptionRankingTimeQuery(
			LogDB.Model(&Summary{}),
			"hour_timestamp",
			start,
			end,
		)
	case ConsumptionRankingTypeModel:
		groupField = "model"
		selectField = "model"
		baseQuery = buildConsumptionRankingTimeQuery(
			LogDB.Model(&Summary{}),
			"hour_timestamp",
			start,
			end,
		)
	case ConsumptionRankingTypeGroup:
		groupField = "group_id"
		selectField = "group_id"
		baseQuery = buildConsumptionRankingTimeQuery(
			LogDB.Model(&GroupSummary{}),
			"hour_timestamp",
			start,
			end,
		)
	default:
		groupField = "group_id"
		selectField = "group_id"
		baseQuery = buildConsumptionRankingTimeQuery(
			LogDB.Model(&GroupSummary{}),
			"hour_timestamp",
			start,
			end,
		)
	}

	normalizedOrder, orderClause := normalizeConsumptionRankingOrder(order, groupField)

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).
		Distinct(selectField).
		Count(&total).
		Error; err != nil {
		return nil, 0, normalizedOrder, err
	}

	items := make([]ConsumptionRankingItem, 0, perPage)
	if err := baseQuery.
		Session(&gorm.Session{}).
		Select(
			selectField + ", " +
				"SUM(request_count) as request_count, " +
				"SUM(used_amount) as used_amount, " +
				"SUM(input_tokens) as input_tokens, " +
				"SUM(output_tokens) as output_tokens, " +
				"SUM(total_tokens) as total_tokens",
		).
		Group(selectField).
		Order(orderClause).
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, normalizedOrder, err
	}

	return items, total, normalizedOrder, nil
}
