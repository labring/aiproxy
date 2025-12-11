package model

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
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
	Model         string `gorm:"size:64;not null;uniqueIndex:idx_summary_unique,priority:2"`
	HourTimestamp int64  `gorm:"not null;uniqueIndex:idx_summary_unique,priority:3,sort:desc"`
}

type Count struct {
	RequestCount   int64         `json:"request_count"`
	RetryCount     ZeroNullInt64 `json:"retry_count"`
	ExceptionCount ZeroNullInt64 `json:"exception_count"`
	Status4xxCount ZeroNullInt64 `json:"status_4xx_count"`
	Status5xxCount ZeroNullInt64 `json:"status_5xx_count"`
	Status400Count ZeroNullInt64 `json:"status_400_count"`
	Status429Count ZeroNullInt64 `json:"status_429_count"`
	Status500Count ZeroNullInt64 `json:"status_500_count"`
}

func (c *Count) AddRequest(status int, isRetry bool) {
	c.RequestCount++

	if status != http.StatusOK {
		c.ExceptionCount++
	}

	if isRetry {
		c.RetryCount++
	}

	if status >= 400 && status < 500 {
		c.Status4xxCount++
	}

	if status >= 500 && status < 600 {
		c.Status5xxCount++
	}

	if status == http.StatusBadRequest {
		c.Status400Count++
	}

	if status == http.StatusTooManyRequests {
		c.Status429Count++
	}

	if status == http.StatusInternalServerError {
		c.Status500Count++
	}
}

func (c *Count) Add(other Count) {
	c.RequestCount += other.RequestCount
	c.RetryCount += other.RetryCount
	c.ExceptionCount += other.ExceptionCount
	c.Status4xxCount += other.Status4xxCount
	c.Status5xxCount += other.Status5xxCount
	c.Status400Count += other.Status400Count
	c.Status429Count += other.Status429Count
	c.Status500Count += other.Status500Count
}

type SummaryData struct {
	Count
	Usage

	UsedAmount float64 `json:"used_amount"`

	TotalTimeMilliseconds int64 `json:"total_time_milliseconds,omitempty"`
	TotalTTFBMilliseconds int64 `json:"total_ttfb_milliseconds,omitempty"`
}

