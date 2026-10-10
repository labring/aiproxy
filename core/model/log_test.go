package model_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common/config"
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

func TestRequestDetailDropInvalidUTF8Bodies(t *testing.T) {
	detail := &model.RequestDetail{
		RequestBody:           string([]byte{0xff, 0xfe}),
		ResponseBody:          "valid",
		RequestBodyTruncated:  true,
		ResponseBodyTruncated: true,
	}

	detail.DropInvalidUTF8Bodies()

	if detail.RequestBody != "" {
		t.Fatalf("expected invalid request body to be cleared, got %q", detail.RequestBody)
	}

	if detail.RequestBodyTruncated {
		t.Fatal("expected request body truncated flag to be cleared")
	}

	if detail.ResponseBody != "valid" {
		t.Fatalf("expected valid response body to remain unchanged, got %q", detail.ResponseBody)
	}

	if !detail.ResponseBodyTruncated {
		t.Fatal("expected valid response body truncated flag to remain unchanged")
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
		model.UsageContext{ServiceTier: "default"},
		model.Price{},
		model.Amount{},
		"",
		nil,
		"",
		"resp_test_websearch",
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

func TestGetLogsAppliesDefaultPagination(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.Log{}, &model.RequestDetail{}, &model.Summary{}); err != nil {
		t.Fatalf("migrate log db: %v", err)
	}

	rows := make([]model.Log, 11)

	baseTime := time.Unix(1777052048, 0)
	for i := range rows {
		rows[i] = model.Log{
			CreatedAt:  baseTime.Add(time.Duration(i) * time.Second),
			RequestAt:  baseTime.Add(time.Duration(i) * time.Second),
			GroupID:    "test-group",
			Model:      "gpt-5.4",
			RequestID:  model.EmptyNullString("req_default_page"),
			UpstreamID: model.EmptyNullString("resp_default_page"),
			Code:       200,
			Mode:       1,
			ChannelID:  1,
			TokenID:    1,
			TokenName:  "test-token",
		}
	}

	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	result, err := model.GetLogs(
		time.Time{},
		time.Time{},
		"",
		"",
		"",
		0,
		"",
		"",
		0,
		false,
		"",
		"",
		0,
		0,
	)
	if err != nil {
		t.Fatalf("get logs: %v", err)
	}

	if result.Total != 11 {
		t.Fatalf("expected total=11, got %d", result.Total)
	}

	if len(result.Logs) != 10 {
		t.Fatalf("expected default page size 10, got %d", len(result.Logs))
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

func TestCleanLogRemovesExpiredGroupChannelLogs(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(
		&model.Log{},
		&model.GroupChannelLog{},
		&model.RequestDetail{},
		&model.GroupChannelRequestDetail{},
		&model.RetryLog{},
		&model.GroupChannelRetryLog{},
		&model.StoreV2{},
		&model.GroupChannelStoreV2{},
		&model.AsyncUsageInfo{},
	); err != nil {
		t.Fatalf("migrate logs: %v", err)
	}

	oldLogStorageHours := config.GetLogStorageHours()
	oldRetryLogStorageHours := config.GetRetryLogStorageHours()
	t.Cleanup(func() {
		config.SetLogStorageHours(oldLogStorageHours)
		config.SetRetryLogStorageHours(oldRetryLogStorageHours)
	})
	config.SetLogStorageHours(1)
	config.SetRetryLogStorageHours(0)

	oldTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now()

	rows := []model.GroupChannelLog{
		{
			GroupID:        "group-1",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "old_group_log",
			CreatedAt:      oldTime,
			RequestAt:      oldTime,
		},
		{
			GroupID:        "group-1",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "recent_group_log",
			CreatedAt:      recentTime,
			RequestAt:      recentTime,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed group channel logs: %v", err)
	}

	if err := model.CleanLog(100, false); err != nil {
		t.Fatalf("clean log: %v", err)
	}

	var ids []string
	if err := db.Model(&model.GroupChannelLog{}).
		Order("request_id").
		Pluck("request_id", &ids).Error; err != nil {
		t.Fatalf("list group channel logs: %v", err)
	}

	want := []string{"recent_group_log"}
	if len(ids) != len(want) {
		t.Fatalf("expected remaining group channel log ids %v, got %v", want, ids)
	}

	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("expected remaining group channel log ids %v, got %v", want, ids)
		}
	}
}

func TestDeleteOldLogPreservesGroupChannelLogs(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.Log{}, &model.GroupChannelLog{}); err != nil {
		t.Fatalf("migrate logs: %v", err)
	}

	oldTime := time.Now().Add(-2 * time.Hour)

	recentTime := time.Now()
	if err := db.Create(&[]model.GroupChannelLog{
		{
			GroupID:        "group-1",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "old_group_log",
			CreatedAt:      oldTime,
			RequestAt:      oldTime,
		},
		{
			GroupID:        "group-1",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "recent_group_log",
			CreatedAt:      recentTime,
			RequestAt:      recentTime,
		},
	}).Error; err != nil {
		t.Fatalf("seed group channel logs: %v", err)
	}

	deleted, err := model.DeleteOldLog(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("delete old logs: %v", err)
	}

	if deleted != 0 {
		t.Fatalf("expected no deleted normal logs, got %d", deleted)
	}

	var ids []string
	if err := db.Model(&model.GroupChannelLog{}).
		Order("request_id").
		Pluck("request_id", &ids).Error; err != nil {
		t.Fatalf("list group channel logs: %v", err)
	}

	if len(ids) != 2 || ids[0] != "old_group_log" || ids[1] != "recent_group_log" {
		t.Fatalf("expected group channel logs to remain, got %v", ids)
	}

	deleted, err = model.DeleteOldGroupChannelLog(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("delete old group channel logs: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("expected one deleted group channel log, got %d", deleted)
	}

	ids = nil
	if err := db.Model(&model.GroupChannelLog{}).
		Order("request_id").
		Pluck("request_id", &ids).Error; err != nil {
		t.Fatalf("list group channel logs after group channel delete: %v", err)
	}

	if len(ids) != 1 || ids[0] != "recent_group_log" {
		t.Fatalf("expected only recent group channel log to remain, got %v", ids)
	}
}

func TestDeleteOldGroupChannelLogForGroupIsScoped(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.GroupChannelLog{}); err != nil {
		t.Fatalf("migrate group channel logs: %v", err)
	}

	oldTime := time.Now().Add(-2 * time.Hour)

	recentTime := time.Now()
	if err := db.Create(&[]model.GroupChannelLog{
		{
			GroupID:        "group-1",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "group_1_old",
			CreatedAt:      oldTime,
			RequestAt:      oldTime,
		},
		{
			GroupID:        "group-1",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "group_1_recent",
			CreatedAt:      recentTime,
			RequestAt:      recentTime,
		},
		{
			GroupID:        "group-2",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "group_2_old",
			CreatedAt:      oldTime,
			RequestAt:      oldTime,
		},
	}).Error; err != nil {
		t.Fatalf("seed group channel logs: %v", err)
	}

	deleted, err := model.DeleteOldGroupChannelLogForGroup("group-1", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("delete scoped old group channel logs: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("expected one deleted group channel log, got %d", deleted)
	}

	var ids []string
	if err := db.Model(&model.GroupChannelLog{}).
		Order("request_id").
		Pluck("request_id", &ids).Error; err != nil {
		t.Fatalf("list group channel logs: %v", err)
	}

	want := []string{"group_1_recent", "group_2_old"}
	if len(ids) != len(want) {
		t.Fatalf("expected remaining group channel logs %v, got %v", want, ids)
	}

	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("expected remaining group channel logs %v, got %v", want, ids)
		}
	}
}

