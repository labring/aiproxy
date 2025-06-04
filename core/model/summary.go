package model

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// only summary result only requests
type Summary struct {
	ID     int           `gorm:"primaryKey"`
	Unique SummaryUnique `gorm:"embedded"`
	Data   SummaryData   `gorm:"embedded"`
}

type SummaryUnique struct {
	ChannelID     int    `gorm:"not null;uniqueIndex:idx_summary_unique,priority:1"`
	Model         string `gorm:"not null;uniqueIndex:idx_summary_unique,priority:2"`
	HourTimestamp int64  `gorm:"not null;uniqueIndex:idx_summary_unique,priority:3,sort:desc"`
}

type SummaryData struct {
	RequestCount   int64   `json:"request_count"`
	UsedAmount     float64 `json:"used_amount"`
	ExceptionCount int64   `json:"exception_count"`
	MaxRPM         int64   `json:"max_rpm,omitempty"`
	MaxRPS         int64   `json:"max_rps,omitempty"`
	MaxTPM         int64   `json:"max_tpm,omitempty"`
	MaxTPS         int64   `json:"max_tps,omitempty"`
	Usage          Usage   `json:"usage,omitempty"   gorm:"embedded"`
}

func (d *SummaryData) buildUpdateData(tableName string) map[string]any {
	data := map[string]any{}
	if d.RequestCount > 0 {
		data["request_count"] = gorm.Expr(tableName+".request_count + ?", d.RequestCount)
	}
	if d.UsedAmount > 0 {
		data["used_amount"] = gorm.Expr(tableName+".used_amount + ?", d.UsedAmount)
	}
	if d.ExceptionCount > 0 {
		data["exception_count"] = gorm.Expr(tableName+".exception_count + ?", d.ExceptionCount)
	}

	// max rpm tpm update
	if d.MaxRPM > 0 {
		data["max_rpm"] = gorm.Expr(
			fmt.Sprintf(
				"CASE WHEN %s.max_rpm < ? THEN ? ELSE %s.max_rpm END",
				tableName,
				tableName,
			),
			d.MaxRPM,
			d.MaxRPM,
		)
	}
	if d.MaxRPS > 0 {
		data["max_rps"] = gorm.Expr(
			fmt.Sprintf(
				"CASE WHEN %s.max_rps < ? THEN ? ELSE %s.max_rps END",
				tableName,
				tableName,
			),
			d.MaxRPS,
			d.MaxRPS,
		)
	}
	if d.MaxTPM > 0 {
		data["max_tpm"] = gorm.Expr(
			fmt.Sprintf(
				"CASE WHEN %s.max_tpm < ? THEN ? ELSE %s.max_tpm END",
				tableName,
				tableName,
			),
			d.MaxTPM,
			d.MaxTPM,
		)
	}
	if d.MaxTPS > 0 {
		data["max_tps"] = gorm.Expr(
			fmt.Sprintf(
				"CASE WHEN %s.max_tps < ? THEN ? ELSE %s.max_tps END",
				tableName,
				tableName,
			),
			d.MaxTPS,
			d.MaxTPS,
		)
	}

	// usage update
	if d.Usage.InputTokens > 0 {
		data["input_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.input_tokens, 0) + ?", tableName),
			d.Usage.InputTokens,
		)
	}
	if d.Usage.ImageInputTokens > 0 {
		data["image_input_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.image_input_tokens, 0) + ?", tableName),
			d.Usage.ImageInputTokens,
		)
	}
	if d.Usage.OutputTokens > 0 {
		data["output_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.output_tokens, 0) + ?", tableName),
			d.Usage.OutputTokens,
		)
	}
	if d.Usage.TotalTokens > 0 {
		data["total_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.total_tokens, 0) + ?", tableName),
			d.Usage.TotalTokens,
		)
	}
	if d.Usage.CachedTokens > 0 {
		data["cached_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.cached_tokens, 0) + ?", tableName),
			d.Usage.CachedTokens,
		)
	}
	if d.Usage.CacheCreationTokens > 0 {
		data["cache_creation_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.cache_creation_tokens, 0) + ?", tableName),
			d.Usage.CacheCreationTokens,
		)
	}
	if d.Usage.WebSearchCount > 0 {
		data["web_search_count"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.web_search_count, 0) + ?", tableName),
			d.Usage.WebSearchCount,
		)
	}
	return data
}

