package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

func parseTimeRange(c *gin.Context) (startTime, endTime time.Time) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	if startTimestamp != 0 {
		startTime = time.UnixMilli(startTimestamp)
	}
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	if startTime.IsZero() || startTime.Before(sevenDaysAgo) {
		startTime = sevenDaysAgo
	}

	if endTimestamp != 0 {
		endTime = time.UnixMilli(endTimestamp)
	}
	return
}

func parseCommonParams(c *gin.Context) (params struct {
	tokenName string
	modelName string
	channelID int
	tokenID   int
	order     string
	requestID string
	codeType  string
	code      int
	withBody  bool
	ip        string
	user      string
},
) {
	params.tokenName = c.Query("token_name")
	params.modelName = c.Query("model_name")
	params.channelID, _ = strconv.Atoi(c.Query("channel"))
	params.tokenID, _ = strconv.Atoi(c.Query("token_id"))
	params.order = c.Query("order")
	params.requestID = c.Query("request_id")
	params.codeType = c.Query("code_type")
	params.code, _ = strconv.Atoi(c.Query("code"))
	params.withBody, _ = strconv.ParseBool(c.Query("with_body"))
	params.ip = c.Query("ip")
	params.user = c.Query("user")
	return
}

// GetLogs godoc
//
//	@Summary		Get all logs
//	@Description	Returns a paginated list of all logs with optional filters
//	@Tags			logs
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			query		string	false	"Group or *"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			start_timestamp	query		int		false	"Start timestamp (milliseconds)"
//	@Param			end_timestamp	query		int		false	"End timestamp (milliseconds)"
//	@Param			token_name		query		string	false	"Token name"
//	@Param			model_name		query		string	false	"Model name"
//	@Param			channel			query		int		false	"Channel ID"
//	@Param			token_id		query		int		false	"Token ID"
//	@Param			order			query		string	false	"Order"
//	@Param			request_id		query		string	false	"Request ID"
//	@Param			code_type		query		string	false	"Status code type"
//	@Param			code			query		int		false	"Status code"
//	@Param			with_body		query		bool	false	"With body"
//	@Param			ip				query		string	false	"IP"
//	@Param			user			query		string	false	"User"
//	@Success		200				{object}	middleware.APIResponse{data=model.GetLogsResult}
//	@Router			/api/logs/ [get]
func GetLogs(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	startTime, endTime := parseTimeRange(c)
	params := parseCommonParams(c)
	group := c.Query("group")

	result, err := model.GetLogs(
		group,
		startTime,
		endTime,
		params.modelName,
		params.requestID,
		params.tokenID,
		params.tokenName,
		params.channelID,
		params.order,
		model.CodeType(params.codeType),
		params.code,
		params.withBody,
		params.ip,
		params.user,
		page,
		perPage,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, result)
}

// GetGroupLogs godoc
//
//	@Summary		Get group logs
//	@Description	Get logs for a specific group
//	@Tags			log
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group name"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			start_timestamp	query		int		false	"Start timestamp (milliseconds)"
//	@Param			end_timestamp	query		int		false	"End timestamp (milliseconds)"
//	@Param			token_name		query		string	false	"Token name"
//	@Param			model_name		query		string	false	"Model name"
//	@Param			channel			query		int		false	"Channel ID"
//	@Param			token_id		query		int		false	"Token ID"
//	@Param			order			query		string	false	"Order"
//	@Param			request_id		query		string	false	"Request ID"
//	@Param			code_type		query		string	false	"Status code type"
//	@Param			code			query		int		false	"Status code"
//	@Param			with_body		query		bool	false	"With body"
//	@Param			ip				query		string	false	"IP"
//	@Param			user			query		string	false	"User"
//	@Success		200				{object}	middleware.APIResponse{data=model.GetGroupLogsResult}
//	@Router			/api/log/{group} [get]
func GetGroupLogs(c *gin.Context) {
	group := c.Param("group")
	if group == "" || group == "*" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	page, perPage := utils.ParsePageParams(c)
	startTime, endTime := parseTimeRange(c)
	params := parseCommonParams(c)

	result, err := model.GetGroupLogs(
		group,
		startTime,
		endTime,
		params.modelName,
		params.requestID,
		params.tokenID,
		params.tokenName,
		params.channelID,
		params.order,
		model.CodeType(params.codeType),
		params.code,
		params.withBody,
		params.ip,
		params.user,
		page,
		perPage,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, result)
}