func (d *SummaryData) buildUpdateData(tableName string) map[string]any {
	data := map[string]any{}

	if d.UsedAmount > 0 {
		data["used_amount"] = gorm.Expr(tableName+".used_amount + ?", d.UsedAmount)
	}

	if d.RequestCount > 0 {
		data["request_count"] = gorm.Expr(tableName+".request_count + ?", d.RequestCount)
	}

	if d.RetryCount > 0 {
		data["retry_count"] = gorm.Expr(tableName+".retry_count + ?", d.RetryCount)
	}

	if d.ExceptionCount > 0 {
		data["exception_count"] = gorm.Expr(
			tableName+".exception_count + ?",
			d.ExceptionCount,
		)
	}

	if d.Status4xxCount > 0 {
		data["status4xx_count"] = gorm.Expr(
			tableName+".status4xx_count + ?",
			d.Status4xxCount,
		)
	}

	if d.Status5xxCount > 0 {
		data["status5xx_count"] = gorm.Expr(
			tableName+".status5xx_count + ?",
			d.Status5xxCount,
		)
	}

	if d.Status400Count > 0 {
		data["status400_count"] = gorm.Expr(
			tableName+".status400_count + ?",
			d.Status400Count,
		)
	}

	if d.Status429Count > 0 {
		data["status429_count"] = gorm.Expr(
			tableName+".status429_count + ?",
			d.Status429Count,
		)
	}

	if d.Status500Count > 0 {
		data["status500_count"] = gorm.Expr(
			tableName+".status500_count + ?",
			d.Status500Count,
		)
	}

	if d.TotalTimeMilliseconds > 0 {
		data["total_time_milliseconds"] = gorm.Expr(
			tableName+".total_time_milliseconds + ?",
			d.TotalTimeMilliseconds,
		)
	}

	if d.TotalTTFBMilliseconds > 0 {
		data["total_ttfb_milliseconds"] = gorm.Expr(
			tableName+".total_ttfb_milliseconds + ?",
			d.TotalTTFBMilliseconds,
		)
	}

	// usage update
	if d.InputTokens > 0 {
		data["input_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.input_tokens, 0) + ?", tableName),
			d.InputTokens,
		)
	}

	if d.ImageInputTokens > 0 {
		data["image_input_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.image_input_tokens, 0) + ?", tableName),
			d.ImageInputTokens,
		)
	}

	if d.AudioInputTokens > 0 {
		data["audio_input_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.audio_input_tokens, 0) + ?", tableName),
			d.AudioInputTokens,
		)
	}

	if d.OutputTokens > 0 {
		data["output_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.output_tokens, 0) + ?", tableName),
			d.OutputTokens,
		)
	}

	if d.ImageOutputTokens > 0 {
		data["image_output_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.image_output_tokens, 0) + ?", tableName),
			d.ImageOutputTokens,
		)
	}

	if d.TotalTokens > 0 {
		data["total_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.total_tokens, 0) + ?", tableName),
			d.TotalTokens,
		)
	}

	if d.CachedTokens > 0 {
		data["cached_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.cached_tokens, 0) + ?", tableName),
			d.CachedTokens,
		)
	}

	if d.CacheCreationTokens > 0 {
		data["cache_creation_tokens"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.cache_creation_tokens, 0) + ?", tableName),
			d.CacheCreationTokens,
		)
	}

	if d.WebSearchCount > 0 {
		data["web_search_count"] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.web_search_count, 0) + ?", tableName),
			d.WebSearchCount,
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

	return err
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
	start, end time.Time,
	channelID int,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]ChartData, error) {
	query := LogDB.Model(&Summary{})

	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
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

	const selectFields = "hour_timestamp as timestamp, sum(used_amount) as used_amount, " +
		"sum(request_count) as request_count, sum(retry_count) as retry_count, sum(exception_count) as exception_count, sum(status4xx_count) as status4xx_count, sum(status5xx_count) as status5xx_count, sum(status400_count) as status400_count, sum(status429_count) as status429_count, sum(status500_count) as status500_count, " +
		"sum(total_time_milliseconds) as total_time_milliseconds, sum(total_ttfb_milliseconds) as total_ttfb_milliseconds, " +
		"sum(input_tokens) as input_tokens, sum(image_input_tokens) as image_input_tokens, sum(audio_input_tokens) as audio_input_tokens, sum(output_tokens) as output_tokens, sum(image_output_tokens) as image_output_tokens, " +
		"sum(cached_tokens) as cached_tokens, sum(cache_creation_tokens) as cache_creation_tokens, " +
		"sum(total_tokens) as total_tokens, sum(web_search_count) as web_search_count"

	query = query.
		Select(selectFields).
		Group("timestamp")

	var chartData []ChartData

	err := query.Find(&chartData).Error
	if err != nil {
		return nil, err
	}

	if len(chartData) > 0 && timeSpan != TimeSpanHour {
		chartData = aggregateDataToSpan(chartData, timeSpan, timezone)
	}

	slices.SortFunc(chartData, func(a, b ChartData) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return chartData, nil
}

func getGroupChartData(
	group string,
	start, end time.Time,
	tokenName, modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]ChartData, error) {
	query := LogDB.Model(&GroupSummary{})
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
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

	const selectFields = "hour_timestamp as timestamp, sum(used_amount) as used_amount, " +
		"sum(request_count) as request_count, sum(exception_count) as exception_count, sum(status4xx_count) as status4xx_count, sum(status5xx_count) as status5xx_count, sum(status400_count) as status400_count, sum(status429_count) as status429_count, sum(status500_count) as status500_count, " +
		"sum(total_time_milliseconds) as total_time_milliseconds, sum(total_ttfb_milliseconds) as total_ttfb_milliseconds, " +
		"sum(input_tokens) as input_tokens, sum(image_input_tokens) as image_input_tokens, sum(audio_input_tokens) as audio_input_tokens, sum(output_tokens) as output_tokens, sum(image_output_tokens) as image_output_tokens, " +
		"sum(cached_tokens) as cached_tokens, sum(cache_creation_tokens) as cache_creation_tokens, " +
		"sum(total_tokens) as total_tokens, sum(web_search_count) as web_search_count"

	query = query.
		Select(selectFields).
		Group("timestamp")

	var chartData []ChartData

	err := query.Find(&chartData).Error
	if err != nil {
		return nil, err
	}

	if len(chartData) > 0 && timeSpan != TimeSpanHour {
		chartData = aggregateDataToSpan(chartData, timeSpan, timezone)
	}

	slices.SortFunc(chartData, func(a, b ChartData) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return chartData, nil
}

func GetUsedChannels(start, end time.Time) ([]int, error) {
	return getLogGroupByValues[int]("channel_id", start, end)
}

func GetUsedModels(start, end time.Time) ([]string, error) {
	return getLogGroupByValues[string]("model", start, end)
}

func GetGroupUsedModels(group, tokenName string, start, end time.Time) ([]string, error) {
	return getGroupLogGroupByValues[string]("model", group, tokenName, start, end)
}

func GetGroupUsedTokenNames(group string, start, end time.Time) ([]string, error) {
	return getGroupLogGroupByValues[string]("token_name", group, "", start, end)
}

func getLogGroupByValues[T cmp.Ordered](
	field string,
	start, end time.Time,
) ([]T, error) {
	type Result struct {
		Value        T
		UsedAmount   float64
		RequestCount int64
	}

	var results []Result

	var query *gorm.DB

	query = LogDB.
		Model(&Summary{})

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
		Find(&results).Error
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

func getGroupLogGroupByValues[T cmp.Ordered](
	field, group, tokenName string,
	start, end time.Time,
) ([]T, error) {
	type Result struct {
		Value        T
		UsedAmount   float64
		RequestCount int64
	}

	var results []Result

	query := LogDB.
		Model(&GroupSummary{})
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
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
		Find(&results).Error
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

type ChartData struct {
	Timestamp  int64   `json:"timestamp"`
	UsedAmount float64 `json:"used_amount"`

	TotalTimeMilliseconds int64 `json:"total_time_milliseconds"`
	TotalTTFBMilliseconds int64 `json:"total_ttfb_milliseconds"`

	Count
	Usage
}

type DashboardResponse struct {
	ChartData             []ChartData `json:"chart_data"`
	TotalTimeMilliseconds int64       `json:"total_time_milliseconds"`
	TotalTTFBMilliseconds int64       `json:"total_ttfb_milliseconds"`

	RPM int64 `json:"rpm"`
	TPM int64 `json:"tpm"`

	MaxRPM int64 `json:"max_rpm"`
	MaxTPM int64 `json:"max_tpm"`

	UsedAmount float64 `json:"used_amount"`

	TotalCount int64 `json:"total_count"` // use Count.RequestCount instead
	Count
	Usage

	Channels []int    `json:"channels,omitempty"`
	Models   []string `json:"models,omitempty"`
}

type GroupDashboardResponse struct {
	DashboardResponse
	TokenNames []string `json:"token_names"`
}

type TimeSpanType string

const (
	TimeSpanMinute TimeSpanType = "minute"
	TimeSpanHour   TimeSpanType = "hour"
	TimeSpanDay    TimeSpanType = "day"
	TimeSpanMonth  TimeSpanType = "month"
)

func aggregateDataToSpan(
	data []ChartData,
	timeSpan TimeSpanType,
	timezone *time.Location,
) []ChartData {
	dataMap := make(map[int64]ChartData)

	if timezone == nil {
		timezone = time.Local
	}

	for _, data := range data {
		// Convert timestamp to time in the specified timezone
		t := time.Unix(data.Timestamp, 0).In(timezone)
		// Get the start of the day in the specified timezone
		var timestamp int64
		switch timeSpan {
		case TimeSpanMonth:
			startOfMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, timezone)
			timestamp = startOfMonth.Unix()
		case TimeSpanDay:
			startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timezone)
			timestamp = startOfDay.Unix()
		case TimeSpanHour:
			startOfHour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, timezone)
			timestamp = startOfHour.Unix()
		case TimeSpanMinute:
			startOfHour := time.Date(
				t.Year(),
				t.Month(),
				t.Day(),
				t.Hour(),
				t.Minute(),
				0,
				0,
				timezone,
			)
			timestamp = startOfHour.Unix()
		}

		currentData, exists := dataMap[timestamp]
		if !exists {
			currentData = ChartData{
				Timestamp: timestamp,
			}
		}

		currentData.Count.Add(data.Count)
		currentData.Usage.Add(data.Usage)

		currentData.TotalTimeMilliseconds += data.TotalTimeMilliseconds
		currentData.TotalTTFBMilliseconds += data.TotalTTFBMilliseconds
		currentData.UsedAmount = decimal.
			NewFromFloat(currentData.UsedAmount).
			Add(decimal.NewFromFloat(data.UsedAmount)).
			InexactFloat64()

		dataMap[timestamp] = currentData
	}

	result := make([]ChartData, 0, len(dataMap))
	for _, data := range dataMap {
		result = append(result, data)
	}

	return result
}