func (l *Summary) BeforeCreate(_ *gorm.DB) (err error) {
	if l.Unique.ChannelID == 0 {
		return errors.New("channel id is required")
	}
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

var hourTimestampDivisor = int64(time.Hour.Seconds())

func validateHourTimestamp(hourTimestamp int64) error {
	if hourTimestamp%hourTimestampDivisor != 0 {
		return errors.New("hour timestamp must be an exact hour")
	}
	return nil
}

func CreateSummaryIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_summary_channel_hour ON summaries (channel_id, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_summary_model_hour ON summaries (model, hour_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertSummary(unique SummaryUnique, data SummaryData) error {
	err := validateHourTimestamp(unique.HourTimestamp)
	if err != nil {
		return err
	}

	for range 3 {
		result := LogDB.
			Model(&Summary{}).
			Where(
				"channel_id = ? AND model = ? AND hour_timestamp = ?",
				unique.ChannelID,
				unique.Model,
				unique.HourTimestamp,
			).
			Updates(data.buildUpdateData("summaries"))
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
	return LogDB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "channel_id"},
				{Name: "model"},
				{Name: "hour_timestamp"},
			},
			DoUpdates: clause.Assignments(data.buildUpdateData("summaries")),
		}).
		Create(&Summary{
			Unique: unique,
			Data:   data,
		}).Error
}

func getChartData(
	group string,
	start, end time.Time,
	tokenName, modelName string,
	channelID int,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]*ChartData, error) {
	var query *gorm.DB

	if group == "*" || channelID != 0 {
		query = LogDB.Model(&Summary{})
		if channelID != 0 {
			query = query.Where("channel_id = ?", channelID)
		}
	} else {
		query = LogDB.Model(&GroupSummary{}).
			Where("group_id = ?", group)
		if tokenName != "" {
			query = query.Where("token_name = ?", tokenName)
		}
	}

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	// Only include max metrics when we have specific channel and model
	selectFields := "hour_timestamp as timestamp, sum(request_count) as request_count, sum(used_amount) as used_amount, " +
		"sum(exception_count) as exception_count, sum(input_tokens) as input_tokens, sum(output_tokens) as output_tokens, " +
		"sum(cached_tokens) as cached_tokens, sum(cache_creation_tokens) as cache_creation_tokens, " +
		"sum(total_tokens) as total_tokens, sum(web_search_count) as web_search_count"

	// Only include max metrics when querying for a specific channel and model
	if (channelID != 0 && modelName != "") || (group != "*" && tokenName != "" && modelName != "") {
		selectFields += ", max(max_rpm) as max_rpm, max(max_rps) as max_rps, max(max_tpm) as max_tpm, max(max_tps) as max_tps"
	} else {
		// Set max metrics to 0 when not querying for specific channel and model
		selectFields += ", 0 as max_rpm, 0 as max_rps, 0 as max_tpm, 0 as max_tps"
	}

	query = query.
		Select(selectFields).
		Group("timestamp").
		Order("timestamp ASC")

	var chartData []*ChartData
	err := query.Scan(&chartData).Error
	if err != nil {
		return nil, err
	}

	// If timeSpan is day, aggregate hour data into day data
	if timeSpan == TimeSpanDay && len(chartData) > 0 {
		return aggregateHourDataToDay(chartData, timezone), nil
	}

	return chartData, nil
}

func GetUsedChannels(group string, start, end time.Time) ([]int, error) {
	if group != "*" {
		return []int{}, nil
	}
	return getLogGroupByValues[int]("channel_id", group, "", start, end)
}

