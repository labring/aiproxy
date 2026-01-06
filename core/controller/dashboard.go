package controller

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"gorm.io/gorm"
)

func getDashboardTime(
	t, timespan string,
	startTime, endTime time.Time,
	timezoneLocation *time.Location,
) (time.Time, time.Time, model.TimeSpanType) {
	end := time.Now()
	if !endTime.IsZero() {
		end = endTime
	}

	if timezoneLocation == nil {
		timezoneLocation = time.Local
	}

	var (
		start    time.Time
		timeSpan model.TimeSpanType
	)

	switch t {
	case "month":
		start = end.AddDate(0, 0, -30)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, timezoneLocation)
		timeSpan = model.TimeSpanDay
	case "two_week":
		start = end.AddDate(0, 0, -15)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, timezoneLocation)
		timeSpan = model.TimeSpanDay
	case "week":
		start = end.AddDate(0, 0, -7)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, timezoneLocation)
		timeSpan = model.TimeSpanDay
	case "day":
		start = end.AddDate(0, 0, -1)
		timeSpan = model.TimeSpanHour
	default:
		start = end.AddDate(0, 0, -7)
		timeSpan = model.TimeSpanHour
	}

	if !startTime.IsZero() {
		start = startTime
	}

	switch model.TimeSpanType(timespan) {
	case model.TimeSpanDay, model.TimeSpanHour, model.TimeSpanMonth:
		timeSpan = model.TimeSpanType(timespan)
	}

	return start, end, timeSpan
}

func fillGaps(
	data []model.ChartData,
	start, end time.Time,
	t model.TimeSpanType,
) []model.ChartData {
	if len(data) == 0 || t == model.TimeSpanMonth {
		return data
	}

	var timeSpan time.Duration
	switch t {
	case model.TimeSpanDay:
		timeSpan = time.Hour * 24
	case model.TimeSpanHour:
		timeSpan = time.Hour
	case model.TimeSpanMinute:
		timeSpan = time.Minute
	default:
		return data
	}

	// Handle first point
	firstPoint := time.Unix(data[0].Timestamp, 0)

	firstAlignedTime := firstPoint
	for !firstAlignedTime.Add(-timeSpan).Before(start) {
		firstAlignedTime = firstAlignedTime.Add(-timeSpan)
	}

	var firstIsZero bool
	if !firstAlignedTime.Equal(firstPoint) {
		data = append([]model.ChartData{
			{
				Timestamp: firstAlignedTime.Unix(),
			},
		}, data...)
		firstIsZero = true
	}

	// Handle last point
	lastPoint := time.Unix(data[len(data)-1].Timestamp, 0)

	lastAlignedTime := lastPoint
	for !lastAlignedTime.Add(timeSpan).After(end) {
		lastAlignedTime = lastAlignedTime.Add(timeSpan)
	}

	var lastIsZero bool
	if !lastAlignedTime.Equal(lastPoint) {
		data = append(data, model.ChartData{
			Timestamp: lastAlignedTime.Unix(),
		})
		lastIsZero = true
	}

	result := make([]model.ChartData, 0, len(data))
	result = append(result, data[0])

	for i := 1; i < len(data); i++ {
		curr := data[i]
		prev := data[i-1]
		hourDiff := (curr.Timestamp - prev.Timestamp) / int64(timeSpan.Seconds())

		// If gap is 1 hour or less, continue
		if hourDiff <= 1 {
			result = append(result, curr)
			continue
		}

		// If gap is more than 3 hours, only add boundary points
		if hourDiff > 3 {
			// Add point for hour after prev
			if i != 1 || (i == 1 && !firstIsZero) {
				result = append(result, model.ChartData{
					Timestamp: prev.Timestamp + int64(timeSpan.Seconds()),
				})
			}
			// Add point for hour before curr
			if i != len(data)-1 || (i == len(data)-1 && !lastIsZero) {
				result = append(result, model.ChartData{
					Timestamp: curr.Timestamp - int64(timeSpan.Seconds()),
				})
			}

			result = append(result, curr)

			continue
		}

		// Fill gaps of 2-3 hours with zero points
		for j := prev.Timestamp + int64(timeSpan.Seconds()); j < curr.Timestamp; j += int64(timeSpan.Seconds()) {
			result = append(result, model.ChartData{
				Timestamp: j,
			})
		}

		result = append(result, curr)
	}

	return result
}

