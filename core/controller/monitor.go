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

type GroupSummaryMetricsResponse struct {
	Groups map[string]RuntimeRateMetric `json:"groups"`
}

type GroupTokenMetricsResponse struct {
	Tokens map[string]RuntimeRateMetric `json:"tokens"`
}

type GroupModelMetricsResponse struct {
	Models map[string]RuntimeRateMetric `json:"models"`
}

type GroupTokennameModelMetricsResponse struct {
	Items []GroupTokennameModelMetricsItem `json:"items"`
}

type GroupTokennameModelMetricsItem struct {
	Group     string `json:"group"`
	TokenName string `json:"token_name"`
	Model     string `json:"model"`
	RuntimeRateMetric
}

type BatchGroupTokenMetricsRequest struct {
	Items []BatchGroupTokenMetricsRequestItem `json:"items"`
}

type BatchGroupTokenMetricsRequestItem struct {
	Group     string `json:"group"`
	TokenName string `json:"token_name"`
}

type BatchGroupTokenMetricsResponse struct {
	Items []BatchGroupTokenMetricsItem `json:"items"`
}

type BatchGroupTokenMetricsItem struct {
	Group     string `json:"group"`
	TokenName string `json:"token_name"`
	RuntimeRateMetric
}

type RuntimeRateMetric struct {
	RPM int64 `json:"rpm"`
	TPM int64 `json:"tpm"`
	RPS int64 `json:"rps"`
	TPS int64 `json:"tps"`
}

