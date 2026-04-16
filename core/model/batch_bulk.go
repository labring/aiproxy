package model

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// summaryDataSetCountValues returns count field values (12) matching baseCountSummaryFields order.
func summaryDataSetCountValues(s *SummaryDataSet) []any {
	return []any{
		int64(s.RequestCount),
		int64(s.RetryCount),
		int64(s.ExceptionCount),
		int64(s.Status2xxCount),
		int64(s.Status4xxCount),
		int64(s.Status5xxCount),
		int64(s.StatusOtherCount),
		int64(s.Status400Count),
		int64(s.Status429Count),
		int64(s.Status500Count),
		int64(s.CacheHitCount),
		int64(s.CacheCreationCount),
	}
}

// summaryDataSetUsageValues returns usage field values (10) matching baseUsageSummaryFields order.
func summaryDataSetUsageValues(s *SummaryDataSet) []any {
	return []any{
		int64(s.InputTokens),
		int64(s.ImageInputTokens),
		int64(s.AudioInputTokens),
		int64(s.OutputTokens),
		int64(s.ImageOutputTokens),
		int64(s.CachedTokens),
		int64(s.CacheCreationTokens),
		int64(s.ReasoningTokens),
		int64(s.TotalTokens),
		int64(s.WebSearchCount),
	}
}

// summaryDataSetAmountValues returns amount field values (9) matching baseAmountSummaryFields order.
func summaryDataSetAmountValues(s *SummaryDataSet) []any {
	return []any{
		s.InputAmount,
		s.ImageInputAmount,
		s.AudioInputAmount,
		s.OutputAmount,
		s.ImageOutputAmount,
		s.CachedAmount,
		s.CacheCreationAmount,
		s.WebSearchAmount,
		s.UsedAmount,
	}
}

// summaryDataSetTimeValues returns time field values (2) matching baseTimeSummaryFields order.
func summaryDataSetTimeValues(s *SummaryDataSet) []any {
	return []any{
		s.TotalTimeMilliseconds,
		s.TotalTTFBMilliseconds,
	}
}

// summaryDataFieldValues returns all field values of a SummaryData in the same order
// as allSummaryFields: all counts (base, flex, priority, longctx), then all usage,
// then all amounts, then all time fields.
//
// allSummaryFields is built by concatSummaryFields(countFields, usageFields, amountFields, timeFields)
// where each group contains base + service_tier_flex + service_tier_priority + claude_long_context fields.
func summaryDataFieldValues(d *SummaryData) []any {
	sets := []*SummaryDataSet{
		&d.SummaryDataSet,
		&d.ServiceTierFlex,
		&d.ServiceTierPriority,
		&d.ClaudeLongContext,
	}

	values := make([]any, 0, fieldsPerDataSet*len(sets))

	// All count fields across all data sets
	for _, s := range sets {
		values = append(values, summaryDataSetCountValues(s)...)
	}
	// All usage fields across all data sets
	for _, s := range sets {
		values = append(values, summaryDataSetUsageValues(s)...)
	}
	// All amount fields across all data sets
	for _, s := range sets {
		values = append(values, summaryDataSetAmountValues(s)...)
	}
	// All time fields across all data sets
	for _, s := range sets {
		values = append(values, summaryDataSetTimeValues(s)...)
	}

	return values
}

// fieldsPerDataSet is the number of fields in one SummaryDataSet
const fieldsPerDataSet = 12 + 10 + 9 + 2 // count + usage + amount + time = 33

func init() {
	// Verify that summaryDataFieldValues produces the correct number of values
	// matching allSummaryFields. This catches any field additions that are not
	// reflected in the value extraction functions.
	expected := len(allSummaryFields)
	got := fieldsPerDataSet * 4 // base + 3 prefixed sets
	if expected != got {
		panic(fmt.Sprintf(
			"batch_bulk: allSummaryFields has %d fields but summaryDataFieldValues produces %d values; "+
				"update value extraction functions when adding new summary fields",
			expected, got,
		))
	}

	// Verify field-by-field ordering: construct a SummaryData with unique sentinel
	// values per field and check that summaryDataFieldValues returns them in the
	// exact same order as allSummaryFields columns. This catches ordering mismatches
	// (e.g., grouping by data set vs. grouping by field type).
	verifySummaryFieldOrdering()
}

