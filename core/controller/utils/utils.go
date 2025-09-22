package utils

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func ParsePageParams(c *gin.Context) (page, perPage int) {
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = c.Query("p")
	}

	page, _ = strconv.Atoi(pageStr)
	perPage, _ = strconv.Atoi(c.Query("per_page"))

	return page, perPage
}

const (
	defaultMaxSpan = time.Hour * 24 * 7
)

func ParseTimeRange(c *gin.Context, maxSpan time.Duration) (startTime, endTime time.Time) {
	if maxSpan == 0 {
		maxSpan = defaultMaxSpan
	}

	startTime, _ = smartParseTimestamp(c.Query("start_timestamp"))
	endTime, _ = smartParseTimestamp(c.Query("end_timestamp"))

	if endTime.IsZero() {
		endTime = time.Now()
	}

	if maxSpan > 0 {
		sevenDaysAgo := endTime.Add(-maxSpan)
		if startTime.IsZero() || startTime.Before(sevenDaysAgo) {
			startTime = sevenDaysAgo
		}
	}

	return startTime, endTime
}

func smartParseTimestamp(timestampStr string) (time.Time, error) {
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	digits := len(timestampStr)

	switch {
	case digits <= 10:
		return time.Unix(timestamp, 0), nil
	case digits <= 13:
		return time.UnixMilli(timestamp), nil
	case digits <= 16:
		return time.UnixMicro(timestamp), nil
	default:
		return time.Unix(0, timestamp), nil
	}
}
