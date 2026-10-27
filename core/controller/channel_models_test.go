package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestGetChannelModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "channels.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))

	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	const channelKey = "catalog-test-secret"
	const failureKey = "provider-secret-from-error"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		if r.URL.Path == "/fail/v1/models" {
			require.Equal(t, "Bearer "+failureKey, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte(`{"error":"upstream detail: provider-secret-from-error"}`))
			require.NoError(t, err)
			return
		}
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer "+channelKey, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"object":"list","data":[{"id":"upstream-only-model","created":1720000000,"owned_by":"provider"}]}`))
		require.NoError(t, err)
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Type:    model.ChannelTypeOpenAI,
		Key:     channelKey,
		BaseURL: upstream.URL + "/v1",
		Models:  []string{"configured-model"},
		Name:    "catalog test",
		Status:  model.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channel.ID)}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/"+strconv.Itoa(channel.ID)+"/models", nil)

	GetChannelModels(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool             `json:"success"`
		Data    []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	require.Equal(t, "upstream-only-model", response.Data[0]["id"])
	require.NotContains(t, recorder.Body.String(), "owned_by")
	require.NotContains(t, recorder.Body.String(), channelKey)

	knownModelChannel := &model.Channel{
		Type:    model.ChannelTypeOpenAI,
		Key:     channelKey,
		BaseURL: upstream.URL + "/v1",
		Name:    "known model catalog test",
		Status:  model.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(knownModelChannel).Error)

	knownModelUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"data":[{"id":"gpt-6-astra"}]}`))
		require.NoError(t, err)
	}))
	defer knownModelUpstream.Close()
	knownModelChannel.BaseURL = knownModelUpstream.URL + "/v1"
	require.NoError(t, db.Save(knownModelChannel).Error)

	failingChannel := &model.Channel{
		Type:    model.ChannelTypeOpenAI,
		Key:     failureKey,
		BaseURL: upstream.URL + "/fail/v1",
		Name:    "failing catalog test",
		Status:  model.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(failingChannel).Error)
	failureRecorder := httptest.NewRecorder()
	failureContext, _ := gin.CreateTestContext(failureRecorder)
	failureContext.Params = gin.Params{{Key: "id", Value: strconv.Itoa(failingChannel.ID)}}
	failureContext.Request = httptest.NewRequest(http.MethodGet, "/api/channel/"+strconv.Itoa(failingChannel.ID)+"/models", nil)

	GetChannelModels(failureContext)

	require.Equal(t, http.StatusBadGateway, failureRecorder.Code)
	require.Contains(t, failureRecorder.Body.String(), "failed to retrieve provider model catalog")
	require.NotContains(t, failureRecorder.Body.String(), failureKey)
	require.NotContains(t, failureRecorder.Body.String(), "upstream detail")
}

func TestGetChannelModelsReadsAIProxyGateway(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "channel-models.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))

	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	const apiKey = "aiproxy-gateway-key"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))
		_, err := w.Write([]byte(`{"data":[{"id":"gateway-model"}]}`))
		require.NoError(t, err)
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Type:    model.ChannelTypeAIProxyHZH,
		Key:     apiKey,
		BaseURL: upstream.URL + "/v1",
		Status:  model.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channel.ID)}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/"+strconv.Itoa(channel.ID)+"/models", nil)
	GetChannelModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "not an upstream provider model catalog")
}
