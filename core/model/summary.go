package model

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SummarySelectFields is a list of field names to select when querying summary data
type SummarySelectFields []string

var (
	baseCountSummaryFields = []string{
		"request_count",
		"retry_count",
		"exception_count",
		"status2xx_count",
		"status4xx_count",
		"status5xx_count",
		"status_other_count",
		"status400_count",
		"status429_count",
		"status500_count",
		"cache_hit_count",
		"cache_creation_count",
	}
	baseUsageSummaryFields = []string{
		"input_tokens",
		"image_input_tokens",
		"audio_input_tokens",
		"output_tokens",
		"image_output_tokens",
		"cached_tokens",
		"cache_creation_tokens",
		"reasoning_tokens",
		"total_tokens",
		"web_search_count",
	}
	baseAmountSummaryFields = []string{
		"input_amount",
		"image_input_amount",
		"audio_input_amount",
		"output_amount",
		"image_output_amount",
		"cached_amount",
		"cache_creation_amount",
		"web_search_amount",
		"used_amount",
	}
	baseTimeSummaryFields = []string{
		"total_time_milliseconds",
		"total_ttfb_milliseconds",
	}
	serviceTierPrefixes = []string{
		"service_tier_flex",
		"service_tier_priority",
	}
	extraSummaryPrefixes = []string{
		"claude_long_context",
	}
)

func buildPrefixedSummaryFields(prefix string, fields []string) []string {
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		result = append(result, prefix+"_"+field)
	}

	return result
}

func concatSummaryFields(groups ...[]string) []string {
	var totalLen int
	for _, group := range groups {
		totalLen += len(group)
	}

	result := make([]string, 0, totalLen)
	for _, group := range groups {
		result = append(result, group...)
	}

	return result
}

// allSummaryFields contains all available summary field names
var allSummaryFields = func() []string {
	countFields := append([]string{}, baseCountSummaryFields...)
	usageFields := append([]string{}, baseUsageSummaryFields...)
	amountFields := append([]string{}, baseAmountSummaryFields...)
	timeFields := append([]string{}, baseTimeSummaryFields...)

	for _, prefix := range serviceTierPrefixes {
		countFields = append(
			countFields,
			buildPrefixedSummaryFields(prefix, baseCountSummaryFields)...)
		usageFields = append(
			usageFields,
			buildPrefixedSummaryFields(prefix, baseUsageSummaryFields)...)
		amountFields = append(
			amountFields,
			buildPrefixedSummaryFields(prefix, baseAmountSummaryFields)...)
		timeFields = append(
			timeFields,
			buildPrefixedSummaryFields(prefix, baseTimeSummaryFields)...)
	}

	for _, prefix := range extraSummaryPrefixes {
		countFields = append(
			countFields,
			buildPrefixedSummaryFields(prefix, baseCountSummaryFields)...)
		usageFields = append(
			usageFields,
			buildPrefixedSummaryFields(prefix, baseUsageSummaryFields)...)
		amountFields = append(
			amountFields,
			buildPrefixedSummaryFields(prefix, baseAmountSummaryFields)...)
		timeFields = append(
			timeFields,
			buildPrefixedSummaryFields(prefix, baseTimeSummaryFields)...)
	}

	return concatSummaryFields(
		countFields,
		usageFields,
		amountFields,
		timeFields,
	)
}()

// summaryFieldGroups maps group names to their fields
var summaryFieldGroups = func() map[string][]string {
	countFields := append([]string{}, baseCountSummaryFields...)
	usageFields := append([]string{}, baseUsageSummaryFields...)
	amountFields := append([]string{}, baseAmountSummaryFields...)
	timeFields := append([]string{}, baseTimeSummaryFields...)

	for _, prefix := range serviceTierPrefixes {
		countFields = append(
			countFields,
			buildPrefixedSummaryFields(prefix, baseCountSummaryFields)...)
		usageFields = append(
			usageFields,
			buildPrefixedSummaryFields(prefix, baseUsageSummaryFields)...)
		amountFields = append(
			amountFields,
			buildPrefixedSummaryFields(prefix, baseAmountSummaryFields)...)
		timeFields = append(
			timeFields,
			buildPrefixedSummaryFields(prefix, baseTimeSummaryFields)...)
	}

	for _, prefix := range extraSummaryPrefixes {
		countFields = append(
			countFields,
			buildPrefixedSummaryFields(prefix, baseCountSummaryFields)...)
		usageFields = append(
			usageFields,
			buildPrefixedSummaryFields(prefix, baseUsageSummaryFields)...)
		amountFields = append(
			amountFields,
			buildPrefixedSummaryFields(prefix, baseAmountSummaryFields)...)
		timeFields = append(
			timeFields,
			buildPrefixedSummaryFields(prefix, baseTimeSummaryFields)...)
	}

	return map[string][]string{
		"count":  countFields,
		"usage":  usageFields,
		"amount": amountFields,
		"time":   timeFields,
	}
}()