// GetDashboard godoc
//
//	@Summary		Get dashboard data
//	@Description	Returns the general dashboard data including usage statistics and metrics
//	@Tags			dashboard
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			channel			query		int		false	"Channel ID"
//	@Param			model			query		string	false	"Model name"
//	@Param			start_timestamp	query		int64	false	"Start second timestamp"
//	@Param			end_timestamp	query		int64	false	"End second timestamp"
//	@Param			timezone		query		string	false	"Timezone, default is Local"
//	@Param			timespan		query		string	false	"Time span type (minute, hour, day, month)"
//	@Param			fields			query		string	false	"Comma-separated list of fields to select (e.g., request_count,exception_count,cache_hit_count). Available: request_count,retry_count,exception_count,status4xx_count,status5xx_count,status400_count,status429_count,status500_count,cache_hit_count,input_tokens,image_input_tokens,audio_input_tokens,output_tokens,image_output_tokens,cached_tokens,cache_creation_tokens,total_tokens,web_search_count,used_amount,total_time,total_ttfb. Groups: count,usage,time,all"
//	@Success		200				{object}	middleware.APIResponse{data=model.DashboardResponse}
//	@Router			/api/dashboard/ [get]
func GetDashboard(c *gin.Context) {
	startTime, endTime := utils.ParseTimeRange(c, -1)
	timezoneLocation, _ := time.LoadLocation(c.DefaultQuery("timezone", "Local"))
	timespan := c.Query("timespan")
	start, end, timeSpan := getDashboardTime(
		c.Query("type"),
		timespan,
		startTime,
		endTime,
		timezoneLocation,
	)
	modelName := c.Query("model")
	channelStr := c.Query("channel")
	channelID, _ := strconv.Atoi(channelStr)
	fields := model.ParseSummaryFields(c.Query("fields"))

	dashboards, err := model.GetDashboardData(
		start,
		end,
		modelName,
		channelID,
		timeSpan,
		timezoneLocation,
		fields,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	dashboards.ChartData = fillGaps(dashboards.ChartData, start, end, timeSpan)

	if channelID == 0 {
		channelStr = "*"
	}

	rpm, _ := reqlimit.GetChannelModelRequest(c.Request.Context(), channelStr, modelName)
	dashboards.RPM = rpm
	tpm, _ := reqlimit.GetChannelModelTokensRequest(c.Request.Context(), channelStr, modelName)
	dashboards.TPM = tpm

	middleware.SuccessResponse(c, dashboards)
}

// GetGroupDashboard godoc
//
//	@Summary		Get dashboard data for a specific group
//	@Description	Returns dashboard data and metrics specific to the given group
//	@Tags			dashboard
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group"
//	@Param			token_name		query		string	false	"Token name"
//	@Param			model			query		string	false	"Model or *"
//	@Param			start_timestamp	query		int64	false	"Start second timestamp"
//	@Param			end_timestamp	query		int64	false	"End second timestamp"
//	@Param			timezone		query		string	false	"Timezone, default is Local"
//	@Param			timespan		query		string	false	"Time span type (minute, hour, day, month)"
//	@Param			fields			query		string	false	"Comma-separated list of fields to select (e.g., request_count,exception_count,cache_hit_count). Available: request_count,retry_count,exception_count,status4xx_count,status5xx_count,status400_count,status429_count,status500_count,cache_hit_count,input_tokens,image_input_tokens,audio_input_tokens,output_tokens,image_output_tokens,cached_tokens,cache_creation_tokens,total_tokens,web_search_count,used_amount,total_time,total_ttfb. Groups: count,usage,time,all"
//	@Success		200				{object}	middleware.APIResponse{data=model.GroupDashboardResponse}
//	@Router			/api/dashboard/{group} [get]
func GetGroupDashboard(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	startTime, endTime := utils.ParseTimeRange(c, -1)
	timezoneLocation, _ := time.LoadLocation(c.DefaultQuery("timezone", "Local"))
	timespan := c.Query("timespan")
	start, end, timeSpan := getDashboardTime(
		c.Query("type"),
		timespan,
		startTime,
		endTime,
		timezoneLocation,
	)
	tokenName := c.Query("token_name")
	modelName := c.Query("model")
	fields := model.ParseSummaryFields(c.Query("fields"))

	dashboards, err := model.GetGroupDashboardData(
		group,
		start,
		end,
		tokenName,
		modelName,
		timeSpan,
		timezoneLocation,
		fields,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to get statistics")
		return
	}

	dashboards.ChartData = fillGaps(dashboards.ChartData, start, end, timeSpan)

	rpm, _ := reqlimit.GetGroupModelTokennameRequest(
		c.Request.Context(),
		group,
		modelName,
		tokenName,
	)
	dashboards.RPM = rpm
	tpm, _ := reqlimit.GetGroupModelTokennameTokensRequest(
		c.Request.Context(),
		group,
		modelName,
		tokenName,
	)
	dashboards.TPM = tpm

	middleware.SuccessResponse(c, dashboards)
}

type GroupModel struct {
	CreatedAt int64                        `json:"created_at,omitempty"`
	UpdatedAt int64                        `json:"updated_at,omitempty"`
	Config    map[model.ModelConfigKey]any `json:"config,omitempty"`
	Model     string                       `json:"model"`
	Owner     model.ModelOwner             `json:"owner"`
	Type      mode.Mode                    `json:"type"`
	RPM       int64                        `json:"rpm,omitempty"`
	TPM       int64                        `json:"tpm,omitempty"`
	// map[size]map[quality]price_per_image
	ImageQualityPrices map[string]map[string]float64 `json:"image_quality_prices,omitempty"`
	// map[size]price_per_image
	ImagePrices    map[string]float64 `json:"image_prices,omitempty"`
	Price          model.Price        `json:"price,omitempty"`
	EnabledPlugins []string           `json:"enabled_plugins,omitempty"`
}

func getEnabledPlugins(plugin map[string]map[string]any) []string {
	enabledPlugins := make([]string, 0, len(plugin))
	for pluginName, pluginConfig := range plugin {
		enable, ok := pluginConfig["enable"]
		if !ok {
			continue
		}

		e, _ := enable.(bool)
		if e {
			enabledPlugins = append(enabledPlugins, pluginName)
		}
	}

	return enabledPlugins
}

func NewGroupModel(mc model.ModelConfig) GroupModel {
	gm := GroupModel{
		Config:             mc.Config,
		Model:              mc.Model,
		Owner:              mc.Owner,
		Type:               mc.Type,
		RPM:                mc.RPM,
		TPM:                mc.TPM,
		ImageQualityPrices: mc.ImageQualityPrices,
		ImagePrices:        mc.ImagePrices,
		Price:              mc.Price,
		EnabledPlugins:     getEnabledPlugins(mc.Plugin),
	}
	if !mc.CreatedAt.IsZero() {
		gm.CreatedAt = mc.CreatedAt.Unix()
	}

	if !mc.UpdatedAt.IsZero() {
		gm.UpdatedAt = mc.UpdatedAt.Unix()
	}

	return gm
}

// GetGroupDashboardModels godoc
//
//	@Summary		Get model usage data for a specific group
//	@Description	Returns model-specific metrics and usage data for the given group
//	@Tags			dashboard
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group"
//	@Success		200		{object}	middleware.APIResponse{data=[]GroupModel}
//	@Router			/api/dashboard/{group}/models [get]
func GetGroupDashboardModels(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	groupCache, err := model.CacheGetGroup(group)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.SuccessResponse(
				c,
				model.LoadModelCaches().EnabledModelConfigsBySet[model.ChannelDefaultSet],
			)
		} else {
			middleware.ErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to get group: %v", err))
		}

		return
	}

	availableSet := groupCache.GetAvailableSets()
	enabledModelConfigs := model.LoadModelCaches().EnabledModelConfigsBySet

	newEnabledModelConfigs := make([]GroupModel, 0)
	for _, set := range availableSet {
		for _, mc := range enabledModelConfigs[set] {
			if slices.ContainsFunc(newEnabledModelConfigs, func(m GroupModel) bool {
				return m.Model == mc.Model
			}) {
				continue
			}

			newEnabledModelConfigs = append(
				newEnabledModelConfigs,
				NewGroupModel(
					middleware.GetGroupAdjustedModelConfig(*groupCache, mc),
				),
			)
		}
	}

	middleware.SuccessResponse(c, newEnabledModelConfigs)
}

