package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
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

	c.JSON(http.StatusOK, openai.SubscriptionResponse{
		HardLimitUSD:       quota + token.UsedAmount,
		SoftLimitUSD:       groupQuota.Remain,
		SystemHardLimitUSD: quota + token.UsedAmount,
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
	c.JSON(http.StatusOK, openai.UsageResponse{TotalUsage: token.UsedAmount * 100})
}
