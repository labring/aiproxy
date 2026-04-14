package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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
	}, location, true, true)
	if err != nil {
		t.Fatalf("build csv: %v", err)
	}

	csvText := string(content)
	if !strings.HasPrefix(csvText, "\xEF\xBB\xBFid,end_time") {
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
	}, time.UTC, false, false)
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
	}, time.UTC, true, false)
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

func TestBuildLogExportCSVExcludesTimezoneModeAndRetryAtByDefault(t *testing.T) {
	content, err := buildLogExportCSV([]*model.Log{
		{
			ID:        1,
			CreatedAt: time.Date(2026, time.April, 14, 12, 0, 0, 0, time.UTC),
			RequestAt: time.Date(2026, time.April, 14, 12, 0, 1, 0, time.UTC),
			RetryAt:   time.Date(2026, time.April, 14, 12, 5, 0, 0, time.UTC),
			Model:     "gpt-test",
			Mode:      2,
		},
	}, time.UTC, false, false)
	if err != nil {
		t.Fatalf("build csv: %v", err)
	}

	csvText := string(content)

	headerLine := strings.SplitN(strings.TrimPrefix(csvText, "\xEF\xBB\xBF"), "\n", 2)[0]
	if strings.Contains(","+headerLine+",", ",timezone,") ||
		strings.Contains(","+headerLine+",", ",mode,") ||
		strings.Contains(","+headerLine+",", ",retry_at,") {
		t.Fatalf(
			"expected timezone, mode and retry_at headers to be excluded, got header %q",
			headerLine,
		)
	}
}

func TestSanitizeFilename(t *testing.T) {
	filename := sanitizeFilename("group/a b?.csv")
	if filename != "group_a_b_.csv" {
		t.Fatalf("unexpected sanitized filename: %q", filename)
	}
}

func TestParseLogExportParamsLimitsTimeRangeToThirtyDays(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now()
	endTime := now.Add(-2 * time.Hour)
	startTime := endTime.Add(-45 * 24 * time.Hour)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "start_timestamp=" + strconv.FormatInt(
				startTime.Unix(),
				10,
			) + "&end_timestamp=" + strconv.FormatInt(
				endTime.Unix(),
				10,
			),
		},
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}

	expectedStart := params.endTime.Add(-logExportMaxSpan)
	if params.startTime.Unix() != expectedStart.Unix() {
		t.Fatalf("expected start time to be clamped to %v, got %v", expectedStart, params.startTime)
	}
}

func TestParseLogExportParamsParsesChunkInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "chunk_interval=1h30m",
		},
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}

	if params.chunkInterval != 90*time.Minute {
		t.Fatalf("expected chunk_interval to be 90m, got %s", params.chunkInterval)
	}
}

func TestParseLogExportParamsDefaultChunkIntervalIsThirtyMinutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{},
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}

	if params.chunkInterval != 30*time.Minute {
		t.Fatalf("expected default chunk_interval to be 30m, got %s", params.chunkInterval)
	}
}

func TestParseLogExportParamsRejectsInvalidChunkInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "chunk_interval=abc",
		},
	}

	_, err := parseLogExportParams(c)
	if err == nil {
		t.Fatal("expected invalid chunk_interval to return error")
	}
}

func TestParseLogExportParamsRejectsTooSmallChunkInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "chunk_interval=5m",
		},
	}

	_, err := parseLogExportParams(c)
	if err == nil {
		t.Fatal("expected too small chunk_interval to return error")
	}
}

func TestParseLogExportParamsRejectsTooLargeChunkInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "chunk_interval=5h",
		},
	}

	_, err := parseLogExportParams(c)
	if err == nil {
		t.Fatal("expected too large chunk_interval to return error")
	}
}

func TestParseLogExportParamsUsesIncludeDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "include_detail=true",
		},
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}

	if !params.includeDetail {
		t.Fatal("expected include_detail=true to enable detail export")
	}
}

func TestParseLogExportParamsAllowsNegativeMaxEntriesAsUnlimited(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "max_entries=-1",
		},
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}

	if params.maxEntries != -1 {
		t.Fatalf("expected negative max_entries to remain unlimited, got %d", params.maxEntries)
	}
}

func TestParseLogExportParamsAllowsZeroMaxEntriesAsUnlimited(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "max_entries=0",
		},
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}

	if params.maxEntries != 0 {
		t.Fatalf("expected zero max_entries to remain unlimited, got %d", params.maxEntries)
	}
}

func TestParseLogExportParamsUsesIncludeRetryAt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "include_retry_at=true",
		},
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}

	if !params.includeRetryAt {
		t.Fatal("expected include_retry_at=true to enable retry_at export")
	}
}

func TestParseLogExportParamsAllowsAscOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "order=asc",
		},
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}

	if params.order != "asc" {
		t.Fatalf("expected order to be asc, got %q", params.order)
	}
}

func TestParseLogExportParamsRejectsUnsupportedOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "order=request_at-desc",
		},
	}

	_, err := parseLogExportParams(c)
	if err == nil {
		t.Fatal("expected unsupported order to return error")
	}
}
