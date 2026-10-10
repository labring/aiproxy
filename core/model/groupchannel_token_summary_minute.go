package model

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupChannelTokenSummaryMinute struct {
	ID     int                                  `gorm:"primaryKey"`
	Unique GroupChannelTokenSummaryMinuteUnique `gorm:"embedded"`
	Data   SummaryData                          `gorm:"embedded"`
}

type GroupChannelTokenSummaryMinuteUnique struct {
	GroupID         string `gorm:"size:64;not null;uniqueIndex:idx_group_channel_token_summary_minute_unique,priority:1"`
	TokenName       string `gorm:"size:32;not null;uniqueIndex:idx_group_channel_token_summary_minute_unique,priority:2"`
	Model           string `gorm:"size:128;not null;uniqueIndex:idx_group_channel_token_summary_minute_unique,priority:3"`
	MinuteTimestamp int64  `gorm:"not null;uniqueIndex:idx_group_channel_token_summary_minute_unique,priority:4,sort:desc"`
}

func (l *GroupChannelTokenSummaryMinute) BeforeCreate(_ *gorm.DB) error {
	if l.Unique.GroupID == "" {
		return errors.New("group id is required")
	}

	if l.Unique.Model == "" {
		return errors.New("model is required")
	}

	if l.Unique.MinuteTimestamp == 0 {
		return errors.New("minute timestamp is required")
	}

	return validateMinuteTimestamp(l.Unique.MinuteTimestamp)
}

