//go:build enterprise

package enterprise

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

func setupAccessInfoTestDB(t *testing.T) {
	t.Helper()

	prevDB := model.DB
	prevLogDB := model.LogDB
	prevUsingSQLite := common.UsingSQLite

	testDB, err := model.OpenSQLite(filepath.Join(t.TempDir(), "enterprise-access-info.db"))
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = testDB
	model.LogDB = testDB
	common.UsingSQLite = true

	t.Cleanup(func() {
		model.DB = prevDB
		model.LogDB = prevLogDB
		common.UsingSQLite = prevUsingSQLite
	})

	if err := testDB.AutoMigrate(&model.Log{}, &model.RequestDetail{}, &models.FeishuUser{}); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}
}

func makeEnterpriseContext(t *testing.T, rawQuery string, enterpriseUser *models.FeishuUser) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	reqURL := "/api/enterprise/my-access/logs"
	if rawQuery != "" {
		reqURL += "?" + rawQuery
	}

	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	c.Request = req
	if enterpriseUser != nil {
		c.Set(CtxEnterpriseUser, enterpriseUser)
	}

	return c, recorder
}

func makeDetailContext(t *testing.T, logID string, enterpriseUser *models.FeishuUser) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/api/enterprise/my-access/logs/"+logID, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "log_id", Value: logID}}
	if enterpriseUser != nil {
		c.Set(CtxEnterpriseUser, enterpriseUser)
	}

	return c, recorder
}

func createLog(t *testing.T, l model.Log, detail *model.RequestDetail) model.Log {
	t.Helper()

	if err := model.LogDB.Create(&l).Error; err != nil {
		t.Fatalf("failed to create log %d: %v", l.ID, err)
	}

	if detail != nil {
		detail.LogID = l.ID
		if err := model.LogDB.Create(detail).Error; err != nil {
			t.Fatalf("failed to create request detail for log %d: %v", l.ID, err)
		}
	}

	return l
}

type testUserLog struct {
	ID        int    `json:"id"`
	Model     string `json:"model"`
	RequestID string `json:"request_id"`
	Code      int    `json:"code"`
	HasDetail bool   `json:"has_detail"`
}

type testGetTokenLogsResult struct {
	Logs    []testUserLog `json:"logs"`
	HasMore bool          `json:"has_more"`
}

func decodeAPIResponse[T any](t *testing.T, recorder *httptest.ResponseRecorder) middleware.APIResponse {
	t.Helper()

	var resp middleware.APIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode API response: %v\nbody=%s", err, recorder.Body.String())
	}

	return resp
}

