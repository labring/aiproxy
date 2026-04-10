package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

type ConsumptionRankingResponseItem struct {
	Rank int `json:"rank"`
	model.ConsumptionRankingItem
}

func normalizeConsumptionRankingType(rankingType string) model.ConsumptionRankingType {
	switch model.ConsumptionRankingType(rankingType) {
	case model.ConsumptionRankingTypeChannel,
		model.ConsumptionRankingTypeModel,
		model.ConsumptionRankingTypeGroup:
		return model.ConsumptionRankingType(rankingType)
	default:
		return model.ConsumptionRankingTypeGroup
	}
}

// GetConsumptionRanking godoc
//
//	@Summary		Get consumption ranking
//	@Description	Returns channel, model, or group consumption ranking aggregated from summary data
//	@Tags			groups
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			type			query		string	false	"Ranking type: channel, model, group"	default(group)
//	@Param			start_timestamp	query		int64	false	"Start timestamp"
//	@Param			end_timestamp	query		int64	false	"End timestamp"
//	@Param			timezone		query		string	false	"Timezone, default is Local"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			order			query		string	false	"Order: used_amount_desc, used_amount_asc, request_count_desc, request_count_asc, total_tokens_desc, total_tokens_asc, channel_id_asc, channel_id_desc, model_asc, model_desc, group_id_asc, group_id_desc"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any}
//	@Router			/api/groups/consumption_ranking [get]
//	@Router			/api/groups/ranking [get]
func GetConsumptionRanking(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)

	timezone := c.DefaultQuery("timezone", "Local")
	if _, err := time.LoadLocation(timezone); err != nil {
		timezone = "Local"
	}

	startTime, endTime := utils.ParseTimeRange(c, -1)

	rankingType := normalizeConsumptionRankingType(
		c.DefaultQuery("type", string(model.ConsumptionRankingTypeGroup)),
	)

	items, total, _, err := model.GetConsumptionRanking(
		rankingType,
		startTime,
		endTime,
		page,
		perPage,
		c.Query("order"),
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	page, perPage = model.NormalizePageParams(page, perPage)

	responseItems := make([]ConsumptionRankingResponseItem, len(items))
	for i, item := range items {
		responseItems[i] = ConsumptionRankingResponseItem{
			Rank:                   (page-1)*perPage + i + 1,
			ConsumptionRankingItem: item,
		}
	}

	middleware.SuccessResponse(c, gin.H{
		"items":    responseItems,
		"total":    total,
		"type":     rankingType,
		"timezone": timezone,
	})
}
