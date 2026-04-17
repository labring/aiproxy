//go:build enterprise

package analytics

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExportAnalyticsReport generates a multi-sheet Excel report.
func ExportAnalyticsReport(
	departments []DepartmentSummary,
	userRanking []UserRankingEntry,
	modelDist []ModelDistributionEntry,
	startTime, endTime time.Time,
) (*excelize.File, error) {
	f := excelize.NewFile()

	// Header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#E8E8F4"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#AAAAAA", Style: 1},
		},
	})

	// Number format style for amounts
	amountStyle, _ := f.NewStyle(&excelize.Style{
		NumFmt: 4, // #,##0.00
	})

	// ── Sheet 1: Summary ──
	summarySheet := "Summary"
	f.SetSheetName("Sheet1", summarySheet)
	addSummarySheet(f, summarySheet, departments, startTime, endTime, headerStyle)

	// ── Sheet 2: Department Details ──
	deptSheet := "Department Details"
	if _, err := f.NewSheet(deptSheet); err != nil {
		return nil, fmt.Errorf("create department sheet: %w", err)
	}

	addDepartmentSheet(f, deptSheet, departments, headerStyle, amountStyle)

	// ── Sheet 3: User Ranking ──
	userSheet := "User Ranking"
	if _, err := f.NewSheet(userSheet); err != nil {
		return nil, fmt.Errorf("create user ranking sheet: %w", err)
	}

	addUserRankingSheet(f, userSheet, userRanking, headerStyle, amountStyle)

	// ── Sheet 4: Model Breakdown ──
	modelSheet := "Model Breakdown"
	if _, err := f.NewSheet(modelSheet); err != nil {
		return nil, fmt.Errorf("create model sheet: %w", err)
	}

	addModelBreakdownSheet(f, modelSheet, modelDist, headerStyle, amountStyle)

	return f, nil
}

func addSummarySheet(
	f *excelize.File,
	sheet string,
	departments []DepartmentSummary,
	startTime, endTime time.Time,
	headerStyle int,
) {
	headers := []string{"Metric", "Value"}
	writeHeaderRow(f, sheet, headers, headerStyle)

	// Calculate totals
	var (
		totalReqs   int64
		totalTokens int64
		totalAmount float64
	)

	activeUsers := 0
	for _, d := range departments {
		totalReqs += d.RequestCount
		totalTokens += d.TotalTokens
		totalAmount += d.UsedAmount
		activeUsers += d.ActiveUsers
	}

	data := [][]any{
		{
			"Report Period",
			fmt.Sprintf("%s ~ %s", startTime.Format(time.DateOnly), endTime.Format(time.DateOnly)),
		},
		{"Generated At", time.Now().Format(time.DateTime)},
		{"Active Departments", len(departments)},
		{"Active Users", activeUsers},
		{"Total Requests", totalReqs},
		{"Total Tokens", totalTokens},
		{"Total Amount ($)", fmt.Sprintf("%.4f", totalAmount)},
	}

	if totalReqs > 0 {
		data = append(data,
			[]any{"Avg Cost per Request ($)", fmt.Sprintf("%.6f", totalAmount/float64(totalReqs))},
			[]any{"Avg Tokens per Request", totalTokens / totalReqs},
		)
	}

	for i, row := range data {
		for j, val := range row {
			f.SetCellValue(sheet, cellName(j+1, i+2), val)
		}
	}

	f.SetColWidth(sheet, "A", "A", 28)
	f.SetColWidth(sheet, "B", "B", 30)
}