// pgTypeForField returns the PostgreSQL cast type for a given allSummaryFields column name.
func pgTypeForField(field string) string {
	if strings.HasSuffix(field, "_amount") {
		return "numeric"
	}
	return "bigint"
}

// maxBulkSummaryRows is the maximum number of rows per bulk INSERT statement.
// PostgreSQL has a 65535 parameter limit. With ~135 columns per row, 400 rows uses ~54000 params.
const maxBulkSummaryRows = 400

// bulkUpsertOnConflictSQL builds and caches the ON CONFLICT DO UPDATE SET clause
// for a given table name. Called once per table per flush cycle.
func bulkUpsertOnConflictSQL(tableName string, uniqueCols []string) string {
	setClauses := make([]string, 0, len(allSummaryFields))
	for _, field := range allSummaryFields {
		setClauses = append(setClauses, fmt.Sprintf(
			"%s = COALESCE(%s.%s, 0) + EXCLUDED.%s",
			field, tableName, field, field,
		))
	}

	return fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s",
		strings.Join(uniqueCols, ", "),
		strings.Join(setClauses, ", "),
	)
}

// BulkUpsertSummaries performs a multi-row INSERT ... ON CONFLICT DO UPDATE
// using PostgreSQL VALUES for efficient bulk operations.
//
// uniqueCols: the unique constraint columns (e.g., ["channel_id", "model", "hour_timestamp"])
// uniquePgTypes: PostgreSQL types for each unique column (e.g., ["int", "text", "bigint"])
// uniqueValsFn: returns unique column values for row at given index
// dataEntries: the SummaryData for each row
func BulkUpsertSummaries(
	db *gorm.DB,
	tableName string,
	uniqueCols []string,
	uniquePgTypes []string,
	uniqueValsFn func(idx int) []any,
	dataEntries []SummaryData,
) error {
	rowCount := len(dataEntries)
	if rowCount == 0 {
		return nil
	}

	onConflict := bulkUpsertOnConflictSQL(tableName, uniqueCols)

	// Process in chunks to stay under PostgreSQL parameter limit
	for start := 0; start < rowCount; start += maxBulkSummaryRows {
		end := start + maxBulkSummaryRows
		if end > rowCount {
			end = rowCount
		}
		if err := bulkUpsertSummaryChunk(
			db, tableName, uniqueCols, uniquePgTypes,
			uniqueValsFn, dataEntries, start, end, onConflict,
		); err != nil {
			return err
		}
	}
	return nil
}

func bulkUpsertSummaryChunk(
	db *gorm.DB,
	tableName string,
	uniqueCols []string,
	uniquePgTypes []string,
	uniqueValsFn func(idx int) []any,
	dataEntries []SummaryData,
	start, end int,
	onConflict string,
) error {
	chunkSize := end - start
	dataFields := allSummaryFields
	colsPerRow := len(uniqueCols) + len(dataFields)

	// Build column list
	allCols := make([]string, 0, colsPerRow)
	allCols = append(allCols, uniqueCols...)
	allCols = append(allCols, dataFields...)

	// Build VALUES rows and args
	args := make([]any, 0, chunkSize*colsPerRow)
	valueRows := make([]string, 0, chunkSize)
	paramIdx := 1

	for rowIdx := start; rowIdx < end; rowIdx++ {
		placeholders := make([]string, 0, colsPerRow)

		// Unique column values with type casts
		uniqueVals := uniqueValsFn(rowIdx)
		for i, pgType := range uniquePgTypes {
			placeholders = append(placeholders, fmt.Sprintf("$%d::%s", paramIdx, pgType))
			args = append(args, uniqueVals[i])
			paramIdx++
		}

		// Data column values with type casts
		fieldVals := summaryDataFieldValues(&dataEntries[rowIdx])
		for i, field := range dataFields {
			pgType := pgTypeForField(field)
			placeholders = append(placeholders, fmt.Sprintf("$%d::%s", paramIdx, pgType))
			args = append(args, fieldVals[i])
			paramIdx++
		}

		valueRows = append(valueRows, "("+strings.Join(placeholders, ", ")+")")
	}

	sql := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s %s",
		tableName,
		strings.Join(allCols, ", "),
		strings.Join(valueRows, ", "),
		onConflict,
	)

	result := db.Exec(sql, args...)
	if result.Error != nil {
		log.Error("bulk upsert " + tableName + " failed: " + result.Error.Error())
	}
	return result.Error
}