func TestDeleteGroupLogsPreservesGroupChannelLogs(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(&model.Log{}, &model.GroupChannelLog{}); err != nil {
		t.Fatalf("migrate logs: %v", err)
	}

	now := time.Now()
	if err := db.Create(&[]model.GroupChannelLog{
		{
			GroupID:        "group-1",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "group_1_log",
			CreatedAt:      now,
			RequestAt:      now,
		},
		{
			GroupID:        "group-2",
			GroupChannelID: 1,
			Model:          "gpt-5",
			RequestID:      "group_2_log",
			CreatedAt:      now,
			RequestAt:      now,
		},
	}).Error; err != nil {
		t.Fatalf("seed group channel logs: %v", err)
	}

	deleted, err := model.DeleteGroupLogs("group-1")
	if err != nil {
		t.Fatalf("delete group logs: %v", err)
	}

	if deleted != 0 {
		t.Fatalf("expected no deleted normal logs, got %d", deleted)
	}

	var ids []string
	if err := db.Model(&model.GroupChannelLog{}).
		Order("request_id").
		Pluck("request_id", &ids).Error; err != nil {
		t.Fatalf("list group channel logs: %v", err)
	}

	if len(ids) != 2 || ids[0] != "group_1_log" || ids[1] != "group_2_log" {
		t.Fatalf("expected group channel logs to remain, got %v", ids)
	}

	deleted, err = model.DeleteGroupChannelLogs("group-1")
	if err != nil {
		t.Fatalf("delete group channel logs: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("expected one deleted group channel log, got %d", deleted)
	}

	ids = nil
	if err := db.Model(&model.GroupChannelLog{}).
		Order("request_id").
		Pluck("request_id", &ids).Error; err != nil {
		t.Fatalf("list group channel logs after group channel delete: %v", err)
	}

	if len(ids) != 1 || ids[0] != "group_2_log" {
		t.Fatalf("expected group-2 group channel log to remain, got %v", ids)
	}
}

func TestGetGroupLogsIsolatedFromGroupChannelLogs(t *testing.T) {
	withGroupChannelLogDB(t, func(dbLogs groupChannelLogFixture) {
		result, err := model.GetGroupLogs(
			"group-1",
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"",
			"",
			0,
			"",
			0,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			true,
			"",
			"",
			1,
			10,
		)
		if err != nil {
			t.Fatalf("get group logs: %v", err)
		}

		if result.Total != 1 {
			t.Fatalf("expected total=1, got %d", result.Total)
		}

		if len(result.Logs) != 1 {
			t.Fatalf("expected 1 log, got %d", len(result.Logs))
		}

		if result.Logs[0].RequestID != "normal_req" {
			t.Fatalf("expected normal log, got %q", result.Logs[0].RequestID)
		}

		filtered, err := model.GetGroupLogs(
			"group-1",
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"",
			"",
			0,
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			false,
			"",
			"",
			1,
			10,
		)
		if err != nil {
			t.Fatalf("get filtered group logs: %v", err)
		}

		if filtered.Total != 0 || len(filtered.Logs) != 0 {
			t.Fatalf(
				"expected no normal logs for group channel id, got total=%d logs=%#v",
				filtered.Total,
				filtered.Logs,
			)
		}

		groupChannelResult, err := model.GetGroupChannelLogs(
			"group-1",
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"",
			"",
			0,
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			true,
			"",
			"",
			1,
			10,
		)
		if err != nil {
			t.Fatalf("get group channel logs: %v", err)
		}

		if groupChannelResult.Total != 1 || len(groupChannelResult.Logs) != 1 ||
			groupChannelResult.Logs[0].RequestID != "group_channel_req" {
			t.Fatalf(
				"expected group channel log from isolated endpoint, got total=%d logs=%#v",
				groupChannelResult.Total,
				groupChannelResult.Logs,
			)
		}

		groupLog := groupChannelResult.Logs[0]
		if groupLog.GroupChannelID != dbLogs.groupChannelID {
			t.Fatalf(
				"expected group channel id %d, got %d",
				dbLogs.groupChannelID,
				groupLog.GroupChannelID,
			)
		}

		if groupLog.RequestDetail == nil || groupLog.RequestDetail.RequestBody != "group request" {
			t.Fatalf("expected group channel request detail, got %#v", groupLog.RequestDetail)
		}

		if len(groupChannelResult.TokenNames) != 1 ||
			groupChannelResult.TokenNames[0] != "group-token" {
			t.Fatalf("expected group channel token facet, got %#v", groupChannelResult.TokenNames)
		}
	})
}

func TestGlobalLogsAreIsolatedFromGroupChannelLogs(t *testing.T) {
	withGroupChannelLogDB(t, func(dbLogs groupChannelLogFixture) {
		result, err := model.GetLogs(
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"group_channel_req",
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			false,
			"",
			"",
			1,
			10,
		)
		if err != nil {
			t.Fatalf("get global logs: %v", err)
		}

		if result.Total != 0 || len(result.Logs) != 0 {
			t.Fatalf(
				"expected no global group channel logs, got total=%d logs=%#v",
				result.Total,
				result.Logs,
			)
		}

		searchResult, err := model.SearchLogs(
			"group_channel_req",
			"",
			"",
			"",
			0,
			"",
			"",
			time.Time{},
			time.Now().Add(time.Hour),
			0,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			false,
			"",
			"",
			1,
			10,
		)
		if err != nil {
			t.Fatalf("search global logs: %v", err)
		}

		if searchResult.Total != 0 || len(searchResult.Logs) != 0 {
			t.Fatalf(
				"expected no searched global group channel logs, got total=%d logs=%#v",
				searchResult.Total,
				searchResult.Logs,
			)
		}

		logs, err := model.ExportLogsRange(
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"group_channel_req",
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			true,
			"",
			"",
			10,
		)
		if err != nil {
			t.Fatalf("export global logs: %v", err)
		}

		if len(logs) != 0 {
			t.Fatalf("expected no exported global group channel logs, got %#v", logs)
		}

		groupChannelLogs, err := model.ExportGroupChannelLogsRange(
			"group-1",
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"group_channel_req",
			"",
			0,
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			true,
			"",
			"",
			10,
		)
		if err != nil {
			t.Fatalf("export isolated group channel logs: %v", err)
		}

		if len(groupChannelLogs) != 1 || groupChannelLogs[0].RequestID != "group_channel_req" ||
			groupChannelLogs[0].RequestDetail == nil {
			t.Fatalf("expected exported group channel log with detail, got %#v", groupChannelLogs)
		}

		detail, err := model.GetGroupChannelLogDetailForGroup(dbLogs.groupChannelLogID, "group-1")
		if err != nil {
			t.Fatalf("get group channel detail: %v", err)
		}

		if detail.RequestBody != "group request" {
			t.Fatalf("unexpected global group channel detail: %#v", detail)
		}
	})
}

func TestGroupAndGroupChannelLogsPaginateIndependently(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(
		&model.Log{},
		&model.RequestDetail{},
		&model.GroupChannelLog{},
		&model.GroupChannelRequestDetail{},
		&model.Summary{},
		&model.GroupSummary{},
		&model.GroupChannelTokenSummary{},
	); err != nil {
		t.Fatalf("migrate log db: %v", err)
	}

	baseTime := time.Now().Add(-time.Hour)
	for i := range 12 {
		createdAt := baseTime.Add(time.Duration(i) * time.Second)
		if err := db.Create(&model.Log{
			GroupID:   "group-1",
			ChannelID: 7,
			Model:     "gpt-5",
			RequestID: model.EmptyNullString("normal_page_req"),
			Code:      200,
			CreatedAt: createdAt,
			RequestAt: createdAt,
		}).Error; err != nil {
			t.Fatalf("seed normal log %d: %v", i, err)
		}
	}

	for i := range 12 {
		createdAt := baseTime.Add(time.Duration(12+i) * time.Second)
		if err := db.Create(&model.GroupChannelLog{
			GroupID:        "group-1",
			GroupChannelID: 11,
			Model:          "gpt-5",
			RequestID:      model.EmptyNullString("group_page_req"),
			Code:           200,
			CreatedAt:      createdAt,
			RequestAt:      createdAt,
		}).Error; err != nil {
			t.Fatalf("seed group channel log %d: %v", i, err)
		}
	}

	result, err := model.GetGroupLogs(
		"group-1",
		time.Time{},
		time.Now().Add(time.Hour),
		"",
		"",
		"",
		0,
		"",
		0,
		"created_at-asc",
		model.CodeTypeAll,
		0,
		false,
		"",
		"",
		2,
		10,
	)
	if err != nil {
		t.Fatalf("get paginated group logs: %v", err)
	}

	if result.Total != 12 {
		t.Fatalf("expected total=12, got %d", result.Total)
	}

	if len(result.Logs) != 2 {
		t.Fatalf("expected second page size 2, got %d", len(result.Logs))
	}

	if result.Logs[0].RequestID != "normal_page_req" {
		t.Fatalf("expected normal log beyond first 10 rows, got %q", result.Logs[0].RequestID)
	}

	searchResult, err := model.SearchGroupLogs(
		"group-1",
		"",
		"",
		"",
		0,
		"",
		"",
		time.Time{},
		time.Now().Add(time.Hour),
		0,
		"created_at-asc",
		model.CodeTypeAll,
		0,
		false,
		"",
		"",
		2,
		10,
	)
	if err != nil {
		t.Fatalf("search paginated group logs: %v", err)
	}

	if searchResult.Total != 12 || len(searchResult.Logs) != 2 ||
		searchResult.Logs[0].RequestID != "normal_page_req" {
		t.Fatalf(
			"expected paginated search to include normal rows beyond first page, total=%d logs=%#v",
			searchResult.Total,
			searchResult.Logs,
		)
	}

	groupChannelResult, err := model.GetGroupChannelLogs(
		"group-1",
		time.Time{},
		time.Now().Add(time.Hour),
		"",
		"",
		"",
		0,
		"",
		0,
		"created_at-asc",
		model.CodeTypeAll,
		0,
		false,
		"",
		"",
		2,
		10,
	)
	if err != nil {
		t.Fatalf("get paginated group channel logs: %v", err)
	}

	if groupChannelResult.Total != 12 || len(groupChannelResult.Logs) != 2 ||
		groupChannelResult.Logs[0].RequestID != "group_page_req" {
		t.Fatalf(
			"expected paginated group channel rows beyond first page, total=%d logs=%#v",
			groupChannelResult.Total,
			groupChannelResult.Logs,
		)
	}

	groupChannelSearch, err := model.SearchGroupChannelLogs(
		"group-1",
		"",
		"",
		"",
		0,
		"",
		"",
		time.Time{},
		time.Now().Add(time.Hour),
		0,
		"created_at-asc",
		model.CodeTypeAll,
		0,
		false,
		"",
		"",
		2,
		10,
	)
	if err != nil {
		t.Fatalf("search paginated group channel logs: %v", err)
	}

	if groupChannelSearch.Total != 12 || len(groupChannelSearch.Logs) != 2 ||
		groupChannelSearch.Logs[0].RequestID != "group_page_req" {
		t.Fatalf(
			"expected paginated group channel search beyond first page, total=%d logs=%#v",
			groupChannelSearch.Total,
			groupChannelSearch.Logs,
		)
	}
}

func TestSearchAndExportGroupChannelLogsAreIsolated(t *testing.T) {
	withGroupChannelLogDB(t, func(dbLogs groupChannelLogFixture) {
		result, err := model.SearchGroupLogs(
			"group-1",
			"group_channel_req",
			"",
			"",
			0,
			"",
			"",
			time.Time{},
			time.Now().Add(time.Hour),
			0,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			false,
			"",
			"",
			1,
			10,
		)
		if err != nil {
			t.Fatalf("search group logs: %v", err)
		}

		if result.Total != 0 || len(result.Logs) != 0 {
			t.Fatalf(
				"expected no normal logs from group channel search term, got total=%d logs=%#v",
				result.Total,
				result.Logs,
			)
		}

		groupChannelResult, err := model.SearchGroupChannelLogs(
			"group-1",
			"group_channel_req",
			"",
			"",
			0,
			"",
			"",
			time.Time{},
			time.Now().Add(time.Hour),
			0,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			false,
			"",
			"",
			1,
			10,
		)
		if err != nil {
			t.Fatalf("search group channel logs: %v", err)
		}

		if groupChannelResult.Total != 1 || len(groupChannelResult.Logs) != 1 ||
			groupChannelResult.Logs[0].GroupChannelID != dbLogs.groupChannelID {
			t.Fatalf(
				"expected searched group channel log, got total=%d logs=%#v",
				groupChannelResult.Total,
				groupChannelResult.Logs,
			)
		}

		logs, err := model.ExportGroupLogsRange(
			"group-1",
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"",
			"",
			0,
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			true,
			"",
			"",
			10,
		)
		if err != nil {
			t.Fatalf("export group logs: %v", err)
		}

		if len(logs) != 0 {
			t.Fatalf(
				"expected no exported group channel log from normal group export, got %#v",
				logs,
			)
		}

		groupChannelLogs, err := model.ExportGroupChannelLogsRange(
			"group-1",
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"",
			"",
			0,
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			true,
			"",
			"",
			10,
		)
		if err != nil {
			t.Fatalf("export group channel logs: %v", err)
		}

		if len(groupChannelLogs) != 1 || groupChannelLogs[0].RequestID != "group_channel_req" {
			t.Fatalf("expected exported group channel log, got %#v", groupChannelLogs)
		}

		if groupChannelLogs[0].RequestDetail == nil ||
			groupChannelLogs[0].RequestDetail.ResponseBody != "group response" {
			t.Fatalf(
				"expected exported group channel detail, got %#v",
				groupChannelLogs[0].RequestDetail,
			)
		}
	})
}

func TestGetGroupChannelLogDetailForGroupReadsGroupChannelDetail(t *testing.T) {
	withGroupChannelLogDB(t, func(dbLogs groupChannelLogFixture) {
		detail, err := model.GetGroupChannelLogDetailForGroup(
			dbLogs.groupChannelLogID,
			"group-1",
		)
		if err != nil {
			t.Fatalf("get group channel detail: %v", err)
		}

		if detail.RequestBody != "group request" || detail.ResponseBody != "group response" {
			t.Fatalf("unexpected group channel detail: %#v", detail)
		}
	})
}

func TestGetGroupChannelLogsRequiresGroupFilter(t *testing.T) {
	withGroupChannelLogDB(t, func(dbLogs groupChannelLogFixture) {
		result, err := model.GetGroupChannelLogs(
			"",
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"",
			"",
			0,
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			true,
			"",
			"",
			1,
			10,
		)
		if err == nil {
			t.Fatalf("expected missing group error, got result %#v", result)
		}

		logs, err := model.ExportGroupChannelLogsRange(
			"",
			time.Time{},
			time.Now().Add(time.Hour),
			"",
			"",
			"",
			0,
			"",
			dbLogs.groupChannelID,
			"created_at-asc",
			model.CodeTypeAll,
			0,
			true,
			"",
			"",
			10,
		)
		if err == nil {
			t.Fatalf("expected missing group error, got logs %#v", logs)
		}
	})
}

type groupChannelLogFixture struct {
	groupChannelID    int
	groupChannelLogID int
}

func withGroupChannelLogDB(t *testing.T, fn func(groupChannelLogFixture)) {
	t.Helper()

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = prevLogDB
	})

	if err := db.AutoMigrate(
		&model.Log{},
		&model.RequestDetail{},
		&model.GroupChannelLog{},
		&model.GroupChannelRequestDetail{},
		&model.Summary{},
		&model.GroupSummary{},
		&model.GroupChannelTokenSummary{},
	); err != nil {
		t.Fatalf("migrate log db: %v", err)
	}

	baseTime := time.Now().Add(-time.Minute)

	normalLog := model.Log{
		GroupID:   "group-1",
		ChannelID: 7,
		Model:     "gpt-5",
		RequestID: "normal_req",
		Code:      200,
		CreatedAt: baseTime,
		RequestAt: baseTime,
		RequestDetail: &model.RequestDetail{
			RequestBody:  "normal request",
			ResponseBody: "normal response",
		},
	}
	if err := db.Create(&normalLog).Error; err != nil {
		t.Fatalf("seed normal log: %v", err)
	}

	groupChannelLog := model.GroupChannelLog{
		GroupID:        "group-1",
		GroupChannelID: 11,
		TokenName:      "group-token",
		Model:          "gpt-5",
		RequestID:      "group_channel_req",
		Code:           200,
		CreatedAt:      baseTime.Add(time.Second),
		RequestAt:      baseTime.Add(time.Second),
		RequestDetail: &model.GroupChannelRequestDetail{
			RequestBody:  "group request",
			ResponseBody: "group response",
		},
	}
	if err := db.Create(&groupChannelLog).Error; err != nil {
		t.Fatalf("seed group channel log: %v", err)
	}

	if err := db.Create(&model.GroupChannelTokenSummary{
		Unique: model.GroupChannelTokenSummaryUnique{
			GroupID:       "group-1",
			TokenName:     groupChannelLog.TokenName,
			Model:         groupChannelLog.Model,
			HourTimestamp: baseTime.Truncate(time.Hour).Unix(),
		},
	}).Error; err != nil {
		t.Fatalf("seed group channel token summary: %v", err)
	}

	otherGroupLog := model.GroupChannelLog{
		GroupID:        "group-2",
		GroupChannelID: 11,
		Model:          "gpt-5",
		RequestID:      "other_group_channel_req",
		Code:           200,
		CreatedAt:      baseTime.Add(2 * time.Second),
		RequestAt:      baseTime.Add(2 * time.Second),
	}
	if err := db.Create(&otherGroupLog).Error; err != nil {
		t.Fatalf("seed other group channel log: %v", err)
	}

	fn(groupChannelLogFixture{
		groupChannelID:    groupChannelLog.GroupChannelID,
		groupChannelLogID: groupChannelLog.ID,
	})
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