func buildRuntimeMetrics(ctx *gin.Context) (RuntimeMetricsResponse, error) {
	rates, err := reqlimit.GetAllChannelModelRates(ctx.Request.Context())
	if err != nil {
		return RuntimeMetricsResponse{}, err
	}

	stats, err := monitor.GetAllModelChannelStats(ctx.Request.Context())
	if err != nil {
		return RuntimeMetricsResponse{}, err
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

	return resp, nil
}

func parseCSVParam(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func collectGroupOverviewMetrics(ctx *gin.Context, groupIDs []string) (map[string]RuntimeRateMetric, error) {
	result := make(map[string]RuntimeRateMetric)
	if len(groupIDs) == 0 {
		return result, nil
	}

	for _, groupID := range groupIDs {
		result[groupID] = RuntimeRateMetric{}

		requestSnapshots, err := reqlimit.GetGroupModelRequestSnapshots(
			ctx.Request.Context(),
			groupID,
			"*",
		)
		if err != nil {
			return nil, err
		}

		tokenSnapshots, err := reqlimit.GetGroupModelTokensRequestSnapshots(
			ctx.Request.Context(),
			groupID,
			"*",
		)
		if err != nil {
			return nil, err
		}

		metric := result[groupID]
		for _, snapshot := range requestSnapshots {
			metric.RPM += snapshot.TotalCount
			metric.RPS += snapshot.SecondCount
		}
		for _, snapshot := range tokenSnapshots {
			metric.TPM += snapshot.TotalCount
			metric.TPS += snapshot.SecondCount
		}
		result[groupID] = metric
	}

	return result, nil
}

func collectGroupTokenMetrics(ctx *gin.Context, groupID string) (map[string]RuntimeRateMetric, error) {
	result := make(map[string]RuntimeRateMetric)

	requestSnapshots, err := reqlimit.GetGroupModelTokennameRequestSnapshots(
		ctx.Request.Context(),
		groupID,
		"*",
		"*",
	)
	if err != nil {
		return nil, err
	}

	tokenSnapshots, err := reqlimit.GetGroupModelTokennameTokensRequestSnapshots(
		ctx.Request.Context(),
		groupID,
		"*",
		"*",
	)
	if err != nil {
		return nil, err
	}

	for _, snapshot := range requestSnapshots {
		if len(snapshot.Keys) != 3 {
			continue
		}
		tokenName := snapshot.Keys[2]
		metric := result[tokenName]
		metric.RPM += snapshot.TotalCount
		metric.RPS += snapshot.SecondCount
		result[tokenName] = metric
	}

	for _, snapshot := range tokenSnapshots {
		if len(snapshot.Keys) != 3 {
			continue
		}
		tokenName := snapshot.Keys[2]
		metric := result[tokenName]
		metric.TPM += snapshot.TotalCount
		metric.TPS += snapshot.SecondCount
		result[tokenName] = metric
	}

	return result, nil
}

func collectGroupModelMetrics(ctx *gin.Context, groupID string) (map[string]RuntimeRateMetric, error) {
	result := make(map[string]RuntimeRateMetric)

	requestSnapshots, err := reqlimit.GetGroupModelRequestSnapshots(
		ctx.Request.Context(),
		groupID,
		"*",
	)
	if err != nil {
		return nil, err
	}

	tokenSnapshots, err := reqlimit.GetGroupModelTokensRequestSnapshots(
		ctx.Request.Context(),
		groupID,
		"*",
	)
	if err != nil {
		return nil, err
	}

	for _, snapshot := range requestSnapshots {
		if len(snapshot.Keys) != 2 {
			continue
		}
		modelName := snapshot.Keys[1]
		metric := result[modelName]
		metric.RPM += snapshot.TotalCount
		metric.RPS += snapshot.SecondCount
		result[modelName] = metric
	}

	for _, snapshot := range tokenSnapshots {
		if len(snapshot.Keys) != 2 {
			continue
		}
		modelName := snapshot.Keys[1]
		metric := result[modelName]
		metric.TPM += snapshot.TotalCount
		metric.TPS += snapshot.SecondCount
		result[modelName] = metric
	}

	return result, nil
}

func collectGroupTokennameModelMetrics(
	ctx *gin.Context,
	groupID string,
) ([]GroupTokennameModelMetricsItem, error) {
	requestSnapshots, err := reqlimit.GetGroupModelTokennameRequestSnapshots(
		ctx.Request.Context(),
		groupID,
		"*",
		"*",
	)
	if err != nil {
		return nil, err
	}

	tokenSnapshots, err := reqlimit.GetGroupModelTokennameTokensRequestSnapshots(
		ctx.Request.Context(),
		groupID,
		"*",
		"*",
	)
	if err != nil {
		return nil, err
	}

	resultMap := make(map[string]GroupTokennameModelMetricsItem)
	for _, snapshot := range requestSnapshots {
		if len(snapshot.Keys) != 3 {
			continue
		}
		modelName := snapshot.Keys[1]
		tokenName := snapshot.Keys[2]
		key := tokenName + "\x00" + modelName
		item := resultMap[key]
		item.Group = groupID
		item.TokenName = tokenName
		item.Model = modelName
		item.RPM += snapshot.TotalCount
		item.RPS += snapshot.SecondCount
		resultMap[key] = item
	}

	for _, snapshot := range tokenSnapshots {
		if len(snapshot.Keys) != 3 {
			continue
		}
		modelName := snapshot.Keys[1]
		tokenName := snapshot.Keys[2]
		key := tokenName + "\x00" + modelName
		item := resultMap[key]
		item.Group = groupID
		item.TokenName = tokenName
		item.Model = modelName
		item.TPM += snapshot.TotalCount
		item.TPS += snapshot.SecondCount
		resultMap[key] = item
	}

	result := make([]GroupTokennameModelMetricsItem, 0, len(resultMap))
	for _, item := range resultMap {
		result = append(result, item)
	}

	return result, nil
}

func collectGroupTokenOverviewMetrics(
	ctx *gin.Context,
	items []BatchGroupTokenMetricsRequestItem,
) ([]BatchGroupTokenMetricsItem, error) {
	result := make([]BatchGroupTokenMetricsItem, 0, len(items))
	if len(items) == 0 {
		return result, nil
	}

	for _, item := range items {
		if item.Group == "" || item.TokenName == "" {
			continue
		}

		requestSnapshots, err := reqlimit.GetGroupModelTokennameRequestSnapshots(
			ctx.Request.Context(),
			item.Group,
			"*",
			item.TokenName,
		)
		if err != nil {
			return nil, err
		}

		tokenSnapshots, err := reqlimit.GetGroupModelTokennameTokensRequestSnapshots(
			ctx.Request.Context(),
			item.Group,
			"*",
			item.TokenName,
		)
		if err != nil {
			return nil, err
		}

		metric := BatchGroupTokenMetricsItem{
			Group:     item.Group,
			TokenName: item.TokenName,
		}
		for _, snapshot := range requestSnapshots {
			metric.RPM += snapshot.TotalCount
			metric.RPS += snapshot.SecondCount
		}
		for _, snapshot := range tokenSnapshots {
			metric.TPM += snapshot.TotalCount
			metric.TPS += snapshot.SecondCount
		}
		result = append(result, metric)
	}

	return result, nil
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
	modelName := strings.TrimPrefix(c.Param("model"), "/")

	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	err = monitor.ClearChannelModelErrors(c.Request.Context(), modelName, int(channelID))
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
	resp, err := buildRuntimeMetrics(c)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, resp)
}

// GetGroupSummaryMetrics godoc
//
//	@Summary		Get summary metrics for multiple groups
//	@Description	Returns rpm/tpm summary metrics for one or more groups
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=controller.GroupSummaryMetricsResponse}
//	@Router			/api/monitor/group_summary_metrics [get]
func GetGroupSummaryMetrics(c *gin.Context) {
	groupIDs := parseCSVParam(c.Query("groups"))
	groups, err := collectGroupOverviewMetrics(c, groupIDs)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, GroupSummaryMetricsResponse{
		Groups: groups,
	})
}