func GetUsedModels(group, tokenName string, start, end time.Time) ([]string, error) {
	return getLogGroupByValues[string]("model", group, tokenName, start, end)
}

func GetUsedTokenNames(group string, start, end time.Time) ([]string, error) {
	return getLogGroupByValues[string]("token_name", group, "", start, end)
}

func getLogGroupByValues[T cmp.Ordered](
	field, group, tokenName string,
	start, end time.Time,
) ([]T, error) {
	type Result struct {
		Value        T
		UsedAmount   float64
		RequestCount int64
	}
	var results []Result

	var query *gorm.DB

	if group == "*" {
		query = LogDB.
			Model(&Summary{})
	} else {
		query = LogDB.
			Model(&GroupSummary{}).
			Where("group_id = ?", group)
		if tokenName != "" {
			query = query.Where("token_name = ?", tokenName)
		}
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	err := query.
		Select(
			field + " as value, SUM(request_count) as request_count, SUM(used_amount) as used_amount",
		).
		Group(field).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	slices.SortFunc(results, func(a, b Result) int {
		if a.UsedAmount != b.UsedAmount {
			return cmp.Compare(b.UsedAmount, a.UsedAmount)
		}
		if a.RequestCount != b.RequestCount {
			return cmp.Compare(b.RequestCount, a.RequestCount)
		}
		return cmp.Compare(a.Value, b.Value)
	})

	values := make([]T, len(results))
	for i, result := range results {
		values[i] = result.Value
	}

	return values, nil
}

type CostRank struct {
	Model               string  `json:"model"`
	UsedAmount          float64 `json:"used_amount"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CachedTokens        int64   `json:"cached_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	RequestCount        int64   `json:"request_count"`
	WebSearchCount      int64   `json:"web_search_count"`

	MaxRPM int64 `json:"max_rpm,omitempty"`
	MaxRPS int64 `json:"max_rps,omitempty"`
	MaxTPM int64 `json:"max_tpm,omitempty"`
	MaxTPS int64 `json:"max_tps,omitempty"`
}

func GetModelCostRank(
	group, tokenName string,
	channelID int,
	start, end time.Time,
) ([]*CostRank, error) {
	var ranks []*CostRank

	var query *gorm.DB
	if group == "*" || channelID != 0 {
		query = LogDB.Model(&Summary{})
		if channelID != 0 {
			query = query.Where("channel_id = ?", channelID)
		}
	} else {
		query = LogDB.Model(&GroupSummary{}).
			Where("group_id = ?", group)
		if tokenName != "" {
			query = query.Where("token_name = ?", tokenName)
		}
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	selectFields := "model, SUM(used_amount) as used_amount, SUM(request_count) as request_count, SUM(input_tokens) as input_tokens, SUM(output_tokens) as output_tokens, SUM(cached_tokens) as cached_tokens, SUM(cache_creation_tokens) as cache_creation_tokens, SUM(total_tokens) as total_tokens"
	if (channelID != 0) || (group != "*" && tokenName != "") {
		selectFields += ", max(max_rpm) as max_rpm, max(max_rps) as max_rps, max(max_tpm) as max_tps, max(max_tps) as max_tps"
	} else {
		selectFields += ", 0 as max_rpm, 0 as max_rps, 0 as max_tpm, 0 as max_tps"
	}

	query = query.
		Select(selectFields).
		Group("model")

	err := query.Scan(&ranks).Error
	if err != nil {
		return nil, err
	}

	slices.SortFunc(ranks, func(a, b *CostRank) int {
		if a.UsedAmount != b.UsedAmount {
			return cmp.Compare(b.UsedAmount, a.UsedAmount)
		}
		if a.TotalTokens != b.TotalTokens {
			return cmp.Compare(b.TotalTokens, a.TotalTokens)
		}
		if a.RequestCount != b.RequestCount {
			return cmp.Compare(b.RequestCount, a.RequestCount)
		}
		return cmp.Compare(a.Model, b.Model)
	})

	return ranks, nil
}