func decodeData[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var envelope struct {
		Data    T      `json:"data"`
		Message string `json:"message"`
		Success bool   `json:"success"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode response data: %v\nbody=%s", err, recorder.Body.String())
	}

	return envelope.Data
}

func TestGetMyLogs_RequiresEnterpriseUser(t *testing.T) {
	setupAccessInfoTestDB(t)

	c, recorder := makeEnterpriseContext(t, "", nil)
	GetMyLogs(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	resp := decodeAPIResponse[any](t, recorder)
	if resp.Success {
		t.Fatalf("expected success=false, got true")
	}
}

func TestGetMyLogs_HonorsThirtyDayRangeAndGroupIsolation(t *testing.T) {
	setupAccessInfoTestDB(t)

	now := time.Now()
	user := &models.FeishuUser{OpenID: "ou_test", GroupID: "group-a", Status: 1}

	insideWindow := createLog(t, model.Log{
		ID:        101,
		GroupID:   "group-a",
		TokenName: "token-a",
		Model:     "claude-sonnet-4-20250514",
		RequestID: "req-inside",
		Code:      200,
		CreatedAt: now.Add(-20 * 24 * time.Hour),
		RequestAt: now.Add(-20 * 24 * time.Hour),
	}, &model.RequestDetail{RequestBody: `{"ok":true}`, ResponseBody: `{"id":"resp_1"}`})

	createLog(t, model.Log{
		ID:        102,
		GroupID:   "group-a",
		TokenName: "token-a",
		Model:     "claude-sonnet-4-20250514",
		RequestID: "req-outside",
		Code:      200,
		CreatedAt: now.Add(-40 * 24 * time.Hour),
		RequestAt: now.Add(-40 * 24 * time.Hour),
	}, nil)

	createLog(t, model.Log{
		ID:        103,
		GroupID:   "group-b",
		TokenName: "token-b",
		Model:     "claude-sonnet-4-20250514",
		RequestID: "req-other-group",
		Code:      200,
		CreatedAt: now.Add(-10 * 24 * time.Hour),
		RequestAt: now.Add(-10 * 24 * time.Hour),
	}, nil)

	query := url.Values{}
	query.Set("start_timestamp", strconv.FormatInt(time.Now().Add(-30*24*time.Hour).Unix(), 10))
	query.Set("end_timestamp", strconv.FormatInt(time.Now().Unix(), 10))

	c, recorder := makeEnterpriseContext(t, query.Encode(), user)
	GetMyLogs(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	result := decodeData[testGetTokenLogsResult](t, recorder)
	if result.HasMore {
		t.Fatalf("expected has_more=false, got true")
	}

	if len(result.Logs) != 1 {
		t.Fatalf("expected 1 log in 30-day range for the current group, got %d", len(result.Logs))
	}

	if result.Logs[0].ID != insideWindow.ID {
		t.Fatalf("expected log id %d, got %d", insideWindow.ID, result.Logs[0].ID)
	}

	if !result.Logs[0].HasDetail {
		t.Fatalf("expected has_detail=true for seeded request detail")
	}
}

func TestGetMyLogs_AcceptsMillisecondTimestampsAndFiltersCodeType(t *testing.T) {
	setupAccessInfoTestDB(t)

	now := time.Now()
	user := &models.FeishuUser{OpenID: "ou_test", GroupID: "group-a", Status: 1}

	createLog(t, model.Log{
		ID:        201,
		GroupID:   "group-a",
		TokenName: "token-a",
		Model:     "deepseek-v3",
		RequestID: "req-2xx",
		Code:      200,
		CreatedAt: now.Add(-48 * time.Hour),
		RequestAt: now.Add(-48 * time.Hour),
	}, nil)

	wanted := createLog(t, model.Log{
		ID:        202,
		GroupID:   "group-a",
		TokenName: "token-a",
		Model:     "deepseek-v3",
		RequestID: "req-5xx",
		Code:      502,
		CreatedAt: now.Add(-24 * time.Hour),
		RequestAt: now.Add(-24 * time.Hour),
	}, nil)

	query := url.Values{}
	query.Set("start_timestamp", strconv.FormatInt(time.Now().Add(-7*24*time.Hour).UnixMilli(), 10))
	query.Set("end_timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	query.Set("code_type", string(model.CodeTypeError))

	c, recorder := makeEnterpriseContext(t, query.Encode(), user)
	GetMyLogs(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	result := decodeData[testGetTokenLogsResult](t, recorder)
	if len(result.Logs) != 1 {
		t.Fatalf("expected exactly one 5xx log, got %d", len(result.Logs))
	}

	if result.Logs[0].ID != wanted.ID {
		t.Fatalf("expected 5xx log id %d, got %d", wanted.ID, result.Logs[0].ID)
	}
}

func TestGetMyLogs_PaginatesWithAfterID(t *testing.T) {
	setupAccessInfoTestDB(t)

	now := time.Now()
	user := &models.FeishuUser{OpenID: "ou_test", GroupID: "group-a", Status: 1}

	first := createLog(t, model.Log{
		ID:        301,
		GroupID:   "group-a",
		TokenName: "token-a",
		Model:     "model-1",
		RequestID: "req-301",
		Code:      200,
		CreatedAt: now.Add(-1 * time.Hour),
		RequestAt: now.Add(-1 * time.Hour),
	}, nil)

	second := createLog(t, model.Log{
		ID:        300,
		GroupID:   "group-a",
		TokenName: "token-a",
		Model:     "model-2",
		RequestID: "req-300",
		Code:      200,
		CreatedAt: now.Add(-2 * time.Hour),
		RequestAt: now.Add(-2 * time.Hour),
	}, nil)

	createLog(t, model.Log{
		ID:        299,
		GroupID:   "group-a",
		TokenName: "token-a",
		Model:     "model-3",
		RequestID: "req-299",
		Code:      200,
		CreatedAt: now.Add(-3 * time.Hour),
		RequestAt: now.Add(-3 * time.Hour),
	}, nil)

	query := url.Values{}
	query.Set("start_timestamp", strconv.FormatInt(time.Now().Add(-7*24*time.Hour).Unix(), 10))
	query.Set("end_timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("limit", "1")
	query.Set("after_id", "301")

	c, recorder := makeEnterpriseContext(t, query.Encode(), user)
	GetMyLogs(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	result := decodeData[testGetTokenLogsResult](t, recorder)
	if len(result.Logs) != 1 {
		t.Fatalf("expected one paginated result, got %d", len(result.Logs))
	}

	if result.Logs[0].ID != second.ID {
		t.Fatalf("expected paginated log id %d, got %d", second.ID, result.Logs[0].ID)
	}

	if !result.HasMore {
		t.Fatalf("expected has_more=true when more older logs remain")
	}

	if result.Logs[0].ID == first.ID {
		t.Fatalf("after_id filter did not skip the newest log")
	}
}

func TestGetMyLogDetail_EnforcesGroupIsolation(t *testing.T) {
	setupAccessInfoTestDB(t)

	user := &models.FeishuUser{OpenID: "ou_test", GroupID: "group-a", Status: 1}

	owned := createLog(t, model.Log{
		ID:        401,
		GroupID:   "group-a",
		TokenName: "token-a",
		Model:     "claude",
		RequestID: "req-owned",
		Code:      200,
		CreatedAt: time.Now(),
		RequestAt: time.Now(),
	}, &model.RequestDetail{RequestBody: `{"prompt":"hello"}`, ResponseBody: `{"text":"world"}`})

	createLog(t, model.Log{
		ID:        402,
		GroupID:   "group-b",
		TokenName: "token-b",
		Model:     "claude",
		RequestID: "req-other",
		Code:      200,
		CreatedAt: time.Now(),
		RequestAt: time.Now(),
	}, &model.RequestDetail{RequestBody: `{"prompt":"other"}`, ResponseBody: `{"text":"group"}`})

	c, recorder := makeDetailContext(t, "401", user)
	GetMyLogDetail(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	detail := decodeData[model.RequestDetail](t, recorder)
	if detail.LogID != owned.ID {
		t.Fatalf("expected detail log id %d, got %d", owned.ID, detail.LogID)
	}

	if detail.RequestBody != `{"prompt":"hello"}` {
		t.Fatalf("unexpected request body: %s", detail.RequestBody)
	}

	c2, recorder2 := makeDetailContext(t, "402", user)
	GetMyLogDetail(c2)

	if recorder2.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", recorder2.Code, http.StatusNotFound, recorder2.Body.String())
	}
}