// SearchLogs godoc
//
//	@Summary		Search logs
//	@Description	Search logs with various filters
//	@Tags			logs
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			query		string	false	"Group or *"
//	@Param			keyword			query		string	true	"Keyword"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			start_timestamp	query		int		false	"Start timestamp (milliseconds)"
//	@Param			end_timestamp	query		int		false	"End timestamp (milliseconds)"
//	@Param			token_name		query		string	false	"Filter by token name"
//	@Param			model_name		query		string	false	"Filter by model name"
//	@Param			channel			query		int		false	"Filter by channel"
//	@Param			token_id		query		int		false	"Filter by token id"
//	@Param			order			query		string	false	"Order"
//	@Param			request_id		query		string	false	"Request ID"
//	@Param			code_type		query		string	false	"Status code type"
//	@Param			code			query		int		false	"Status code"
//	@Param			with_body		query		bool	false	"With body"
//	@Param			ip				query		string	false	"IP"
//	@Param			user			query		string	false	"User"
//	@Success		200				{object}	middleware.APIResponse{data=model.GetLogsResult}
//	@Router			/api/logs/search [get]
func SearchLogs(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	startTime, endTime := parseTimeRange(c)
	params := parseCommonParams(c)

	keyword := c.Query("keyword")
	group := c.Query("group")

	result, err := model.SearchLogs(
		group,
		keyword,
		params.requestID,
		params.tokenID,
		params.tokenName,
		params.modelName,
		startTime,
		endTime,
		params.channelID,
		params.order,
		model.CodeType(params.codeType),
		params.code,
		params.withBody,
		params.ip,
		params.user,
		page,
		perPage,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, result)
}

// SearchGroupLogs godoc
//
//	@Summary		Search group logs
//	@Description	Search logs for a specific group with filters
//	@Tags			log
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group name"
//	@Param			keyword			query		string	true	"Keyword"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			start_timestamp	query		int		false	"Start timestamp (milliseconds)"
//	@Param			end_timestamp	query		int		false	"End timestamp (milliseconds)"
//	@Param			token_name		query		string	false	"Filter by token name"
//	@Param			model_name		query		string	false	"Filter by model name"
//	@Param			channel			query		int		false	"Filter by channel"
//	@Param			token_id		query		int		false	"Filter by token id"
//	@Param			order			query		string	false	"Order"
//	@Param			request_id		query		string	false	"Request ID"
//	@Param			code_type		query		string	false	"Status code type"
//	@Param			code			query		int		false	"Status code"
//	@Param			with_body		query		bool	false	"With body"
//	@Param			ip				query		string	false	"IP"
//	@Param			user			query		string	false	"User"
//	@Success		200				{object}	middleware.APIResponse{data=model.GetGroupLogsResult}
//	@Router			/api/log/{group}/search [get]
func SearchGroupLogs(c *gin.Context) {
	group := c.Param("group")
	if group == "" || group == "*" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	page, perPage := utils.ParsePageParams(c)
	startTime, endTime := parseTimeRange(c)
	params := parseCommonParams(c)
	keyword := c.Query("keyword")

	result, err := model.SearchGroupLogs(
		group,
		keyword,
		params.requestID,
		params.tokenID,
		params.tokenName,
		params.modelName,
		startTime,
		endTime,
		params.channelID,
		params.order,
		model.CodeType(params.codeType),
		params.code,
		params.withBody,
		params.ip,
		params.user,
		page,
		perPage,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, result)
}

// GetUsedModels godoc
//
//	@Summary		Get used models
//	@Description	Get a list of models that have been used in logs
//	@Tags			logs
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	query		string	false	"Group or *"
//	@Success		200		{object}	middleware.APIResponse{data=[]string}
//	@Router			/api/logs/used/models [get]
func GetUsedModels(c *gin.Context) {
	group := c.Query("group")
	startTime, endTime := parseTimeRange(c)
	models, err := model.GetUsedModelsFromLog(group, startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, models)
}

// GetGroupUsedModels godoc
//
//	@Summary		Get group used models
//	@Description	Get a list of models that have been used in a specific group's logs
//	@Tags			log
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Success		200		{object}	middleware.APIResponse{data=[]string}
//	@Router			/api/log/{group}/used/models [get]
func GetGroupUsedModels(c *gin.Context) {
	group := c.Param("group")
	if group == "" || group == "*" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}
	startTime, endTime := parseTimeRange(c)
	models, err := model.GetUsedModelsFromLog(group, startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, models)
}

