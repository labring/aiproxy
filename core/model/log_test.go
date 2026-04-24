package model_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/model"
)

func TestRequestDetailApplyBodySizeLimits(t *testing.T) {
	detail := &model.RequestDetail{
		RequestBody:  "abcdef",
		ResponseBody: "uvwxyz",
	}

	detail.ApplyBodySizeLimits(4, -1)

	if detail.RequestBody != "a..." {
		t.Fatalf("expected request body to be truncated to a..., got %q", detail.RequestBody)
	}

	if !detail.RequestBodyTruncated {
		t.Fatal("expected request body truncated flag to be true")
	}

	if detail.ResponseBody != "" {
		t.Fatalf("expected response body to be cleared, got %q", detail.ResponseBody)
	}

	if !detail.ResponseBodyTruncated {
		t.Fatal("expected response body truncated flag to be true")
	}
}

func TestRequestDetailApplyBodySizeLimitsZeroKeepsOriginalBody(t *testing.T) {
	detail := &model.RequestDetail{
		RequestBody:  "abcdef",
		ResponseBody: "你好世界",
	}

	detail.ApplyBodySizeLimits(0, 0)

	if detail.RequestBody != "abcdef" {
		t.Fatalf("expected request body to remain unchanged, got %q", detail.RequestBody)
	}

	if detail.RequestBodyTruncated {
		t.Fatal("expected request body truncated flag to remain false")
	}

	if detail.ResponseBody != "你好世界" {
		t.Fatalf("expected response body to remain unchanged, got %q", detail.ResponseBody)
	}

	if detail.ResponseBodyTruncated {
		t.Fatal("expected response body truncated flag to remain false")
	}
}

func TestRecordConsumeLogPersistsWebSearchCount(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.Log{}, &model.RequestDetail{}); err != nil {
		t.Fatalf("migrate log db: %v", err)
	}

	now := time.Unix(1777052048, 0)

	err = model.RecordConsumeLog(
		"req_test_websearch",
		now,
		now.Add(-2*time.Second),
		time.Time{},
		now.Add(-1500*time.Millisecond),
		"test-group",
		200,
		1,
		"gpt-5.4",
		2,
		"test-token",
		"/v1/responses",
		"",
		1,
		"127.0.0.1",
		0,
		nil,
		model.Usage{
			InputTokens:    10,
			OutputTokens:   5,
			TotalTokens:    15,
			WebSearchCount: 1,
		},
		model.Price{},
		model.Amount{},
		"",
		nil,
		"",
		"resp_test_websearch",
		"default",
	)
	if err != nil {
		t.Fatalf("record consume log: %v", err)
	}

	var got model.Log
	if err := db.Where("upstream_id = ?", "resp_test_websearch").First(&got).Error; err != nil {
		t.Fatalf("query log: %v", err)
	}

	if got.Usage.WebSearchCount != 1 {
		t.Fatalf("expected web_search_count=1, got %d", got.Usage.WebSearchCount)
	}
}

func TestRecordConsumeLogLoadsNullWebSearchCountAsZero(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&model.Log{}, &model.RequestDetail{}); err != nil {
		t.Fatalf("migrate log db: %v", err)
	}

	row := map[string]any{
		"request_at":   time.Unix(1777052048, 0),
		"created_at":   time.Unix(1777052048, 0),
		"group_id":     "test-group",
		"model":        "gpt-5.4",
		"code":         200,
		"mode":         1,
		"channel_id":   1,
		"token_id":     1,
		"token_name":   "test-token",
		"request_id":   "req_null_ws",
		"upstream_id":  "resp_null_ws",
		"total_tokens": 15,
	}

	if err := db.Table("logs").Create(row).Error; err != nil {
		t.Fatalf("insert row: %v", err)
	}

	var got model.Log
	if err := db.Where("upstream_id = ?", "resp_null_ws").First(&got).Error; err != nil {
		t.Fatalf("query log: %v", err)
	}

	if got.Usage.WebSearchCount != 0 {
		t.Fatalf("expected web_search_count=0 for null column, got %d", got.Usage.WebSearchCount)
	}
}
