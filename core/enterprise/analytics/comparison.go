//go:build enterprise

package analytics

import (
	"fmt"
	"time"

	"github.com/labring/aiproxy/core/model"
)

// PeriodStats holds aggregated stats for a single period.
type PeriodStats struct {
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
	UsedAmount   float64 `json:"used_amount"`
	ActiveUsers  int     `json:"active_users"`
}

// PeriodChanges holds percentage changes between two periods.
type PeriodChanges struct {
	RequestCountPct float64 `json:"request_count_pct"`
	TotalTokensPct  float64 `json:"total_tokens_pct"`
	UsedAmountPct   float64 `json:"used_amount_pct"`
	ActiveUsersPct  float64 `json:"active_users_pct"`
}

// ComparisonData holds current vs previous period comparison.
type ComparisonData struct {
	PeriodType     string        `json:"period_type"`
	CurrentPeriod  PeriodStats   `json:"current_period"`
	PreviousPeriod PeriodStats   `json:"previous_period"`
	Changes        PeriodChanges `json:"changes"`
}

// GetPeriodComparison calculates period-over-period comparison.
// The previous period is computed by shifting startTime back by the same duration.
func GetPeriodComparison(
	startTime, endTime time.Time,
	departmentIDs []string,
) (*ComparisonData, error) {
	duration := endTime.Sub(startTime)
	prevStart := startTime.Add(-duration)
	prevEnd := startTime
	periodType := inferPeriodType(duration)

	groupIDs, err := GetGroupIDsForDepartments(departmentIDs)
	if err != nil {
		return nil, err
	}

	// Department filter requested but no matching users → return zero stats.
	if len(departmentIDs) > 0 && len(groupIDs) == 0 {
		return &ComparisonData{PeriodType: periodType}, nil
	}

	current, err := queryPeriodStats(startTime, endTime, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("query current period: %w", err)
	}

	previous, err := queryPeriodStats(prevStart, prevEnd, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("query previous period: %w", err)
	}

	result := &ComparisonData{
		PeriodType:     periodType,
		CurrentPeriod:  *current,
		PreviousPeriod: *previous,
		Changes: PeriodChanges{
			RequestCountPct: calcPctInt(current.RequestCount, previous.RequestCount),
			TotalTokensPct:  calcPctInt(current.TotalTokens, previous.TotalTokens),
			UsedAmountPct:   calcPctFloat(current.UsedAmount, previous.UsedAmount),
			ActiveUsersPct:  calcPctInt(int64(current.ActiveUsers), int64(previous.ActiveUsers)),
		},
	}

	return result, nil
}

// inferPeriodType returns a human-readable period label based on the duration.
func inferPeriodType(d time.Duration) string {
	hours := d.Hours()
	switch {
	case hours <= 25:
		return "daily"
	case hours <= 24*8:
		return "weekly"
	default:
		return "monthly"
	}
}

func queryPeriodStats(startTime, endTime time.Time, groupIDs []string) (*PeriodStats, error) {
	startTimestamp := startTime.Unix()
	endTimestamp := endTime.Unix()

	type periodAgg struct {
		RequestCount int64   `gorm:"column:request_count"`
		TotalTokens  int64   `gorm:"column:total_tokens"`
		UsedAmount   float64 `gorm:"column:used_amount"`
		ActiveUsers  int     `gorm:"column:active_users"`
	}

	var result periodAgg

	query := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"SUM(request_count) as request_count",
			"SUM(total_tokens) as total_tokens",
			"SUM(used_amount) as used_amount",
			"COUNT(DISTINCT group_id) as active_users",
		).
		Where("hour_timestamp >= ? AND hour_timestamp < ?", startTimestamp, endTimestamp)

	if len(groupIDs) > 0 {
		query = query.Where("group_id IN ?", groupIDs)
	}

	if err := query.Find(&result).Error; err != nil {
		return nil, err
	}

	return &PeriodStats{
		RequestCount: result.RequestCount,
		TotalTokens:  result.TotalTokens,
		UsedAmount:   result.UsedAmount,
		ActiveUsers:  result.ActiveUsers,
	}, nil
}

func calcPctInt(current, previous int64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}

	return float64(current-previous) / float64(previous) * 100.0
}

func calcPctFloat(current, previous float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}

	return (current - previous) / previous * 100.0
}
