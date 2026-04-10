package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
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

// GetHealth returns 200 if the server and its DB connection are healthy, 503 otherwise.
// Used by external health checkers (e.g., wireguard-health.sh) that need to verify
// the application can actually serve requests, not just that the process is running.
//
//	@Summary		Health check with DB ping
//	@Description	Returns 200 if DB is reachable, 503 otherwise
//	@Tags			misc
//	@Produce		json
//	@Success		200	{object}	middleware.APIResponse{data=StatusData}
//	@Failure		503	{object}	middleware.APIResponse
//	@Router			/api/health [get]
func GetHealth(c *gin.Context) {
	sqlDB, err := model.DB.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, middleware.APIResponse{
			Success: false,
			Message: "db connection unavailable",
		})
		return
	}
	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, middleware.APIResponse{
			Success: false,
			Message: "db ping failed",
		})
		return
	}
	middleware.SuccessResponse(c, &StatusData{
		StartTime:    common.StartTime,
		IsEnterprise: IsEnterprise,
	})
}
