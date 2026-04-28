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
		model.AsyncUsageStatusNone,
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

func TestCleanupFinishedAsyncUsagesKeepsPending(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.AsyncUsageInfo{}); err != nil {
		t.Fatalf("migrate async usage info: %v", err)
	}

	oldTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now()

	rows := []model.AsyncUsageInfo{
		{RequestID: "pending_old", Status: model.AsyncUsageStatusPending, UpdatedAt: oldTime},
		{RequestID: "completed_old", Status: model.AsyncUsageStatusCompleted, UpdatedAt: oldTime},
		{RequestID: "failed_old", Status: model.AsyncUsageStatusFailed, UpdatedAt: oldTime},
		{
			RequestID: "completed_recent",
			Status:    model.AsyncUsageStatusCompleted,
			UpdatedAt: recentTime,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed async usage info: %v", err)
	}

	if err := model.CleanupFinishedAsyncUsages(time.Hour, 100); err != nil {
		t.Fatalf("cleanup finished async usages: %v", err)
	}

	var ids []string
	if err := db.Model(&model.AsyncUsageInfo{}).
		Order("request_id").
		Pluck("request_id", &ids).Error; err != nil {
		t.Fatalf("list async usage info: %v", err)
	}

	want := []string{"completed_recent", "pending_old"}
	if len(ids) != len(want) {
		t.Fatalf("expected remaining ids %v, got %v", want, ids)
	}

	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("expected remaining ids %v, got %v", want, ids)
		}
	}
}

func TestGetPendingAsyncUsagesOrdersByUpdatedAt(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.AsyncUsageInfo{}); err != nil {
		t.Fatalf("migrate async usage info: %v", err)
	}

	baseTime := time.Now().Add(-time.Hour)

	rows := []model.AsyncUsageInfo{
		{
			RequestID: "oldest",
			Status:    model.AsyncUsageStatusPending,
			CreatedAt: baseTime,
			UpdatedAt: baseTime,
		},
		{
			RequestID: "next",
			Status:    model.AsyncUsageStatusPending,
			CreatedAt: baseTime.Add(time.Second),
			UpdatedAt: baseTime.Add(time.Second),
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed async usage info: %v", err)
	}

	firstBatch, err := model.GetPendingAsyncUsages(1)
	if err != nil {
		t.Fatalf("get first pending async usage: %v", err)
	}

	if len(firstBatch) != 1 || firstBatch[0].RequestID != "oldest" {
		t.Fatalf("expected oldest first, got %+v", firstBatch)
	}

	firstBatch[0].UpdatedAt = time.Now()
	if err := model.UpdateAsyncUsageInfo(firstBatch[0]); err != nil {
		t.Fatalf("touch first pending async usage: %v", err)
	}

	secondBatch, err := model.GetPendingAsyncUsages(1)
	if err != nil {
		t.Fatalf("get second pending async usage: %v", err)
	}

	if len(secondBatch) != 1 || secondBatch[0].RequestID != "next" {
		t.Fatalf("expected next pending row after touch, got %+v", secondBatch)
	}
}

func TestGetPendingAsyncUsagesSkipsFutureNextPollAt(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.AsyncUsageInfo{}); err != nil {
		t.Fatalf("migrate async usage info: %v", err)
	}

	now := time.Now()

	rows := []model.AsyncUsageInfo{
		{
			RequestID:  "due",
			Status:     model.AsyncUsageStatusPending,
			NextPollAt: now.Add(-time.Second),
		},
		{
			RequestID:  "future",
			Status:     model.AsyncUsageStatusPending,
			NextPollAt: now.Add(time.Minute),
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed async usage info: %v", err)
	}

	got, err := model.GetPendingAsyncUsagesDue(10, now)
	if err != nil {
		t.Fatalf("get pending async usages: %v", err)
	}

	if len(got) != 1 || got[0].RequestID != "due" {
		t.Fatalf("expected only due row, got %+v", got)
	}
}

func TestTryClaimAsyncUsageInfoIsAtomic(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.AsyncUsageInfo{}); err != nil {
		t.Fatalf("migrate async usage info: %v", err)
	}

	now := time.Now()

	info := &model.AsyncUsageInfo{
		RequestID:  "claim",
		Status:     model.AsyncUsageStatusPending,
		NextPollAt: now.Add(-time.Second),
	}
	if err := db.Create(info).Error; err != nil {
		t.Fatalf("seed async usage info: %v", err)
	}

	claimed, err := model.TryClaimAsyncUsageInfo(
		info,
		"token-1",
		now.Add(time.Minute),
		now,
	)
	if err != nil {
		t.Fatalf("claim async usage: %v", err)
	}

	if !claimed {
		t.Fatal("expected first claim to succeed")
	}

	second := &model.AsyncUsageInfo{ID: info.ID}

	claimed, err = model.TryClaimAsyncUsageInfo(
		second,
		"token-2",
		now.Add(time.Minute),
		now,
	)
	if err != nil {
		t.Fatalf("claim async usage second time: %v", err)
	}

	if claimed {
		t.Fatal("expected second claim to fail")
	}

	var got model.AsyncUsageInfo
	if err := db.First(&got, info.ID).Error; err != nil {
		t.Fatalf("get async usage info: %v", err)
	}

	if got.ProcessingToken != "token-1" {
		t.Fatalf("expected token-1, got %q", got.ProcessingToken)
	}
}

func TestRenewAsyncUsageClaimRequiresToken(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.AsyncUsageInfo{}); err != nil {
		t.Fatalf("migrate async usage info: %v", err)
	}

	now := time.Now()

	info := &model.AsyncUsageInfo{
		RequestID:       "renew",
		Status:          model.AsyncUsageStatusPending,
		NextPollAt:      now.Add(time.Minute),
		ProcessingToken: "token-1",
	}
	if err := db.Create(info).Error; err != nil {
		t.Fatalf("seed async usage info: %v", err)
	}

	renewed, err := model.RenewAsyncUsageClaim(
		info.ID,
		"token-2",
		now.Add(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("renew wrong token: %v", err)
	}

	if renewed {
		t.Fatal("expected wrong token renewal to fail")
	}

	renewUntil := now.Add(3 * time.Minute)

	renewed, err = model.RenewAsyncUsageClaim(info.ID, "token-1", renewUntil)
	if err != nil {
		t.Fatalf("renew correct token: %v", err)
	}

	if !renewed {
		t.Fatal("expected correct token renewal to succeed")
	}

	var got model.AsyncUsageInfo
	if err := db.First(&got, info.ID).Error; err != nil {
		t.Fatalf("get async usage info: %v", err)
	}

	if !got.NextPollAt.After(now.Add(2 * time.Minute)) {
		t.Fatalf("expected renewed next poll at, got %s", got.NextPollAt)
	}
}

func TestAsyncUsageBackoffDelay(t *testing.T) {
	tests := []struct {
		retry int
		want  time.Duration
	}{
		{retry: 0, want: 3 * time.Second},
		{retry: 1, want: 3 * time.Second},
		{retry: 2, want: 6 * time.Second},
		{retry: 3, want: 12 * time.Second},
		{retry: 5, want: 48 * time.Second},
		{retry: 7, want: 3 * time.Minute},
		{retry: 10, want: 3 * time.Minute},
	}

	for _, tt := range tests {
		got := model.AsyncUsageBackoffDelay(tt.retry)
		if got != tt.want {
			t.Fatalf("retry %d: expected %s, got %s", tt.retry, tt.want, got)
		}
	}
}
