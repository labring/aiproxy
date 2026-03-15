//go:build enterprise

package analytics

import (
	"fmt"
	"time"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
)

// UserRankingEntry holds a single user's ranking in usage.
type UserRankingEntry struct {
	GroupID      string  `json:"group_id"`
	TokenName    string  `json:"token_name"`
	UserName     string  `json:"user_name"`
	DepartmentID string  `json:"department_id"`
	UsedAmount   float64 `json:"used_amount"`
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
}

// DepartmentRankingEntry holds a single department's ranking in usage.
type DepartmentRankingEntry struct {
	DepartmentID   string  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	UsedAmount     float64 `json:"used_amount"`
	RequestCount   int64   `json:"request_count"`
	TotalTokens    int64   `json:"total_tokens"`
}

// GetUserRanking returns users ranked by usage amount within the given time range.
// Optionally filters by department. Limit controls how many results to return.
func GetUserRanking(startTime, endTime time.Time, departmentID string, limit int) ([]UserRankingEntry, error) {
	startTimestamp := startTime.Unix()
	endTimestamp := endTime.Unix()

	if limit <= 0 {
		limit = 50
	}

	// Get feishu users (optionally filtered by department)
	query := model.DB.Model(&models.FeishuUser{}).Select("group_id", "name", "department_id")
	if departmentID != "" {
		query = query.Where("department_id = ?", departmentID)
	}

	var feishuUsers []models.FeishuUser
	if err := query.Find(&feishuUsers).Error; err != nil {
		return nil, fmt.Errorf("query feishu users: %w", err)
	}

	if len(feishuUsers) == 0 {
		return []UserRankingEntry{}, nil
	}

	// Build group → user info mapping
	type userInfo struct {
		Name         string
		DepartmentID string
	}

	groupToUser := make(map[string]userInfo, len(feishuUsers))
	groupIDs := make([]string, 0, len(feishuUsers))

	for _, u := range feishuUsers {
		groupToUser[u.GroupID] = userInfo{
			Name:         u.Name,
			DepartmentID: u.DepartmentID,
		}
		groupIDs = append(groupIDs, u.GroupID)
	}

	// Query aggregated usage from group_summaries
	type groupAgg struct {
		GroupID      string  `gorm:"column:group_id"`
		UsedAmount   float64 `gorm:"column:used_amount"`
		RequestCount int64   `gorm:"column:request_count"`
		TotalTokens  int64   `gorm:"column:total_tokens"`
	}

	var results []groupAgg

	err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"group_id",
			"SUM(used_amount) as used_amount",
			"SUM(request_count) as request_count",
			"SUM(total_tokens) as total_tokens",
		).
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTimestamp, endTimestamp).
		Group("group_id").
		Order("used_amount DESC").
		Limit(limit).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query user ranking: %w", err)
	}

	entries := make([]UserRankingEntry, 0, len(results))
	for _, r := range results {
		ui := groupToUser[r.GroupID]
		entries = append(entries, UserRankingEntry{
			GroupID:      r.GroupID,
			UserName:     ui.Name,
			DepartmentID: ui.DepartmentID,
			UsedAmount:   r.UsedAmount,
			RequestCount: r.RequestCount,
			TotalTokens:  r.TotalTokens,
		})
	}

	return entries, nil
}

// GetDepartmentRanking returns departments ranked by usage amount within the given time range.
func GetDepartmentRanking(startTime, endTime time.Time, limit int) ([]DepartmentRankingEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	summaries, err := GetDepartmentSummaries(startTime, endTime)
	if err != nil {
		return nil, err
	}

	// Build department name map
	var departments []models.FeishuDepartment
	if err := model.DB.Find(&departments).Error; err != nil {
		return nil, fmt.Errorf("query departments: %w", err)
	}

	deptNameMap := make(map[string]string, len(departments))
	for _, d := range departments {
		deptNameMap[d.DepartmentID] = d.Name
	}

	entries := make([]DepartmentRankingEntry, 0, len(summaries))
	for _, s := range summaries {
		entries = append(entries, DepartmentRankingEntry{
			DepartmentID:   s.DepartmentID,
			DepartmentName: s.DepartmentName,
			UsedAmount:     s.UsedAmount,
			RequestCount:   s.RequestCount,
			TotalTokens:    s.TotalTokens,
		})
	}

	// Sort by used_amount descending
	sortByUsedAmountDesc(entries)

	if len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

func sortByUsedAmountDesc(entries []DepartmentRankingEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].UsedAmount > entries[j-1].UsedAmount; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}
