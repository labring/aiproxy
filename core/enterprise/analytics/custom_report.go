//go:build enterprise

package analytics

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
	"gorm.io/gorm"
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
	// TimezoneOffsetSeconds shifts day/week bucket boundaries so they align with
	// the client's local timezone (e.g. CST=+28800). Without this, a query for
	// "April 10" would bucket the data to UTC midnight, which a CST user sees as
	// "April 10 08:00" — producing a day bucket that straddles two local days.
	// Sane range is [-43200, 50400] (UTC-12 to UTC+14); values outside fall back to 0.
	//
	// DST caveat: the frontend sends a single fixed offset derived from the
	// browser's *current* getTimezoneOffset(). Queries spanning a DST transition
	// in affected zones (most of the Americas and Europe) will bucket the hours
	// around the transition into the wrong local day. China/CST is DST-free and
	// unaffected. A proper fix would be server-side `AT TIME ZONE 'X'` bucketing
	// using IANA zone names; deferred because current deployment is CST-only.
	TimezoneOffsetSeconds int64 `json:"timezone_offset_seconds"`
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
	Columns []ColumnDef      `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Total   int              `json:"total"`
	// Totals holds correctly-aggregated grand totals over the full (un-limited) result set.
	// Base measures are SUMmed; derived measures (ratios, averages) are re-computed from
	// the summed bases — which is mathematically equivalent to a proper weighted average.
	Totals map[string]any `json:"totals,omitempty"`
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
	"cache_creation_count":  "SUM(cache_creation_count)",
	"input_tokens":          "SUM(input_tokens)",
	"output_tokens":         "SUM(output_tokens)",
	"total_tokens":          "SUM(total_tokens)",
	"cached_tokens":         "SUM(cached_tokens)",
	"reasoning_tokens":      "SUM(reasoning_tokens)",
	"image_input_tokens":    "SUM(image_input_tokens)",
	"audio_input_tokens":    "SUM(audio_input_tokens)",
	"web_search_count":      "SUM(web_search_count)",
	"used_amount":           "SUM(used_amount)",
	"input_amount":          "SUM(input_amount)",
	"output_amount":         "SUM(output_amount)",
	"cached_amount":         "SUM(cached_amount)",
	"image_input_amount":    "SUM(image_input_amount)",
	"audio_input_amount":    "SUM(audio_input_amount)",
	"image_output_amount":   "SUM(image_output_amount)",
	"reasoning_amount":      "SUM(reasoning_amount)",
	"cache_creation_amount": "SUM(cache_creation_amount)",
	"web_search_amount":     "SUM(web_search_amount)",
	"total_time_ms":         "SUM(total_time_milliseconds)",
	"total_ttfb_ms":         "SUM(total_ttfb_milliseconds)",
	"unique_models":         "COUNT(DISTINCT model)",
	"active_users":          "COUNT(DISTINCT group_id)",
	"image_output_tokens":   "SUM(image_output_tokens)",
	"cache_creation_tokens": "SUM(cache_creation_tokens)",
}

// computedMeasures lists measures that are derived from base measures.
var computedMeasures = map[string][]string{
	// Rate metrics
	"success_rate":      {"status_2xx", "request_count"},
	"error_rate":        {"exception_count", "status_429", "request_count"},
	"exception_rate":    {"exception_count", "request_count"},
	"throttle_rate":     {"status_429", "request_count"},
	"cache_hit_rate":    {"cache_hit_count", "request_count"},
	"retry_rate":        {"retry_count", "request_count"},
	"client_error_rate": {"status_4xx", "request_count"},
	"server_error_rate": {"status_5xx", "request_count"},

	// Per-request efficiency
	"avg_tokens_per_req":    {"total_tokens", "request_count"},
	"avg_input_per_req":     {"input_tokens", "request_count"},
	"avg_output_per_req":    {"output_tokens", "request_count"},
	"avg_cached_per_req":    {"cached_tokens", "request_count"},
	"avg_reasoning_per_req": {"reasoning_tokens", "request_count"},
	"avg_cost_per_req":      {"used_amount", "request_count"},
	"avg_latency":           {"total_time_ms", "request_count"},
	"avg_ttfb":              {"total_ttfb_ms", "request_count"},

	// Throughput
	"tokens_per_second": {"total_tokens", "total_time_ms"},
	"output_speed":      {"output_tokens", "total_time_ms"},

	// Per-user averages
	"avg_tokens_per_user":   {"total_tokens", "active_users"},
	"avg_cost_per_user":     {"used_amount", "active_users"},
	"avg_requests_per_user": {"request_count", "active_users"},

	// Cost structure
	"output_input_ratio": {"output_tokens", "input_tokens"},
	"cost_per_1k_tokens": {"used_amount", "total_tokens"},
	// Blended per-1K input cost: (input_amount + cached_amount + cache_creation_amount)
	// / input_tokens * 1000. This is the true average cost per input token including
	// all cache-related charges. For models without CachedPrice the extra amounts are
	// zero, so this reduces to input_amount / input_tokens * 1000.
	"cost_per_input_1k": {
		"input_amount",
		"cached_amount",
		"cache_creation_amount",
		"input_tokens",
	},
	"cost_per_output_1k": {"output_amount", "output_tokens"},
	"input_cost_pct":     {"input_amount", "used_amount"},
	"output_cost_pct":    {"output_amount", "used_amount"},
	"cached_cost_pct":    {"cached_amount", "used_amount"},

	// Misc
	"reconciliation_tokens": {
		"input_tokens",
		"output_tokens",
		"cached_tokens",
		"cache_creation_tokens",
	},
}

// deprecatedMeasureAliases maps renamed measure keys to their current names.
// Old keys are still accepted so that saved report templates keep working.
var deprecatedMeasureAliases = map[string]string{
	"cache_savings_pct": "cached_cost_pct", // renamed 2026-04: name conflicted with formula
}

// measureLabels provides human-readable labels for measures.
var measureLabels = map[string]string{
	// Base: requests
	"request_count": "请求数",
	// retry_count is a per-request binary counter in group_summaries: +1 per
	// request that went through >=1 retry. NOT the total retry-attempt count.
	// Keeps retry_rate in [0%, 100%].
	"retry_count":          "重试请求数",
	"exception_count":      "异常次数",
	"status_2xx":           "成功请求数",
	"status_4xx":           "客户端错误数",
	"status_5xx":           "服务端错误数",
	"status_429":           "限流请求数",
	"cache_hit_count":      "缓存命中数",
	"cache_creation_count": "缓存创建次数",

	// Base: tokens
	// input_tokens includes cached + cache_creation (OpenAI prompt_tokens semantics).
	// See model.Usage struct doc for the cross-protocol invariant.
	"input_tokens":          "输入 Token (含缓存)",
	"output_tokens":         "输出 Token",
	"total_tokens":          "总 Token",
	"cached_tokens":         "缓存 Token",
	"reasoning_tokens":      "推理 Token",
	"image_input_tokens":    "图片输入 Token",
	"audio_input_tokens":    "音频输入 Token",
	"image_output_tokens":   "图片输出 Token",
	"cache_creation_tokens": "缓存创建 Token",
	"web_search_count":      "联网搜索次数",

	// Base: cost
	"used_amount":           "总费用",
	"input_amount":          "输入费用",
	"output_amount":         "输出费用",
	"cached_amount":         "缓存费用",
	"image_input_amount":    "图片输入费用",
	"audio_input_amount":    "音频输入费用",
	"image_output_amount":   "图片输出费用",
	"reasoning_amount":      "推理费用",
	"cache_creation_amount": "缓存创建费用",
	"web_search_amount":     "联网搜索费用",
	"total_time_ms":         "总耗时(ms)",
	"total_ttfb_ms":         "总首Token耗时(ms)",

	// Base: stats
	"unique_models": "使用模型数",
	"active_users":  "活跃用户数",

	// Computed: rates
	"success_rate":      "成功率 (%)",
	"error_rate":        "错误率 (%)",
	"exception_rate":    "异常率 (%)",
	"throttle_rate":     "限流率 (%)",
	"cache_hit_rate":    "缓存命中率 (%)",
	"retry_rate":        "重试请求率 (%)",
	"client_error_rate": "客户端错误率 (%)",
	"server_error_rate": "服务端错误率 (%)",

	// Computed: per-request efficiency
	"avg_tokens_per_req":    "平均每请求 Token",
	"avg_input_per_req":     "平均输入 Token/请求",
	"avg_output_per_req":    "平均输出 Token/请求",
	"avg_cached_per_req":    "平均缓存 Token/请求",
	"avg_reasoning_per_req": "平均推理 Token/请求",
	"avg_cost_per_req":      "平均单次费用",
	"avg_latency":           "平均响应时间 (ms)",
	"avg_ttfb":              "平均首Token时间 (ms)",
	// These are SUM(tokens) / SUM(wall_clock_time) — mathematically a
	// request-time-weighted average of per-request throughput, NOT system
	// wall-clock throughput. If N requests run concurrently, sum(time) > elapsed,
	// so this under-reports actual system TPS by the concurrency factor. Label
	// explicitly signals "per-request" to prevent the obvious misread.
	"tokens_per_second": "单请求平均速率 (token/s)",
	"output_speed":      "单请求输出速率 (token/s)",

	// Computed: per-user averages.
	// Denominator is active_users *scoped to the row's dimension bucket*
	// (e.g. active users of a specific model, on a specific day). The generic
	// "人均" reads as a global per-capita average of all employees, which it
	// is NOT — always a per-row ratio over active users in that slice.
	"avg_tokens_per_user":   "活跃用户人均 Token",
	"avg_cost_per_user":     "活跃用户人均费用",
	"avg_requests_per_user": "活跃用户人均请求数",

	// Computed: cost structure
	"output_input_ratio":    "输出/输入比",
	"cost_per_1k_tokens":    "千Token成本",
	"cost_per_input_1k":     "千输入Token混合成本",
	"cost_per_output_1k":    "千输出Token成本",
	"input_cost_pct":        "输入费用占比 (%)",
	"output_cost_pct":       "输出费用占比 (%)",
	"cached_cost_pct":       "缓存费用占比 (%)",
	"reconciliation_tokens": "对账 Token (不含缓存)",
}

// dimensionLabels provides human-readable labels for dimensions.
var dimensionLabels = map[string]string{
	"user_name":         "用户名",
	"department":        "部门",
	"level1_department": "一级部门",
	"level2_department": "二级部门",
	"model":             "模型",
	"time_hour":         "小时",
	"time_day":          "天",
	"time_week":         "周",
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
		return nil, errors.New("at least one dimension is required")
	}

	if len(req.Measures) == 0 {
		return nil, errors.New("at least one measure is required")
	}

	// Normalize deprecated measure aliases (kept for backwards compatibility
	// with templates saved before the rename).
	for i, m := range req.Measures {
		if newName, ok := deprecatedMeasureAliases[m]; ok {
			req.Measures[i] = newName
		}
	}

	// Clamp timezone offset to a sane range. Out-of-range values (including the
	// unset zero default outside this window) fall back to UTC.
	if req.TimezoneOffsetSeconds < -43200 || req.TimezoneOffsetSeconds > 50400 {
		req.TimezoneOffsetSeconds = 0
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
			Rows:    []map[string]any{},
			Total:   0,
		}, nil
	}

	// Run the main aggregation query and the exact-distinct totals query
	// concurrently — they share the same filters but compute independent results.
	var (
		rows                    []rawRow
		exactUsers, exactModels int64
		queryErr, distinctErr   error
		distinctDone            = make(chan struct{})
	)

	go func() {
		defer close(distinctDone)

		exactUsers, exactModels, distinctErr = computeExactDistinctTotals(
			req, groupIDs, req.Measures,
		)
	}()

	rows, queryErr = executeQuery(req, requiredBase, groupIDs)
	if queryErr != nil {
		<-distinctDone // don't leak the goroutine
		return nil, fmt.Errorf("query custom report: %w", queryErr)
	}

	// Compute grand totals over the full (un-limited) result set.
	// SUM all rawRows into one, then derive ratio measures from the sums.
	totals, agg := computeTotals(rows, req.Measures)

	// Wait for the distinct-count query, then override approximate SUM-based
	// totals with exact values.
	<-distinctDone

	if distinctErr == nil {
		if _, ok := totals["active_users"]; ok {
			totals["active_users"] = exactUsers
		}

		if _, ok := totals["unique_models"]; ok {
			totals["unique_models"] = exactModels
		}
		// Rewire per-user averages against the corrected denominator, using
		// the aggregate's raw base values (so this works even when the base
		// measure itself was not requested).
		if exactUsers > 0 {
			if _, need := totals["avg_tokens_per_user"]; need {
				totals["avg_tokens_per_user"] = float64(agg.TotalTokens) / float64(exactUsers)
			}

			if _, need := totals["avg_cost_per_user"]; need {
				totals["avg_cost_per_user"] = agg.UsedAmount / float64(exactUsers)
			}

			if _, need := totals["avg_requests_per_user"]; need {
				totals["avg_requests_per_user"] = float64(agg.RequestCount) / float64(exactUsers)
			}
		}
	}
	// On error: fall back to the (approximate, upper-bound) SUM-based totals —
	// less misleading than failing the whole report.

	// Post-process: map group_id to user/department, compute derived fields
	result := postProcess(rows, req, groupIDs, groupToUser, deptNameMap)

	// Sort results — always apply a deterministic fallback sort so that
	// repeated queries with identical parameters return rows in the same order.
	sortResults(result, req.SortBy, req.SortOrder, req.Dimensions)

	// Apply limit (default cap: 1000 rows)
	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}

	total := len(result)
	if len(result) > limit {
		result = result[:limit]
	}

	// Build columns
	columns := buildColumns(req)

	return &CustomReportResponse{
		Columns: columns,
		Rows:    result,
		Total:   total,
		Totals:  totals,
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

		return l1Name, l2Name
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
	CacheCrCnt    int64   `gorm:"column:cache_creation_count"`
	InputTokens   int64   `gorm:"column:input_tokens"`
	OutputTokens  int64   `gorm:"column:output_tokens"`
	TotalTokens   int64   `gorm:"column:total_tokens"`
	CachedTokens  int64   `gorm:"column:cached_tokens"`
	ReasonTokens  int64   `gorm:"column:reasoning_tokens"`
	ImgInTokens   int64   `gorm:"column:image_input_tokens"`
	AudioInTokens int64   `gorm:"column:audio_input_tokens"`
	WebSearchCnt  int64   `gorm:"column:web_search_count"`
	UsedAmount    float64 `gorm:"column:used_amount"`
	InputAmount   float64 `gorm:"column:input_amount"`
	OutputAmount  float64 `gorm:"column:output_amount"`
	CachedAmount  float64 `gorm:"column:cached_amount"`
	ImgInAmount   float64 `gorm:"column:image_input_amount"`
	AudioInAmount float64 `gorm:"column:audio_input_amount"`
	ImgOutAmount  float64 `gorm:"column:image_output_amount"`
	ReasonAmount  float64 `gorm:"column:reasoning_amount"`
	CacheCrAmount float64 `gorm:"column:cache_creation_amount"`
	WebSearchAmt  float64 `gorm:"column:web_search_amount"`
	TotalTimeMs   int64   `gorm:"column:total_time_ms"`
	TotalTtfbMs   int64   `gorm:"column:total_ttfb_ms"`
	UniqueModels  int64   `gorm:"column:unique_models"`
	ActiveUsers   int64   `gorm:"column:active_users"`
	ImgOutTokens  int64   `gorm:"column:image_output_tokens"`
	CacheCrTokens int64   `gorm:"column:cache_creation_tokens"`
}

// applySummaryFilters attaches the shared time/group/model WHERE clauses to the
// query. Used by both executeQuery and computeExactDistinctTotals so that the
// two always see the same underlying row set.
func applySummaryFilters(
	query *gorm.DB,
	req CustomReportRequest,
	groupIDs []string,
) *gorm.DB {
	if req.TimeRange.StartTimestamp > 0 {
		query = query.Where("hour_timestamp >= ?", req.TimeRange.StartTimestamp)
	}

	if req.TimeRange.EndTimestamp > 0 {
		query = query.Where("hour_timestamp <= ?", req.TimeRange.EndTimestamp)
	}

	if len(groupIDs) > 0 {
		query = query.Where("group_id IN ?", groupIDs)
	}

	if len(req.Filters.Models) > 0 {
		query = query.Where("model IN ?", req.Filters.Models)
	}

	return query
}

// summaryQuery returns a filtered GroupSummary query with a 30-second timeout.
// The returned cancel must be deferred by the caller.
func summaryQuery(req CustomReportRequest, groupIDs []string) (*gorm.DB, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	query := applySummaryFilters(
		model.LogDB.WithContext(ctx).Model(&model.GroupSummary{}),
		req, groupIDs,
	)

	return query, cancel
}

func executeQuery(
	req CustomReportRequest,
	requiredBase map[string]bool,
	groupIDs []string,
) ([]rawRow, error) {
	// Build SELECT clause
	selectParts := buildSelectParts(req.Dimensions, requiredBase, req.TimezoneOffsetSeconds)

	// Build GROUP BY clause
	groupByParts := buildGroupByParts(req.Dimensions, req.TimezoneOffsetSeconds)

	query, cancel := summaryQuery(req, groupIDs)
	defer cancel()

	var results []rawRow

	err := query.
		Select(strings.Join(selectParts, ", ")).
		Group(strings.Join(groupByParts, ", ")).
		Order(strings.Join(groupByParts, ", ")).
		Find(&results).Error

	return results, err
}

// computeExactDistinctTotals runs a single COUNT(DISTINCT) query to get the
// true distinct user/model counts over the un-grouped result set. This fixes
// the over-counting that happens when active_users / unique_models are
// SUMmed across per-row COUNT(DISTINCT) values (e.g. with dimensions=[model]
// or [time_day], a user using two models gets counted twice).
//
// Only executed when at least one of active_users/unique_models is requested,
// so the extra round-trip is paid only when it matters.
func computeExactDistinctTotals(
	req CustomReportRequest,
	groupIDs []string,
	measures []string,
) (activeUsers, uniqueModels int64, err error) {
	needUsers := false

	needModels := false
	for _, m := range measures {
		switch m {
		case "active_users",
			"avg_tokens_per_user",
			"avg_cost_per_user",
			"avg_requests_per_user":
			needUsers = true
		case "unique_models":
			needModels = true
		}
	}

	if !needUsers && !needModels {
		return 0, 0, nil
	}

	type result struct {
		ActiveUsers  int64 `gorm:"column:active_users"`
		UniqueModels int64 `gorm:"column:unique_models"`
	}

	var r result

	selects := make([]string, 0, 2)
	if needUsers {
		selects = append(selects, "COUNT(DISTINCT group_id) as active_users")
	}

	if needModels {
		selects = append(selects, "COUNT(DISTINCT model) as unique_models")
	}

	query, cancel := summaryQuery(req, groupIDs)
	defer cancel()

	if err := query.Select(strings.Join(selects, ", ")).Scan(&r).Error; err != nil {
		return 0, 0, err
	}

	return r.ActiveUsers, r.UniqueModels, nil
}

// deptTimeKey identifies a (dept, time-bucket) slot used by the exact-distinct
// unique_models computation for department aggregation. TimeKey is 0 when the
// report has no time dimension.
type deptTimeKey struct {
	Dept    string
	TimeKey int64
}

// computeExactUniqueModelsByDept returns the true COUNT(DISTINCT model) per
// (dept[, time]) bucket used by aggregateByDepartment. Without this, merging
// rows double-counts models: each rawRow carries per-group COUNT(DISTINCT model),
// so summing across groups in the same dept counts shared models twice; and when
// "model" itself is a dimension, each per-row value is 1, summed to the group
// count (still wrong — should be 1 for that single-model bucket).
//
// We resolve the department from the in-memory groupToUser mapping rather than
// joining against feishu_users in SQL, keeping this function self-contained and
// consistent with how aggregateByDepartment assigns departments.
//
// Only called when unique_models is a requested measure. Caller falls back to
// the (over-counted) SUM-based value if this query errors.
func computeExactUniqueModelsByDept(
	req CustomReportRequest,
	groupIDs []string,
	groupToUser map[string]userMapping,
	deptNameMap map[string]string,
	deptLevel string, // "department" | "level1_department" | "level2_department"
	timeDim string, // "", "time_hour", "time_day", "time_week"
) (map[deptTimeKey]int64, error) {
	type distinctRow struct {
		GroupID string `gorm:"column:group_id"`
		Model   string `gorm:"column:model"`
		TimeKey int64  `gorm:"column:time_key"`
	}

	selects := []string{"group_id", "model"}
	groupBy := []string{"group_id", "model"}

	if timeDim != "" {
		if expr := timeBucketExpr(timeDim, req.TimezoneOffsetSeconds); expr != "" {
			selects = append(selects, expr+" as time_key")
			groupBy = append(groupBy, expr)
		}
	}

	query, cancel := summaryQuery(req, groupIDs)
	defer cancel()

	var rows []distinctRow

	err := query.
		Select(strings.Join(selects, ", ")).
		Group(strings.Join(groupBy, ", ")).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	buckets := make(map[deptTimeKey]map[string]struct{})

	for _, r := range rows {
		deptName := ""
		if um, ok := groupToUser[r.GroupID]; ok {
			switch deptLevel {
			case "level1_department":
				deptName = um.Level1DeptName
			case "level2_department":
				deptName = um.Level2DeptName
			default:
				deptName = deptNameMap[um.DepartmentID]
			}
		}

		key := deptTimeKey{Dept: deptName, TimeKey: r.TimeKey}

		models := buckets[key]
		if models == nil {
			models = make(map[string]struct{})
			buckets[key] = models
		}

		models[r.Model] = struct{}{}
	}

	result := make(map[deptTimeKey]int64, len(buckets))
	for k, m := range buckets {
		result[k] = int64(len(m))
	}

	return result, nil
}

// timeBucketExpr returns the SQL expression that maps hour_timestamp to the
// start-of-bucket Unix timestamp for the given granularity. The offset shifts
// bucket boundaries to align with the client's local timezone — e.g. with
// offset=28800 (CST), a day bucket covers local 00:00 to 23:59. The resulting
// time_key is still a UTC Unix timestamp pointing at the local midnight instant.
//
// offset is an int64 already clamped by the caller, so inlining it is safe
// (no SQL-injection surface). Using placeholders would complicate the shared
// SELECT/GROUP BY/ORDER BY string building.
func timeBucketExpr(granularity string, offset int64) string {
	switch granularity {
	case "time_hour":
		return "hour_timestamp"
	case "time_day":
		if offset == 0 {
			return "(hour_timestamp / 86400 * 86400)"
		}
		return fmt.Sprintf("((hour_timestamp + %d) / 86400 * 86400 - %d)", offset, offset)
	case "time_week":
		// Align to Monday: epoch (1970-01-01) = Thursday, first Monday = 1970-01-05 = 4 days = 345600s.
		// Offset shifts to local-time Monday 00:00; subtracting offset returns a UTC
		// Unix timestamp equivalent to that local moment.
		if offset == 0 {
			return "((hour_timestamp - 345600) / 604800 * 604800 + 345600)"
		}

		return fmt.Sprintf(
			"(((hour_timestamp + %d - 345600) / 604800 * 604800 + 345600) - %d)",
			offset, offset,
		)
	}

	return ""
}

func buildSelectParts(dimensions []string, requiredBase map[string]bool, tzOffset int64) []string {
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
		if expr := timeBucketExpr(timeGranularity, tzOffset); expr != "" {
			parts = append(parts, expr+" as time_key")
		}
	}

	// Derive SELECT expressions from baseMeasures (single source of truth).
	added := make(map[string]bool)
	for measure := range requiredBase {
		if expr, ok := baseMeasures[measure]; ok && !added[measure] {
			parts = append(parts, expr+" as "+measure)
			added[measure] = true
		}
	}

	return parts
}

func buildGroupByParts(dimensions []string, tzOffset int64) []string {
	parts := make([]string, 0, 3)

	for _, d := range dimensions {
		switch d {
		case "user_name", "department", "level1_department", "level2_department":
			if !slices.Contains(parts, "group_id") {
				parts = append(parts, "group_id")
			}
		case "model":
			parts = append(parts, "model")
		case "time_hour", "time_day", "time_week":
			// Must match the SELECT expression in buildSelectParts.
			if expr := timeBucketExpr(d, tzOffset); expr != "" {
				parts = append(parts, expr)
			}
		}
	}

	return parts
}

func postProcess(
	rows []rawRow,
	req CustomReportRequest,
	groupIDs []string,
	groupToUser map[string]userMapping,
	deptNameMap map[string]string,
) []map[string]any {
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
		return aggregateByDepartment(rows, req, groupIDs, groupToUser, deptNameMap)
	}

	result := make([]map[string]any, 0, len(rows))

	for _, r := range rows {
		row := make(map[string]any)

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

		// Compute derived measures — user_name dimension means per-user grouping
		computeDerivedMeasures(row, r, req.Measures, hasUserDim)

		result = append(result, row)
	}

	return result
}

func aggregateByDepartment(
	rows []rawRow,
	req CustomReportRequest,
	groupIDs []string,
	groupToUser map[string]userMapping,
	deptNameMap map[string]string,
) []map[string]any {
	// Build a composite key for aggregation
	type aggKey struct {
		DeptName string
		Model    string
		TimeKey  int64
	}

	aggMap := make(map[aggKey]*rawRow)

	hasModel := false
	hasTime := false
	hasLevel1 := false
	hasLevel2 := false
	timeDim := ""

	for _, d := range req.Dimensions {
		switch d {
		case "department":
			// handled by deptDimKey default
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

	// Fix unique_models over-counting: mergeRawRows SUMs per-group COUNT(DISTINCT model),
	// which double-counts when groups in the same dept share models. When "model" is a
	// dimension, each bucket covers exactly one model so the answer is trivially 1; when
	// not, run a targeted query to compute exact distinct counts per (dept[, time]) bucket.
	wantsUniqueModels := slices.Contains(req.Measures, "unique_models")

	var exactDistinct map[deptTimeKey]int64

	if wantsUniqueModels && !hasModel {
		deptLevel := "department"

		switch {
		case hasLevel1:
			deptLevel = "level1_department"
		case hasLevel2:
			deptLevel = "level2_department"
		}

		tdim := ""
		if hasTime {
			tdim = timeDim
		}

		if m, err := computeExactUniqueModelsByDept(
			req, groupIDs, groupToUser, deptNameMap, deptLevel, tdim,
		); err == nil {
			exactDistinct = m
		}
		// On error: fall back to the SUM-based (over-counted) value rather
		// than failing the whole report.
	}

	result := make([]map[string]any, 0, len(aggMap))

	// Determine which department dimension key to use in the output row
	deptDimKey := "department"
	if hasLevel1 {
		deptDimKey = "level1_department"
	} else if hasLevel2 {
		deptDimKey = "level2_department"
	}

	for key, r := range aggMap {
		// Override the rawRow's UniqueModels before it flows into
		// fillBaseMeasures / computeDerivedMeasures.
		if wantsUniqueModels {
			switch {
			case hasModel:
				// Each bucket is a single model.
				r.UniqueModels = 1
			case exactDistinct != nil:
				lookup := deptTimeKey{Dept: key.DeptName}
				if hasTime {
					lookup.TimeKey = key.TimeKey
				}

				if v, ok := exactDistinct[lookup]; ok {
					r.UniqueModels = v
				}
			}
		}

		row := make(map[string]any)
		row[deptDimKey] = key.DeptName

		if hasModel {
			row["model"] = key.Model
		}

		if hasTime {
			row[timeDim] = key.TimeKey
		}

		fillBaseMeasures(row, *r, req.Measures)
		computeDerivedMeasures(row, *r, req.Measures, false)
		result = append(result, row)
	}

	return result
}

// computeTotals aggregates all rawRows into a single grand-total row and then
// derives ratio/average measures from the summed bases.
//
// This produces mathematically correct weighted aggregates:
//   - SUM(request_count), SUM(used_amount), etc. — additive measures
//   - success_rate = SUM(status_2xx) / SUM(request_count) — properly weighted
//   - cost_per_1k_tokens = SUM(used_amount) / SUM(total_tokens) * 1000 — properly weighted
//
// Unlike the per-row values, these totals are not affected by result pagination.
//
// Caveats:
//   - active_users: SUMmed from per-row COUNT(DISTINCT group_id). When group_id
//     appears in GROUP BY, each row contributes 1, and the sum equals true distinct
//     count. Otherwise it is an upper bound, and GenerateCustomReport overrides
//     with an exact COUNT(DISTINCT) query.
//   - unique_models: same — overridden with exact count.
//
// Returns both the totals map and the raw aggregate, so callers can recompute
// derived measures after fixing up base values (e.g. exact distinct counts).
func computeTotals(rows []rawRow, measures []string) (map[string]any, rawRow) {
	if len(rows) == 0 {
		return map[string]any{}, rawRow{}
	}

	var agg rawRow
	for i := range rows {
		mergeRawRows(&agg, &rows[i])
	}

	totals := make(map[string]any, len(measures))
	fillBaseMeasures(totals, agg, measures)
	computeDerivedMeasures(totals, agg, measures, false)

	return totals, agg
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
	dst.CacheCrCnt += src.CacheCrCnt
	dst.ReasonTokens += src.ReasonTokens
	dst.ImgInAmount += src.ImgInAmount
	dst.AudioInAmount += src.AudioInAmount
	dst.ImgOutAmount += src.ImgOutAmount
	dst.ReasonAmount += src.ReasonAmount
	dst.CacheCrAmount += src.CacheCrAmount
	dst.WebSearchAmt += src.WebSearchAmt
}

func fillBaseMeasures(row map[string]any, r rawRow, measures []string) {
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
		case "cache_creation_count":
			row[m] = r.CacheCrCnt
		case "input_tokens":
			row[m] = r.InputTokens
		case "output_tokens":
			row[m] = r.OutputTokens
		case "total_tokens":
			row[m] = r.TotalTokens
		case "cached_tokens":
			row[m] = r.CachedTokens
		case "reasoning_tokens":
			row[m] = r.ReasonTokens
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
		case "image_input_amount":
			row[m] = r.ImgInAmount
		case "audio_input_amount":
			row[m] = r.AudioInAmount
		case "image_output_amount":
			row[m] = r.ImgOutAmount
		case "reasoning_amount":
			row[m] = r.ReasonAmount
		case "cache_creation_amount":
			row[m] = r.CacheCrAmount
		case "web_search_amount":
			row[m] = r.WebSearchAmt
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

// computeDerivedMeasures fills derived (ratio/average) measures into row.
// perUserGroup indicates whether each row represents a single user's data
// (GROUP BY includes group_id), in which case per-user averages are
// suppressed (they would equal the raw value and mislead).
func computeDerivedMeasures(row map[string]any, r rawRow, measures []string, perUserGroup bool) {
	for _, m := range measures {
		switch m {
		case "success_rate":
			row[m] = safePercent(float64(r.Status2xx), float64(r.RequestCount))
		case "error_rate":
			// Non-200 responses minus throttle (429 has its own throttle_rate).
			// ExceptionCnt counts status != 200, which includes non-200 2xx (201, 204 etc.)
			// — irrelevant for LLM APIs but technically not "non-2xx".
			// max(0, …) guards against negative values from inconsistent historical data.
			row[m] = safePercent(
				float64(max(r.ExceptionCnt-r.Status429, 0)),
				float64(r.RequestCount),
			)
		case "throttle_rate":
			row[m] = safePercent(float64(r.Status429), float64(r.RequestCount))
		case "client_error_rate":
			row[m] = safePercent(float64(r.Status4xx), float64(r.RequestCount))
		case "server_error_rate":
			row[m] = safePercent(float64(r.Status5xx), float64(r.RequestCount))
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
			row[m] = mulOrNil(safeDivide(r.UsedAmount, float64(r.TotalTokens)), 1000)
		case "retry_rate":
			row[m] = safePercent(float64(r.RetryCount), float64(r.RequestCount))
		case "reconciliation_tokens":
			row[m] = max(0, r.InputTokens-r.CachedTokens-r.CacheCrTokens) + r.OutputTokens
		case "exception_rate":
			row[m] = safePercent(float64(r.ExceptionCnt), float64(r.RequestCount))
		case "avg_input_per_req":
			row[m] = safeDivide(float64(r.InputTokens), float64(r.RequestCount))
		case "avg_output_per_req":
			row[m] = safeDivide(float64(r.OutputTokens), float64(r.RequestCount))
		case "avg_cached_per_req":
			row[m] = safeDivide(float64(r.CachedTokens), float64(r.RequestCount))
		case "avg_reasoning_per_req":
			row[m] = safeDivide(float64(r.ReasonTokens), float64(r.RequestCount))
		case "tokens_per_second":
			row[m] = safeDivide(float64(r.TotalTokens), float64(r.TotalTimeMs)/1000)
		case "output_speed":
			row[m] = safeDivide(float64(r.OutputTokens), float64(r.TotalTimeMs)/1000)
		case "avg_tokens_per_user":
			// Suppress per-user averages when each row is a single user's data
			// (perUserGroup=true): the "average" trivially equals the raw value.
			// For department/model/time groupings, even active_users=1 is meaningful.
			if !perUserGroup {
				row[m] = safeDivide(float64(r.TotalTokens), float64(r.ActiveUsers))
			}
		case "avg_cost_per_user":
			if !perUserGroup {
				row[m] = safeDivide(r.UsedAmount, float64(r.ActiveUsers))
			}
		case "avg_requests_per_user":
			if !perUserGroup {
				row[m] = safeDivide(float64(r.RequestCount), float64(r.ActiveUsers))
			}
		case "input_cost_pct":
			row[m] = safePercent(r.InputAmount, r.UsedAmount)
		case "output_cost_pct":
			row[m] = safePercent(r.OutputAmount, r.UsedAmount)
		case "cached_cost_pct":
			row[m] = safePercent(r.CachedAmount, r.UsedAmount)
		case "cost_per_input_1k":
			// Blended cost per 1K input tokens: includes base-rate input, cached-read,
			// and cache-creation charges. Denominator is total input_tokens (which
			// includes cached + creation per the cross-protocol invariant).
			totalInputCost := r.InputAmount + r.CachedAmount + r.CacheCrAmount
			row[m] = mulOrNil(safeDivide(totalInputCost, float64(r.InputTokens)), 1000)
		case "cost_per_output_1k":
			row[m] = mulOrNil(safeDivide(r.OutputAmount, float64(r.OutputTokens)), 1000)
		}
	}
}

// safePercent returns a percentage value (0-100) with full precision, or nil
// when the denominator is zero (= no data). Returning nil instead of 0 lets the
// frontend distinguish "no requests at all" from "0% success rate" and display
// "-" accordingly. Rounding is the frontend's responsibility (via toFixed).
func safePercent(numerator, denominator float64) any {
	if denominator == 0 {
		return nil
	}

	return numerator / denominator * 100
}

// safeDivide returns a raw ratio with full precision, or nil when the
// denominator is zero (= no data). The frontend formats the value.
func safeDivide(numerator, denominator float64) any {
	if denominator == 0 {
		return nil
	}

	return numerator / denominator
}

// mulOrNil multiplies v by factor when v is a non-nil float64; returns nil otherwise.
// Used for post-scaling safeDivide results (e.g. cost_per_1k = ratio * 1000).
func mulOrNil(v any, factor float64) any {
	if f, ok := v.(float64); ok {
		return f * factor
	}

	return nil
}

func sortResults(rows []map[string]any, sortBy, sortOrder string, dimensions []string) {
	desc := strings.EqualFold(sortOrder, "desc")

	sort.SliceStable(rows, func(i, j int) bool {
		// Primary sort: user-specified sort key
		if sortBy != "" {
			vi := rows[i][sortBy]

			vj := rows[j][sortBy]
			if cmp := compareValues(vi, vj); cmp != 0 {
				if desc {
					return cmp > 0
				}
				return cmp < 0
			}
		}

		// Fallback: sort by dimensions in order for deterministic output
		for _, d := range dimensions {
			vi := rows[i][d]

			vj := rows[j][d]
			if cmp := compareValues(vi, vj); cmp != 0 {
				return cmp < 0
			}
		}

		return false
	})
}

func compareValues(a, b any) int {
	// nil (no data) always sorts after non-nil values so that "no data"
	// rows sink to the bottom regardless of sort direction.
	if a == nil && b == nil {
		return 0
	}

	if a == nil {
		return 1
	}

	if b == nil {
		return -1
	}

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

func toFloat64(v any) float64 {
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
func GetAvailableFields() map[string]any {
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

	return map[string]any{
		"dimensions":        dims,
		"measures":          baseMeasureList,
		"computed_measures": computedList,
	}
}