func sumDashboardResponse(chartData []ChartData) DashboardResponse {
	dashboardResponse := DashboardResponse{
		ChartData: chartData,
	}

	usedAmount := decimal.NewFromFloat(0)
	for _, data := range chartData {
		dashboardResponse.Count.Add(data.Count)
		dashboardResponse.TotalCount = dashboardResponse.RequestCount

		dashboardResponse.Usage.Add(data.Usage)

		dashboardResponse.TotalTimeMilliseconds += data.TotalTimeMilliseconds
		dashboardResponse.TotalTTFBMilliseconds += data.TotalTTFBMilliseconds
		usedAmount = usedAmount.Add(decimal.NewFromFloat(data.UsedAmount))
		dashboardResponse.UsedAmount = decimal.
			NewFromFloat(dashboardResponse.UsedAmount).
			Add(decimal.NewFromFloat(data.UsedAmount)).
			InexactFloat64()
	}

	dashboardResponse.UsedAmount = usedAmount.InexactFloat64()

	return dashboardResponse
}

func GetDashboardData(
	start,
	end time.Time,
	modelName string,
	channelID int,
	timeSpan TimeSpanType,
	timezone *time.Location,
) (*DashboardResponse, error) {
	if timeSpan == TimeSpanMinute {
		return getDashboardDataMinute(start, end, modelName, channelID, timeSpan, timezone)
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	var (
		chartData []ChartData
		channels  []int
		models    []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		chartData, err = getChartData(start, end, channelID, modelName, timeSpan, timezone)
		return err
	})

	g.Go(func() error {
		var err error

		channels, err = GetUsedChannels(start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetUsedModels(start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Channels = channels
	dashboardResponse.Models = models

	return &dashboardResponse, nil
}

func GetGroupDashboardData(
	group string,
	start, end time.Time,
	tokenName string,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
) (*GroupDashboardResponse, error) {
	if timeSpan == TimeSpanMinute {
		return getGroupDashboardDataMinute(
			group,
			start,
			end,
			tokenName,
			modelName,
			timeSpan,
			timezone,
		)
	}

	if group == "" {
		return nil, errors.New("group is required")
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	var (
		chartData  []ChartData
		tokenNames []string
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		chartData, err = getGroupChartData(
			group,
			start,
			end,
			tokenName,
			modelName,
			timeSpan,
			timezone,
		)

		return err
	})

	g.Go(func() error {
		var err error

		tokenNames, err = GetGroupUsedTokenNames(group, start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupUsedModels(group, tokenName, start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Models = models

	return &GroupDashboardResponse{
		DashboardResponse: dashboardResponse,
		TokenNames:        tokenNames,
	}, nil
}