// summaryFieldAliases maps alternative field names to canonical names
var summaryFieldAliases = map[string]string{
	"total_time": "total_time_milliseconds",
	"total_ttfb": "total_ttfb_milliseconds",
}

// IsEmpty checks if no fields are selected (nil or empty slice means all fields)
func (f SummarySelectFields) IsEmpty() bool {
	return len(f) == 0
}

// ParseSummaryFields parses a comma-separated string of field names into SummarySelectFields
// Returns nil (meaning all fields) if the input is empty
// Supports field groups: "count", "usage", "time", "all"
// Example: "request_count,exception_count,cache_hit_count" or "count,used_amount"
func ParseSummaryFields(fieldsStr string) SummarySelectFields {
	if fieldsStr == "" {
		return nil
	}

	// Create a set to avoid duplicates
	fieldSet := make(map[string]struct{})

	for part := range strings.SplitSeq(fieldsStr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if it's a group
		if part == "all" {
			return nil // nil means all fields
		}

		if groupFields, ok := summaryFieldGroups[part]; ok {
			for _, f := range groupFields {
				fieldSet[f] = struct{}{}
			}

			continue
		}

		// Check if it's an alias
		if canonical, ok := summaryFieldAliases[part]; ok {
			part = canonical
		}

		// Check if it's a valid field
		if slices.Contains(allSummaryFields, part) {
			fieldSet[part] = struct{}{}
		}
	}

	// If no valid fields were found, return nil (all fields)
	if len(fieldSet) == 0 {
		return nil
	}

	result := make(SummarySelectFields, 0, len(fieldSet))
	for field := range fieldSet {
		result = append(result, field)
	}

	return result
}

// BuildSelectFields generates SQL select clause based on selected fields
// timestampField should be "hour_timestamp" or "minute_timestamp"
// Security: Only fields in allSummaryFields whitelist are allowed to prevent SQL injection
func (f SummarySelectFields) BuildSelectFields(timestampField string) string {
	var sb strings.Builder

	sb.WriteString(timestampField)
	sb.WriteString(" as timestamp")

	// If no fields specified, select all
	fields := f
	if fields.IsEmpty() {
		fields = allSummaryFields
	}

	// Deduplicate and validate fields against whitelist
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		// Skip duplicates
		if _, exists := seen[field]; exists {
			continue
		}

		// Security: Only allow fields in whitelist (defense in depth)
		if !slices.Contains(allSummaryFields, field) {
			continue
		}

		seen[field] = struct{}{}

		sb.WriteString(", sum(")
		sb.WriteString(field)
		sb.WriteString(") as ")
		sb.WriteString(field)
	}

	return sb.String()
}

// BuildSelectFieldsV2 generates SQL select clause with additional grouping fields for V2 API
// groupFields should be fields like "channel_id, model" or "group_id, token_name, model"
func (f SummarySelectFields) BuildSelectFieldsV2(timestampField, groupFields string) string {
	var sb strings.Builder

	sb.WriteString(timestampField)
	sb.WriteString(" as timestamp, ")
	sb.WriteString(groupFields)

	// If no fields specified, select all
	fields := f
	if fields.IsEmpty() {
		fields = allSummaryFields
	}

	// Deduplicate and validate fields against whitelist
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		// Skip duplicates
		if _, exists := seen[field]; exists {
			continue
		}

		// Security: Only allow fields in whitelist (defense in depth)
		if !slices.Contains(allSummaryFields, field) {
			continue
		}

		seen[field] = struct{}{}

		sb.WriteString(", sum(")
		sb.WriteString(field)
		sb.WriteString(") as ")
		sb.WriteString(field)
	}

	return sb.String()
}

// only summary result only requests
type Summary struct {
	ID     int           `gorm:"primaryKey"`
	Unique SummaryUnique `gorm:"embedded"`
	Data   SummaryData   `gorm:"embedded"`
}

