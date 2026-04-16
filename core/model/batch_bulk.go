package model

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// summaryDataSetFieldValues returns all field values of a SummaryDataSet in the same order
// as baseCountSummaryFields + baseUsageSummaryFields + baseAmountSummaryFields + baseTimeSummaryFields.
// This is the canonical ordering used by allSummaryFields.
func summaryDataSetFieldValues(s *SummaryDataSet) []any {
	return []any{
		// Count fields (12) — same order as baseCountSummaryFields
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
		// Usage fields (10) — same order as baseUsageSummaryFields
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
		// Amount fields (9) — same order as baseAmountSummaryFields
		s.InputAmount,
		s.ImageInputAmount,
		s.AudioInputAmount,
		s.OutputAmount,
		s.ImageOutputAmount,
		s.CachedAmount,
		s.CacheCreationAmount,
		s.WebSearchAmount,
		s.UsedAmount,
		// Time fields (2) — same order as baseTimeSummaryFields
		s.TotalTimeMilliseconds,
		s.TotalTTFBMilliseconds,
	}
}

// summaryDataFieldValues returns all field values of a SummaryData in the same order
// as allSummaryFields: base fields, then service_tier_flex_, service_tier_priority_,
// claude_long_context_ prefixed fields.
func summaryDataFieldValues(d *SummaryData) []any {
	values := make([]any, 0, fieldsPerDataSet*4)
	values = append(values, summaryDataSetFieldValues(&d.SummaryDataSet)...)
	// serviceTierPrefixes order: service_tier_flex, service_tier_priority
	values = append(values, summaryDataSetFieldValues(&d.ServiceTierFlex)...)
	values = append(values, summaryDataSetFieldValues(&d.ServiceTierPriority)...)
	// extraSummaryPrefixes order: claude_long_context
	values = append(values, summaryDataSetFieldValues(&d.ClaudeLongContext)...)
	return values
}

// fieldsPerDataSet is the number of fields in one SummaryDataSet
const fieldsPerDataSet = 12 + 10 + 9 + 2 // count + usage + amount + time = 33

func init() {
	// Verify that summaryDataFieldValues produces the correct number of values
	// matching allSummaryFields. This catches any field additions that are not
	// reflected in summaryDataSetFieldValues.
	expected := len(allSummaryFields)
	got := fieldsPerDataSet * 4 // base + 3 prefixed sets
	if expected != got {
		panic(fmt.Sprintf(
			"batch_bulk: allSummaryFields has %d fields but summaryDataFieldValues produces %d values; "+
				"update summaryDataSetFieldValues when adding new summary fields",
			expected, got,
		))
	}
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
