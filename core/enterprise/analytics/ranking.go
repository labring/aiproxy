//go:build enterprise

package analytics

import (
	"fmt"
	"sort"
	"time"

	"github.com/labring/aiproxy/core/enterprise/feishu"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
)

// UserRankingEntry holds a single user's ranking in usage.
type UserRankingEntry struct {
	Rank           int     `json:"rank"`
	GroupID        string  `json:"group_id"`
	UserName       string  `json:"user_name"`
	DepartmentID   string  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	UsedAmount     float64 `json:"used_amount"`
	RequestCount   int64   `json:"request_count"`
	TotalTokens    int64   `json:"total_tokens"`
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	SuccessRate    float64 `json:"success_rate"`
	UniqueModels   int     `json:"unique_models"`
}

// DepartmentRankingEntry holds a single department's ranking in usage.
type DepartmentRankingEntry struct {
	Rank           int     `json:"rank"`
	DepartmentID   string  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	ActiveUsers    int     `json:"active_users"`
	UsedAmount     float64 `json:"used_amount"`
	RequestCount   int64   `json:"request_count"`
	TotalTokens    int64   `json:"total_tokens"`
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
}

// GetUserRanking returns users ranked by usage amount within the given time range.
// Optionally filters by department. Limit controls how many results to return.
func GetUserRanking(startTime, endTime time.Time, departmentID string, limit int) ([]UserRankingEntry, error) {
	startTimestamp := startTime.Unix()
	endTimestamp := endTime.Unix()

	if limit < 0 {
		limit = 50
	}

	// Get feishu users (optionally filtered by department with descendant expansion).
	query := model.DB.Model(&models.FeishuUser{}).Select("group_id", "name", "department_id")
	if departmentID != "" {
		matchingDepts := feishu.GetDescendantDepartmentIDs(departmentID)
		if len(matchingDepts) == 0 {
			return []UserRankingEntry{}, nil
		}
		query = query.Where("department_id IN ?", matchingDepts)
	}

	var feishuUsers []models.FeishuUser
	if err := query.Find(&feishuUsers).Error; err != nil {
		return nil, fmt.Errorf("query feishu users: %w", err)
	}

	if len(feishuUsers) == 0 {
		return []UserRankingEntry{}, nil
	}

	// Deduplicate by group_id — multiple feishu users may share a group.
	type userInfo struct {
		Name         string
		DepartmentID string
	}
	groupUserMap := make(map[string]userInfo, len(feishuUsers))
	groupIDs := make([]string, 0, len(feishuUsers))
	for _, u := range feishuUsers {
		if _, exists := groupUserMap[u.GroupID]; !exists {
			groupUserMap[u.GroupID] = userInfo{Name: u.Name, DepartmentID: u.DepartmentID}
			groupIDs = append(groupIDs, u.GroupID)
		}
	}

	// Query aggregated usage from group_summaries by group_id
	type groupAgg struct {
		GroupID      string  `gorm:"column:group_id"`
		UsedAmount   float64 `gorm:"column:used_amount"`
		RequestCount int64   `gorm:"column:request_count"`
		TotalTokens  int64   `gorm:"column:total_tokens"`
		InputTokens  int64   `gorm:"column:input_tokens"`
		OutputTokens int64   `gorm:"column:output_tokens"`
		SuccessCount int64   `gorm:"column:success_count"`
		UniqueModels int     `gorm:"column:unique_models"`
	}

	var results []groupAgg

	err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"group_id",
			"SUM(used_amount) as used_amount",
			"SUM(request_count) as request_count",
			"SUM(total_tokens) as total_tokens",
			"SUM(input_tokens) as input_tokens",
			"SUM(output_tokens) as output_tokens",
			"SUM(status2xx_count) as success_count",
			"COUNT(DISTINCT model) as unique_models",
		).
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTimestamp, endTimestamp).
		Group("group_id").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query user ranking: %w", err)
	}

	// Build group_id → usage map
	usageMap := make(map[string]*groupAgg, len(results))
	for i := range results {
		usageMap[results[i].GroupID] = &results[i]
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

	// Build entries per unique group_id, including those with zero usage
	entries := make([]UserRankingEntry, 0, len(groupUserMap))
	for gid, info := range groupUserMap {
		entry := UserRankingEntry{
			GroupID:        gid,
			UserName:       info.Name,
			DepartmentID:   info.DepartmentID,
			DepartmentName: deptNameMap[info.DepartmentID],
		}

		if agg := usageMap[gid]; agg != nil {
			entry.UsedAmount = agg.UsedAmount
			entry.RequestCount = agg.RequestCount
			entry.TotalTokens = agg.TotalTokens
			entry.InputTokens = agg.InputTokens
			entry.OutputTokens = agg.OutputTokens
			entry.UniqueModels = agg.UniqueModels
			if agg.RequestCount > 0 {
				entry.SuccessRate = float64(agg.SuccessCount) / float64(agg.RequestCount) * 100.0
			}
		}

		entries = append(entries, entry)
	}

	// Sort: by used_amount DESC, then by user_name ASC for zero-usage users
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].UsedAmount != entries[j].UsedAmount {
			return entries[i].UsedAmount > entries[j].UsedAmount
		}
		return entries[i].UserName < entries[j].UserName
	})

	// Apply limit
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	// Assign ranks
	for i := range entries {
		entries[i].Rank = i + 1
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
			ActiveUsers:    s.ActiveUsers,
			UsedAmount:     s.UsedAmount,
			RequestCount:   s.RequestCount,
			TotalTokens:    s.TotalTokens,
			InputTokens:    s.InputTokens,
			OutputTokens:   s.OutputTokens,
		})
	}

	// Sort by used_amount descending
	sortByUsedAmountDesc(entries)

	if len(entries) > limit {
		entries = entries[:limit]
	}

	// Assign ranks
	for i := range entries {
		entries[i].Rank = i + 1
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