type SummaryUnique struct {
	ChannelID     int    `gorm:"not null;uniqueIndex:idx_summary_unique,priority:1"`
	Model         string `gorm:"size:64;not null;uniqueIndex:idx_summary_unique,priority:2"`
	HourTimestamp int64  `gorm:"not null;uniqueIndex:idx_summary_unique,priority:3,sort:desc"`
}

type Count struct {
	RequestCount       int64         `json:"request_count"`
	RetryCount         ZeroNullInt64 `json:"retry_count,omitempty"`
	ExceptionCount     ZeroNullInt64 `json:"exception_count,omitempty"`
	Status2xxCount     ZeroNullInt64 `json:"status_2xx_count,omitempty"`
	Status4xxCount     ZeroNullInt64 `json:"status_4xx_count,omitempty"`
	Status5xxCount     ZeroNullInt64 `json:"status_5xx_count,omitempty"`
	StatusOtherCount   ZeroNullInt64 `json:"status_other_count,omitempty"`
	Status400Count     ZeroNullInt64 `json:"status_400_count,omitempty"`
	Status429Count     ZeroNullInt64 `json:"status_429_count,omitempty"`
	Status500Count     ZeroNullInt64 `json:"status_500_count,omitempty"`
	CacheHitCount      ZeroNullInt64 `json:"cache_hit_count,omitempty"`
	CacheCreationCount ZeroNullInt64 `json:"cache_creation_count,omitempty"`
}

func (c *Count) AddRequest(status int, isRetry bool) {
	c.RequestCount++

	if status != http.StatusOK {
		c.ExceptionCount++
	}

	if isRetry {
		c.RetryCount++
	}

	switch {
	case status >= 200 && status < 300:
		c.Status2xxCount++
	case status >= 400 && status < 500:
		c.Status4xxCount++
		if status == http.StatusBadRequest {
			c.Status400Count++
		}

		if status == http.StatusTooManyRequests {
			c.Status429Count++
		}
	case status >= 500 && status < 600:
		c.Status5xxCount++
		if status == http.StatusInternalServerError {
			c.Status500Count++
		}
	default:
		c.StatusOtherCount++
	}
}

func (c *Count) Add(other Count) {
	c.RequestCount += other.RequestCount
	c.RetryCount += other.RetryCount
	c.ExceptionCount += other.ExceptionCount
	c.Status2xxCount += other.Status2xxCount
	c.Status4xxCount += other.Status4xxCount
	c.Status5xxCount += other.Status5xxCount
	c.StatusOtherCount += other.StatusOtherCount
	c.Status400Count += other.Status400Count
	c.Status429Count += other.Status429Count
	c.Status500Count += other.Status500Count
	c.CacheHitCount += other.CacheHitCount
	c.CacheCreationCount += other.CacheCreationCount
}

type SummaryDataSet struct {
	Count                 `      json:",inline"                           gorm:"embedded"`
	Usage                 `      json:",inline"                           gorm:"embedded"`
	Amount                `      json:",inline"                           gorm:"embedded"`
	TotalTimeMilliseconds int64 `json:"total_time_milliseconds,omitempty"`
	TotalTTFBMilliseconds int64 `json:"total_ttfb_milliseconds,omitempty"`
}

func (s *SummaryDataSet) Add(other SummaryDataSet) {
	s.Count.Add(other.Count)
	s.Usage.Add(other.Usage)
	s.Amount.Add(other.Amount)
	s.TotalTimeMilliseconds += other.TotalTimeMilliseconds
	s.TotalTTFBMilliseconds += other.TotalTTFBMilliseconds
}

type SummaryData struct {
	SummaryDataSet
	ServiceTierFlex     SummaryDataSet `json:"service_tier_flex,omitempty"     gorm:"embedded;embeddedPrefix:service_tier_flex_"`
	ServiceTierPriority SummaryDataSet `json:"service_tier_priority,omitempty" gorm:"embedded;embeddedPrefix:service_tier_priority_"`
	ClaudeLongContext   SummaryDataSet `json:"claude_long_context,omitempty"   gorm:"embedded;embeddedPrefix:claude_long_context_"`
}

const ClaudeLongContextInputTokensThreshold = int64(200000)

