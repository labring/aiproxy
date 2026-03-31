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
	Level1DeptID   string  `json:"level1_dept_id"`
	Level2DeptID   string  `json:"level2_dept_id"`
	MemberCount    int     `json:"member_count"`
	ActiveUsers    int     `json:"active_users"`
	RequestCount   int64   `json:"request_count"`
	UsedAmount     float64 `json:"used_amount"`
	TotalTokens    int64   `json:"total_tokens"`
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	SuccessRate    float64 `json:"success_rate"`
	AvgCost        float64 `json:"avg_cost"`
	UniqueModels   int     `json:"unique_models"`
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

	// Get all feishu users to map group_id → department hierarchy
	var feishuUsers []models.FeishuUser
	if err := model.DB.Select("group_id", "department_id", "level1_dept_id", "level2_dept_id").Find(&feishuUsers).Error; err != nil {
		return nil, fmt.Errorf("query feishu users: %w", err)
	}

	type deptHierarchy struct {
		DepartmentID string
		Level1DeptID string
		Level2DeptID string
	}

	// Cache resolved hierarchy per department_id to avoid repeated parent-chain walks
	resolvedHierarchy := make(map[string]deptHierarchy)

	// Build group → department hierarchy mapping
	groupToHierarchy := make(map[string]deptHierarchy, len(feishuUsers))
	for _, u := range feishuUsers {
		if u.DepartmentID == "" {
			continue
		}

		if cached, ok := resolvedHierarchy[u.DepartmentID]; ok {
			groupToHierarchy[u.GroupID] = cached
			continue
		}

		h := deptHierarchy{
			DepartmentID: u.DepartmentID,
			Level1DeptID: u.Level1DeptID,
			Level2DeptID: u.Level2DeptID,
		}
		if h.Level1DeptID == "" {
			h.Level1DeptID, h.Level2DeptID = resolveHierarchyFromDeptMap(u.DepartmentID, deptMap)
		}
		resolvedHierarchy[u.DepartmentID] = h
		groupToHierarchy[u.GroupID] = h
	}

	if len(groupToHierarchy) == 0 {
		return []DepartmentSummary{}, nil
	}

	// Collect group IDs
	groupIDs := make([]string, 0, len(groupToHierarchy))
	for gid := range groupToHierarchy {
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
		SuccessCount int64   `gorm:"column:success_count"`
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
			"SUM(status2xx_count) as success_count",
		).
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTimestamp, endTimestamp).
		Group("group_id").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query group summaries: %w", err)
	}

	// Separate query for accurate per-department unique model counts (can't be derived from
	// per-group COUNT(DISTINCT model) without double-counting shared models across users).
	type groupModel struct {
		GroupID string `gorm:"column:group_id"`
		Model   string `gorm:"column:model"`
	}

	var gmPairs []groupModel
	if err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select("DISTINCT group_id, model").
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTimestamp, endTimestamp).
		Find(&gmPairs).Error; err != nil {
		return nil, fmt.Errorf("query group models: %w", err)
	}

	deptModelSets := make(map[string]map[string]struct{})
	for _, gm := range gmPairs {
		deptID := groupToHierarchy[gm.GroupID].DepartmentID
		if deptID == "" {
			continue
		}

		if deptModelSets[deptID] == nil {
			deptModelSets[deptID] = make(map[string]struct{})
		}

		deptModelSets[deptID][gm.Model] = struct{}{}
	}

	// Track active users (unique group_ids) per department
	deptActiveUsers := make(map[string]map[string]bool)

	// Aggregate by department
	deptAgg := make(map[string]*DepartmentSummary)
	deptSuccessCount := make(map[string]int64)

	for _, r := range results {
		deptID := groupToHierarchy[r.GroupID].DepartmentID
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

			hier := groupToHierarchy[r.GroupID]
			agg = &DepartmentSummary{
				DepartmentID:   deptID,
				DepartmentName: name,
				Level1DeptID:   hier.Level1DeptID,
				Level2DeptID:   hier.Level2DeptID,
				MemberCount:    memberCount,
			}
			deptAgg[deptID] = agg
			deptActiveUsers[deptID] = make(map[string]bool)
		}

		agg.RequestCount += r.RequestCount
		agg.UsedAmount += r.UsedAmount
		agg.TotalTokens += r.TotalTokens
		agg.InputTokens += r.InputTokens
		agg.OutputTokens += r.OutputTokens
		deptSuccessCount[deptID] += r.SuccessCount
		deptActiveUsers[deptID][r.GroupID] = true
	}

	summaries := make([]DepartmentSummary, 0, len(deptAgg))
	for deptID, v := range deptAgg {
		v.ActiveUsers = len(deptActiveUsers[deptID])
		v.UniqueModels = len(deptModelSets[deptID])

		if v.RequestCount > 0 {
			v.SuccessRate = float64(deptSuccessCount[deptID]) / float64(v.RequestCount) * 100.0
			v.AvgCost = v.UsedAmount / float64(v.RequestCount)
		}

		summaries = append(summaries, *v)
	}

	return summaries, nil
}

// GetDepartmentTrend returns hourly usage trend data for a specific department.
func GetDepartmentTrend(departmentID string, startTime, endTime time.Time) ([]DepartmentTrendPoint, error) {
	startTimestamp := startTime.Unix()
	endTimestamp := endTime.Unix()

	groupIDs, err := GetGroupIDsForDepartments([]string{departmentID})
	if err != nil {
		return nil, err
	}

	if len(groupIDs) == 0 {
		return []DepartmentTrendPoint{}, nil
	}

	var results []DepartmentTrendPoint

	err = model.LogDB.
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
		Limit(744). // ~31 days * 24 hours — safety cap
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query department trend: %w", err)
	}

	return results, nil
}

// resolveHierarchyFromDeptMap walks the parent chain in deptMap to find level-1 and level-2
// department IDs. Level-1 is a direct child of root (parent_id = "0" or ""),
// level-2 is a child of level-1.
func resolveHierarchyFromDeptMap(deptID string, deptMap map[string]*models.FeishuDepartment) (level1, level2 string) {
	// Collect ancestor chain: [self, parent, grandparent, ...]
	const maxDepth = 10
	chain := make([]string, 0, maxDepth)
	cur := deptID

	for i := 0; i < maxDepth; i++ {
		dept, ok := deptMap[cur]
		if !ok {
			break
		}

		chain = append(chain, cur)
		if dept.ParentID == "" || dept.ParentID == "0" {
			break // reached root
		}

		cur = dept.ParentID
	}

	// chain is [leaf, ..., level2, level1] where last element's parent is root
	if len(chain) == 0 {
		return "", ""
	}

	// Last element in chain is level-1 (its parent is root)
	level1 = chain[len(chain)-1]
	if len(chain) >= 2 {
		level2 = chain[len(chain)-2]
	}

	return level1, level2
}
