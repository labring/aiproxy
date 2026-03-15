//go:build enterprise

package analytics

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExportDepartmentReport generates an Excel report with department summaries and user rankings.
func ExportDepartmentReport(
	departments []DepartmentSummary,
	userRanking []UserRankingEntry,
	startTime, endTime time.Time,
) (*excelize.File, error) {
	f := excelize.NewFile()

	// Department Summary sheet
	deptSheet := "Department Summary"

	idx, err := f.NewSheet(deptSheet)
	if err != nil {
		return nil, fmt.Errorf("create department sheet: %w", err)
	}

	f.SetActiveSheet(idx)

	// Header row
	deptHeaders := []string{
		"Department ID", "Department Name", "Member Count",
		"Request Count", "Used Amount", "Total Tokens",
		"Input Tokens", "Output Tokens",
	}
	for i, h := range deptHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(deptSheet, cell, h)
	}

	// Data rows
	for i, d := range departments {
		row := i + 2
		f.SetCellValue(deptSheet, cellName(1, row), d.DepartmentID)
		f.SetCellValue(deptSheet, cellName(2, row), d.DepartmentName)
		f.SetCellValue(deptSheet, cellName(3, row), d.MemberCount)
		f.SetCellValue(deptSheet, cellName(4, row), d.RequestCount)
		f.SetCellValue(deptSheet, cellName(5, row), d.UsedAmount)
		f.SetCellValue(deptSheet, cellName(6, row), d.TotalTokens)
		f.SetCellValue(deptSheet, cellName(7, row), d.InputTokens)
		f.SetCellValue(deptSheet, cellName(8, row), d.OutputTokens)
	}

	// User Ranking sheet
	userSheet := "User Ranking"

	if _, err := f.NewSheet(userSheet); err != nil {
		return nil, fmt.Errorf("create user ranking sheet: %w", err)
	}

	userHeaders := []string{
		"User Name", "Group ID", "Department ID",
		"Used Amount", "Request Count", "Total Tokens",
	}
	for i, h := range userHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(userSheet, cell, h)
	}

	for i, u := range userRanking {
		row := i + 2
		f.SetCellValue(userSheet, cellName(1, row), u.UserName)
		f.SetCellValue(userSheet, cellName(2, row), u.GroupID)
		f.SetCellValue(userSheet, cellName(3, row), u.DepartmentID)
		f.SetCellValue(userSheet, cellName(4, row), u.UsedAmount)
		f.SetCellValue(userSheet, cellName(5, row), u.RequestCount)
		f.SetCellValue(userSheet, cellName(6, row), u.TotalTokens)
	}

	// Remove default "Sheet1"
	f.DeleteSheet("Sheet1")

	return f, nil
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