func (d *SummaryData) AddServiceTierBreakdown(
	serviceTier string,
	usage Usage,
	amount Amount,
	totalTimeMilliseconds int64,
	totalTTFBMilliseconds int64,
	isRetry bool,
	status int,
) {
	switch normalizeSummaryServiceTier(serviceTier) {
	case "flex":
		d.ServiceTierFlex.AddRequest(status, isRetry)

		if usage.CachedTokens > 0 {
			d.ServiceTierFlex.CacheHitCount++
		}

		if usage.CacheCreationTokens > 0 {
			d.ServiceTierFlex.CacheCreationCount++
		}

		d.ServiceTierFlex.Usage.Add(usage)
		d.ServiceTierFlex.Amount.Add(amount)
		d.ServiceTierFlex.TotalTimeMilliseconds += totalTimeMilliseconds
		d.ServiceTierFlex.TotalTTFBMilliseconds += totalTTFBMilliseconds
	case "priority":
		d.ServiceTierPriority.AddRequest(status, isRetry)

		if usage.CachedTokens > 0 {
			d.ServiceTierPriority.CacheHitCount++
		}

		if usage.CacheCreationTokens > 0 {
			d.ServiceTierPriority.CacheCreationCount++
		}

		d.ServiceTierPriority.Usage.Add(usage)
		d.ServiceTierPriority.Amount.Add(amount)
		d.ServiceTierPriority.TotalTimeMilliseconds += totalTimeMilliseconds
		d.ServiceTierPriority.TotalTTFBMilliseconds += totalTTFBMilliseconds
	}
}

func (d *SummaryData) Add(other SummaryData) {
	d.SummaryDataSet.Add(other.SummaryDataSet)
	d.ServiceTierFlex.Add(other.ServiceTierFlex)
	d.ServiceTierPriority.Add(other.ServiceTierPriority)
	d.ClaudeLongContext.Add(other.ClaudeLongContext)
}

func IsClaudeLongContextSummary(modelName string, usage Usage) bool {
	return strings.Contains(strings.ToLower(modelName), "claude") &&
		int64(usage.InputTokens) > ClaudeLongContextInputTokensThreshold
}

func (d *SummaryData) AddClaudeLongContextBreakdown(
	usage Usage,
	amount Amount,
	isRetry bool,
	status int,
) {
	d.ClaudeLongContext.AddRequest(status, isRetry)

	if usage.CachedTokens > 0 {
		d.ClaudeLongContext.CacheHitCount++
	}

	if usage.CacheCreationTokens > 0 {
		d.ClaudeLongContext.CacheCreationCount++
	}

	d.ClaudeLongContext.Usage.Add(usage)
	d.ClaudeLongContext.Amount.Add(amount)
}

func normalizeSummaryServiceTier(serviceTier string) string {
	switch normalizeServiceTier(serviceTier) {
	case "", "auto", "default", "standard":
		return "standard"
	case "flex":
		return "flex"
	case "priority":
		return "priority"
	default:
		return "standard"
	}
}

func appendSummaryCountUpdateData(
	data map[string]any,
	tableName, prefix string,
	count Count,
) {
	fields := []struct {
		column string
		value  int64
	}{
		{column: "request_count", value: count.RequestCount},
		{column: "retry_count", value: int64(count.RetryCount)},
		{column: "exception_count", value: int64(count.ExceptionCount)},
		{column: "status2xx_count", value: int64(count.Status2xxCount)},
		{column: "status4xx_count", value: int64(count.Status4xxCount)},
		{column: "status5xx_count", value: int64(count.Status5xxCount)},
		{column: "status_other_count", value: int64(count.StatusOtherCount)},
		{column: "status400_count", value: int64(count.Status400Count)},
		{column: "status429_count", value: int64(count.Status429Count)},
		{column: "status500_count", value: int64(count.Status500Count)},
		{column: "cache_hit_count", value: int64(count.CacheHitCount)},
		{column: "cache_creation_count", value: int64(count.CacheCreationCount)},
	}

	for _, field := range fields {
		if field.value <= 0 {
			continue
		}

		columnName := prefix + field.column
		data[columnName] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.%s, 0) + ?", tableName, columnName),
			field.value,
		)
	}
}

func appendSummaryUsageUpdateData(
	data map[string]any,
	tableName, prefix string,
	usage Usage,
) {
	fields := []struct {
		column string
		value  int64
	}{
		{column: "input_tokens", value: int64(usage.InputTokens)},
		{column: "image_input_tokens", value: int64(usage.ImageInputTokens)},
		{column: "audio_input_tokens", value: int64(usage.AudioInputTokens)},
		{column: "output_tokens", value: int64(usage.OutputTokens)},
		{column: "image_output_tokens", value: int64(usage.ImageOutputTokens)},
		{column: "cached_tokens", value: int64(usage.CachedTokens)},
		{column: "cache_creation_tokens", value: int64(usage.CacheCreationTokens)},
		{column: "reasoning_tokens", value: int64(usage.ReasoningTokens)},
		{column: "total_tokens", value: int64(usage.TotalTokens)},
		{column: "web_search_count", value: int64(usage.WebSearchCount)},
	}

	for _, field := range fields {
		if field.value <= 0 {
			continue
		}

		columnName := prefix + field.column
		data[columnName] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.%s, 0) + ?", tableName, columnName),
			field.value,
		)
	}
}

