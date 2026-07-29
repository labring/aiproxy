//nolint:testpackage
package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGeminiPathModelModelScopedOperation(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1beta/models/veo-3.1-generate-preview/operations/video-123",
		nil,
	)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	ctx.Params = gin.Params{
		{Key: "model", Value: "veo-3.1-generate-preview"},
		{Key: "operation_id", Value: "/video-123"},
	}

	assert.Equal(
		t,
		"models/veo-3.1-generate-preview/operations/video-123",
		getGeminiPathModel(ctx),
	)

	modelName, operationID := getGeminiPathModelAndOperationID(ctx)
	assert.Equal(t, "veo-3.1-generate-preview", modelName)
	assert.Equal(t, "video-123", operationID)
}

func TestGetRequestModelVideoGenerationsJobsMultipart(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "wan2.7-i2v"))
	require.NoError(t, writer.WriteField("prompt", "Animate the reference"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		&body,
	)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	modelName, err := getRequestModel(ctx, mode.VideoGenerationsJobs, "group-1", 7)
	require.NoError(t, err)
	assert.Equal(t, "wan2.7-i2v", modelName)
}

func TestGetRequestModelVideoGenerationsJobsJSON(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		bytes.NewBufferString(`{"model":"wan2.5-t2v-preview","prompt":"A city street"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	modelName, err := getRequestModel(ctx, mode.VideoGenerationsJobs, "group-1", 7)
	require.NoError(t, err)
	assert.Equal(t, "wan2.5-t2v-preview", modelName)
}

func TestGetRequestModelVideosMultipart(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "sora-2"))
	require.NoError(t, writer.WriteField("prompt", "Animate the reference"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/videos",
		&body,
	)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	modelName, err := getRequestModel(ctx, mode.Videos, "group-1", 7)
	require.NoError(t, err)
	assert.Equal(t, "sora-2", modelName)
}

func TestGetRequestModelVideosMultipartWithoutContentLength(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "sora-2"))
	require.NoError(t, writer.WriteField("prompt", "Animate the reference"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/videos",
		&body,
	)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = -1

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	modelName, err := getRequestModel(ctx, mode.Videos, "group-1", 7)
	require.NoError(t, err)
	assert.Equal(t, "sora-2", modelName)
	require.NotNil(t, req.MultipartForm)
}

func TestGetRequestModelVideosJSON(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/videos",
		bytes.NewBufferString(`{"model":"sora-2","prompt":"A city street"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	modelName, err := getRequestModel(ctx, mode.Videos, "group-1", 7)
	require.NoError(t, err)
	assert.Equal(t, "sora-2", modelName)
}

func TestGetRequestModelVideosEditFallsBackToStoredVideoModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := coremodel.SaveStoreByScope(&coremodel.StoreV2{
			ID:        coremodel.VideoGenerationStoreID("video-123"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 42,
			Model:     "sora-2-pro",
			ExpiresAt: time.Now().Add(time.Hour),
		}, coremodel.ChannelScopeGroup)
		require.NoError(t, err)
		require.NoError(t, coremodel.CacheSetStoreByScope(&coremodel.StoreCache{
			ID:        coremodel.VideoGenerationStoreID("video-123"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 42,
			Model:     "sora-2-pro",
			ExpiresAt: time.Now().Add(time.Hour),
		}, coremodel.ChannelScopeGroup))

		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/videos/edits",
			bytes.NewBufferString(`{"prompt":"edit it","video":"video-123"}`),
		)
		req.Header.Set("Content-Type", "application/json")

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req
		ctx.Set(Mode, mode.VideosEdits)
		ctx.Set(GroupChannelMode, GroupChannelModeOwn)

		modelName, err := getRequestModel(ctx, mode.VideosEdits, "group-1", 7)
		require.NoError(t, err)
		assert.Equal(t, "sora-2-pro", modelName)
		assert.Equal(t, "video-123", GetVideoID(ctx))
		assert.Equal(t, 42, GetChannelID(ctx))
		assert.Equal(t, coremodel.ChannelScopeGroup, GetChannelScope(ctx))
	})
}