func CreateGroupChannelTokenSummaryMinuteIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_group_channel_token_summary_minute_group_minute ON group_channel_token_summary_minutes (group_id, minute_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_token_summary_minute_group_token_minute ON group_channel_token_summary_minutes (group_id, token_name, minute_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_token_summary_minute_group_model_minute ON group_channel_token_summary_minutes (group_id, model, minute_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertGroupChannelTokenSummaryMinute(
	unique GroupChannelTokenSummaryMinuteUnique,
	data SummaryData,
) error {
	if err := validateMinuteTimestamp(unique.MinuteTimestamp); err != nil {
		return err
	}

	var err error
	for range 3 {
		result := LogDB.
			Model(&GroupChannelTokenSummaryMinute{}).
			Where(
				"group_id = ? AND token_name = ? AND model = ? AND minute_timestamp = ?",
				unique.GroupID,
				unique.TokenName,
				unique.Model,
				unique.MinuteTimestamp,
			).
			Updates(data.buildUpdateData("group_channel_token_summary_minutes"))
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected > 0 {
			return nil
		}

		err = createGroupChannelTokenSummaryMinute(unique, data)
		if err == nil {
			return nil
		}

		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createGroupChannelTokenSummaryMinute(
	unique GroupChannelTokenSummaryMinuteUnique,
	data SummaryData,
) error {
	return LogDB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "group_id"},
				{Name: "token_name"},
				{Name: "model"},
				{Name: "minute_timestamp"},
			},
			DoUpdates: clause.Assignments(
				data.buildUpdateData("group_channel_token_summary_minutes"),
			),
		}).
		Create(&GroupChannelTokenSummaryMinute{Unique: unique, Data: data}).Error
}

func getGroupChannelTokenChartDataMinute(
	group string,
	start, end time.Time,
	tokenName, modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]ChartData, error) {
	modelName = normalizeSummaryModelFilter(modelName)

	query := LogDB.Model(&GroupChannelTokenSummaryMinute{}).Where("group_id = ?", group)
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

	var chartData []ChartData

	err := query.
		Select(fields.BuildSelectFields("minute_timestamp")).
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

func GetGroupChannelTokenUsedModelsMinute(
	group, tokenName string,
	start, end time.Time,
) ([]string, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return getGroupChannelTokenGroupByValuesMinute[string]("model", group, tokenName, start, end)
}

func GetGlobalGroupChannelTokenUsedModelsMinute(
	group, tokenName string,
	start, end time.Time,
) ([]string, error) {
	return getGroupChannelTokenGroupByValuesMinute[string]("model", group, tokenName, start, end)
}

func GetGroupChannelTokenUsedTokenNamesMinute(
	group string,
	start, end time.Time,
) ([]string, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return getGroupChannelTokenGroupByValuesMinute[string]("token_name", group, "", start, end)
}

func GetGlobalGroupChannelTokenUsedTokenNamesMinute(
	group string,
	start, end time.Time,
) ([]string, error) {
	return getGroupChannelTokenGroupByValuesMinute[string]("token_name", group, "", start, end)
}

func getGroupChannelTokenGroupByValuesMinute[T cmp.Ordered](
	field, group, tokenName string,
	start, end time.Time,
) ([]T, error) {
	type Result struct {
		Value        T
		UsedAmount   float64
		RequestCount int64
	}

	query := LogDB.Model(&GroupChannelTokenSummaryMinute{})
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

func getGroupChannelTokenDashboardDataMinute(
	group string,
	start, end time.Time,
	tokenName string,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
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
		currentRPM int64
		currentTPM int64
	)

	chartData, err := getGroupChannelTokenChartDataMinute(
		group,
		start,
		end,
		tokenName,
		modelName,
		timeSpan,
		timezone,
		fields,
	)
	if err != nil {
		return nil, err
	}

	tokenNames, err = GetGroupChannelTokenUsedTokenNamesMinute(group, start, end)
	if err != nil {
		return nil, err
	}

	models, err = GetGroupChannelTokenUsedModelsMinute(group, tokenName, start, end)
	if err != nil {
		return nil, err
	}

	currentRPM, currentTPM = getGroupChannelTokenCurrentRPM(group, tokenName, modelName)

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Models = models
	dashboardResponse.RPM = currentRPM
	dashboardResponse.TPM = currentTPM

	return &GroupDashboardResponse{
		DashboardResponse: dashboardResponse,
		TokenNames:        tokenNames,
	}, nil
}

func getGlobalGroupChannelTokenDashboardDataMinute(
	group string,
	start, end time.Time,
	tokenName string,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*GroupDashboardResponse, error) {
	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	var (
		chartData  []ChartData
		tokenNames []string
		models     []string
		currentRPM int64
		currentTPM int64
	)

	chartData, err := getGroupChannelTokenChartDataMinute(
		group,
		start,
		end,
		tokenName,
		modelName,
		timeSpan,
		timezone,
		fields,
	)
	if err != nil {
		return nil, err
	}

	tokenNames, err = GetGlobalGroupChannelTokenUsedTokenNamesMinute(group, start, end)
	if err != nil {
		return nil, err
	}

	models, err = GetGlobalGroupChannelTokenUsedModelsMinute(group, tokenName, start, end)
	if err != nil {
		return nil, err
	}

	currentRPM, currentTPM = getGroupChannelTokenCurrentRPM(group, tokenName, modelName)

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Models = models
	dashboardResponse.RPM = currentRPM
	dashboardResponse.TPM = currentTPM

	return &GroupDashboardResponse{
		DashboardResponse: dashboardResponse,
		TokenNames:        tokenNames,
	}, nil
}

func getGroupChannelTokenTimeSeriesModelDataMinute(
	group string,
	tokenName string,
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

	query := LogDB.Model(&GroupChannelTokenSummaryMinute{}).Where("group_id = ?", group)
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

	var rawData []SummaryDataV2

	err := query.
		Select(fields.BuildSelectFieldsV2("minute_timestamp", "group_id, token_name, model")).
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

func getGlobalGroupChannelTokenTimeSeriesModelDataMinute(
	group string,
	tokenName string,
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

	query := LogDB.Model(&GroupChannelTokenSummaryMinute{})
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

	var rawData []SummaryDataV2

	err := query.
		Select(fields.BuildSelectFieldsV2("minute_timestamp", "group_id, token_name, model")).
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

func getGroupChannelTokenCurrentRPM(group, tokenName, modelName string) (int64, int64) {
	modelName = normalizeSummaryModelFilter(modelName)
	now := time.Now()
	recentStart := now.Add(-2 * time.Minute).Unix()
	recentEnd := now.Unix()

	query := LogDB.Model(&GroupChannelTokenSummaryMinute{}).
		Where("minute_timestamp >= ? AND minute_timestamp <= ?", recentStart, recentEnd)
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}

	type Result struct {
		RPM int64 `json:"rpm"`
		TPM int64 `json:"tpm"`
	}

	var result Result

	err := query.
		Select("SUM(request_count) as rpm, SUM(total_tokens) as tpm").
		Where("minute_timestamp = (?)",
			LogDB.Model(&GroupChannelTokenSummaryMinute{}).
				Select("MAX(minute_timestamp)").
				Where("minute_timestamp >= ? AND minute_timestamp <= ?", recentStart, recentEnd).
				Scopes(func(db *gorm.DB) *gorm.DB {
					if group != "" {
						db = db.Where("group_id = ?", group)
					}

					if tokenName != "" {
						db = db.Where("token_name = ?", tokenName)
					}

					if modelName != "" {
						db = db.Where("model = ?", modelName)
					}

					return db
				}),
		).
		Find(&result).Error
	if err != nil {
		return 0, 0
	}

	return result.RPM, result.TPM
}