func appendSummaryAmountUpdateData(
	data map[string]any,
	tableName, prefix string,
	amount Amount,
) {
	fields := []struct {
		column string
		value  float64
	}{
		{column: "input_amount", value: amount.InputAmount},
		{column: "image_input_amount", value: amount.ImageInputAmount},
		{column: "audio_input_amount", value: amount.AudioInputAmount},
		{column: "output_amount", value: amount.OutputAmount},
		{column: "image_output_amount", value: amount.ImageOutputAmount},
		{column: "cached_amount", value: amount.CachedAmount},
		{column: "cache_creation_amount", value: amount.CacheCreationAmount},
		{column: "web_search_amount", value: amount.WebSearchAmount},
		{column: "used_amount", value: amount.UsedAmount},
	}

	for _, field := range fields {
		if field.value <= 0 {
			continue
		}

		columnName := prefix + field.column
		data[columnName] = gorm.Expr(
			fmt.Sprintf("COALESCE(%s.%s, 0) + ?", tableName, columnName),
			field.value,
		)
	}
}

func (d *SummaryData) buildUpdateData(tableName string) map[string]any {
	data := map[string]any{}

	appendSummaryAmountUpdateData(data, tableName, "", d.Amount)
	appendSummaryCountUpdateData(data, tableName, "", d.Count)

	if d.TotalTimeMilliseconds > 0 {
		data["total_time_milliseconds"] = gorm.Expr(
			tableName+".total_time_milliseconds + ?",
			d.TotalTimeMilliseconds,
		)
	}

	if d.TotalTTFBMilliseconds > 0 {
		data["total_ttfb_milliseconds"] = gorm.Expr(
			tableName+".total_ttfb_milliseconds + ?",
			d.TotalTTFBMilliseconds,
		)
	}

	appendSummaryUsageUpdateData(data, tableName, "", d.Usage)
	appendSummaryCountUpdateData(data, tableName, "service_tier_flex_", d.ServiceTierFlex.Count)
	appendSummaryUsageUpdateData(data, tableName, "service_tier_flex_", d.ServiceTierFlex.Usage)
	appendSummaryAmountUpdateData(data, tableName, "service_tier_flex_", d.ServiceTierFlex.Amount)

	if d.ServiceTierFlex.TotalTimeMilliseconds > 0 {
		data["service_tier_flex_total_time_milliseconds"] = gorm.Expr(
			tableName+".service_tier_flex_total_time_milliseconds + ?",
			d.ServiceTierFlex.TotalTimeMilliseconds,
		)
	}

	if d.ServiceTierFlex.TotalTTFBMilliseconds > 0 {
		data["service_tier_flex_total_ttfb_milliseconds"] = gorm.Expr(
			tableName+".service_tier_flex_total_ttfb_milliseconds + ?",
			d.ServiceTierFlex.TotalTTFBMilliseconds,
		)
	}

	appendSummaryCountUpdateData(
		data,
		tableName,
		"service_tier_priority_",
		d.ServiceTierPriority.Count,
	)
	appendSummaryUsageUpdateData(
		data,
		tableName,
		"service_tier_priority_",
		d.ServiceTierPriority.Usage,
	)
	appendSummaryAmountUpdateData(
		data,
		tableName,
		"service_tier_priority_",
		d.ServiceTierPriority.Amount,
	)

	if d.ServiceTierPriority.TotalTimeMilliseconds > 0 {
		data["service_tier_priority_total_time_milliseconds"] = gorm.Expr(
			tableName+".service_tier_priority_total_time_milliseconds + ?",
			d.ServiceTierPriority.TotalTimeMilliseconds,
		)
	}

	if d.ServiceTierPriority.TotalTTFBMilliseconds > 0 {
		data["service_tier_priority_total_ttfb_milliseconds"] = gorm.Expr(
			tableName+".service_tier_priority_total_ttfb_milliseconds + ?",
			d.ServiceTierPriority.TotalTTFBMilliseconds,
		)
	}

	appendSummaryCountUpdateData(data, tableName, "claude_long_context_", d.ClaudeLongContext.Count)
	appendSummaryUsageUpdateData(data, tableName, "claude_long_context_", d.ClaudeLongContext.Usage)
	appendSummaryAmountUpdateData(
		data,
		tableName,
		"claude_long_context_",
		d.ClaudeLongContext.Amount,
	)

	if d.ClaudeLongContext.TotalTimeMilliseconds > 0 {
		data["claude_long_context_total_time_milliseconds"] = gorm.Expr(
			tableName+".claude_long_context_total_time_milliseconds + ?",
			d.ClaudeLongContext.TotalTimeMilliseconds,
		)
	}

	if d.ClaudeLongContext.TotalTTFBMilliseconds > 0 {
		data["claude_long_context_total_ttfb_milliseconds"] = gorm.Expr(
			tableName+".claude_long_context_total_ttfb_milliseconds + ?",
			d.ClaudeLongContext.TotalTTFBMilliseconds,
		)
	}

	return data
}