// GetTimeSeriesModelData godoc
//
//	@Summary		Get model usage data for a specific channel
//	@Description	Returns model-specific metrics and usage data for the given channel
//	@Tags			dashboard
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			channel			query		int		false	"Channel ID"
//	@Param			model			query		string	false	"Model name"
//	@Param			start_timestamp	query		int64	false	"Start timestamp"
//	@Param			end_timestamp	query		int64	false	"End timestamp"
//	@Param			timezone		query		string	false	"Timezone, default is Local"
//	@Param			timespan		query		string	false	"Time span type (minute, hour, day, month)"
//	@Param			fields			query		string	false	"Comma-separated list of fields to select (e.g., request_count,exception_count,cache_hit_count). Available: request_count,retry_count,exception_count,status4xx_count,status5xx_count,status400_count,status429_count,status500_count,cache_hit_count,input_tokens,image_input_tokens,audio_input_tokens,output_tokens,image_output_tokens,cached_tokens,cache_creation_tokens,total_tokens,web_search_count,used_amount,total_time,total_ttfb. Groups: count,usage,time,all"
//	@Success		200				{object}	middleware.APIResponse{data=[]model.TimeSummaryDataV2}
//	@Router			/api/dashboardv2/ [get]
func GetTimeSeriesModelData(c *gin.Context) {
	channelID, _ := strconv.Atoi(c.Query("channel"))
	modelName := c.Query("model")
	startTime, endTime := utils.ParseTimeRange(c, -1)
	timezoneLocation, _ := time.LoadLocation(c.DefaultQuery("timezone", "Local"))
	fields := model.ParseSummaryFields(c.Query("fields"))

	models, err := model.GetTimeSeriesModelData(
		channelID,
		modelName,
		startTime,
		endTime,
		model.TimeSpanType(c.Query("timespan")),
		timezoneLocation,
		fields,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, models)
}

