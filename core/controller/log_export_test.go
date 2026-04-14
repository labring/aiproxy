package controller

import (
	"strings"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/model"
)

func TestBuildLogExportCSVFormatsTimezoneAndSanitizesCells(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	content, err := buildLogExportCSV([]*model.Log{
		{
			ID:        1,
			CreatedAt: time.Date(2026, time.April, 14, 12, 0, 0, 0, time.UTC),
			RequestAt: time.Date(2026, time.April, 14, 12, 0, 1, 0, time.UTC),
			GroupID:   "demo",
			TokenID:   2,
			TokenName: "token-a",
			ChannelID: 3,
			Model:     "gpt-test",
			RequestID: model.EmptyNullString("req-1"),
			Content:   model.EmptyNullString("=sum(1,1)"),
			RequestDetail: &model.RequestDetail{
				RequestBody:  "@danger",
				ResponseBody: "-payload",
			},
		},
	}, location, "Asia/Shanghai", true)
	if err != nil {
		t.Fatalf("build csv: %v", err)
	}

	csvText := string(content)
	if !strings.HasPrefix(csvText, "\xEF\xBB\xBFid,created_at") {
		sample := csvText
		if len(sample) > 32 {
			sample = sample[:32]
		}

		t.Fatalf("expected utf-8 bom and header, got %q", sample)
	}

	if !strings.Contains(csvText, "2026-04-14 20:00:00.000 CST") {
		t.Fatalf("expected created_at to be formatted in Asia/Shanghai timezone, got %q", csvText)
	}

	if !strings.Contains(csvText, "'=sum(1,1)") {
		t.Fatalf("expected content to be sanitized for csv formulas, got %q", csvText)
	}

	if !strings.Contains(csvText, "'@danger") || !strings.Contains(csvText, "'-payload") {
		t.Fatalf("expected request and response bodies to be sanitized, got %q", csvText)
	}
}

func TestBuildLogExportCSVExcludesChannelByDefaultForGroupExport(t *testing.T) {
	content, err := buildLogExportCSV([]*model.Log{
		{
			ID:        1,
			CreatedAt: time.Date(2026, time.April, 14, 12, 0, 0, 0, time.UTC),
			RequestAt: time.Date(2026, time.April, 14, 12, 0, 1, 0, time.UTC),
			ChannelID: 9,
			Model:     "gpt-test",
		},
	}, time.UTC, "UTC", false)
	if err != nil {
		t.Fatalf("build csv: %v", err)
	}

	csvText := string(content)
	if strings.Contains(csvText, ",channel,") {
		t.Fatalf("expected channel header to be excluded by default, got %q", csvText)
	}

	if strings.Contains(csvText, ",9,") {
		t.Fatalf("expected channel value to be excluded by default, got %q", csvText)
	}
}

func TestBuildLogExportCSVIncludesChannelWhenRequested(t *testing.T) {
	content, err := buildLogExportCSV([]*model.Log{
		{
			ID:        1,
			CreatedAt: time.Date(2026, time.April, 14, 12, 0, 0, 0, time.UTC),
			RequestAt: time.Date(2026, time.April, 14, 12, 0, 1, 0, time.UTC),
			ChannelID: 9,
			Model:     "gpt-test",
		},
	}, time.UTC, "UTC", true)
	if err != nil {
		t.Fatalf("build csv: %v", err)
	}

	csvText := string(content)
	if !strings.Contains(csvText, ",channel,") {
		t.Fatalf("expected channel header to be included, got %q", csvText)
	}

	if !strings.Contains(csvText, ",9,gpt-test,") {
		t.Fatalf("expected channel value to be included, got %q", csvText)
	}
}

func TestSanitizeFilename(t *testing.T) {
	filename := sanitizeFilename("group/a b?.csv")
	if filename != "group_a_b_.csv" {
		t.Fatalf("unexpected sanitized filename: %q", filename)
	}
}