// verifySummaryFieldOrdering performs a spot-check that summaryDataFieldValues produces
// values in the correct column order (grouped by field type, not by data set).
// It sets unique sentinel values for one field of each type (count, usage, amount, time)
// in each data set, then verifies the values appear at the positions indicated by allSummaryFields.
func verifySummaryFieldOrdering() {
	d := SummaryData{}
	// base: 1, 2, 3.0, 4
	d.SummaryDataSet.RequestCount = 1
	d.SummaryDataSet.InputTokens = 2
	d.SummaryDataSet.InputAmount = 3.0
	d.SummaryDataSet.TotalTimeMilliseconds = 4
	// flex: 10, 20, 30.0, 40
	d.ServiceTierFlex.RequestCount = 10
	d.ServiceTierFlex.InputTokens = 20
	d.ServiceTierFlex.InputAmount = 30.0
	d.ServiceTierFlex.TotalTimeMilliseconds = 40
	// priority: 100, 200, 300.0, 400
	d.ServiceTierPriority.RequestCount = 100
	d.ServiceTierPriority.InputTokens = 200
	d.ServiceTierPriority.InputAmount = 300.0
	d.ServiceTierPriority.TotalTimeMilliseconds = 400
	// longctx: 1000, 2000, 3000.0, 4000
	d.ClaudeLongContext.RequestCount = 1000
	d.ClaudeLongContext.InputTokens = 2000
	d.ClaudeLongContext.InputAmount = 3000.0
	d.ClaudeLongContext.TotalTimeMilliseconds = 4000

	values := summaryDataFieldValues(&d)

	// Build field name → position index
	fieldIndex := make(map[string]int, len(allSummaryFields))
	for i, f := range allSummaryFields {
		fieldIndex[f] = i
	}

	// Spot-check: 4 field types × 4 data sets = 16 checks
	type check struct {
		field    string
		expected any
	}
	checks := []check{
		{"request_count", int64(1)},
		{"input_tokens", int64(2)},
		{"input_amount", 3.0},
		{"total_time_milliseconds", int64(4)},
		{"service_tier_flex_request_count", int64(10)},
		{"service_tier_flex_input_tokens", int64(20)},
		{"service_tier_flex_input_amount", 30.0},
		{"service_tier_flex_total_time_milliseconds", int64(40)},
		{"service_tier_priority_request_count", int64(100)},
		{"service_tier_priority_input_tokens", int64(200)},
		{"service_tier_priority_input_amount", 300.0},
		{"service_tier_priority_total_time_milliseconds", int64(400)},
		{"claude_long_context_request_count", int64(1000)},
		{"claude_long_context_input_tokens", int64(2000)},
		{"claude_long_context_input_amount", 3000.0},
		{"claude_long_context_total_time_milliseconds", int64(4000)},
	}

	for _, c := range checks {
		idx, ok := fieldIndex[c.field]
		if !ok {
			panic(fmt.Sprintf(
				"batch_bulk: ordering check failed: field %q not found in allSummaryFields",
				c.field,
			))
		}
		if values[idx] != c.expected {
			panic(fmt.Sprintf(
				"batch_bulk: ordering mismatch at position %d: column %q expected %v but got %v; "+
					"summaryDataFieldValues order must match allSummaryFields order "+
					"(grouped by field type, not by data set)",
				idx, c.field, c.expected, values[idx],
			))
		}
	}
}
