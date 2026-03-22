//go:build enterprise

package analytics

import (
	"fmt"
	"time"

	"github.com/labring/aiproxy/core/enterprise/feishu"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
)

// ModelDistributionEntry holds usage data for a specific model.
type ModelDistributionEntry struct {
	Model        string  `json:"model"`
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	UsedAmount   float64 `json:"used_amount"`
	UniqueUsers  int     `json:"unique_users"`
	Percentage   float64 `json:"percentage"`
}

// GetModelDistribution returns model usage distribution within the given time range.
// Optionally filters by departments (supports multiple IDs with descendant expansion).
func GetModelDistribution(startTime, endTime time.Time, departmentIDs []string) ([]ModelDistributionEntry, error) {
	startTimestamp := startTime.Unix()
	endTimestamp := endTime.Unix()

	groupIDs, err := getGroupIDsForDepartments(departmentIDs)
	if err != nil {
		return nil, err
	}

	// Department filter requested but no matching users → empty result (not full scan).
	if len(departmentIDs) > 0 && len(groupIDs) == 0 {
		return []ModelDistributionEntry{}, nil
	}

	type modelAgg struct {
		Model        string  `gorm:"column:model"`
		RequestCount int64   `gorm:"column:request_count"`
		TotalTokens  int64   `gorm:"column:total_tokens"`
		InputTokens  int64   `gorm:"column:input_tokens"`
		OutputTokens int64   `gorm:"column:output_tokens"`
		UsedAmount   float64 `gorm:"column:used_amount"`
		UniqueUsers  int     `gorm:"column:unique_users"`
	}

	var results []modelAgg

	query := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"model",
			"SUM(request_count) as request_count",
			"SUM(total_tokens) as total_tokens",
			"SUM(input_tokens) as input_tokens",
			"SUM(output_tokens) as output_tokens",
			"SUM(used_amount) as used_amount",
			"COUNT(DISTINCT group_id) as unique_users",
		).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTimestamp, endTimestamp)

	if len(groupIDs) > 0 {
		query = query.Where("group_id IN ?", groupIDs)
	}

	if err := query.Group("model").Order("used_amount DESC").Find(&results).Error; err != nil {
		return nil, fmt.Errorf("query model distribution: %w", err)
	}

	// Calculate total amount for percentage
	var totalAmount float64
	for _, r := range results {
		totalAmount += r.UsedAmount
	}

	entries := make([]ModelDistributionEntry, 0, len(results))
	for _, r := range results {
		var pct float64
		if totalAmount > 0 {
			pct = r.UsedAmount / totalAmount * 100.0
		}
		entries = append(entries, ModelDistributionEntry{
			Model:        r.Model,
			RequestCount: r.RequestCount,
			TotalTokens:  r.TotalTokens,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			UsedAmount:   r.UsedAmount,
			UniqueUsers:  r.UniqueUsers,
			Percentage:   pct,
		})
	}

	return entries, nil
}

// expandDepartmentIDs expands department IDs to include all descendants and
// dual-ID forms (department_id / open_department_id) via GetDescendantDepartmentIDs.
func expandDepartmentIDs(departmentIDs []string) []string {
	seen := make(map[string]struct{})
	for _, id := range departmentIDs {
		for _, expanded := range feishu.GetDescendantDepartmentIDs(id) {
			seen[expanded] = struct{}{}
		}
	}

	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}

	return result
}

// getAllFeishuGroupIDs returns all distinct group_ids that have feishu user mappings.
// Used to scope enterprise analytics to feishu-synced users only.
func getAllFeishuGroupIDs() ([]string, error) {
	var groupIDs []string
	if err := model.DB.
		Model(&models.FeishuUser{}).
		Distinct("group_id").
		Pluck("group_id", &groupIDs).Error; err != nil {
		return nil, fmt.Errorf("query all feishu group ids: %w", err)
	}
	return groupIDs, nil
}

// getGroupIDsForDepartments returns group IDs for a set of department filters.
// Returns all feishu group IDs when departmentIDs is empty (scoped to enterprise users).
// Returns empty slice when departments are specified but no users match.
func getGroupIDsForDepartments(departmentIDs []string) ([]string, error) {
	if len(departmentIDs) == 0 {
		return getAllFeishuGroupIDs()
	}

	expanded := expandDepartmentIDs(departmentIDs)
	if len(expanded) == 0 {
		return []string{}, nil
	}

	var groupIDs []string
	if err := model.DB.
		Model(&models.FeishuUser{}).
		Distinct("group_id").
		Where("department_id IN ?", expanded).
		Pluck("group_id", &groupIDs).Error; err != nil {
		return nil, fmt.Errorf("query feishu users: %w", err)
	}

	return groupIDs, nil
}

