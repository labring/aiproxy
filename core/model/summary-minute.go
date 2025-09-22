package model

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SummaryMinute struct {
	ID     int                 `gorm:"primaryKey"`
	Unique SummaryMinuteUnique `gorm:"embedded"`
	Data   SummaryData         `gorm:"embedded"`
}

type SummaryMinuteUnique struct {
	ChannelID       int    `gorm:"not null;uniqueIndex:idx_summary_minute_unique,priority:1"`
	Model           string `gorm:"size:64;not null;uniqueIndex:idx_summary_minute_unique,priority:2"`
	MinuteTimestamp int64  `gorm:"not null;uniqueIndex:idx_summary_minute_unique,priority:3,sort:desc"`
}

func (l *SummaryMinute) BeforeCreate(_ *gorm.DB) (err error) {
	if l.Unique.ChannelID == 0 {
		return errors.New("channel id is required")
	}

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

var minuteTimestampDivisor = int64(time.Minute.Seconds())

func validateMinuteTimestamp(minuteTimestamp int64) error {
	if minuteTimestamp%minuteTimestampDivisor != 0 {
		return errors.New("minute timestamp must be an exact minute")
	}
	return nil
}

func CreateSummaryMinuteIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_summary_minute_channel_minute ON summary_minutes (channel_id, minute_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_summary_minute_model_minute ON summary_minutes (model, minute_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertSummaryMinute(unique SummaryMinuteUnique, data SummaryData) error {
	err := validateMinuteTimestamp(unique.MinuteTimestamp)
	if err != nil {
		return err
	}

	for range 3 {
		result := LogDB.
			Model(&SummaryMinute{}).
			Where(
				"channel_id = ? AND model = ? AND minute_timestamp = ?",
				unique.ChannelID,
				unique.Model,
				unique.MinuteTimestamp,
			).
			Updates(data.buildUpdateData("summary_minutes"))

		err = result.Error
		if err != nil {
			return err
		}

		if result.RowsAffected > 0 {
			return nil
		}

		err = createSummaryMinute(unique, data)
		if err == nil {
			return nil
		}

		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createSummaryMinute(unique SummaryMinuteUnique, data SummaryData) error {
	return LogDB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "channel_id"},
				{Name: "model"},
				{Name: "minute_timestamp"},
			},
			DoUpdates: clause.Assignments(data.buildUpdateData("summary_minutes")),
		}).
		Create(&SummaryMinute{
			Unique: unique,
			Data:   data,
		}).Error
}

func getChartDataMinute(
	start, end time.Time,
	channelID int,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]ChartData, error) {
	query := LogDB.Model(&SummaryMinute{})

	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
	}

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("minute_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("minute_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("minute_timestamp <= ?", end.Unix())
	}

	// Only include max metrics when we have specific channel and model
	const selectFields = "minute_timestamp as timestamp, sum(used_amount) as used_amount, " +
		"sum(request_count) as request_count, sum(retry_count) as retry_count, sum(exception_count) as exception_count, sum(status4xx_count) as status4xx_count, sum(status5xx_count) as status5xx_count, sum(status400_count) as status400_count, sum(status429_count) as status429_count, sum(status500_count) as status500_count, " +
		"sum(total_time_milliseconds) as total_time_milliseconds, sum(total_ttfb_milliseconds) as total_ttfb_milliseconds, " +
		"sum(input_tokens) as input_tokens, sum(image_input_tokens) as image_input_tokens, sum(audio_input_tokens) as audio_input_tokens, sum(output_tokens) as output_tokens, " +
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

	if len(chartData) > 0 && timeSpan != TimeSpanMinute {
		chartData = aggregateDataToSpan(chartData, timeSpan, timezone)
	}

	slices.SortFunc(chartData, func(a, b ChartData) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return chartData, nil
}

