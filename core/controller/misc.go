package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
)

// IsEnterprise is set to true by the enterprise build tag.
// See misc_enterprise.go.
var IsEnterprise bool

type StatusData struct {
	StartTime    int64 `json:"startTime"`
	IsEnterprise bool  `json:"isEnterprise"`
}

// GetStatus godoc
//
//	@Summary		Get status
//	@Description	Returns the status of the server
//	@Tags			misc
//	@Produce		json
//	@Success		200	{object}	middleware.APIResponse{data=StatusData}
//	@Router			/api/status [get]
func GetStatus(c *gin.Context) {
	middleware.SuccessResponse(c, &StatusData{
		StartTime:    common.StartTime,
		IsEnterprise: IsEnterprise,
	})
}