// GetGroupTokenMetrics godoc
//
//	@Summary		Get token runtime metrics for a group
//	@Description	Returns runtime rpm/tpm metrics grouped by token name in a group
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Success		200		{object}	middleware.APIResponse{data=controller.GroupTokenMetricsResponse}
//	@Router			/api/monitor/group_token_metrics/{group} [get]
func GetGroupTokenMetrics(c *gin.Context) {
	groupID := strings.TrimPrefix(c.Param("group"), "/")
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	tokens, err := collectGroupTokenMetrics(c, groupID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, GroupTokenMetricsResponse{
		Tokens: tokens,
	})
}

// GetGroupModelMetrics godoc
//
//	@Summary		Get model runtime metrics for a group
//	@Description	Returns runtime rpm/tpm metrics grouped by model in a group
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Success		200		{object}	middleware.APIResponse{data=controller.GroupModelMetricsResponse}
//	@Router			/api/monitor/group_model_metrics/{group} [get]
func GetGroupModelMetrics(c *gin.Context) {
	groupID := strings.TrimPrefix(c.Param("group"), "/")
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	models, err := collectGroupModelMetrics(c, groupID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, GroupModelMetricsResponse{
		Models: models,
	})
}

// GetGroupTokennameModelMetrics godoc
//
//	@Summary		Get token-model runtime metrics for a group
//	@Description	Returns runtime rpm/tpm metrics for token name and model combinations in a group
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Success		200		{object}	middleware.APIResponse{data=controller.GroupTokennameModelMetricsResponse}
//	@Router			/api/monitor/group_tokenname_model_metrics/{group} [get]
func GetGroupTokennameModelMetrics(c *gin.Context) {
	groupID := strings.TrimPrefix(c.Param("group"), "/")
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	items, err := collectGroupTokennameModelMetrics(c, groupID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, GroupTokennameModelMetricsResponse{
		Items: items,
	})
}

// BatchGetGroupTokenMetrics godoc
//
//	@Summary		Batch get runtime metrics for api key rows
//	@Description	Returns runtime rpm/tpm summary metrics for explicit group and token-name pairs
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=controller.BatchGroupTokenMetricsResponse}
//	@Router			/api/monitor/batch_group_token_metrics [post]
func BatchGetGroupTokenMetrics(c *gin.Context) {
	var req BatchGroupTokenMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	items, err := collectGroupTokenOverviewMetrics(c, req.Items)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, BatchGroupTokenMetricsResponse{
		Items: items,
	})
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