// GetLogDetail godoc
//
//	@Summary		Get log detail
//	@Description	Get detailed information about a specific log entry
//	@Tags			logs
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			log_id	path		string	true	"Log ID"
//	@Success		200		{object}	middleware.APIResponse{data=model.RequestDetail}
//	@Router			/api/logs/detail/{log_id} [get]
func GetLogDetail(c *gin.Context) {
	logID, _ := strconv.Atoi(c.Param("log_id"))
	log, err := model.GetLogDetail(logID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, log)
}

// GetGroupLogDetail godoc
//
//	@Summary		Get group log detail
//	@Description	Get detailed information about a specific log entry in a group
//	@Tags			log
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Param			log_id	path		string	true	"Log ID"
//	@Success		200		{object}	middleware.APIResponse{data=model.RequestDetail}
//	@Router			/api/log/{group}/detail/{log_id} [get]
func GetGroupLogDetail(c *gin.Context) {
	group := c.Param("group")
	if group == "" || group == "*" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}
	logID, _ := strconv.Atoi(c.Param("log_id"))
	log, err := model.GetGroupLogDetail(logID, group)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, log)
}

// GetUsedTokenNames godoc
//
//	@Summary		Get used token names
//	@Description	Get a list of token names that have been used in logs
//	@Tags			logs
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	query		string	false	"Group or *"
//	@Success		200		{object}	middleware.APIResponse{data=[]string}
//	@Router			/api/logs/used/token_names [get]
func GetUsedTokenNames(c *gin.Context) {
	group := c.Query("group")
	startTime, endTime := parseTimeRange(c)
	tokenNames, err := model.GetUsedTokenNamesFromLog(group, startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, tokenNames)
}

// GetGroupUsedTokenNames godoc
//
//	@Summary		Get group used token names
//	@Description	Get a list of token names that have been used in a specific group's logs
//	@Tags			log
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Success		200		{object}	middleware.APIResponse{data=[]string}
//	@Router			/api/log/{group}/used/token_names [get]
func GetGroupUsedTokenNames(c *gin.Context) {
	group := c.Param("group")
	if group == "" || group == "*" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}
	startTime, endTime := parseTimeRange(c)
	tokenNames, err := model.GetUsedTokenNamesFromLog(group, startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, tokenNames)
}

// DeleteHistoryLogs godoc
//
//	@Summary		Delete historical logs
//	@Description	Deletes logs older than the specified retention period
//	@Tags			logs
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			timestamp	query		int	true	"Timestamp (milliseconds)"
//	@Success		200			{object}	middleware.APIResponse{data=int}
//	@Router			/api/logs/ [delete]
func DeleteHistoryLogs(c *gin.Context) {
	timestamp, _ := strconv.ParseInt(c.Query("timestamp"), 10, 64)
	if timestamp == 0 {
		middleware.ErrorResponse(c, http.StatusBadRequest, "timestamp is required")
		return
	}
	count, err := model.DeleteOldLog(time.UnixMilli(timestamp))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, count)
}

// SearchConsumeError godoc
//
//	@Summary		Search consumption errors
//	@Description	Search for logs with consumption errors
//	@Tags			logs
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			start_timestamp	query		int		false	"Start timestamp (milliseconds)"
//	@Param			end_timestamp	query		int		false	"End timestamp (milliseconds)"
//	@Param			keyword			query		string	false	"Keyword"
//	@Param			group			query		string	false	"Group"
//	@Param			token_name		query		string	false	"Token name"
//	@Param			model_name		query		string	false	"Model name"
//	@Param			content			query		string	false	"Content"
//	@Param			token_id		query		int		false	"Token ID"
//	@Param			order			query		string	false	"Order"
//	@Param			request_id		query		string	false	"Request ID"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{logs=[]model.RequestDetail,total=int}}
//	@Router			/api/logs/consume_error [get]
func SearchConsumeError(c *gin.Context) {
	keyword := c.Query("keyword")
	group := c.Query("group")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	tokenID, _ := strconv.Atoi(c.Query("token_id"))
	page, perPage := utils.ParsePageParams(c)
	order := c.Query("order")
	requestID := c.Query("request_id")

	logs, total, err := model.SearchConsumeError(
		keyword,
		requestID,
		group,
		tokenName,
		modelName,
		tokenID,
		page,
		perPage,
		order,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, gin.H{
		"logs":  logs,
		"total": total,
	})
}