func (l *Summary) BeforeCreate(_ *gorm.DB) (err error) {
	if l.Unique.ChannelID == 0 {
		return errors.New("channel id is required")
	}

	if l.Unique.Model == "" {
		return errors.New("model is required")
	}

	if l.Unique.HourTimestamp == 0 {
		return errors.New("hour timestamp is required")
	}

	if err := validateHourTimestamp(l.Unique.HourTimestamp); err != nil {
		return err
	}

	return err
}

var hourTimestampDivisor = int64(time.Hour.Seconds())

func validateHourTimestamp(hourTimestamp int64) error {
	if hourTimestamp%hourTimestampDivisor != 0 {
		return errors.New("hour timestamp must be an exact hour")
	}
	return nil
}

func CreateSummaryIndexs(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_summary_channel_hour ON summaries (channel_id, hour_timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_summary_model_hour ON summaries (model, hour_timestamp DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func UpsertSummary(unique SummaryUnique, data SummaryData) error {
	err := validateHourTimestamp(unique.HourTimestamp)
	if err != nil {
		return err
	}

	for range 3 {
		result := LogDB.
			Model(&Summary{}).
			Where(
				"channel_id = ? AND model = ? AND hour_timestamp = ?",
				unique.ChannelID,
				unique.Model,
				unique.HourTimestamp,
			).
			Updates(data.buildUpdateData("summaries"))

		err = result.Error
		if err != nil {
			return err
		}

		if result.RowsAffected > 0 {
			return nil
		}

		err = createSummary(unique, data)
		if err == nil {
			return nil
		}

		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}

	return err
}

func createSummary(unique SummaryUnique, data SummaryData) error {
	return LogDB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "channel_id"},
				{Name: "model"},
				{Name: "hour_timestamp"},
			},
			DoUpdates: clause.Assignments(data.buildUpdateData("summaries")),
		}).
		Create(&Summary{
			Unique: unique,
			Data:   data,
		}).Error
}

func getChartData(
	start, end time.Time,
	channelID int,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]ChartData, error) {
	query := LogDB.Model(&Summary{})

	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
	}

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	selectFields := fields.BuildSelectFields("hour_timestamp")

	query = query.
		Select(selectFields).
		Group("timestamp")

	var chartData []ChartData

	err := query.Find(&chartData).Error
	if err != nil {
		return nil, err
	}

	if len(chartData) > 0 && timeSpan != TimeSpanHour {
		chartData = aggregateDataToSpan(chartData, timeSpan, timezone)
	}

	slices.SortFunc(chartData, func(a, b ChartData) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return chartData, nil
}

