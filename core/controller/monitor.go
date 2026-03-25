package controller

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
)

type RuntimeChannelModelMetric struct {
	RPM       int64   `json:"rpm"`
	TPM       int64   `json:"tpm"`
	RPS       int64   `json:"rps"`
	TPS       int64   `json:"tps"`
	Requests  int64   `json:"requests"`
	Errors    int64   `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
	Banned    bool    `json:"banned"`
}

type RuntimeModelMetric struct {
	RPM              int64                          `json:"rpm"`
	TPM              int64                          `json:"tpm"`
	RPS              int64                          `json:"rps"`
	TPS              int64                          `json:"tps"`
	Requests         int64                          `json:"requests"`
	Errors           int64                          `json:"errors"`
	ErrorRate        float64                        `json:"error_rate"`
	BannedChannels   int                            `json:"banned_channels"`
	Channels         map[int64]RuntimeChannelMetric `json:"channels"`
	AccessibleSets   []string                       `json:"accessible_sets"`
	AccessibleGroups int                            `json:"accessible_groups"`
}

type RuntimeChannelMetric struct {
	RPM          int64                         `json:"rpm"`
	TPM          int64                         `json:"tpm"`
	RPS          int64                         `json:"rps"`
	TPS          int64                         `json:"tps"`
	Requests     int64                         `json:"requests"`
	Errors       int64                         `json:"errors"`
	ErrorRate    float64                       `json:"error_rate"`
	BannedModels int                           `json:"banned_models"`
	Models       map[string]RuntimeModelMetric `json:"models"`
}

type RuntimeMetricsResponse struct {
	Models        map[string]RuntimeModelMetric                  `json:"models"`
	Channels      map[int64]RuntimeChannelMetric                 `json:"channels"`
	ChannelModels map[int64]map[string]RuntimeChannelModelMetric `json:"channel_models"`
}

// GetAllChannelModelErrorRates godoc
//
//	@Summary		Get all channel model error rates
//	@Description	Returns a list of all channel model error rates
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[int64]map[string]float64}
//	@Router			/api/monitor/ [get]
func GetAllChannelModelErrorRates(c *gin.Context) {
	rates, err := monitor.GetAllChannelModelErrorRates(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, rates)
}

// GetChannelModelErrorRates godoc
//
//	@Summary		Get channel model error rates
//	@Description	Returns a list of channel model error rates
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Channel ID"
//	@Success		200	{object}	middleware.APIResponse{data=[]map[string]float64}
//	@Router			/api/monitor/{id} [get]
func GetChannelModelErrorRates(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	rates, err := monitor.GetChannelModelErrorRates(c.Request.Context(), channelID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, rates)
}

// ClearAllModelErrors godoc
//
//	@Summary		Clear all model errors
//	@Description	Clears all model errors
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/monitor/ [delete]
func ClearAllModelErrors(c *gin.Context) {
	err := monitor.ClearAllModelErrors(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// ClearChannelAllModelErrors godoc
//
//	@Summary		Clear channel all model errors
//	@Description	Clears all model errors for a specific channel
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Channel ID"
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/monitor/{id} [delete]
func ClearChannelAllModelErrors(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	err = monitor.ClearChannelAllModelErrors(c.Request.Context(), int(channelID))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// ClearChannelModelErrors godoc
//
//	@Summary		Clear channel model errors
//	@Description	Clears model errors for a specific channel and model
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int		true	"Channel ID"
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/monitor/{id}/{model} [delete]
func ClearChannelModelErrors(c *gin.Context) {
	model := strings.TrimPrefix(c.Param("model"), "/")

	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	err = monitor.ClearChannelModelErrors(c.Request.Context(), model, int(channelID))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// GetModelsErrorRate godoc
//
//	@Summary		Get models error rate
//	@Description	Returns a list of models error rate
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[string]float64}
//	@Router			/api/monitor/models [get]
func GetModelsErrorRate(c *gin.Context) {
	rates, err := monitor.GetModelsErrorRate(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, rates)
}

// GetAllBannedModelChannels godoc
//
//	@Summary		Get all banned model channels
//	@Description	Returns a list of all banned model channels
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[string][]int64}
//	@Router			/api/monitor/banned_channels [get]
func GetAllBannedModelChannels(c *gin.Context) {
	channels, err := monitor.GetAllBannedModelChannels(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, channels)
}

// GetRuntimeMetrics godoc
//
//	@Summary		Get runtime metrics for models and channels
//	@Description	Returns runtime rpm/tpm/error metrics sourced from reqlimit and monitor
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=controller.RuntimeMetricsResponse}
//	@Router			/api/monitor/runtime_metrics [get]
func GetRuntimeMetrics(c *gin.Context) {
	rates, err := reqlimit.GetAllChannelModelRates(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	stats, err := monitor.GetAllModelChannelStats(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp := RuntimeMetricsResponse{
		Models:        make(map[string]RuntimeModelMetric),
		Channels:      make(map[int64]RuntimeChannelMetric),
		ChannelModels: make(map[int64]map[string]RuntimeChannelModelMetric),
	}

	modelSets := model.LoadModelCaches().EnabledModel2ChannelsBySet

	modelSetNames := make(map[string][]string)
	for setName, modelChannels := range modelSets {
		for modelName := range modelChannels {
			modelSetNames[modelName] = append(modelSetNames[modelName], setName)
		}
	}

	for modelName := range modelSetNames {
		slices.Sort(modelSetNames[modelName])
	}

	ensureModel := func(modelName string) RuntimeModelMetric {
		metric, exists := resp.Models[modelName]
		if !exists {
			metric = RuntimeModelMetric{
				Channels:       make(map[int64]RuntimeChannelMetric),
				AccessibleSets: modelSetNames[modelName],
			}
			metric.AccessibleGroups = len(metric.AccessibleSets)
		}

		return metric
	}

	ensureChannel := func(channelID int64) RuntimeChannelMetric {
		metric, exists := resp.Channels[channelID]
		if !exists {
			metric = RuntimeChannelMetric{
				Models: make(map[string]RuntimeModelMetric),
			}
		}

		return metric
	}

	for channelID, modelRates := range rates {
		channelMetric := ensureChannel(channelID)
		if _, exists := resp.ChannelModels[channelID]; !exists {
			resp.ChannelModels[channelID] = make(map[string]RuntimeChannelModelMetric)
		}

		for modelName, rate := range modelRates {
			modelMetric := ensureModel(modelName)
			pair := resp.ChannelModels[channelID][modelName]
			pair.RPM = rate.RPM
			pair.TPM = rate.TPM
			pair.RPS = rate.RPS
			pair.TPS = rate.TPS
			resp.ChannelModels[channelID][modelName] = pair

			modelMetric.RPM += rate.RPM
			modelMetric.TPM += rate.TPM
			modelMetric.RPS += rate.RPS
			modelMetric.TPS += rate.TPS
			resp.Models[modelName] = modelMetric

			channelMetric.RPM += rate.RPM
			channelMetric.TPM += rate.TPM
			channelMetric.RPS += rate.RPS
			channelMetric.TPS += rate.TPS
		}

		resp.Channels[channelID] = channelMetric
	}

	for modelName, channelStats := range stats {
		modelMetric := ensureModel(modelName)
		for channelID, stat := range channelStats {
			if _, exists := resp.ChannelModels[channelID]; !exists {
				resp.ChannelModels[channelID] = make(map[string]RuntimeChannelModelMetric)
			}

			pair := resp.ChannelModels[channelID][modelName]
			pair.Requests = stat.Requests
			pair.Errors = stat.Errors

			pair.Banned = stat.Banned
			if stat.Requests > 0 {
				pair.ErrorRate = float64(stat.Errors) / float64(stat.Requests)
			}

			resp.ChannelModels[channelID][modelName] = pair

			modelMetric.Requests += stat.Requests

			modelMetric.Errors += stat.Errors
			if stat.Banned {
				modelMetric.BannedChannels++
			}

			channelMetric := ensureChannel(channelID)
			channelMetric.Requests += stat.Requests

			channelMetric.Errors += stat.Errors
			if stat.Banned {
				channelMetric.BannedModels++
			}

			modelMetric.Channels[channelID] = RuntimeChannelMetric{
				RPM:          pair.RPM,
				TPM:          pair.TPM,
				RPS:          pair.RPS,
				TPS:          pair.TPS,
				Requests:     stat.Requests,
				Errors:       stat.Errors,
				ErrorRate:    pair.ErrorRate,
				BannedModels: boolToInt(stat.Banned),
			}
			channelMetric.Models[modelName] = RuntimeModelMetric{
				RPM:            pair.RPM,
				TPM:            pair.TPM,
				RPS:            pair.RPS,
				TPS:            pair.TPS,
				Requests:       stat.Requests,
				Errors:         stat.Errors,
				ErrorRate:      pair.ErrorRate,
				BannedChannels: boolToInt(stat.Banned),
			}

			resp.Models[modelName] = modelMetric
			resp.Channels[channelID] = channelMetric
		}
	}

	for modelName, metric := range resp.Models {
		if metric.Requests > 0 {
			metric.ErrorRate = float64(metric.Errors) / float64(metric.Requests)
		}

		resp.Models[modelName] = metric
	}

	for channelID, metric := range resp.Channels {
		if metric.Requests > 0 {
			metric.ErrorRate = float64(metric.Errors) / float64(metric.Requests)
		}

		resp.Channels[channelID] = metric
	}

	middleware.SuccessResponse(c, resp)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