func getGroupChartDataMinute(
	group string,
	start, end time.Time,
	tokenName, modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]ChartData, error) {
	query := LogDB.Model(&GroupSummaryMinute{})
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
		query = query.Where("minute_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("minute_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("minute_timestamp <= ?", end.Unix())
	}

	// Only include max metrics when we have specific channel and model
	const selectFields = "minute_timestamp as timestamp, sum(used_amount) as used_amount, " +
		"sum(request_count) as request_count, sum(exception_count) as exception_count, sum(status4xx_count) as status4xx_count, sum(status5xx_count) as status5xx_count, sum(status400_count) as status400_count, sum(status429_count) as status429_count, sum(status500_count) as status500_count, " +
		"sum(total_time_milliseconds) as total_time_milliseconds, sum(total_ttfb_milliseconds) as total_ttfb_milliseconds, " +
		"sum(input_tokens) as input_tokens, sum(image_input_tokens) as image_input_tokens, sum(audio_input_tokens) as audio_input_tokens, sum(output_tokens) as output_tokens, " +
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

	if len(chartData) > 0 && timeSpan != TimeSpanMinute {
		chartData = aggregateDataToSpan(chartData, timeSpan, timezone)
	}

	slices.SortFunc(chartData, func(a, b ChartData) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return chartData, nil
}

func GetUsedChannelsMinute(start, end time.Time) ([]int, error) {
	return getLogGroupByValuesMinute[int]("channel_id", start, end)
}

func GetUsedModelsMinute(start, end time.Time) ([]string, error) {
	return getLogGroupByValuesMinute[string]("model", start, end)
}

func GetGroupUsedModelsMinute(group, tokenName string, start, end time.Time) ([]string, error) {
	return getGroupLogGroupByValuesMinute[string]("model", group, tokenName, start, end)
}

func GetGroupUsedTokenNamesMinute(group string, start, end time.Time) ([]string, error) {
	return getGroupLogGroupByValuesMinute[string]("token_name", group, "", start, end)
}

func getLogGroupByValuesMinute[T cmp.Ordered](
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
		Model(&SummaryMinute{})

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("minute_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("minute_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("minute_timestamp <= ?", end.Unix())
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

func getGroupLogGroupByValuesMinute[T cmp.Ordered](
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
		Model(&GroupSummaryMinute{})
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("minute_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("minute_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("minute_timestamp <= ?", end.Unix())
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

func getDashboardDataMinute(
	start,
	end time.Time,
	modelName string,
	channelID int,
	timeSpan TimeSpanType,
	timezone *time.Location,
) (*DashboardResponse, error) {
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

		chartData, err = getChartDataMinute(start, end, channelID, modelName, timeSpan, timezone)
		return err
	})

	g.Go(func() error {
		var err error

		channels, err = GetUsedChannelsMinute(start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetUsedModelsMinute(start, end)
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

func getGroupDashboardDataMinute(
	group string,
	start, end time.Time,
	tokenName string,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
) (*GroupDashboardResponse, error) {
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

		chartData, err = getGroupChartDataMinute(
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

		tokenNames, err = GetGroupUsedTokenNamesMinute(group, start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupUsedModelsMinute(group, tokenName, start, end)
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

type SummaryDataV2 struct {
	Timestamp  int64   `json:"timestamp,omitempty"`
	ChannelID  int     `json:"channel_id,omitempty"`
	GroupID    string  `json:"group_id,omitempty"`
	TokenName  string  `json:"token_name,omitempty"`
	Model      string  `json:"model"`
	UsedAmount float64 `json:"used_amount"`

	TotalTimeMilliseconds int64 `json:"total_time_milliseconds"`
	TotalTTFBMilliseconds int64 `json:"total_ttfb_milliseconds"`

	Count
	Usage

	MaxRPM int64 `json:"max_rpm"`
	MaxTPM int64 `json:"max_tpm"`
}

type TimeSummaryDataV2 struct {
	Timestamp int64           `json:"timestamp"`
	Summary   []SummaryDataV2 `json:"summary"`
}

func GetTimeSeriesModelData(
	channelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]TimeSummaryDataV2, error) {
	if timeSpan == TimeSpanMinute {
		return getTimeSeriesModelDataMinute(channelID, modelName, start, end, timeSpan, timezone)
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

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

	const selectFields = "hour_timestamp as timestamp, channel_id, model, " +
		"sum(used_amount) as used_amount, " +
		"sum(request_count) as request_count, sum(retry_count) as retry_count, sum(exception_count) as exception_count, sum(status4xx_count) as status4xx_count, sum(status5xx_count) as status5xx_count, sum(status400_count) as status400_count, sum(status429_count) as status429_count, sum(status500_count) as status500_count, " +
		"sum(total_time_milliseconds) as total_time_milliseconds, sum(total_ttfb_milliseconds) as total_ttfb_milliseconds, " +
		"sum(input_tokens) as input_tokens, sum(image_input_tokens) as image_input_tokens, sum(audio_input_tokens) as audio_input_tokens, " +
		"sum(output_tokens) as output_tokens, sum(cached_tokens) as cached_tokens, " +
		"sum(cache_creation_tokens) as cache_creation_tokens, sum(total_tokens) as total_tokens, " +
		"sum(web_search_count) as web_search_count"

	var rawData []SummaryDataV2

	err := query.
		Select(selectFields).
		Group("timestamp, channel_id, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	if len(rawData) > 0 {
		err = batchFillMaxValues(rawData, channelID, modelName, start, end)
		if err != nil {
			return nil, err
		}

		if timeSpan != TimeSpanHour {
			rawData = aggregatToSpan(rawData, timeSpan, timezone)
		}
	}

	result := convertToTimeModelData(rawData)

	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func GetGroupTimeSeriesModelData(
	group string,
	tokenName string,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]TimeSummaryDataV2, error) {
	if timeSpan == TimeSpanMinute {
		return getGroupTimeSeriesModelDataMinute(
			group,
			tokenName,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
		)
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	query := LogDB.Model(&GroupSummary{}).
		Where("group_id = ?", group)
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

	const selectFields = "hour_timestamp as timestamp, group_id, token_name, model, " +
		"sum(used_amount) as used_amount, " +
		"sum(request_count) as request_count, sum(exception_count) as exception_count, sum(status4xx_count) as status4xx_count, sum(status5xx_count) as status5xx_count, sum(status400_count) as status400_count, sum(status429_count) as status429_count, sum(status500_count) as status500_count, " +
		"sum(total_time_milliseconds) as total_time_milliseconds, sum(total_ttfb_milliseconds) as total_ttfb_milliseconds, " +
		"sum(input_tokens) as input_tokens, sum(image_input_tokens) as image_input_tokens, sum(audio_input_tokens) as audio_input_tokens, " +
		"sum(output_tokens) as output_tokens, sum(cached_tokens) as cached_tokens, " +
		"sum(cache_creation_tokens) as cache_creation_tokens, sum(total_tokens) as total_tokens, " +
		"sum(web_search_count) as web_search_count"

	var rawData []SummaryDataV2

	err := query.
		Select(selectFields).
		Group("timestamp, group_id, token_name, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	if len(rawData) > 0 {
		err = batchFillGroupMaxValues(rawData, group, tokenName, modelName, start, end)
		if err != nil {
			return nil, err
		}

		if timeSpan != TimeSpanHour {
			rawData = aggregatToSpan(rawData, timeSpan, timezone)
		}
	}

	result := convertToTimeModelData(rawData)

	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func batchFillMaxValues(
	rawData []SummaryDataV2,
	channelID int,
	modelName string,
	start, end time.Time,
) error {
	minuteQuery := LogDB.Model(&SummaryMinute{})

	if channelID != 0 {
		minuteQuery = minuteQuery.Where("channel_id = ?", channelID)
	}

	if modelName != "" {
		minuteQuery = minuteQuery.Where("model = ?", modelName)
	}

	minuteStart := start.Unix()

	minuteEnd := end.Unix()
	if end.IsZero() {
		minuteEnd = time.Now().Unix()
	}

	minuteQuery = minuteQuery.Where(
		"minute_timestamp >= ? AND minute_timestamp <= ?",
		minuteStart,
		minuteEnd,
	)

	type MaxResult struct {
		HourTimestamp int64  `json:"hour_timestamp"`
		ChannelID     int    `json:"channel_id"`
		Model         string `json:"model"`
		MaxRPM        int64  `json:"max_rpm"`
		MaxTPM        int64  `json:"max_tpm"`
	}

	var maxResults []MaxResult

	err := minuteQuery.
		Select(`
			(minute_timestamp - minute_timestamp % 3600) as hour_timestamp,
			channel_id,
			model,
			MAX(request_count) as max_rpm,
			MAX(total_tokens) as max_tpm
		`).
		Group("hour_timestamp, channel_id, model").
		Find(&maxResults).Error
	if err != nil {
		return err
	}

	type Key struct {
		HourTimestamp int64
		ChannelID     int
		Model         string
	}

	maxMap := make(map[Key]MaxResult)
	for _, result := range maxResults {
		key := Key{
			HourTimestamp: result.HourTimestamp,
			ChannelID:     result.ChannelID,
			Model:         result.Model,
		}
		maxMap[key] = result
	}

	for i := range rawData {
		data := &rawData[i]

		key := Key{
			HourTimestamp: data.Timestamp,
			ChannelID:     data.ChannelID,
			Model:         data.Model,
		}
		if maxResult, exists := maxMap[key]; exists {
			data.MaxRPM = maxResult.MaxRPM
			data.MaxTPM = maxResult.MaxTPM
		}
	}

	return nil
}

func batchFillGroupMaxValues(
	rawData []SummaryDataV2,
	group, tokenName, modelName string,
	start, end time.Time,
) error {
	minuteQuery := LogDB.Model(&GroupSummaryMinute{}).
		Where("group_id = ?", group)

	if tokenName != "" {
		minuteQuery = minuteQuery.Where("token_name = ?", tokenName)
	}

	if modelName != "" {
		minuteQuery = minuteQuery.Where("model = ?", modelName)
	}

	minuteStart := start.Unix()

	minuteEnd := end.Unix()
	if end.IsZero() {
		minuteEnd = time.Now().Unix()
	}

	minuteQuery = minuteQuery.Where(
		"minute_timestamp >= ? AND minute_timestamp <= ?",
		minuteStart,
		minuteEnd,
	)

	type MaxResult struct {
		HourTimestamp int64  `json:"hour_timestamp"`
		GroupID       string `json:"group_id"`
		TokenName     string `json:"token_name"`
		Model         string `json:"model"`
		MaxRPM        int64  `json:"max_rpm"`
		MaxTPM        int64  `json:"max_tpm"`
	}

	var maxResults []MaxResult

	err := minuteQuery.
		Select(`
			(minute_timestamp - minute_timestamp % 3600) as hour_timestamp,
			group_id,
			token_name,
			model,
			MAX(request_count) as max_rpm,
			MAX(total_tokens) as max_tpm
		`).
		Group("hour_timestamp, group_id, token_name, model").
		Find(&maxResults).Error
	if err != nil {
		return err
	}

	type Key struct {
		HourTimestamp int64
		GroupID       string
		TokenName     string
		Model         string
	}

	maxMap := make(map[Key]MaxResult)
	for _, result := range maxResults {
		key := Key{
			HourTimestamp: result.HourTimestamp,
			GroupID:       result.GroupID,
			TokenName:     result.TokenName,
			Model:         result.Model,
		}
		maxMap[key] = result
	}

	for i := range rawData {
		data := &rawData[i]

		key := Key{
			HourTimestamp: data.Timestamp,
			GroupID:       data.GroupID,
			TokenName:     data.TokenName,
			Model:         data.Model,
		}
		if maxResult, exists := maxMap[key]; exists {
			data.MaxRPM = maxResult.MaxRPM
			data.MaxTPM = maxResult.MaxTPM
		}
	}

	return nil
}

func getTimeSeriesModelDataMinute(
	channelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]TimeSummaryDataV2, error) {
	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	query := LogDB.Model(&SummaryMinute{})

	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
	}

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("minute_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("minute_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("minute_timestamp <= ?", end.Unix())
	}

	const selectFields = "minute_timestamp as timestamp, channel_id, model, " +
		"sum(used_amount) as used_amount, " +
		"sum(request_count) as request_count, sum(retry_count) as retry_count, sum(exception_count) as exception_count, sum(status4xx_count) as status4xx_count, sum(status5xx_count) as status5xx_count, sum(status400_count) as status400_count, sum(status429_count) as status429_count, sum(status500_count) as status500_count, " +
		"sum(total_time_milliseconds) as total_time_milliseconds, sum(total_ttfb_milliseconds) as total_ttfb_milliseconds, " +
		"sum(input_tokens) as input_tokens, sum(image_input_tokens) as image_input_tokens, sum(audio_input_tokens) as audio_input_tokens, " +
		"sum(output_tokens) as output_tokens, sum(cached_tokens) as cached_tokens, " +
		"sum(cache_creation_tokens) as cache_creation_tokens, sum(total_tokens) as total_tokens, " +
		"sum(web_search_count) as web_search_count"

	var rawData []SummaryDataV2

	err := query.
		Select(selectFields).
		Group("timestamp, channel_id, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	for i, data := range rawData {
		rawData[i].MaxRPM = data.RequestCount
		rawData[i].MaxTPM = int64(data.TotalTokens)
	}

	if len(rawData) > 0 && timeSpan != TimeSpanMinute {
		rawData = aggregatToSpan(rawData, timeSpan, timezone)
	}

	result := convertToTimeModelData(rawData)

	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func getGroupTimeSeriesModelDataMinute(
	group string,
	tokenName string,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
) ([]TimeSummaryDataV2, error) {
	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	query := LogDB.Model(&GroupSummaryMinute{}).
		Where("group_id = ?", group)
	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("minute_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("minute_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("minute_timestamp <= ?", end.Unix())
	}

	const selectFields = "minute_timestamp as timestamp, group_id, token_name, model, " +
		"sum(used_amount) as used_amount, " +
		"sum(request_count) as request_count, sum(exception_count) as exception_count, sum(status4xx_count) as status4xx_count, sum(status5xx_count) as status5xx_count, sum(status400_count) as status400_count, sum(status429_count) as status429_count, sum(status500_count) as status500_count, " +
		"sum(total_time_milliseconds) as total_time_milliseconds, sum(total_ttfb_milliseconds) as total_ttfb_milliseconds, " +
		"sum(input_tokens) as input_tokens, sum(image_input_tokens) as image_input_tokens, sum(audio_input_tokens) as audio_input_tokens, " +
		"sum(output_tokens) as output_tokens, sum(cached_tokens) as cached_tokens, " +
		"sum(cache_creation_tokens) as cache_creation_tokens, sum(total_tokens) as total_tokens, " +
		"sum(web_search_count) as web_search_count"

	var rawData []SummaryDataV2

	err := query.
		Select(selectFields).
		Group("timestamp, group_id, token_name, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	for i, data := range rawData {
		rawData[i].MaxRPM = data.RequestCount
		rawData[i].MaxTPM = int64(data.TotalTokens)
	}

	if len(rawData) > 0 && timeSpan != TimeSpanMinute {
		rawData = aggregatToSpanGroup(rawData, timeSpan, timezone)
	}

	result := convertToTimeModelData(rawData)

	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func aggregatToSpan(
	minuteData []SummaryDataV2,
	timeSpan TimeSpanType,
	timezone *time.Location,
) []SummaryDataV2 {
	if timezone == nil {
		timezone = time.Local
	}

	type AggKey struct {
		Timestamp int64
		ChannelID int
		Model     string
	}

	dataMap := make(map[AggKey]SummaryDataV2)

	for _, data := range minuteData {
		t := time.Unix(data.Timestamp, 0).In(timezone)

		key := AggKey{
			ChannelID: data.ChannelID,
			Model:     data.Model,
		}

		switch timeSpan {
		case TimeSpanMonth:
			startOfMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, timezone)
			key.Timestamp = startOfMonth.Unix()
		case TimeSpanDay:
			startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timezone)
			key.Timestamp = startOfDay.Unix()
		case TimeSpanHour:
			startOfHour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, timezone)
			key.Timestamp = startOfHour.Unix()
		case TimeSpanMinute:
			fallthrough
		default:
			startOfMinute := time.Date(
				t.Year(),
				t.Month(),
				t.Day(),
				t.Hour(),
				t.Minute(),
				0,
				0,
				timezone,
			)
			key.Timestamp = startOfMinute.Unix()
		}

		currentData, exists := dataMap[key]
		if !exists {
			currentData = SummaryDataV2{
				Timestamp: key.Timestamp,
				ChannelID: data.ChannelID,
				Model:     data.Model,
			}
		}

		currentData.Count.Add(data.Count)
		currentData.Usage.Add(data.Usage)

		currentData.UsedAmount = decimal.
			NewFromFloat(currentData.UsedAmount).
			Add(decimal.NewFromFloat(data.UsedAmount)).
			InexactFloat64()
		currentData.TotalTimeMilliseconds += data.TotalTimeMilliseconds
		currentData.TotalTTFBMilliseconds += data.TotalTTFBMilliseconds

		if data.MaxRPM > currentData.MaxRPM {
			currentData.MaxRPM = data.MaxRPM
		}

		if data.MaxTPM > currentData.MaxTPM {
			currentData.MaxTPM = data.MaxTPM
		}

		dataMap[key] = currentData
	}

	result := make([]SummaryDataV2, 0, len(dataMap))
	for _, data := range dataMap {
		result = append(result, data)
	}

	return result
}

func aggregatToSpanGroup(
	minuteData []SummaryDataV2,
	timeSpan TimeSpanType,
	timezone *time.Location,
) []SummaryDataV2 {
	if timezone == nil {
		timezone = time.Local
	}

	type AggKey struct {
		Timestamp int64
		GroupID   string
		TokenName string
		Model     string
	}

	dataMap := make(map[AggKey]SummaryDataV2)

	for _, data := range minuteData {
		t := time.Unix(data.Timestamp, 0).In(timezone)

		key := AggKey{
			GroupID:   data.GroupID,
			TokenName: data.TokenName,
			Model:     data.Model,
		}

		switch timeSpan {
		case TimeSpanMonth:
			startOfMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, timezone)
			key.Timestamp = startOfMonth.Unix()
		case TimeSpanDay:
			startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timezone)
			key.Timestamp = startOfDay.Unix()
		case TimeSpanHour:
			startOfHour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, timezone)
			key.Timestamp = startOfHour.Unix()
		case TimeSpanMinute:
			fallthrough
		default:
			startOfMinute := time.Date(
				t.Year(),
				t.Month(),
				t.Day(),
				t.Hour(),
				t.Minute(),
				0,
				0,
				timezone,
			)
			key.Timestamp = startOfMinute.Unix()
		}

		currentData, exists := dataMap[key]
		if !exists {
			currentData = SummaryDataV2{
				Timestamp: key.Timestamp,
				GroupID:   data.GroupID,
				TokenName: data.TokenName,
				Model:     data.Model,
			}
		}

		currentData.Count.Add(data.Count)
		currentData.Usage.Add(data.Usage)

		currentData.UsedAmount = decimal.
			NewFromFloat(currentData.UsedAmount).
			Add(decimal.NewFromFloat(data.UsedAmount)).
			InexactFloat64()
		currentData.TotalTimeMilliseconds += data.TotalTimeMilliseconds
		currentData.TotalTTFBMilliseconds += data.TotalTTFBMilliseconds

		if data.MaxRPM > currentData.MaxRPM {
			currentData.MaxRPM = data.MaxRPM
		}

		if data.MaxTPM > currentData.MaxTPM {
			currentData.MaxTPM = data.MaxTPM
		}

		dataMap[key] = currentData
	}

	result := make([]SummaryDataV2, 0, len(dataMap))
	for _, data := range dataMap {
		result = append(result, data)
	}

	return result
}

func convertToTimeModelData(rawData []SummaryDataV2) []TimeSummaryDataV2 {
	timeMap := make(map[int64][]SummaryDataV2)

	for _, data := range rawData {
		timeMap[data.Timestamp] = append(timeMap[data.Timestamp], data)
	}

	result := make([]TimeSummaryDataV2, 0, len(timeMap))
	for timestamp, models := range timeMap {
		slices.SortFunc(models, func(a, b SummaryDataV2) int {
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

		result = append(result, TimeSummaryDataV2{
			Timestamp: timestamp,
			Summary:   models,
		})
	}

	return result
}