// GetGroupTimeSeriesModelData godoc
//
//	@Summary		Get model usage data for a specific group
//	@Description	Returns model-specific metrics and usage data for the given group
//	@Tags			dashboard
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group"
//	@Param			token_name		query		string	false	"Token name"
//	@Param			model			query		string	false	"Model name"
//	@Param			start_timestamp	query		int64	false	"Start timestamp"
//	@Param			end_timestamp	query		int64	false	"End timestamp"
//	@Param			timezone		query		string	false	"Timezone, default is Local"
//	@Param			timespan		query		string	false	"Time span type (minute, hour, day, month)"
//	@Param			fields			query		string	false	"Comma-separated list of fields to select (e.g., request_count,exception_count,cache_hit_count). Available: request_count,retry_count,exception_count,status4xx_count,status5xx_count,status400_count,status429_count,status500_count,cache_hit_count,input_tokens,image_input_tokens,audio_input_tokens,output_tokens,image_output_tokens,cached_tokens,cache_creation_tokens,total_tokens,web_search_count,used_amount,total_time,total_ttfb. Groups: count,usage,time,all"
//	@Success		200				{object}	middleware.APIResponse{data=[]model.TimeSummaryDataV2}
//	@Router			/api/dashboardv2/{group} [get]
func GetGroupTimeSeriesModelData(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	tokenName := c.Query("token_name")
	modelName := c.Query("model")
	startTime, endTime := utils.ParseTimeRange(c, -1)
	timezoneLocation, _ := time.LoadLocation(c.DefaultQuery("timezone", "Local"))
	fields := model.ParseSummaryFields(c.Query("fields"))

	models, err := model.GetGroupTimeSeriesModelData(
		group,
		tokenName,
		modelName,
		startTime,
		endTime,
		model.TimeSpanType(c.Query("timespan")),
		timezoneLocation,
		fields,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, models)
}
