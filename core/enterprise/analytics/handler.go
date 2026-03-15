//go:build enterprise

package analytics

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
)

// HandleDepartmentSummary returns department-level aggregated usage data.
func HandleDepartmentSummary(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	summaries, err := GetDepartmentSummaries(startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"departments": summaries,
		"total":       len(summaries),
	})
}

// HandleDepartmentTrend returns hourly usage trend for a specific department.
func HandleDepartmentTrend(c *gin.Context) {
	departmentID := c.Param("id")
	if departmentID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "department id is required")
		return
	}

	startTime, endTime := parseTimeRange(c)

	trend, err := GetDepartmentTrend(departmentID, startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"department_id": departmentID,
		"trend":         trend,
	})
}

// HandleUserRanking returns users ranked by usage amount.
func HandleUserRanking(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	departmentID := c.Query("department_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	ranking, err := GetUserRanking(startTime, endTime, departmentID, limit)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"ranking": ranking,
		"total":   len(ranking),
	})
}

// HandleExport generates and returns an Excel report of department analytics.
func HandleExport(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	summaries, err := GetDepartmentSummaries(startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	ranking, err := GetUserRanking(startTime, endTime, "", 500)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	f, err := ExportDepartmentReport(summaries, ranking, startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	filename := fmt.Sprintf("enterprise_analytics_%s_%s.xlsx",
		startTime.Format("20060102"),
		endTime.Format("20060102"),
	)

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	if err := f.Write(c.Writer); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to write excel file")
	}
}

// parseTimeRange extracts start_timestamp and end_timestamp from query parameters.
// Defaults to the last 7 days if not provided.
func parseTimeRange(c *gin.Context) (time.Time, time.Time) {
	now := time.Now()
	endTime := now
	startTime := now.AddDate(0, 0, -7)

	if startStr := c.Query("start_timestamp"); startStr != "" {
		if ts, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			startTime = time.Unix(ts, 0)
		}
	}

	if endStr := c.Query("end_timestamp"); endStr != "" {
		if ts, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			endTime = time.Unix(ts, 0)
		}
	}

	return startTime, endTime
}
