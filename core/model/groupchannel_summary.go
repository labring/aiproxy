package model

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupChannelSummary struct {
	ID     int                       `gorm:"primaryKey"`
	Unique GroupChannelSummaryUnique `gorm:"embedded"`
	Data   SummaryData               `gorm:"embedded"`
}

type GroupChannelSummaryUnique struct {
	GroupID        string `gorm:"size:64;not null;uniqueIndex:idx_group_channel_summary_unique,priority:1"`
	GroupChannelID int    `gorm:"not null;uniqueIndex:idx_group_channel_summary_unique,priority:2"`
	Model          string `gorm:"size:128;not null;uniqueIndex:idx_group_channel_summary_unique,priority:3"`
	HourTimestamp  int64  `gorm:"not null;uniqueIndex:idx_group_channel_summary_unique,priority:4,sort:desc"`
}

func (l *GroupChannelSummary) BeforeCreate(_ *gorm.DB) error {
	if l.Unique.GroupID == "" {
		return errors.New("group id is required")
	}

	if l.Unique.GroupChannelID == 0 {
		return errors.New("group channel id is required")
	}

	if l.Unique.Model == "" {
		return errors.New("model is required")
	}

	if l.Unique.HourTimestamp == 0 {
		return errors.New("hour timestamp is required")
	}

	return validateHourTimestamp(l.Unique.HourTimestamp)
}

func CreateGroupChannelSummaryIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_group_channel_summary_group_hour ON group_channel_summaries (group_id, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_summary_channel_hour ON group_channel_summaries (group_id, group_channel_id, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_summary_model_hour ON group_channel_summaries (group_id, model, hour_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertGroupChannelSummary(unique GroupChannelSummaryUnique, data SummaryData) error {
	if err := validateHourTimestamp(unique.HourTimestamp); err != nil {
		return err
	}

	var err error
	for range 3 {
		result := LogDB.
			Model(&GroupChannelSummary{}).
			Where(
				"group_id = ? AND group_channel_id = ? AND model = ? AND hour_timestamp = ?",
				unique.GroupID,
				unique.GroupChannelID,
				unique.Model,
				unique.HourTimestamp,
			).
			Updates(data.buildUpdateData("group_channel_summaries"))

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected > 0 {
			return nil
		}

		err = createGroupChannelSummary(unique, data)
		if err == nil {
			return nil
		}

		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createGroupChannelSummary(unique GroupChannelSummaryUnique, data SummaryData) error {
	return LogDB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "group_id"},
				{Name: "group_channel_id"},
				{Name: "model"},
				{Name: "hour_timestamp"},
			},
			DoUpdates: clause.Assignments(data.buildUpdateData("group_channel_summaries")),
		}).
		Create(&GroupChannelSummary{Unique: unique, Data: data}).Error
}

func GetGroupChannelLastRequestTimeMinute(group string, groupChannelID int) (time.Time, error) {
	if group == "" {
		return time.Time{}, errors.New("group id is required")
	}

	if groupChannelID == 0 {
		return time.Time{}, errors.New("group channel id is required")
	}

	var summary GroupChannelSummaryMinute

	err := LogDB.
		Model(&GroupChannelSummaryMinute{}).
		Where("group_id = ? AND group_channel_id = ?", group, groupChannelID).
		Order("minute_timestamp desc").
		First(&summary).Error
	if summary.Unique.MinuteTimestamp == 0 {
		return time.Time{}, nil
	}

	return time.Unix(summary.Unique.MinuteTimestamp, 0), err
}

type groupChannelLastRequestTimeRow struct {
	GroupChannelID int
	LastRequestAt  int64
}

func GetGroupChannelLastRequestTimesMinute(
	group string,
	groupChannelIDs []int,
) (map[int]time.Time, error) {
	if group == "" {
		return nil, errors.New("group id is required")
	}

	if len(groupChannelIDs) == 0 {
		return map[int]time.Time{}, nil
	}

	rows := make([]groupChannelLastRequestTimeRow, 0, len(groupChannelIDs))

	err := LogDB.
		Model(&GroupChannelSummaryMinute{}).
		Select("group_channel_id, MAX(minute_timestamp) AS last_request_at").
		Where("group_id = ? AND group_channel_id IN ?", group, groupChannelIDs).
		Group("group_channel_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[int]time.Time, len(rows))
	for _, row := range rows {
		if row.LastRequestAt == 0 {
			continue
		}

		result[row.GroupChannelID] = time.Unix(row.LastRequestAt, 0)
	}

	return result, nil
}

func getGroupChannelChartData(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]ChartData, error) {
	modelName = normalizeSummaryModelFilter(modelName)

	query := LogDB.Model(&GroupChannelSummary{}).Where("group_id = ?", group)
	if groupChannelID != 0 {
		query = query.Where("group_channel_id = ?", groupChannelID)
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

	var chartData []ChartData

	err := query.
		Select(fields.BuildSelectFields("hour_timestamp")).
		Group("timestamp").
		Find(&chartData).Error
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

func GetGroupChannelUsedChannels(
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]int, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return getGroupChannelGroupByValues[int]("group_channel_id", group, groupChannelID, start, end)
}

func GetGlobalGroupChannelUsedChannels(
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]int, error) {
	return getGroupChannelGroupByValues[int]("group_channel_id", group, groupChannelID, start, end)
}

func GetGroupChannelUsedModels(
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]string, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return getGroupChannelGroupByValues[string]("model", group, groupChannelID, start, end)
}

