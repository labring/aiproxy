//go:build enterprise

package analytics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
)

// CustomReportRequest defines the request body for custom report generation.
type CustomReportRequest struct {
	Dimensions []string           `json:"dimensions"`
	Measures   []string           `json:"measures"`
	Filters    CustomReportFilter `json:"filters"`
	TimeRange  TimeRangeParam     `json:"time_range"`
	SortBy     string             `json:"sort_by"`
	SortOrder  string             `json:"sort_order"`
	Limit      int                `json:"limit"`
}

type CustomReportFilter struct {
	DepartmentIDs []string `json:"department_ids"`
	Models        []string `json:"models"`
	UserNames     []string `json:"user_names"`
}

type TimeRangeParam struct {
	StartTimestamp int64 `json:"start_timestamp"`
	EndTimestamp   int64 `json:"end_timestamp"`
}

// ColumnDef describes a column in the result.
type ColumnDef struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"` // "dimension", "measure", "computed"
}

// CustomReportResponse is the API response for custom reports.
type CustomReportResponse struct {
	Columns []ColumnDef              `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Total   int                      `json:"total"`
}

// baseMeasures maps measure names to their SQL aggregation expressions.
var baseMeasures = map[string]string{
	"request_count":         "SUM(request_count)",
	"retry_count":           "SUM(retry_count)",
	"exception_count":       "SUM(exception_count)",
	"status_2xx":            "SUM(status2xx_count)",
	"status_4xx":            "SUM(status4xx_count)",
	"status_5xx":            "SUM(status5xx_count)",
	"status_429":            "SUM(status429_count)",
	"cache_hit_count":       "SUM(cache_hit_count)",
	"input_tokens":          "SUM(input_tokens)",
	"output_tokens":         "SUM(output_tokens)",
	"total_tokens":          "SUM(total_tokens)",
	"cached_tokens":         "SUM(cached_tokens)",
	"image_input_tokens":    "SUM(image_input_tokens)",
	"audio_input_tokens":    "SUM(audio_input_tokens)",
	"web_search_count":      "SUM(web_search_count)",
	"used_amount":           "SUM(used_amount)",
	"input_amount":          "SUM(input_amount)",
	"output_amount":         "SUM(output_amount)",
	"cached_amount":         "SUM(cached_amount)",
	"total_time_ms":         "SUM(total_time_milliseconds)",
	"total_ttfb_ms":         "SUM(total_ttfb_milliseconds)",
	"unique_models":         "COUNT(DISTINCT model)",
	"active_users":          "COUNT(DISTINCT group_id)",
	"image_output_tokens":   "SUM(image_output_tokens)",
	"cache_creation_tokens": "SUM(cache_creation_tokens)",
}

// computedMeasures lists measures that are derived from base measures.
var computedMeasures = map[string][]string{
	"success_rate":        {"status_2xx", "request_count"},
	"error_rate":          {"status_4xx", "status_5xx", "request_count"},
	"throttle_rate":       {"status_429", "request_count"},
	"cache_hit_rate":      {"cache_hit_count", "request_count"},
	"avg_tokens_per_req":  {"total_tokens", "request_count"},
	"avg_cost_per_req":    {"used_amount", "request_count"},
	"avg_latency":         {"total_time_ms", "request_count"},
	"avg_ttfb":            {"total_ttfb_ms", "request_count"},
	"output_input_ratio":  {"output_tokens", "input_tokens"},
	"cost_per_1k_tokens":  {"used_amount", "total_tokens"},
	"retry_rate":             {"retry_count", "request_count"},
	"reconciliation_tokens":  {"input_tokens", "output_tokens", "cached_tokens", "cache_creation_tokens"},
}

// measureLabels provides human-readable labels for measures.
var measureLabels = map[string]string{
	"request_count":         "请求数",
	"retry_count":           "重试次数",
	"exception_count":       "异常次数",
	"status_2xx":            "成功请求数",
	"status_4xx":            "客户端错误数",
	"status_5xx":            "服务端错误数",
	"status_429":            "限流请求数",
	"cache_hit_count":       "缓存命中数",
	"input_tokens":          "输入 Token",
	"output_tokens":         "输出 Token",
	"total_tokens":          "总 Token",
	"cached_tokens":         "缓存 Token",
	"image_input_tokens":    "图片输入 Token",
	"audio_input_tokens":    "音频输入 Token",
	"web_search_count":      "联网搜索次数",
	"used_amount":           "总费用",
	"input_amount":          "输入费用",
	"output_amount":         "输出费用",
	"cached_amount":         "缓存费用",
	"total_time_ms":         "总耗时(ms)",
	"total_ttfb_ms":         "总首Token耗时(ms)",
	"unique_models":         "使用模型数",
	"active_users":          "活跃用户数",
	"success_rate":          "成功率 (%)",
	"error_rate":            "错误率 (%)",
	"throttle_rate":         "限流率 (%)",
	"cache_hit_rate":        "缓存命中率 (%)",
	"avg_tokens_per_req":    "平均每请求 Token",
	"avg_cost_per_req":      "平均单次费用",
	"avg_latency":           "平均响应时间 (ms)",
	"avg_ttfb":              "平均首Token时间 (ms)",
	"output_input_ratio":    "输出/输入比",
	"cost_per_1k_tokens":    "千Token成本",
	"retry_rate":            "重试率 (%)",
	"image_output_tokens":   "图片输出 Token",
	"cache_creation_tokens": "缓存创建 Token",
	"reconciliation_tokens": "对账 Token (不含缓存)",
}

// dimensionLabels provides human-readable labels for dimensions.
var dimensionLabels = map[string]string{
	"user_name":          "用户名",
	"department":         "部门",
	"level1_department":  "一级部门",
	"level2_department":  "二级部门",
	"model":              "模型",
	"time_hour":          "小时",
	"time_day":           "天",
	"time_week":          "周",
}

// validDimensions lists all allowed dimension names.
var validDimensions = map[string]bool{
	"user_name":         true,
	"department":        true,
	"level1_department": true,
	"level2_department": true,
	"model":             true,
	"time_hour":         true,
	"time_day":          true,
	"time_week":         true,
}

// GenerateCustomReport executes the custom report query and returns results.
func GenerateCustomReport(req CustomReportRequest) (*CustomReportResponse, error) {
	if len(req.Dimensions) == 0 {
		return nil, fmt.Errorf("at least one dimension is required")
	}

	if len(req.Measures) == 0 {
		return nil, fmt.Errorf("at least one measure is required")
	}

	// Validate dimensions
	for _, d := range req.Dimensions {
		if !validDimensions[d] {
			return nil, fmt.Errorf("invalid dimension: %s", d)
		}
	}

	// Validate measures
	for _, m := range req.Measures {
		if _, ok := baseMeasures[m]; !ok {
			if _, ok := computedMeasures[m]; !ok {
				return nil, fmt.Errorf("invalid measure: %s", m)
			}
		}
	}

	// Determine required base measures (including dependencies of computed measures)
	requiredBase := resolveRequiredBaseMeasures(req.Measures)

	// Determine if we need user/department info
	needUserMapping := dimensionOrFilterNeedsUsers(req)

	// Load user and department mappings if needed
	groupToUser, deptNameMap, err := loadMappings(needUserMapping, req.Filters)
	if err != nil {
		return nil, err
	}

	// Determine which group_ids to query
	groupIDs, hasGroupFilter := resolveGroupIDs(groupToUser, req.Filters, needUserMapping)

	// Filter was active but no matching users → return empty result (not full scan).
	if hasGroupFilter && len(groupIDs) == 0 {
		return &CustomReportResponse{
			Columns: buildColumns(req),
			Rows:    []map[string]interface{}{},
			Total:   0,
		}, nil
	}

	// Build and execute the SQL query
	rows, err := executeQuery(req, requiredBase, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("query custom report: %w", err)
	}

	// Post-process: map group_id to user/department, compute derived fields
	result := postProcess(rows, req, groupToUser, deptNameMap)

	// Sort results — always apply a deterministic fallback sort so that
	// repeated queries with identical parameters return rows in the same order.
	sortResults(result, req.SortBy, req.SortOrder, req.Dimensions)

	// Apply limit (default cap: 1000 rows)
	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}

	if len(result) > limit {
		result = result[:limit]
	}

	// Build columns
	columns := buildColumns(req)

	return &CustomReportResponse{
		Columns: columns,
		Rows:    result,
		Total:   len(result),
	}, nil
}

// resolveRequiredBaseMeasures collects all base measures needed, including dependencies of computed measures.
func resolveRequiredBaseMeasures(measures []string) map[string]bool {
	required := make(map[string]bool)

	for _, m := range measures {
		if _, ok := baseMeasures[m]; ok {
			required[m] = true
		} else if deps, ok := computedMeasures[m]; ok {
			for _, dep := range deps {
				required[dep] = true
			}
		}
	}

	return required
}

func dimensionOrFilterNeedsUsers(req CustomReportRequest) bool {
	for _, d := range req.Dimensions {
		switch d {
		case "user_name", "department", "level1_department", "level2_department":
			return true
		}
	}

	return len(req.Filters.DepartmentIDs) > 0 || len(req.Filters.UserNames) > 0
}

type userMapping struct {
	Name           string
	DepartmentID   string
	Level1DeptName string
	Level2DeptName string
}

func loadMappings(needUsers bool, filters CustomReportFilter) (
	map[string]userMapping, map[string]string, error,
) {
	if !needUsers {
		return nil, nil, nil
	}

	// Load feishu users
	query := model.DB.Model(&models.FeishuUser{}).Select(
		"group_id", "name", "department_id",
		"level1_dept_id", "level1_dept_name",
		"level2_dept_id", "level2_dept_name",
	)

	if len(filters.DepartmentIDs) > 0 {
		expanded := expandDepartmentIDs(filters.DepartmentIDs)
		if len(expanded) > 0 {
			query = query.Where("department_id IN ?", expanded)
		}
	}

	var feishuUsers []models.FeishuUser
	if err := query.Find(&feishuUsers).Error; err != nil {
		return nil, nil, fmt.Errorf("query feishu users: %w", err)
	}

	// Load all departments (needed for name resolution and hierarchy)
	var departments []models.FeishuDepartment
	if err := model.DB.Find(&departments).Error; err != nil {
		return nil, nil, fmt.Errorf("query departments: %w", err)
	}

	// Build department lookup maps for hierarchy resolution
	deptByID := make(map[string]*models.FeishuDepartment, len(departments))
	for i := range departments {
		d := &departments[i]
		deptByID[d.DepartmentID] = d
		if d.OpenDepartmentID != "" {
			deptByID[d.OpenDepartmentID] = d
		}
	}

	// computeDeptHierarchy resolves level1/level2 names from department parent chain
	computeDeptHierarchy := func(departmentID string) (l1Name, l2Name string) {
		var chain []string
		currentID := departmentID
		for i := 0; i < 10 && currentID != "" && currentID != "0"; i++ {
			dept, ok := deptByID[currentID]
			if !ok {
				break
			}
			name := dept.Name
			if name == "" {
				name = dept.DepartmentID
			}
			chain = append(chain, name)
			currentID = dept.ParentID
		}
		// chain is leaf-to-root; reverse to get root-to-leaf
		for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
			chain[i], chain[j] = chain[j], chain[i]
		}
		if len(chain) >= 1 {
			l1Name = chain[0]
		}
		if len(chain) >= 2 {
			l2Name = chain[1]
		}
		return
	}

	groupToUser := make(map[string]userMapping, len(feishuUsers))
	for _, u := range feishuUsers {
		l1Name := u.Level1DeptName
		l2Name := u.Level2DeptName

		// Resolve from already-loaded department map if stored name is empty but ID exists
		if l1Name == "" && u.Level1DeptID != "" {
			if d, ok := deptByID[u.Level1DeptID]; ok {
				l1Name = d.Name
			}
		}
		if l2Name == "" && u.Level2DeptID != "" {
			if d, ok := deptByID[u.Level2DeptID]; ok {
				l2Name = d.Name
			}
		}

		// If still empty, compute from department hierarchy
		if l1Name == "" || l2Name == "" {
			cl1, cl2 := computeDeptHierarchy(u.DepartmentID)
			if l1Name == "" {
				l1Name = cl1
			}
			if l2Name == "" {
				l2Name = cl2
			}
		}

		groupToUser[u.GroupID] = userMapping{
			Name:           u.Name,
			DepartmentID:   u.DepartmentID,
			Level1DeptName: l1Name,
			Level2DeptName: l2Name,
		}
	}

	// Filter by user names if specified
	if len(filters.UserNames) > 0 {
		nameSet := make(map[string]bool, len(filters.UserNames))
		for _, n := range filters.UserNames {
			nameSet[n] = true
		}

		for gid, um := range groupToUser {
			if !nameSet[um.Name] {
				delete(groupToUser, gid)
			}
		}
	}

	deptNameMap := make(map[string]string, len(departments))
	for _, d := range departments {
		deptNameMap[d.DepartmentID] = d.Name
	}

	return groupToUser, deptNameMap, nil
}


// resolveGroupIDs returns (groupIDs, hasFilter).
// hasFilter=true means a department or user filter was active, so empty groupIDs means "no results".
// hasFilter=false means no restriction — groupIDs is nil.
func resolveGroupIDs(
	groupToUser map[string]userMapping,
	filters CustomReportFilter,
	needUserMapping bool,
) ([]string, bool) {
	if !needUserMapping {
		return nil, false
	}

	ids := make([]string, 0, len(groupToUser))
	for gid := range groupToUser {
		ids = append(ids, gid)
	}

	hasFilter := len(filters.DepartmentIDs) > 0 || len(filters.UserNames) > 0
	return ids, hasFilter
}

// rawRow holds a single row from the SQL aggregation query.
type rawRow struct {
	GroupID       string  `gorm:"column:group_id"`
	Model         string  `gorm:"column:model"`
	TimeKey       int64   `gorm:"column:time_key"`
	RequestCount  int64   `gorm:"column:request_count"`
	RetryCount    int64   `gorm:"column:retry_count"`
	ExceptionCnt  int64   `gorm:"column:exception_count"`
	Status2xx     int64   `gorm:"column:status_2xx"`
	Status4xx     int64   `gorm:"column:status_4xx"`
	Status5xx     int64   `gorm:"column:status_5xx"`
	Status429     int64   `gorm:"column:status_429"`
	CacheHitCnt   int64   `gorm:"column:cache_hit_count"`
	InputTokens   int64   `gorm:"column:input_tokens"`
	OutputTokens  int64   `gorm:"column:output_tokens"`
	TotalTokens   int64   `gorm:"column:total_tokens"`
	CachedTokens  int64   `gorm:"column:cached_tokens"`
	ImgInTokens   int64   `gorm:"column:image_input_tokens"`
	AudioInTokens int64   `gorm:"column:audio_input_tokens"`
	WebSearchCnt  int64   `gorm:"column:web_search_count"`
	UsedAmount    float64 `gorm:"column:used_amount"`
	InputAmount   float64 `gorm:"column:input_amount"`
	OutputAmount  float64 `gorm:"column:output_amount"`
	CachedAmount  float64 `gorm:"column:cached_amount"`
	TotalTimeMs   int64   `gorm:"column:total_time_ms"`
	TotalTtfbMs   int64   `gorm:"column:total_ttfb_ms"`
	UniqueModels  int64   `gorm:"column:unique_models"`
	ActiveUsers   int64   `gorm:"column:active_users"`
	ImgOutTokens  int64   `gorm:"column:image_output_tokens"`
	CacheCrTokens int64   `gorm:"column:cache_creation_tokens"`
}

func executeQuery(
	req CustomReportRequest,
	requiredBase map[string]bool,
	groupIDs []string,
) ([]rawRow, error) {
	// Build SELECT clause
	selectParts := buildSelectParts(req.Dimensions, requiredBase)

	// Build GROUP BY clause
	groupByParts := buildGroupByParts(req.Dimensions)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := model.LogDB.WithContext(ctx).Model(&model.GroupSummary{})

	// Apply time range filter
	if req.TimeRange.StartTimestamp > 0 {
		query = query.Where("hour_timestamp >= ?", req.TimeRange.StartTimestamp)
	}

	if req.TimeRange.EndTimestamp > 0 {
		query = query.Where("hour_timestamp <= ?", req.TimeRange.EndTimestamp)
	}

	// Apply group_id filter
	if len(groupIDs) > 0 {
		query = query.Where("group_id IN ?", groupIDs)
	}

	// Apply model filter
	if len(req.Filters.Models) > 0 {
		query = query.Where("model IN ?", req.Filters.Models)
	}

	var results []rawRow

	err := query.
		Select(strings.Join(selectParts, ", ")).
		Group(strings.Join(groupByParts, ", ")).
		Order(strings.Join(groupByParts, ", ")).
		Find(&results).Error

	return results, err
}

func buildSelectParts(dimensions []string, requiredBase map[string]bool) []string {
	parts := make([]string, 0, 20)

	// Always include group_id for user/department resolution
	hasGroupDim := false
	hasModelDim := false
	hasTimeDim := false
	timeGranularity := ""

	for _, d := range dimensions {
		switch d {
		case "user_name", "department", "level1_department", "level2_department":
			hasGroupDim = true
		case "model":
			hasModelDim = true
		case "time_hour", "time_day", "time_week":
			hasTimeDim = true
			timeGranularity = d
		}
	}

	if hasGroupDim {
		parts = append(parts, "group_id")
	}

	if hasModelDim {
		parts = append(parts, "model")
	}

	if hasTimeDim {
		switch timeGranularity {
		case "time_hour":
			parts = append(parts, "hour_timestamp as time_key")
		case "time_day":
			parts = append(parts, "(hour_timestamp / 86400 * 86400) as time_key")
		case "time_week":
			// Align to Monday (Unix epoch was Thursday, offset by 3 days)
			parts = append(parts, "((hour_timestamp - 259200) / 604800 * 604800 + 259200) as time_key")
		}
	}

	// Add aggregation columns based on required measures
	aggMap := map[string]string{
		"request_count":         "SUM(request_count) as request_count",
		"retry_count":           "SUM(retry_count) as retry_count",
		"exception_count":       "SUM(exception_count) as exception_count",
		"status_2xx":            "SUM(status2xx_count) as status_2xx",
		"status_4xx":            "SUM(status4xx_count) as status_4xx",
		"status_5xx":            "SUM(status5xx_count) as status_5xx",
		"status_429":            "SUM(status429_count) as status_429",
		"cache_hit_count":       "SUM(cache_hit_count) as cache_hit_count",
		"input_tokens":          "SUM(input_tokens) as input_tokens",
		"output_tokens":         "SUM(output_tokens) as output_tokens",
		"total_tokens":          "SUM(total_tokens) as total_tokens",
		"cached_tokens":         "SUM(cached_tokens) as cached_tokens",
		"image_input_tokens":    "SUM(image_input_tokens) as image_input_tokens",
		"audio_input_tokens":    "SUM(audio_input_tokens) as audio_input_tokens",
		"web_search_count":      "SUM(web_search_count) as web_search_count",
		"used_amount":           "SUM(used_amount) as used_amount",
		"input_amount":          "SUM(input_amount) as input_amount",
		"output_amount":         "SUM(output_amount) as output_amount",
		"cached_amount":         "SUM(cached_amount) as cached_amount",
		"total_time_ms":         "SUM(total_time_milliseconds) as total_time_ms",
		"total_ttfb_ms":         "SUM(total_ttfb_milliseconds) as total_ttfb_ms",
		"unique_models":         "COUNT(DISTINCT model) as unique_models",
		"active_users":          "COUNT(DISTINCT group_id) as active_users",
		"image_output_tokens":   "SUM(image_output_tokens) as image_output_tokens",
		"cache_creation_tokens": "SUM(cache_creation_tokens) as cache_creation_tokens",
	}

	added := make(map[string]bool)
	for measure := range requiredBase {
		if expr, ok := aggMap[measure]; ok && !added[measure] {
			parts = append(parts, expr)
			added[measure] = true
		}
	}

	return parts
}

func buildGroupByParts(dimensions []string) []string {
	parts := make([]string, 0, 3)

	for _, d := range dimensions {
		switch d {
		case "user_name", "department", "level1_department", "level2_department":
			if !containsStr(parts, "group_id") {
				parts = append(parts, "group_id")
			}
		case "model":
			parts = append(parts, "model")
		case "time_hour":
			parts = append(parts, "hour_timestamp")
		case "time_day":
			parts = append(parts, "(hour_timestamp / 86400 * 86400)")
		case "time_week":
			parts = append(parts, "((hour_timestamp - 259200) / 604800 * 604800 + 259200)")
		}
	}

	return parts
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}

	return false
}

func postProcess(
	rows []rawRow,
	req CustomReportRequest,
	groupToUser map[string]userMapping,
	deptNameMap map[string]string,
) []map[string]interface{} {
	// Check which dimensions and measures are requested
	hasDeptDim := false
	hasUserDim := false
	hasLevel1Dept := false
	hasLevel2Dept := false

	for _, d := range req.Dimensions {
		switch d {
		case "department":
			hasDeptDim = true
		case "level1_department":
			hasLevel1Dept = true
		case "level2_department":
			hasLevel2Dept = true
		case "user_name":
			hasUserDim = true
		}
	}

	// If any department-level dimension is present (without user), aggregate by department
	if (hasDeptDim || hasLevel1Dept || hasLevel2Dept) && !hasUserDim {
		return aggregateByDepartment(rows, req, groupToUser, deptNameMap)
	}

	result := make([]map[string]interface{}, 0, len(rows))

	for _, r := range rows {
		row := make(map[string]interface{})

		// Fill dimension values
		for _, d := range req.Dimensions {
			switch d {
			case "user_name":
				if um, ok := groupToUser[r.GroupID]; ok {
					row["user_name"] = um.Name
				} else {
					row["user_name"] = r.GroupID
				}
			case "department":
				if um, ok := groupToUser[r.GroupID]; ok {
					row["department"] = deptNameMap[um.DepartmentID]
				} else {
					row["department"] = ""
				}
			case "level1_department":
				if um, ok := groupToUser[r.GroupID]; ok {
					row["level1_department"] = um.Level1DeptName
				} else {
					row["level1_department"] = ""
				}
			case "level2_department":
				if um, ok := groupToUser[r.GroupID]; ok {
					row["level2_department"] = um.Level2DeptName
				} else {
					row["level2_department"] = ""
				}
			case "model":
				row["model"] = r.Model
			case "time_hour", "time_day", "time_week":
				row[d] = r.TimeKey
			}
		}

		// Fill base measures
		fillBaseMeasures(row, r, req.Measures)

		// Compute derived measures
		computeDerivedMeasures(row, r, req.Measures)

		result = append(result, row)
	}

	return result
}

func aggregateByDepartment(
	rows []rawRow,
	req CustomReportRequest,
	groupToUser map[string]userMapping,
	deptNameMap map[string]string,
) []map[string]interface{} {
	// Build a composite key for aggregation
	type aggKey struct {
		DeptName string
		Model    string
		TimeKey  int64
	}

	aggMap := make(map[aggKey]*rawRow)

	hasModel := false
	hasTime := false
	hasDept := false
	hasLevel1 := false
	hasLevel2 := false
	timeDim := ""

	for _, d := range req.Dimensions {
		switch d {
		case "department":
			hasDept = true
		case "level1_department":
			hasLevel1 = true
		case "level2_department":
			hasLevel2 = true
		case "model":
			hasModel = true
		case "time_hour", "time_day", "time_week":
			hasTime = true
			timeDim = d
		}
	}

	for i := range rows {
		r := &rows[i]
		deptName := ""

		if um, ok := groupToUser[r.GroupID]; ok {
			switch {
			case hasLevel1:
				deptName = um.Level1DeptName
			case hasLevel2:
				deptName = um.Level2DeptName
			default:
				deptName = deptNameMap[um.DepartmentID]
			}
		}

		key := aggKey{DeptName: deptName}
		if hasModel {
			key.Model = r.Model
		}

		if hasTime {
			key.TimeKey = r.TimeKey
		}

		if existing, ok := aggMap[key]; ok {
			mergeRawRows(existing, r)
		} else {
			clone := *r
			aggMap[key] = &clone
		}
	}

	result := make([]map[string]interface{}, 0, len(aggMap))

	// Determine which department dimension key to use in the output row
	deptDimKey := "department"
	if hasLevel1 {
		deptDimKey = "level1_department"
	} else if hasLevel2 {
		deptDimKey = "level2_department"
	}
	_ = hasDept // default fallback

	for key, r := range aggMap {
		row := make(map[string]interface{})
		row[deptDimKey] = key.DeptName

		if hasModel {
			row["model"] = key.Model
		}

		if hasTime {
			row[timeDim] = key.TimeKey
		}

		fillBaseMeasures(row, *r, req.Measures)
		computeDerivedMeasures(row, *r, req.Measures)
		result = append(result, row)
	}

	return result
}

func mergeRawRows(dst, src *rawRow) {
	dst.RequestCount += src.RequestCount
	dst.RetryCount += src.RetryCount
	dst.ExceptionCnt += src.ExceptionCnt
	dst.Status2xx += src.Status2xx
	dst.Status4xx += src.Status4xx
	dst.Status5xx += src.Status5xx
	dst.Status429 += src.Status429
	dst.CacheHitCnt += src.CacheHitCnt
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.TotalTokens += src.TotalTokens
	dst.CachedTokens += src.CachedTokens
	dst.ImgInTokens += src.ImgInTokens
	dst.AudioInTokens += src.AudioInTokens
	dst.WebSearchCnt += src.WebSearchCnt
	dst.UsedAmount += src.UsedAmount
	dst.InputAmount += src.InputAmount
	dst.OutputAmount += src.OutputAmount
	dst.CachedAmount += src.CachedAmount
	dst.TotalTimeMs += src.TotalTimeMs
	dst.TotalTtfbMs += src.TotalTtfbMs
	// active_users: SQL GROUP BY group_id ensures each rawRow has exactly one group_id,
	// so COUNT(DISTINCT group_id) = 1 per row. Summing gives the exact active user count
	// per department bucket.
	dst.ActiveUsers += src.ActiveUsers
	// unique_models: when "model" is not a dimension, this is COUNT(DISTINCT model) per group.
	// Summing across groups is an upper-bound approximation (models shared between groups are
	// double-counted). Exact counts would require a separate SQL query.
	dst.UniqueModels += src.UniqueModels
	dst.ImgOutTokens += src.ImgOutTokens
	dst.CacheCrTokens += src.CacheCrTokens
}

func fillBaseMeasures(row map[string]interface{}, r rawRow, measures []string) {
	for _, m := range measures {
		switch m {
		case "request_count":
			row[m] = r.RequestCount
		case "retry_count":
			row[m] = r.RetryCount
		case "exception_count":
			row[m] = r.ExceptionCnt
		case "status_2xx":
			row[m] = r.Status2xx
		case "status_4xx":
			row[m] = r.Status4xx
		case "status_5xx":
			row[m] = r.Status5xx
		case "status_429":
			row[m] = r.Status429
		case "cache_hit_count":
			row[m] = r.CacheHitCnt
		case "input_tokens":
			row[m] = r.InputTokens
		case "output_tokens":
			row[m] = r.OutputTokens
		case "total_tokens":
			row[m] = r.TotalTokens
		case "cached_tokens":
			row[m] = r.CachedTokens
		case "image_input_tokens":
			row[m] = r.ImgInTokens
		case "audio_input_tokens":
			row[m] = r.AudioInTokens
		case "web_search_count":
			row[m] = r.WebSearchCnt
		case "used_amount":
			row[m] = r.UsedAmount
		case "input_amount":
			row[m] = r.InputAmount
		case "output_amount":
			row[m] = r.OutputAmount
		case "cached_amount":
			row[m] = r.CachedAmount
		case "total_time_ms":
			row[m] = r.TotalTimeMs
		case "total_ttfb_ms":
			row[m] = r.TotalTtfbMs
		case "unique_models":
			row[m] = r.UniqueModels
		case "active_users":
			row[m] = r.ActiveUsers
		case "image_output_tokens":
			row[m] = r.ImgOutTokens
		case "cache_creation_tokens":
			row[m] = r.CacheCrTokens
		}
	}
}

func computeDerivedMeasures(row map[string]interface{}, r rawRow, measures []string) {
	for _, m := range measures {
		switch m {
		case "success_rate":
			row[m] = safePercent(float64(r.Status2xx), float64(r.RequestCount))
		case "error_rate":
			row[m] = safePercent(float64(r.Status4xx+r.Status5xx), float64(r.RequestCount))
		case "throttle_rate":
			row[m] = safePercent(float64(r.Status429), float64(r.RequestCount))
		case "cache_hit_rate":
			row[m] = safePercent(float64(r.CacheHitCnt), float64(r.RequestCount))
		case "avg_tokens_per_req":
			row[m] = safeDivide(float64(r.TotalTokens), float64(r.RequestCount))
		case "avg_cost_per_req":
			row[m] = safeDivide(r.UsedAmount, float64(r.RequestCount))
		case "avg_latency":
			row[m] = safeDivide(float64(r.TotalTimeMs), float64(r.RequestCount))
		case "avg_ttfb":
			row[m] = safeDivide(float64(r.TotalTtfbMs), float64(r.RequestCount))
		case "output_input_ratio":
			row[m] = safeDivide(float64(r.OutputTokens), float64(r.InputTokens))
		case "cost_per_1k_tokens":
			row[m] = safeDivide(r.UsedAmount, float64(r.TotalTokens)) * 1000
		case "retry_rate":
			row[m] = safePercent(float64(r.RetryCount), float64(r.RequestCount))
		case "reconciliation_tokens":
			row[m] = max(0, r.InputTokens-r.CachedTokens-r.CacheCrTokens) + r.OutputTokens
		}
	}
}

func safePercent(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}

	return math.Round(numerator/denominator*10000) / 100
}

func safeDivide(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}

	return math.Round(numerator/denominator*100) / 100
}

func sortResults(rows []map[string]interface{}, sortBy, sortOrder string, dimensions []string) {
	desc := strings.EqualFold(sortOrder, "desc")

	sort.SliceStable(rows, func(i, j int) bool {
		// Primary sort: user-specified sort key
		if sortBy != "" {
			vi, _ := rows[i][sortBy]
			vj, _ := rows[j][sortBy]
			if cmp := compareValues(vi, vj); cmp != 0 {
				if desc {
					return cmp > 0
				}
				return cmp < 0
			}
		}

		// Fallback: sort by dimensions in order for deterministic output
		for _, d := range dimensions {
			vi, _ := rows[i][d]
			vj, _ := rows[j][d]
			if cmp := compareValues(vi, vj); cmp != 0 {
				return cmp < 0
			}
		}

		return false
	})
}

func compareValues(a, b interface{}) int {
	fa := toFloat64(a)
	fb := toFloat64(b)

	switch {
	case fa < fb:
		return -1
	case fa > fb:
		return 1
	default:
		// String comparison fallback
		sa := fmt.Sprintf("%v", a)
		sb := fmt.Sprintf("%v", b)

		return strings.Compare(sa, sb)
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	default:
		return 0
	}
}

func buildColumns(req CustomReportRequest) []ColumnDef {
	columns := make([]ColumnDef, 0, len(req.Dimensions)+len(req.Measures))

	for _, d := range req.Dimensions {
		label := dimensionLabels[d]
		if label == "" {
			label = d
		}

		columns = append(columns, ColumnDef{
			Key:   d,
			Label: label,
			Type:  "dimension",
		})
	}

	for _, m := range req.Measures {
		label := measureLabels[m]
		if label == "" {
			label = m
		}

		colType := "measure"
		if _, ok := computedMeasures[m]; ok {
			colType = "computed"
		}

		columns = append(columns, ColumnDef{
			Key:   m,
			Label: label,
			Type:  colType,
		})
	}

	return columns
}

// GetAvailableFields returns the field catalog for the frontend.
func GetAvailableFields() map[string]interface{} {
	dims := make([]map[string]string, 0, len(validDimensions))
	for key := range validDimensions {
		dims = append(dims, map[string]string{
			"key":   key,
			"label": dimensionLabels[key],
		})
	}

	baseMeasureList := make([]map[string]string, 0, len(baseMeasures))
	for key := range baseMeasures {
		baseMeasureList = append(baseMeasureList, map[string]string{
			"key":   key,
			"label": measureLabels[key],
			"type":  "measure",
		})
	}

	computedList := make([]map[string]string, 0, len(computedMeasures))
	for key := range computedMeasures {
		computedList = append(computedList, map[string]string{
			"key":   key,
			"label": measureLabels[key],
			"type":  "computed",
		})
	}

	return map[string]interface{}{
		"dimensions":       dims,
		"measures":         baseMeasureList,
		"computed_measures": computedList,
	}
}

