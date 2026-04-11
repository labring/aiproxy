//go:build enterprise

package quota

import (
	"time"

	"github.com/labring/aiproxy/core/enterprise/models"
)

// PeriodStartByType returns the calendar-aligned start of the current period
// for a given policy PeriodType (1=daily, 2=weekly, 3=monthly).
func PeriodStartByType(periodType int) time.Time {
	now := time.Now()

	switch periodType {
	case models.PeriodTypeDaily:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case models.PeriodTypeWeekly:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location())
	default: // monthly or unknown
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
}
