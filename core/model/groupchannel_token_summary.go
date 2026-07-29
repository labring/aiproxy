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

type GroupChannelTokenSummary struct {
	ID     int                            `gorm:"primaryKey"`
	Unique GroupChannelTokenSummaryUnique `gorm:"embedded"`
	Data   SummaryData                    `gorm:"embedded"`
}

type GroupChannelTokenSummaryUnique struct {
	GroupID       string `gorm:"size:64;not null;uniqueIndex:idx_group_channel_token_summary_unique,priority:1"`
	TokenName     string `gorm:"size:32;not null;uniqueIndex:idx_group_channel_token_summary_unique,priority:2"`
	Model         string `gorm:"size:128;not null;uniqueIndex:idx_group_channel_token_summary_unique,priority:3"`
	HourTimestamp int64  `gorm:"not null;uniqueIndex:idx_group_channel_token_summary_unique,priority:4,sort:desc"`
}

func (l *GroupChannelTokenSummary) BeforeCreate(_ *gorm.DB) error {
	if l.Unique.GroupID == "" {
		return errors.New("group id is required")
	}

	if l.Unique.Model == "" {
		return errors.New("model is required")
	}

	if l.Unique.HourTimestamp == 0 {
		return errors.New("hour timestamp is required")
	}

	return validateHourTimestamp(l.Unique.HourTimestamp)
}

func CreateGroupChannelTokenSummaryIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_group_channel_token_summary_group_hour ON group_channel_token_summaries (group_id, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_token_summary_group_token_hour ON group_channel_token_summaries (group_id, token_name, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_token_summary_group_model_hour ON group_channel_token_summaries (group_id, model, hour_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertGroupChannelTokenSummary(
	unique GroupChannelTokenSummaryUnique,
	data SummaryData,
) error {
	if err := validateHourTimestamp(unique.HourTimestamp); err != nil {
		return err
	}

	var err error
	for range 3 {
		result := LogDB.
			Model(&GroupChannelTokenSummary{}).
			Where(
				"group_id = ? AND token_name = ? AND model = ? AND hour_timestamp = ?",
				unique.GroupID,
				unique.TokenName,
				unique.Model,
				unique.HourTimestamp,
			).
			Updates(data.buildUpdateData("group_channel_token_summaries"))

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected > 0 {
			return nil
		}

		err = createGroupChannelTokenSummary(unique, data)
		if err == nil {
			return nil
		}

		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createGroupChannelTokenSummary(
	unique GroupChannelTokenSummaryUnique,
	data SummaryData,
) error {
	return LogDB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "group_id"},
				{Name: "token_name"},
				{Name: "model"},
				{Name: "hour_timestamp"},
			},
			DoUpdates: clause.Assignments(data.buildUpdateData("group_channel_token_summaries")),
		}).
		Create(&GroupChannelTokenSummary{Unique: unique, Data: data}).Error
}

func getGroupChannelTokenChartData(
	group string,
	start, end time.Time,
	tokenName, modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]ChartData, error) {
	modelName = normalizeSummaryModelFilter(modelName)

	query := LogDB.Model(&GroupChannelTokenSummary{}).Where("group_id = ?", group)
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

func GetGroupChannelTokenUsedModels(
	group, tokenName string,
	start, end time.Time,
) ([]string, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return getGroupChannelTokenGroupByValues[string]("model", group, tokenName, start, end)
}

func GetGlobalGroupChannelTokenUsedModels(
	group, tokenName string,
	start, end time.Time,
) ([]string, error) {
	return getGroupChannelTokenGroupByValues[string]("model", group, tokenName, start, end)
}

func GetGroupChannelTokenUsedTokenNames(group string, start, end time.Time) ([]string, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return getGroupChannelTokenGroupByValues[string]("token_name", group, "", start, end)
}

func GetGlobalGroupChannelTokenUsedTokenNames(
	group string,
	start, end time.Time,
) ([]string, error) {
	return getGroupChannelTokenGroupByValues[string]("token_name", group, "", start, end)
}

func getGroupChannelTokenGroupByValues[T cmp.Ordered](
	field, group, tokenName string,
	start, end time.Time,
) ([]T, error) {
	type Result struct {
		Value        T
		UsedAmount   float64
		RequestCount int64
	}

	query := LogDB.Model(&GroupChannelTokenSummary{})
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

func GetGroupChannelTokenDashboardData(
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

	if timeSpan == TimeSpanMinute {
		return getGroupChannelTokenDashboardDataMinute(
			group,
			start,
			end,
			tokenName,
			modelName,
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
		chartData  []ChartData
		tokenNames []string
		models     []string
		currentRPM int64
		currentTPM int64
	)

	g := new(errgroup.Group)
	g.Go(func() error {
		var err error

		chartData, err = getGroupChannelTokenChartData(
			group,
			start,
			end,
			tokenName,
			modelName,
			timeSpan,
			timezone,
			fields,
		)

		return err
	})
	g.Go(func() error {
		var err error

		tokenNames, err = GetGroupChannelTokenUsedTokenNames(group, start, end)
		return err
	})
	g.Go(func() error {
		var err error

		models, err = GetGroupChannelTokenUsedModels(group, tokenName, start, end)
		return err
	})
	g.Go(func() error {
		currentRPM, currentTPM = getGroupChannelTokenCurrentRPM(group, tokenName, modelName)
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Models = models
	dashboardResponse.RPM = currentRPM
	dashboardResponse.TPM = currentTPM

	return &GroupDashboardResponse{
		DashboardResponse: dashboardResponse,
		TokenNames:        tokenNames,
	}, nil
}

func GetGlobalGroupChannelTokenDashboardData(
	group string,
	start, end time.Time,
	tokenName string,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*GroupDashboardResponse, error) {
	if timeSpan == TimeSpanMinute {
		return getGlobalGroupChannelTokenDashboardDataMinute(
			group,
			start,
			end,
			tokenName,
			modelName,
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
		chartData  []ChartData
		tokenNames []string
		models     []string
		currentRPM int64
		currentTPM int64
	)

	g := new(errgroup.Group)
	g.Go(func() error {
		var err error

		chartData, err = getGroupChannelTokenChartData(
			group,
			start,
			end,
			tokenName,
			modelName,
			timeSpan,
			timezone,
			fields,
		)

		return err
	})
	g.Go(func() error {
		var err error

		tokenNames, err = GetGlobalGroupChannelTokenUsedTokenNames(group, start, end)
		return err
	})
	g.Go(func() error {
		var err error

		models, err = GetGlobalGroupChannelTokenUsedModels(group, tokenName, start, end)
		return err
	})
	g.Go(func() error {
		currentRPM, currentTPM = getGroupChannelTokenCurrentRPM(group, tokenName, modelName)
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Models = models
	dashboardResponse.RPM = currentRPM
	dashboardResponse.TPM = currentTPM

	return &GroupDashboardResponse{
		DashboardResponse: dashboardResponse,
		TokenNames:        tokenNames,
	}, nil
}

func GetGroupChannelTokenTimeSeriesModelData(
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

	if timeSpan == TimeSpanMinute {
		return getGroupChannelTokenTimeSeriesModelDataMinute(
			group,
			tokenName,
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

	query := LogDB.Model(&GroupChannelTokenSummary{}).Where("group_id = ?", group)
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

	var rawData []SummaryDataV2

	err := query.
		Select(fields.BuildSelectFieldsV2("hour_timestamp", "group_id, token_name, model")).
		Group("timestamp, group_id, token_name, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	if len(rawData) > 0 {
		err = batchFillGroupChannelTokenMaxValues(rawData, group, tokenName, modelName, start, end)
		if err != nil {
			return nil, err
		}

		if timeSpan != TimeSpanHour {
			rawData = aggregatToSpanGroup(rawData, timeSpan, timezone)
		}
	}

	result := convertToTimeModelData(rawData)
	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func GetGlobalGroupChannelTokenTimeSeriesModelData(
	group string,
	tokenName string,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]TimeSummaryDataV2, error) {
	modelName = normalizeSummaryModelFilter(modelName)

	if timeSpan == TimeSpanMinute {
		return getGlobalGroupChannelTokenTimeSeriesModelDataMinute(
			group,
			tokenName,
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

	query := LogDB.Model(&GroupChannelTokenSummary{})
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

	var rawData []SummaryDataV2

	err := query.
		Select(fields.BuildSelectFieldsV2("hour_timestamp", "group_id, token_name, model")).
		Group("timestamp, group_id, token_name, model").
		Find(&rawData).Error
	if err != nil {
		return nil, err
	}

	if len(rawData) > 0 {
		err = batchFillGlobalGroupChannelTokenMaxValues(
			rawData,
			group,
			tokenName,
			modelName,
			start,
			end,
		)
		if err != nil {
			return nil, err
		}

		if timeSpan != TimeSpanHour {
			rawData = aggregatToSpanGroup(rawData, timeSpan, timezone)
		}
	}

	result := convertToTimeModelData(rawData)
	slices.SortFunc(result, func(a, b TimeSummaryDataV2) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result, nil
}

func GetGroupChannelTokenDashboardV2Data(
	group string,
	tokenName string,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*GroupDashboardV2Response, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	var (
		timeSeries []TimeSummaryDataV2
		currentRPM int64
		currentTPM int64
		models     []string
		tokenNames []string
	)

	g := new(errgroup.Group)
	g.Go(func() error {
		var err error

		timeSeries, err = GetGroupChannelTokenTimeSeriesModelData(
			group,
			tokenName,
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
		currentRPM, currentTPM = getGroupChannelTokenCurrentRPM(group, tokenName, modelName)
		return nil
	})
	g.Go(func() error {
		var err error

		models, err = GetGroupChannelTokenUsedModels(group, tokenName, start, end)
		return err
	})
	g.Go(func() error {
		var err error

		tokenNames, err = GetGroupChannelTokenUsedTokenNames(group, start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &GroupDashboardV2Response{
		DashboardV2Response: DashboardV2Response{
			TimeSeries: timeSeries,
			RPM:        currentRPM,
			TPM:        currentTPM,
			Models:     models,
		},
		TokenNames: tokenNames,
	}, nil
}

func GetGlobalGroupChannelTokenDashboardV2Data(
	group string,
	tokenName string,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*GroupDashboardV2Response, error) {
	var (
		timeSeries []TimeSummaryDataV2
		currentRPM int64
		currentTPM int64
		models     []string
		tokenNames []string
	)

	g := new(errgroup.Group)
	g.Go(func() error {
		var err error

		timeSeries, err = GetGlobalGroupChannelTokenTimeSeriesModelData(
			group,
			tokenName,
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
		currentRPM, currentTPM = getGroupChannelTokenCurrentRPM(group, tokenName, modelName)
		return nil
	})
	g.Go(func() error {
		var err error

		models, err = GetGlobalGroupChannelTokenUsedModels(group, tokenName, start, end)
		return err
	})
	g.Go(func() error {
		var err error

		tokenNames, err = GetGlobalGroupChannelTokenUsedTokenNames(group, start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &GroupDashboardV2Response{
		DashboardV2Response: DashboardV2Response{
			TimeSeries: timeSeries,
			RPM:        currentRPM,
			TPM:        currentTPM,
			Models:     models,
		},
		TokenNames: tokenNames,
	}, nil
}

func GetGroupChannelTokenDashboardV3Data(
	group string,
	tokenName string,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*GroupDashboardV3Response, error) {
	return GetGroupChannelTokenDashboardV2Data(
		group,
		tokenName,
		modelName,
		start,
		end,
		timeSpan,
		timezone,
		fields,
	)
}

func GetGlobalGroupChannelTokenDashboardV3Data(
	group string,
	tokenName string,
	modelName string,
	start, end time.Time,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*GroupDashboardV3Response, error) {
	return GetGlobalGroupChannelTokenDashboardV2Data(
		group,
		tokenName,
		modelName,
		start,
		end,
		timeSpan,
		timezone,
		fields,
	)
}

func batchFillGroupChannelTokenMaxValues(
	rawData []SummaryDataV2,
	group, tokenName, modelName string,
	start, end time.Time,
) error {
	modelName = normalizeSummaryModelFilter(modelName)

	minuteQuery := LogDB.Model(&GroupChannelTokenSummaryMinute{}).Where("group_id = ?", group)
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

func batchFillGlobalGroupChannelTokenMaxValues(
	rawData []SummaryDataV2,
	group, tokenName, modelName string,
	start, end time.Time,
) error {
	modelName = normalizeSummaryModelFilter(modelName)

	minuteQuery := LogDB.Model(&GroupChannelTokenSummaryMinute{})
	if group != "" {
		minuteQuery = minuteQuery.Where("group_id = ?", group)
	}

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