func getGroupChartData(
	group string,
	start, end time.Time,
	tokenName, modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) ([]ChartData, error) {
	query := LogDB.Model(&GroupSummary{})
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	selectFields := fields.BuildSelectFields("hour_timestamp")

	query = query.
		Select(selectFields).
		Group("timestamp")

	var chartData []ChartData

	err := query.Find(&chartData).Error
	if err != nil {
		return nil, err
	}

	if len(chartData) > 0 && timeSpan != TimeSpanHour {
		chartData = aggregateDataToSpan(chartData, timeSpan, timezone)
	}

	slices.SortFunc(chartData, func(a, b ChartData) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return chartData, nil
}

func GetUsedChannels(start, end time.Time) ([]int, error) {
	return getLogGroupByValues[int]("channel_id", 0, start, end)
}

func GetUsedModels(channelID int, start, end time.Time) ([]string, error) {
	return getLogGroupByValues[string]("model", channelID, start, end)
}

func GetGroupUsedModels(group, tokenName string, start, end time.Time) ([]string, error) {
	return getGroupLogGroupByValues[string]("model", group, tokenName, start, end)
}

func GetGroupUsedTokenNames(group string, start, end time.Time) ([]string, error) {
	return getGroupLogGroupByValues[string]("token_name", group, "", start, end)
}

func getLogGroupByValues[T cmp.Ordered](
	field string,
	channelID int,
	start, end time.Time,
) ([]T, error) {
	type Result struct {
		Value        T
		UsedAmount   float64
		RequestCount int64
	}

	var results []Result

	var query *gorm.DB

	query = LogDB.
		Model(&Summary{})

	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	err := query.
		Select(
			field + " as value, SUM(request_count) as request_count, SUM(used_amount) as used_amount",
		).
		Group(field).
		Find(&results).Error
	if err != nil {
		return nil, err
	}

	slices.SortFunc(results, func(a, b Result) int {
		if a.UsedAmount != b.UsedAmount {
			return cmp.Compare(b.UsedAmount, a.UsedAmount)
		}

		if a.RequestCount != b.RequestCount {
			return cmp.Compare(b.RequestCount, a.RequestCount)
		}

		return cmp.Compare(a.Value, b.Value)
	})

	values := make([]T, len(results))
	for i, result := range results {
		values[i] = result.Value
	}

	return values, nil
}

func getGroupLogGroupByValues[T cmp.Ordered](
	field, group, tokenName string,
	start, end time.Time,
) ([]T, error) {
	type Result struct {
		Value        T
		UsedAmount   float64
		RequestCount int64
	}

	var results []Result

	query := LogDB.
		Model(&GroupSummary{})
	if group != "" {
		query = query.Where("group_id = ?", group)
	}

	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("hour_timestamp BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("hour_timestamp >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("hour_timestamp <= ?", end.Unix())
	}

	err := query.
		Select(
			field + " as value, SUM(request_count) as request_count, SUM(used_amount) as used_amount",
		).
		Group(field).
		Find(&results).Error
	if err != nil {
		return nil, err
	}

	slices.SortFunc(results, func(a, b Result) int {
		if a.UsedAmount != b.UsedAmount {
			return cmp.Compare(b.UsedAmount, a.UsedAmount)
		}

		if a.RequestCount != b.RequestCount {
			return cmp.Compare(b.RequestCount, a.RequestCount)
		}

		return cmp.Compare(a.Value, b.Value)
	})

	values := make([]T, len(results))
	for i, result := range results {
		values[i] = result.Value
	}

	return values, nil
}

type ChartData struct {
	Timestamp int64 `json:"timestamp"`
	SummaryDataSet
	ServiceTierFlex     SummaryDataSet `json:"service_tier_flex,omitempty"     gorm:"embedded;embeddedPrefix:service_tier_flex_"`
	ServiceTierPriority SummaryDataSet `json:"service_tier_priority,omitempty" gorm:"embedded;embeddedPrefix:service_tier_priority_"`
	ClaudeLongContext   SummaryDataSet `json:"claude_long_context,omitempty"   gorm:"embedded;embeddedPrefix:claude_long_context_"`
}

type DashboardResponse struct {
	ChartData []ChartData `json:"chart_data"`

	RPM int64 `json:"rpm"`
	TPM int64 `json:"tpm"`

	MaxRPM int64 `json:"max_rpm"`
	MaxTPM int64 `json:"max_tpm"`

	TotalCount int64 `json:"total_count"` // use Count.RequestCount instead
	SummaryDataSet
	ServiceTierFlex     SummaryDataSet `json:"service_tier_flex,omitempty"     gorm:"embedded;embeddedPrefix:service_tier_flex_"`
	ServiceTierPriority SummaryDataSet `json:"service_tier_priority,omitempty" gorm:"embedded;embeddedPrefix:service_tier_priority_"`
	ClaudeLongContext   SummaryDataSet `json:"claude_long_context,omitempty"   gorm:"embedded;embeddedPrefix:claude_long_context_"`

	Channels []int    `json:"channels,omitempty"`
	Models   []string `json:"models,omitempty"`
}

type GroupDashboardResponse struct {
	DashboardResponse
	TokenNames []string `json:"token_names"`
}

type TimeSpanType string

const (
	TimeSpanMinute TimeSpanType = "minute"
	TimeSpanHour   TimeSpanType = "hour"
	TimeSpanDay    TimeSpanType = "day"
	TimeSpanMonth  TimeSpanType = "month"
)

func aggregateDataToSpan(
	data []ChartData,
	timeSpan TimeSpanType,
	timezone *time.Location,
) []ChartData {
	dataMap := make(map[int64]ChartData)

	if timezone == nil {
		timezone = time.Local
	}

	for _, data := range data {
		// Convert timestamp to time in the specified timezone
		t := time.Unix(data.Timestamp, 0).In(timezone)
		// Get the start of the day in the specified timezone
		var timestamp int64
		switch timeSpan {
		case TimeSpanMonth:
			startOfMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, timezone)
			timestamp = startOfMonth.Unix()
		case TimeSpanDay:
			startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timezone)
			timestamp = startOfDay.Unix()
		case TimeSpanHour:
			startOfHour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, timezone)
			timestamp = startOfHour.Unix()
		case TimeSpanMinute:
			startOfHour := time.Date(
				t.Year(),
				t.Month(),
				t.Day(),
				t.Hour(),
				t.Minute(),
				0,
				0,
				timezone,
			)
			timestamp = startOfHour.Unix()
		}

		currentData, exists := dataMap[timestamp]
		if !exists {
			currentData = ChartData{
				Timestamp: timestamp,
			}
		}

		currentData.Add(data.SummaryDataSet)
		currentData.ServiceTierFlex.Add(data.ServiceTierFlex)
		currentData.ServiceTierPriority.Add(data.ServiceTierPriority)
		currentData.ClaudeLongContext.Add(data.ClaudeLongContext)

		dataMap[timestamp] = currentData
	}

	result := make([]ChartData, 0, len(dataMap))
	for _, data := range dataMap {
		result = append(result, data)
	}

	return result
}

