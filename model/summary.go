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
	ChannelID     int    `gorm:"uniqueIndex:idx_summary_unique,priority:1"`
	Model         string `gorm:"uniqueIndex:idx_summary_unique,priority:2"`
	HourTimestamp int64  `gorm:"uniqueIndex:idx_summary_unique,priority:3,sort:desc"`
}

type SummaryData struct {
	RequestCount   int64   `json:"request_count"`
	UsedAmount     float64 `json:"used_amount"`
	ExceptionCount int64   `json:"exception_count"`
	Usage          Usage   `gorm:"embedded"        json:"usage,omitempty"`
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

func getChartDataFromSummary(
	group string,
	start, end time.Time,
	tokenName, modelName string,
	channelID int,
	timeSpan TimeSpanType,
) ([]*ChartData, error) {
	var query *gorm.DB

	if group == "*" || channelID != 0 {
		query = LogDB.Model(&Summary{}).
			Select("hour_timestamp as timestamp, count(*) as request_count, sum(used_amount) as used_amount, sum(exception_count) as exception_count, sum(input_tokens) as input_tokens, sum(output_tokens) as output_tokens, sum(cached_tokens) as cached_tokens, sum(cache_creation_tokens) as cache_creation_tokens, sum(total_tokens) as total_tokens").
			Group("timestamp").
			Order("timestamp ASC")

		if channelID != 0 {
			query = query.Where("channel_id = ?", channelID)
		}
	} else {
		query = LogDB.Model(&GroupSummary{}).
			Select("hour_timestamp as timestamp, count(*) as request_count, sum(used_amount) as used_amount, sum(exception_count) as exception_count, sum(input_tokens) as input_tokens, sum(output_tokens) as output_tokens, sum(cached_tokens) as cached_tokens, sum(cache_creation_tokens) as cache_creation_tokens, sum(total_tokens) as total_tokens").
			Group("timestamp").
			Order("timestamp ASC").
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

	var chartData []*ChartData
	err := query.Scan(&chartData).Error
	if err != nil {
		return nil, err
	}

	// If timeSpan is day, aggregate hour data into day data
	if timeSpan == TimeSpanDay && len(chartData) > 0 {
		return aggregateHourDataToDay(chartData), nil
	}

	return chartData, nil
}