func TestGetRequestModelVideosEditPinsStoredVideoChannelWithRequestModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := coremodel.SaveStore(&coremodel.StoreV2{
			ID:        coremodel.VideoGenerationStoreID("video-123"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 42,
			Model:     "sora-2-pro",
			ExpiresAt: time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
		require.NoError(t, coremodel.CacheSetStore(&coremodel.StoreCache{
			ID:        coremodel.VideoGenerationStoreID("video-123"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 42,
			Model:     "sora-2-pro",
			ExpiresAt: time.Now().Add(time.Hour),
		}))

		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/videos/edits",
			bytes.NewBufferString(
				`{"model":"sora-2","prompt":"edit it","video":"video-123"}`,
			),
		)
		req.Header.Set("Content-Type", "application/json")

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req
		ctx.Set(Mode, mode.VideosEdits)

		modelName, err := getRequestModel(ctx, mode.VideosEdits, "group-1", 7)
		require.NoError(t, err)
		assert.Equal(t, "sora-2", modelName)
		assert.Equal(t, "video-123", GetVideoID(ctx))
		assert.Equal(t, 42, GetChannelID(ctx))
	})
}

func TestGetRequestModelVideosEditKeepsRequestModelForVideoURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/videos/edits",
			bytes.NewBufferString(
				`{"model":"sora-2","prompt":"edit it","video":"https://example.com/input.mp4"}`,
			),
		)
		req.Header.Set("Content-Type", "application/json")

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req
		ctx.Set(Mode, mode.VideosEdits)

		modelName, err := getRequestModel(ctx, mode.VideosEdits, "group-1", 7)
		require.NoError(t, err)
		assert.Equal(t, "sora-2", modelName)
		assert.Empty(t, GetVideoID(ctx))
		assert.Zero(t, GetChannelID(ctx))
	})
}

func TestGetRequestModelVideosExtensionFallsBackToStoredVideoModelMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := coremodel.SaveStore(&coremodel.StoreV2{
			ID:        coremodel.VideoGenerationStoreID("video-456"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 43,
			Model:     "sora-2-pro",
			ExpiresAt: time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
		require.NoError(t, coremodel.CacheSetStore(&coremodel.StoreCache{
			ID:        coremodel.VideoGenerationStoreID("video-456"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 43,
			Model:     "sora-2-pro",
			ExpiresAt: time.Now().Add(time.Hour),
		}))

		var body bytes.Buffer

		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("prompt", "extend it"))
		require.NoError(t, writer.WriteField("video", "video-456"))
		require.NoError(t, writer.Close())

		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/videos/extensions",
			&body,
		)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req
		ctx.Set(Mode, mode.VideosExtensions)

		modelName, err := getRequestModel(ctx, mode.VideosExtensions, "group-1", 7)
		require.NoError(t, err)
		assert.Equal(t, "sora-2-pro", modelName)
		assert.Equal(t, "video-456", GetVideoID(ctx))
		assert.Equal(t, 43, GetChannelID(ctx))
	})
}

func TestGetRequestModelVideosExtensionPinsStoredVideoChannelWithMultipartRequestModel(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		_, err := coremodel.SaveStore(&coremodel.StoreV2{
			ID:        coremodel.VideoGenerationStoreID("video-456"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 43,
			Model:     "sora-2-pro",
			ExpiresAt: time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
		require.NoError(t, coremodel.CacheSetStore(&coremodel.StoreCache{
			ID:        coremodel.VideoGenerationStoreID("video-456"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 43,
			Model:     "sora-2-pro",
			ExpiresAt: time.Now().Add(time.Hour),
		}))

		var body bytes.Buffer

		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "sora-2"))
		require.NoError(t, writer.WriteField("prompt", "extend it"))
		require.NoError(t, writer.WriteField("video", "video-456"))
		require.NoError(t, writer.Close())

		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/videos/extensions",
			&body,
		)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req
		ctx.Set(Mode, mode.VideosExtensions)

		modelName, err := getRequestModel(ctx, mode.VideosExtensions, "group-1", 7)
		require.NoError(t, err)
		assert.Equal(t, "sora-2", modelName)
		assert.Equal(t, "video-456", GetVideoID(ctx))
		assert.Equal(t, 43, GetChannelID(ctx))
	})
}

func withTestStoreDB(t *testing.T, fn func()) {
	t.Helper()

	oldLogDB := coremodel.LogDB
	oldDB := coremodel.DB
	oldRedisEnabled := common.RedisEnabled

	db, err := coremodel.OpenSQLite(filepath.Join(t.TempDir(), "middleware_store_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&coremodel.StoreV2{}, &coremodel.GroupChannelStoreV2{}))

	coremodel.LogDB = db
	coremodel.DB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		coremodel.LogDB = oldLogDB
		coremodel.DB = oldDB
		common.RedisEnabled = oldRedisEnabled

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	fn()
}