func GetGlobalGroupChannelUsedModels(
	group string,
	groupChannelID int,
	start, end time.Time,
) ([]string, error) {
	return getGroupChannelGroupByValues[string]("model", group, groupChannelID, start, end)
}

func getGroupChannelGroupByValues[T cmp.Ordered](
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

	query := LogDB.Model(&GroupChannelSummary{})
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if groupChannelID != 0 {
		query = query.Where("group_channel_id = ?", groupChannelID)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
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

func GetGroupChannelDashboardData(
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

	if timeSpan == TimeSpanMinute {
		return getGroupChannelDashboardDataMinute(
			group,
			groupChannelID,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
			fields,
		)
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

		chartData, err = getGroupChannelChartData(
			group,
			groupChannelID,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
			fields,
		)

		return err
	})

	g.Go(func() error {
		var err error

		channels, err = GetGroupChannelUsedChannels(group, groupChannelID, start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupChannelUsedModels(group, groupChannelID, start, end)
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

func GetGlobalGroupChannelDashboardData(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*DashboardResponse, error) {
	if timeSpan == TimeSpanMinute {
		return getGlobalGroupChannelDashboardDataMinute(
			group,
			groupChannelID,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
			fields,
		)
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

		chartData, err = getGroupChannelChartData(
			group,
			groupChannelID,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
			fields,
		)

		return err
	})

	g.Go(func() error {
		var err error

		channels, err = GetGlobalGroupChannelUsedChannels(group, groupChannelID, start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGlobalGroupChannelUsedModels(group, groupChannelID, start, end)
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

func GetGroupChannelTimeSeriesModelData(
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

	if timeSpan == TimeSpanMinute {
		return getGroupChannelTimeSeriesModelDataMinute(
			group,
			groupChannelID,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
			fields,
		)
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	query := LogDB.Model(&GroupChannelSummary{}).Where("group_id = ?", group)
	if groupChannelID != 0 {
		query = query.Where("group_channel_id = ?", groupChannelID)
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

	var rawData []SummaryDataV2

	err := query.
		Select(fields.BuildSelectFieldsV2("hour_timestamp", "group_id, group_channel_id, model")).
		Group("timestamp, group_id, group_channel_id, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	for i := range rawData {
		rawData[i].ChannelID = rawData[i].GroupChannelID
	}

	if len(rawData) > 0 {
		if err := batchFillGroupChannelMaxValues(
			rawData,
			group,
			groupChannelID,
			modelName,
			start,
			end,
		); err != nil {
			return nil, err
		}

		if timeSpan != TimeSpanHour {
			rawData = aggregateGroupChannelDataToSpan(rawData, timeSpan, timezone)
		}
	}

	result := convertToTimeModelData(rawData)
	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func GetGlobalGroupChannelTimeSeriesModelData(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]TimeSummaryDataV2, error) {
	modelName = normalizeSummaryModelFilter(modelName)

	if timeSpan == TimeSpanMinute {
		return getGlobalGroupChannelTimeSeriesModelDataMinute(
			group,
			groupChannelID,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
			fields,
		)
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	query := LogDB.Model(&GroupChannelSummary{})
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
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	var rawData []SummaryDataV2

	err := query.
		Select(fields.BuildSelectFieldsV2("hour_timestamp", "group_id, group_channel_id, model")).
		Group("timestamp, group_id, group_channel_id, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	for i := range rawData {
		rawData[i].ChannelID = rawData[i].GroupChannelID
	}

	if len(rawData) > 0 {
		if err := batchFillGlobalGroupChannelMaxValues(
			rawData,
			group,
			groupChannelID,
			modelName,
			start,
			end,
		); err != nil {
			return nil, err
		}

		if timeSpan != TimeSpanHour {
			rawData = aggregateGroupChannelDataToSpan(rawData, timeSpan, timezone)
		}
	}

	result := convertToTimeModelData(rawData)
	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func aggregateGroupChannelDataToSpan(
	data []SummaryDataV2,
	timeSpan TimeSpanType,
	timezone *time.Location,
) []SummaryDataV2 {
	if timezone == nil {
		timezone = time.Local
	}

	type AggKey struct {
		Timestamp      int64
		GroupID        string
		GroupChannelID int
		Model          string
	}

	dataMap := make(map[AggKey]SummaryDataV2)

	for _, item := range data {
		t := time.Unix(item.Timestamp, 0).In(timezone)

		key := AggKey{
			GroupID:        item.GroupID,
			GroupChannelID: item.GroupChannelID,
			Model:          item.Model,
		}

		switch timeSpan {
		case TimeSpanMonth:
			key.Timestamp = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, timezone).Unix()
		case TimeSpanDay:
			key.Timestamp = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timezone).Unix()
		case TimeSpanHour:
			key.Timestamp = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, timezone).
				Unix()
		case TimeSpanMinute:
			fallthrough
		default:
			key.Timestamp = time.Date(
				t.Year(),
				t.Month(),
				t.Day(),
				t.Hour(),
				t.Minute(),
				0,
				0,
				timezone,
			).Unix()
		}

		currentData, exists := dataMap[key]
		if !exists {
			currentData = SummaryDataV2{
				Timestamp:      key.Timestamp,
				ChannelID:      item.GroupChannelID,
				GroupChannelID: item.GroupChannelID,
				GroupID:        item.GroupID,
				Model:          item.Model,
			}
		}

		currentData.Add(item.SummaryDataSet)
		currentData.ServiceTierFlex.Add(item.ServiceTierFlex)
		currentData.ServiceTierPriority.Add(item.ServiceTierPriority)
		currentData.ClaudeLongContext.Add(item.ClaudeLongContext)

		if item.MaxRPM > currentData.MaxRPM {
			currentData.MaxRPM = item.MaxRPM
		}

		if item.MaxTPM > currentData.MaxTPM {
			currentData.MaxTPM = item.MaxTPM
		}

		dataMap[key] = currentData
	}

	result := make([]SummaryDataV2, 0, len(dataMap))
	for _, item := range dataMap {
		result = append(result, item)
	}

	return result
}

func GetGroupChannelDashboardV2Data(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*DashboardV2Response, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	var (
		timeSeries []TimeSummaryDataV2
		channels   []int
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		timeSeries, err = GetGroupChannelTimeSeriesModelData(
			group,
			groupChannelID,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
			fields,
		)

		return err
	})

	g.Go(func() error {
		var err error

		channels, err = GetGroupChannelUsedChannels(group, groupChannelID, start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupChannelUsedModels(group, groupChannelID, start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &DashboardV2Response{
		TimeSeries: timeSeries,
		Channels:   channels,
		Models:     models,
	}, nil
}

func GetGlobalGroupChannelDashboardV2Data(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*DashboardV2Response, error) {
	var (
		timeSeries []TimeSummaryDataV2
		channels   []int
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		timeSeries, err = GetGlobalGroupChannelTimeSeriesModelData(
			group,
			groupChannelID,
			modelName,
			start,
			end,
			timeSpan,
			timezone,
			fields,
		)

		return err
	})

	g.Go(func() error {
		var err error

		channels, err = GetGlobalGroupChannelUsedChannels(group, groupChannelID, start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGlobalGroupChannelUsedModels(group, groupChannelID, start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &DashboardV2Response{
		TimeSeries: timeSeries,
		Channels:   channels,
		Models:     models,
	}, nil
}

func GetGroupChannelDashboardV3Data(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*DashboardV3Response, error) {
	return GetGroupChannelDashboardV2Data(
		group,
		groupChannelID,
		modelName,
		start,
		end,
		timeSpan,
		timezone,
		fields,
	)
}

func GetGlobalGroupChannelDashboardV3Data(
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*DashboardV3Response, error) {
	return GetGlobalGroupChannelDashboardV2Data(
		group,
		groupChannelID,
		modelName,
		start,
		end,
		timeSpan,
		timezone,
		fields,
	)
}

func batchFillGroupChannelMaxValues(
	rawData []SummaryDataV2,
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
) error {
	modelName = normalizeSummaryModelFilter(modelName)

	minuteQuery := LogDB.Model(&GroupChannelSummaryMinute{}).Where("group_id = ?", group)
	if groupChannelID != 0 {
		minuteQuery = minuteQuery.Where("group_channel_id = ?", groupChannelID)
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
		HourTimestamp  int64  `json:"hour_timestamp"`
		GroupID        string `json:"group_id"`
		GroupChannelID int    `json:"group_channel_id"`
		Model          string `json:"model"`
		MaxRPM         int64  `json:"max_rpm"`
		MaxTPM         int64  `json:"max_tpm"`
	}

	var maxResults []MaxResult

	err := minuteQuery.
		Select(`
			(minute_timestamp - minute_timestamp % 3600) as hour_timestamp,
			group_id,
			group_channel_id,
			model,
			MAX(request_count) as max_rpm,
			MAX(total_tokens) as max_tpm
		`).
		Group("hour_timestamp, group_id, group_channel_id, model").
		Find(&maxResults).Error
	if err != nil {
		return err
	}

	type Key struct {
		HourTimestamp  int64
		GroupID        string
		GroupChannelID int
		Model          string
	}

	maxMap := make(map[Key]MaxResult)
	for _, result := range maxResults {
		key := Key{
			HourTimestamp:  result.HourTimestamp,
			GroupID:        result.GroupID,
			GroupChannelID: result.GroupChannelID,
			Model:          result.Model,
		}
		maxMap[key] = result
	}

	for i := range rawData {
		data := &rawData[i]

		groupChannelID := data.GroupChannelID
		if groupChannelID == 0 {
			groupChannelID = data.ChannelID
		}

		key := Key{
			HourTimestamp:  data.Timestamp,
			GroupID:        data.GroupID,
			GroupChannelID: groupChannelID,
			Model:          data.Model,
		}
		if maxResult, exists := maxMap[key]; exists {
			data.MaxRPM = maxResult.MaxRPM
			data.MaxTPM = maxResult.MaxTPM
		}
	}

	return nil
}

func batchFillGlobalGroupChannelMaxValues(
	rawData []SummaryDataV2,
	group string,
	groupChannelID int,
	modelName string,
	start, end time.Time,
) error {
	modelName = normalizeSummaryModelFilter(modelName)

	minuteQuery := LogDB.Model(&GroupChannelSummaryMinute{})
	if group != "" {
		minuteQuery = minuteQuery.Where("group_id = ?", group)
	}

	if groupChannelID != 0 {
		minuteQuery = minuteQuery.Where("group_channel_id = ?", groupChannelID)
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
		HourTimestamp  int64  `json:"hour_timestamp"`
		GroupID        string `json:"group_id"`
		GroupChannelID int    `json:"group_channel_id"`
		Model          string `json:"model"`
		MaxRPM         int64  `json:"max_rpm"`
		MaxTPM         int64  `json:"max_tpm"`
	}

	var maxResults []MaxResult

	err := minuteQuery.
		Select(`
			(minute_timestamp - minute_timestamp % 3600) as hour_timestamp,
			group_id,
			group_channel_id,
			model,
			MAX(request_count) as max_rpm,
			MAX(total_tokens) as max_tpm
		`).
		Group("hour_timestamp, group_id, group_channel_id, model").
		Find(&maxResults).Error
	if err != nil {
		return err
	}

	type Key struct {
		HourTimestamp  int64
		GroupID        string
		GroupChannelID int
		Model          string
	}

	maxMap := make(map[Key]MaxResult)
	for _, result := range maxResults {
		key := Key{
			HourTimestamp:  result.HourTimestamp,
			GroupID:        result.GroupID,
			GroupChannelID: result.GroupChannelID,
			Model:          result.Model,
		}
		maxMap[key] = result
	}

	for i := range rawData {
		data := &rawData[i]

		groupChannelID := data.GroupChannelID
		if groupChannelID == 0 {
			groupChannelID = data.ChannelID
		}

		key := Key{
			HourTimestamp:  data.Timestamp,
			GroupID:        data.GroupID,
			GroupChannelID: groupChannelID,
			Model:          data.Model,
		}
		if maxResult, exists := maxMap[key]; exists {
			data.MaxRPM = maxResult.MaxRPM
			data.MaxTPM = maxResult.MaxTPM
		}
	}

	return nil
}
