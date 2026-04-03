package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

// GetQuota godoc
//
//	@Summary		Get quota
//	@Description	Get quota
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	balance.GroupQuota
//	@Router			/v1/dashboard/billing/quota [get]
func GetQuota(c *gin.Context) {
	group := middleware.GetGroup(c)

	groupQuota, err := balance.GetGroupQuota(c.Request.Context(), group)
	if err != nil {
		log.Errorf("get group (%s) balance failed: %s", group.ID, err)
		middleware.ErrorResponse(
			c,
			http.StatusInternalServerError,
			fmt.Sprintf("get group (%s) balance failed", group.ID),
		)

		return
	}

	token := middleware.GetToken(c)
	if token.Quota > 0 {
		groupQuota.Total = min(groupQuota.Total, token.Quota)
		groupQuota.Remain = min(groupQuota.Remain, decimal.NewFromFloat(token.Quota).
			Sub(decimal.NewFromFloat(token.UsedAmount)).
			InexactFloat64())
	}

	c.JSON(http.StatusOK, groupQuota)
}

// GetSubscription godoc
//
//	@Summary		Get subscription
//	@Description	Get subscription
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	openai.SubscriptionResponse
//	@Router			/v1/dashboard/billing/subscription [get]
func GetSubscription(c *gin.Context) {
	group := middleware.GetGroup(c)

	groupQuota, err := balance.GetGroupQuota(c.Request.Context(), group)
	if err != nil {
		if errors.Is(err, balance.ErrNoRealNameUsedAmountLimit) {
			middleware.ErrorResponse(c, http.StatusForbidden, err.Error())
			return
		}

		log.Errorf("get group (%s) balance failed: %s", group.ID, err)
		middleware.ErrorResponse(
			c,
			http.StatusInternalServerError,
			fmt.Sprintf("get group (%s) balance failed", group.ID),
		)

		return
	}

	token := middleware.GetToken(c)

	quota := token.Quota
	if quota <= 0 {
		quota = groupQuota.Total
	} else {
		quota = min(quota, groupQuota.Total)
	}

	hlimit := decimal.NewFromFloat(quota).
		Add(decimal.NewFromFloat(token.UsedAmount)).
		InexactFloat64()

	c.JSON(http.StatusOK, openai.SubscriptionResponse{
		HardLimitUSD:       hlimit,
		SoftLimitUSD:       groupQuota.Remain,
		SystemHardLimitUSD: hlimit,
	})
}

// GetUsage godoc
//
//	@Summary		Get usage
//	@Description	Get usage
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	openai.UsageResponse
//	@Router			/v1/dashboard/billing/usage [get]
func GetUsage(c *gin.Context) {
	token := middleware.GetToken(c)
	c.JSON(
		http.StatusOK,
		openai.UsageResponse{
			TotalUsage: decimal.NewFromFloat(token.UsedAmount).
				Mul(decimal.NewFromFloat(100)).
				InexactFloat64(),
		},
	)
}

// GetTokenLogs godoc
//
//	@Summary		Get token logs
//	@Description	Get request logs for the current token (cursor-based pagination)
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			start_timestamp	query		int		true	"Start timestamp (milliseconds)"
//	@Param			end_timestamp	query		int		false	"End timestamp (milliseconds, default now)"
//	@Param			limit			query		int		false	"Items per page (default 20, max 50)"
//	@Param			after			query		int		false	"Cursor: return logs with id < after"
//	@Param			model_name		query		string	false	"Filter by model name"
//	@Param			request_id		query		string	false	"Filter by request ID"
//	@Param			code_type		query		string	false	"Status code type: success/error/all"
//	@Success		200				{object}	middleware.APIResponse{data=model.GetTokenLogsResult}
//	@Router			/v1/dashboard/logs [get]
func GetTokenLogs(c *gin.Context) {
	if c.Query("start_timestamp") == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "start_timestamp is required")
		return
	}

	startTime, endTime := utils.ParseTimeRange(c, 0)

	group := middleware.GetGroup(c)
	token := middleware.GetToken(c)

	limit, _ := strconv.Atoi(c.Query("limit"))
	afterID, _ := strconv.Atoi(c.Query("after"))
	modelName := c.Query("model_name")
	requestID := c.Query("request_id")
	codeType := c.Query("code_type")

	result, err := model.GetTokenLogs(
		group.ID,
		token.Name,
		startTime,
		endTime,
		modelName,
		requestID,
		model.CodeType(codeType),
		afterID,
		limit,
	)
	if err != nil {
		log.Errorf("get token logs failed for group %s token %s: %v", group.ID, token.Name, err)
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to get logs")
		return
	}

	middleware.SuccessResponse(c, result)
}

// GetTokenLogDetail godoc
//
//	@Summary		Get token log detail
//	@Description	Get request/response body for a specific log entry owned by the current token
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			log_id	path		int	true	"Log ID"
//	@Success		200		{object}	middleware.APIResponse{data=model.RequestDetail}
//	@Router			/v1/dashboard/logs/{log_id} [get]
func GetTokenLogDetail(c *gin.Context) {
	logID, _ := strconv.Atoi(c.Param("log_id"))
	if logID <= 0 {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid log_id")
		return
	}

	group := middleware.GetGroup(c)
	token := middleware.GetToken(c)

	detail, err := model.GetTokenLogDetail(logID, group.ID, token.Name)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, "log detail not found")
		return
	}

	middleware.SuccessResponse(c, detail)
}