func sumDashboardResponse(chartData []ChartData) DashboardResponse {
	dashboardResponse := DashboardResponse{
		ChartData: chartData,
	}

	for _, data := range chartData {
		dashboardResponse.Add(data.SummaryDataSet)
		dashboardResponse.TotalCount = dashboardResponse.RequestCount
		dashboardResponse.ServiceTierFlex.Add(data.ServiceTierFlex)
		dashboardResponse.ServiceTierPriority.Add(data.ServiceTierPriority)
		dashboardResponse.ClaudeLongContext.Add(data.ClaudeLongContext)
	}

	return dashboardResponse
}

func GetDashboardData(
	start,
	end time.Time,
	modelName string,
	channelID int,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*DashboardResponse, error) {
	if timeSpan == TimeSpanMinute {
		return getDashboardDataMinute(start, end, modelName, channelID, timeSpan, timezone, fields)
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	var (
		chartData  []ChartData
		channels   []int
		models     []string
		currentRPM int64
		currentTPM int64
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		chartData, err = getChartData(start, end, channelID, modelName, timeSpan, timezone, fields)
		return err
	})

	g.Go(func() error {
		var err error

		channels, err = GetUsedChannels(start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetUsedModels(channelID, start, end)
		return err
	})

	g.Go(func() error {
		currentRPM, currentTPM = getCurrentRPM(channelID, modelName)
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Channels = channels
	dashboardResponse.Models = models
	dashboardResponse.RPM = currentRPM
	dashboardResponse.TPM = currentTPM

	return &dashboardResponse, nil
}

func GetGroupDashboardData(
	group string,
	start, end time.Time,
	tokenName string,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
	fields SummarySelectFields,
) (*GroupDashboardResponse, error) {
	if timeSpan == TimeSpanMinute {
		return getGroupDashboardDataMinute(
			group,
			start,
			end,
			tokenName,
			modelName,
			timeSpan,
			timezone,
			fields,
		)
	}

	if group == "" {
		return nil, errors.New("group is required")
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	var (
		chartData  []ChartData
		tokenNames []string
		models     []string
		currentRPM int64
		currentTPM int64
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		chartData, err = getGroupChartData(
			group,
			start,
			end,
			tokenName,
			modelName,
			timeSpan,
			timezone,
			fields,
		)

		return err
	})

	g.Go(func() error {
		var err error

		tokenNames, err = GetGroupUsedTokenNames(group, start, end)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupUsedModels(group, tokenName, start, end)
		return err
	})

	g.Go(func() error {
		currentRPM, currentTPM = getGroupCurrentRPM(group, tokenName, modelName)
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Models = models
	dashboardResponse.RPM = currentRPM
	dashboardResponse.TPM = currentTPM

	return &GroupDashboardResponse{
		DashboardResponse: dashboardResponse,
		TokenNames:        tokenNames,
	}, nil
}
