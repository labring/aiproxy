package model

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupChannelSummaryMinute struct {
	ID     int                             `gorm:"primaryKey"`
	Unique GroupChannelSummaryMinuteUnique `gorm:"embedded"`
	Data   SummaryData                     `gorm:"embedded"`
}

type GroupChannelSummaryMinuteUnique struct {
	GroupID         string `gorm:"size:64;not null;uniqueIndex:idx_group_channel_summary_minute_unique,priority:1"`
	GroupChannelID  int    `gorm:"not null;uniqueIndex:idx_group_channel_summary_minute_unique,priority:2"`
	Model           string `gorm:"size:128;not null;uniqueIndex:idx_group_channel_summary_minute_unique,priority:3"`
	MinuteTimestamp int64  `gorm:"not null;uniqueIndex:idx_group_channel_summary_minute_unique,priority:4,sort:desc"`
}

func (l *GroupChannelSummaryMinute) BeforeCreate(_ *gorm.DB) error {
	if l.Unique.GroupID == "" {
		return errors.New("group id is required")
	}

	if l.Unique.GroupChannelID == 0 {
		return errors.New("group channel id is required")
	}

	if l.Unique.Model == "" {
		return errors.New("model is required")
	}

	if l.Unique.MinuteTimestamp == 0 {
		return errors.New("minute timestamp is required")
	}

	return validateMinuteTimestamp(l.Unique.MinuteTimestamp)
}

func CreateGroupChannelSummaryMinuteIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_group_channel_summary_minute_group_minute ON group_channel_summary_minutes (group_id, minute_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_summary_minute_channel_minute ON group_channel_summary_minutes (group_id, group_channel_id, minute_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_summary_minute_model_minute ON group_channel_summary_minutes (group_id, model, minute_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertGroupChannelSummaryMinute(
	unique GroupChannelSummaryMinuteUnique,
	data SummaryData,
) error {
	if err := validateMinuteTimestamp(unique.MinuteTimestamp); err != nil {
		return err
	}

	var err error
	for range 3 {
		result := LogDB.
			Model(&GroupChannelSummaryMinute{}).
			Where(
				"group_id = ? AND group_channel_id = ? AND model = ? AND minute_timestamp = ?",
				unique.GroupID,
				unique.GroupChannelID,
				unique.Model,
				unique.MinuteTimestamp,
			).
			Updates(data.buildUpdateData("group_channel_summary_minutes"))
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected > 0 {
			return nil
		}

		err = createGroupChannelSummaryMinute(unique, data)
		if err == nil {
			return nil
		}

		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createGroupChannelSummaryMinute(
	unique GroupChannelSummaryMinuteUnique,
	data SummaryData,
) error {
	return LogDB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "group_id"},
				{Name: "group_channel_id"},
				{Name: "model"},
				{Name: "minute_timestamp"},
			},
			DoUpdates: clause.Assignments(data.buildUpdateData("group_channel_summary_minutes")),
		}).
		Create(&GroupChannelSummaryMinute{Unique: unique, Data: data}).Error
}

func getGroupChannelChartDataMinute(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]ChartData, error) {
	modelName = normalizeSummaryModelFilter(modelName)

	query := LogDB.Model(&GroupChannelSummaryMinute{}).Where("group_id = ?", group)
	if groupChannelID != 0 {
		query = query.Where("group_channel_id = ?", groupChannelID)
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

	var chartData []ChartData

	err := query.Select(fields.BuildSelectFields("minute_timestamp")).
		Group("timestamp").
		Find(&chartData).Error
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

func GetGroupChannelUsedModelsMinute(
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]string, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return getGroupChannelGroupByValuesMinute[string]("model", group, groupChannelID, start, end)
}

func GetGlobalGroupChannelUsedModelsMinute(
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]string, error) {
	return getGroupChannelGroupByValuesMinute[string]("model", group, groupChannelID, start, end)
}

func GetGroupChannelUsedChannelsMinute(
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]int, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return getGroupChannelGroupByValuesMinute[int](
		"group_channel_id",
		group,
		groupChannelID,
		start,
		end,
	)
}

