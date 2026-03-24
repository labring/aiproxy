package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

type GroupConsumptionRankingResponseItem struct {
	Rank int `json:"rank"`
	model.GroupConsumptionRankingItem
}

// GetGroupConsumptionRanking godoc
//
//	@Summary		Get group consumption ranking
//	@Description	Returns group consumption ranking aggregated from group summary data
//	@Tags			groups
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			start_timestamp	query		int64	false	"Start timestamp"
//	@Param			end_timestamp	query		int64	false	"End timestamp"
//	@Param			timezone		query		string	false	"Timezone, default is Local"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			order			query		string	false	"Order: used_amount_desc, used_amount_asc, request_count_desc, request_count_asc, total_tokens_desc, total_tokens_asc, group_id_asc, group_id_desc"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any}
//	@Router			/api/groups/ranking [get]
func GetGroupConsumptionRanking(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	startTime, endTime := utils.ParseTimeRange(c, 0)

	items, total, _, err := model.GetGroupConsumptionRanking(
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

	responseItems := make([]GroupConsumptionRankingResponseItem, len(items))
	for i, item := range items {
		responseItems[i] = GroupConsumptionRankingResponseItem{
			Rank:                        (page-1)*perPage + i + 1,
			GroupConsumptionRankingItem: item,
		}
	}

	middleware.SuccessResponse(c, gin.H{
		"items": responseItems,
		"total": total,
	})
}
