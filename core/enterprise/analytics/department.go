//go:build enterprise

package analytics

import (
	"fmt"
	"time"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
)

// DepartmentSummary holds aggregated usage data for a department.
type DepartmentSummary struct {
	DepartmentID   string  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	MemberCount    int     `json:"member_count"`
	RequestCount   int64   `json:"request_count"`
	UsedAmount     float64 `json:"used_amount"`
	TotalTokens    int64   `json:"total_tokens"`
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
}

// DepartmentTrendPoint holds a single data point in a department's usage trend.
type DepartmentTrendPoint struct {
	HourTimestamp int64   `json:"hour_timestamp"`
	RequestCount  int64   `json:"request_count"`
	UsedAmount    float64 `json:"used_amount"`
	TotalTokens   int64   `json:"total_tokens"`
}

// GetDepartmentSummaries returns aggregated usage data for all departments
// within the given time range.
func GetDepartmentSummaries(startTime, endTime time.Time) ([]DepartmentSummary, error) {
	startTimestamp := startTime.Unix()
	endTimestamp := endTime.Unix()

	// Get all departments
	var departments []models.FeishuDepartment
	if err := model.DB.Find(&departments).Error; err != nil {
		return nil, fmt.Errorf("query departments: %w", err)
	}

	if len(departments) == 0 {
		return []DepartmentSummary{}, nil
	}

	// Build department map for lookups
	deptMap := make(map[string]*models.FeishuDepartment, len(departments))
	for i := range departments {
		deptMap[departments[i].DepartmentID] = &departments[i]
	}

	// Get all feishu users to map group_id → department_id
	var feishuUsers []models.FeishuUser
	if err := model.DB.Select("group_id", "department_id").Find(&feishuUsers).Error; err != nil {
		return nil, fmt.Errorf("query feishu users: %w", err)
	}

	// Build group → department mapping
	groupToDept := make(map[string]string, len(feishuUsers))
	for _, u := range feishuUsers {
		if u.DepartmentID != "" {
			groupToDept[u.GroupID] = u.DepartmentID
		}
	}

	if len(groupToDept) == 0 {
		return []DepartmentSummary{}, nil
	}

	// Collect group IDs
	groupIDs := make([]string, 0, len(groupToDept))
	for gid := range groupToDept {
		groupIDs = append(groupIDs, gid)
	}

	// Query group_summaries for these groups in the time range
	type groupAgg struct {
		GroupID      string  `gorm:"column:group_id"`
		RequestCount int64   `gorm:"column:request_count"`
		UsedAmount   float64 `gorm:"column:used_amount"`
		TotalTokens  int64   `gorm:"column:total_tokens"`
		InputTokens  int64   `gorm:"column:input_tokens"`
		OutputTokens int64   `gorm:"column:output_tokens"`
	}

	var results []groupAgg

	err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"group_id",
			"SUM(request_count) as request_count",
			"SUM(used_amount) as used_amount",
			"SUM(total_tokens) as total_tokens",
			"SUM(input_tokens) as input_tokens",
			"SUM(output_tokens) as output_tokens",
		).
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTimestamp, endTimestamp).
		Group("group_id").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query group summaries: %w", err)
	}

	// Aggregate by department
	deptAgg := make(map[string]*DepartmentSummary)
	for _, r := range results {
		deptID := groupToDept[r.GroupID]
		if deptID == "" {
			continue
		}

		agg, ok := deptAgg[deptID]
		if !ok {
			dept := deptMap[deptID]
			name := deptID
			memberCount := 0

			if dept != nil {
				name = dept.Name
				memberCount = dept.MemberCount
			}

			agg = &DepartmentSummary{
				DepartmentID:   deptID,
				DepartmentName: name,
				MemberCount:    memberCount,
			}
			deptAgg[deptID] = agg
		}

		agg.RequestCount += r.RequestCount
		agg.UsedAmount += r.UsedAmount
		agg.TotalTokens += r.TotalTokens
		agg.InputTokens += r.InputTokens
		agg.OutputTokens += r.OutputTokens
	}

	summaries := make([]DepartmentSummary, 0, len(deptAgg))
	for _, v := range deptAgg {
		summaries = append(summaries, *v)
	}

	return summaries, nil
}

// GetDepartmentTrend returns hourly usage trend data for a specific department.
func GetDepartmentTrend(departmentID string, startTime, endTime time.Time) ([]DepartmentTrendPoint, error) {
	startTimestamp := startTime.Unix()
	endTimestamp := endTime.Unix()

	// Get all feishu users in this department
	var feishuUsers []models.FeishuUser
	if err := model.DB.
		Select("group_id").
		Where("department_id = ?", departmentID).
		Find(&feishuUsers).Error; err != nil {
		return nil, fmt.Errorf("query feishu users: %w", err)
	}

	if len(feishuUsers) == 0 {
		return []DepartmentTrendPoint{}, nil
	}

	groupIDs := make([]string, 0, len(feishuUsers))
	for _, u := range feishuUsers {
		groupIDs = append(groupIDs, u.GroupID)
	}

	var results []DepartmentTrendPoint

	err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"hour_timestamp",
			"SUM(request_count) as request_count",
			"SUM(used_amount) as used_amount",
			"SUM(total_tokens) as total_tokens",
		).
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTimestamp, endTimestamp).
		Group("hour_timestamp").
		Order("hour_timestamp ASC").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query department trend: %w", err)
	}

	return results, nil
}