func addDepartmentSheet(
	f *excelize.File,
	sheet string,
	departments []DepartmentSummary,
	headerStyle, amountStyle int,
) {
	headers := []string{
		"Rank", "Department", "Active Users", "Members",
		"Requests", "Amount ($)", "Total Tokens",
		"Input Tokens", "Output Tokens", "Success Rate (%)", "Avg Cost ($)",
	}
	writeHeaderRow(f, sheet, headers, headerStyle)

	// Sort by used_amount descending
	sorted := make([]DepartmentSummary, len(departments))
	copy(sorted, departments)
	sortDeptByAmount(sorted)

	for i, d := range sorted {
		row := i + 2
		f.SetCellValue(sheet, cellName(1, row), i+1)
		f.SetCellValue(sheet, cellName(2, row), d.DepartmentName)
		f.SetCellValue(sheet, cellName(3, row), d.ActiveUsers)
		f.SetCellValue(sheet, cellName(4, row), d.MemberCount)
		f.SetCellValue(sheet, cellName(5, row), d.RequestCount)
		f.SetCellValue(sheet, cellName(6, row), d.UsedAmount)
		f.SetCellValue(sheet, cellName(7, row), d.TotalTokens)
		f.SetCellValue(sheet, cellName(8, row), d.InputTokens)
		f.SetCellValue(sheet, cellName(9, row), d.OutputTokens)
		f.SetCellValue(sheet, cellName(10, row), fmt.Sprintf("%.1f", d.SuccessRate))
		f.SetCellValue(sheet, cellName(11, row), fmt.Sprintf("%.6f", d.AvgCost))

		// Apply amount formatting
		f.SetCellStyle(sheet, cellName(6, row), cellName(6, row), amountStyle)
	}

	f.SetColWidth(sheet, "A", "A", 6)
	f.SetColWidth(sheet, "B", "B", 20)
	f.SetColWidth(sheet, "F", "F", 14)
}

func addUserRankingSheet(
	f *excelize.File,
	sheet string,
	ranking []UserRankingEntry,
	headerStyle, amountStyle int,
) {
	headers := []string{
		"Rank", "User Name", "Department",
		"Requests", "Amount ($)", "Total Tokens",
		"Input Tokens", "Output Tokens", "Models Used", "Success Rate (%)",
	}
	writeHeaderRow(f, sheet, headers, headerStyle)

	for i, u := range ranking {
		row := i + 2
		f.SetCellValue(sheet, cellName(1, row), u.Rank)
		f.SetCellValue(sheet, cellName(2, row), u.UserName)
		f.SetCellValue(sheet, cellName(3, row), u.DepartmentName)
		f.SetCellValue(sheet, cellName(4, row), u.RequestCount)
		f.SetCellValue(sheet, cellName(5, row), u.UsedAmount)
		f.SetCellValue(sheet, cellName(6, row), u.TotalTokens)
		f.SetCellValue(sheet, cellName(7, row), u.InputTokens)
		f.SetCellValue(sheet, cellName(8, row), u.OutputTokens)
		f.SetCellValue(sheet, cellName(9, row), u.UniqueModels)
		f.SetCellValue(sheet, cellName(10, row), fmt.Sprintf("%.1f", u.SuccessRate))

		f.SetCellStyle(sheet, cellName(5, row), cellName(5, row), amountStyle)
	}

	f.SetColWidth(sheet, "A", "A", 6)
	f.SetColWidth(sheet, "B", "B", 18)
	f.SetColWidth(sheet, "C", "C", 18)
	f.SetColWidth(sheet, "E", "E", 14)
}

func addModelBreakdownSheet(
	f *excelize.File,
	sheet string,
	modelDist []ModelDistributionEntry,
	headerStyle, amountStyle int,
) {
	headers := []string{
		"Model", "Requests", "Amount ($)", "Total Tokens",
		"Input Tokens", "Output Tokens", "Unique Users", "Share (%)",
	}
	writeHeaderRow(f, sheet, headers, headerStyle)

	for i, m := range modelDist {
		row := i + 2
		f.SetCellValue(sheet, cellName(1, row), m.Model)
		f.SetCellValue(sheet, cellName(2, row), m.RequestCount)
		f.SetCellValue(sheet, cellName(3, row), m.UsedAmount)
		f.SetCellValue(sheet, cellName(4, row), m.TotalTokens)
		f.SetCellValue(sheet, cellName(5, row), m.InputTokens)
		f.SetCellValue(sheet, cellName(6, row), m.OutputTokens)
		f.SetCellValue(sheet, cellName(7, row), m.UniqueUsers)
		f.SetCellValue(sheet, cellName(8, row), fmt.Sprintf("%.1f", m.Percentage))

		f.SetCellStyle(sheet, cellName(3, row), cellName(3, row), amountStyle)
	}

	f.SetColWidth(sheet, "A", "A", 30)
	f.SetColWidth(sheet, "C", "C", 14)
}

func writeHeaderRow(f *excelize.File, sheet string, headers []string, style int) {
	for i, h := range headers {
		cell := cellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, style)
	}
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

func sortDeptByAmount(depts []DepartmentSummary) {
	for i := 1; i < len(depts); i++ {
		for j := i; j > 0 && depts[j].UsedAmount > depts[j-1].UsedAmount; j-- {
			depts[j], depts[j-1] = depts[j-1], depts[j]
		}
	}
}