func GetGlobalGroupChannelUsedChannelsMinute(
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]int, error) {
	return getGroupChannelGroupByValuesMinute[int](
		"group_channel_id",
		group,
		groupChannelID,
		start,
		end,
	)
}

func getGroupChannelGroupByValuesMinute[T cmp.Ordered](
	field string,
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]T, error) {
	type Result struct {
		Value        T
		UsedAmount   float64
		RequestCount int64
	}

	query := LogDB.Model(&GroupChannelSummaryMinute{})
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if groupChannelID != 0 {
		query = query.Where("group_channel_id = ?", groupChannelID)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("minute_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("minute_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("minute_timestamp <= ?", end.Unix())
	}

	var results []Result

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

func getGroupChannelDashboardDataMinute(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*DashboardResponse, error) {
	if group == "" {
		return nil, errors.New("group is required")
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

	chartData, err := getGroupChannelChartDataMinute(
		group,
		groupChannelID,
		modelName,
		start,
		end,
		timeSpan,
		timezone,
		fields,
	)
	if err != nil {
		return nil, err
	}

	channels, err = GetGroupChannelUsedChannelsMinute(group, groupChannelID, start, end)
	if err != nil {
		return nil, err
	}

	models, err = GetGroupChannelUsedModelsMinute(group, groupChannelID, start, end)
	if err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Channels = channels
	dashboardResponse.Models = models

	return &dashboardResponse, nil
}

func getGlobalGroupChannelDashboardDataMinute(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
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

	chartData, err := getGroupChannelChartDataMinute(
		group,
		groupChannelID,
		modelName,
		start,
		end,
		timeSpan,
		timezone,
		fields,
	)
	if err != nil {
		return nil, err
	}

	channels, err = GetGlobalGroupChannelUsedChannelsMinute(group, groupChannelID, start, end)
	if err != nil {
		return nil, err
	}

	models, err = GetGlobalGroupChannelUsedModelsMinute(group, groupChannelID, start, end)
	if err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Channels = channels
	dashboardResponse.Models = models

	return &dashboardResponse, nil
}

func getGroupChannelTimeSeriesModelDataMinute(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]TimeSummaryDataV2, error) {
	modelName = normalizeSummaryModelFilter(modelName)

	if group == "" {
		return nil, errors.New("group is required")
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	query := LogDB.Model(&GroupChannelSummaryMinute{}).Where("group_id = ?", group)
	if groupChannelID != 0 {
		query = query.Where("group_channel_id = ?", groupChannelID)
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

	var rawData []SummaryDataV2

	err := query.
		Select(fields.BuildSelectFieldsV2("minute_timestamp", "group_id, group_channel_id, model")).
		Group("timestamp, group_id, group_channel_id, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	for i, data := range rawData {
		rawData[i].ChannelID = data.GroupChannelID
		rawData[i].MaxRPM = data.RequestCount
		rawData[i].MaxTPM = int64(data.TotalTokens)
	}

	if len(rawData) > 0 && timeSpan != TimeSpanMinute {
		rawData = aggregateGroupChannelDataToSpan(rawData, timeSpan, timezone)
	}

	result := convertToTimeModelData(rawData)
	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func getGlobalGroupChannelTimeSeriesModelDataMinute(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]TimeSummaryDataV2, error) {
	modelName = normalizeSummaryModelFilter(modelName)

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	query := LogDB.Model(&GroupChannelSummaryMinute{})
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if groupChannelID != 0 {
		query = query.Where("group_channel_id = ?", groupChannelID)
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

	var rawData []SummaryDataV2

	err := query.
		Select(fields.BuildSelectFieldsV2("minute_timestamp", "group_id, group_channel_id, model")).
		Group("timestamp, group_id, group_channel_id, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	for i, data := range rawData {
		rawData[i].ChannelID = data.GroupChannelID
		rawData[i].MaxRPM = data.RequestCount
		rawData[i].MaxTPM = int64(data.TotalTokens)
	}

	if len(rawData) > 0 && timeSpan != TimeSpanMinute {
		rawData = aggregateGroupChannelDataToSpan(rawData, timeSpan, timezone)
	}

	result := convertToTimeModelData(rawData)
	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}
