//nolint:testpackage
package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestChannelRequestOptionsForModel(t *testing.T) {
	defaultMode := mode.ChatCompletions
	overrideMode := mode.Completions
	request := &TestChannelRequest{
		RequestBody: json.RawMessage(`{"model":"default"}`),
		Mode:        &defaultMode,
		ModelOverrides: map[string]TestModelOverride{
			"special": {
				RequestBody: json.RawMessage(`{"model":"special"}`),
				Mode:        &overrideMode,
			},
			"body-only": {RequestBody: json.RawMessage(`{"input":"hello"}`)},
			"mode-only": {Mode: &overrideMode},
		},
	}

	defaultOptions := request.optionsForModel("default")
	require.Equal(t, request.RequestBody, defaultOptions.RequestBody)
	require.Same(t, request.Mode, defaultOptions.Mode)

	overrideOptions := request.optionsForModel("special")
	require.Equal(t, json.RawMessage(`{"model":"special"}`), overrideOptions.RequestBody)
	require.Same(t, &overrideMode, overrideOptions.Mode)

	bodyOnly := request.optionsForModel("body-only")
	require.Equal(t, json.RawMessage(`{"input":"hello"}`), bodyOnly.RequestBody)
	require.Equal(t, &defaultMode, bodyOnly.Mode)
	modeOnly := request.optionsForModel("mode-only")
	require.Equal(t, request.RequestBody, modeOnly.RequestBody)
	require.Equal(t, &overrideMode, modeOnly.Mode)
	require.Equal(t, testOptions{}, (&TestChannelRequest{}).optionsForModel("default"))
}

func TestGetChannelNotFound(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "channels.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelTest{}))

	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	deleted := &model.Channel{Name: "Deleted channel", Type: model.ChannelTypeOpenAI}
	require.NoError(t, db.Create(deleted).Error)
	require.NoError(t, db.Delete(deleted).Error)

	for _, id := range []string{strconv.Itoa(deleted.ID), "999999"} {
		t.Run(id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "id", Value: id}}
			GetChannel(c)
			require.Equal(t, http.StatusNotFound, recorder.Code)
			require.Contains(t, recorder.Body.String(), "channel record not found")
		})
	}

	t.Run("database failure", func(t *testing.T) {
		require.NoError(t, db.Migrator().DropTable(&model.Channel{}))

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		GetChannel(c)
		require.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}

func TestAddChannelRequestToChannelPreservesNewlinesInKey(t *testing.T) {
	const key = "first-key\nsecond-key"

	channel, err := (&AddChannelRequest{
		Type: model.ChannelTypeOpenAI,
		Name: "channel",
		Key:  key,
	}).ToChannel()

	require.NoError(t, err)
	require.Equal(t, key, channel.Key)
}

func TestAddChannelRequestToChannelPreservesBackupOnly(t *testing.T) {
	t.Parallel()

	for _, backupOnly := range []bool{false, true} {
		channel, err := (&AddChannelRequest{
			Type: model.ChannelTypeOpenAI, Name: "channel", Key: "test-key", BackupOnly: backupOnly,
		}).ToChannel()
		require.NoError(t, err)
		require.Equal(t, backupOnly, channel.BackupOnly)
	}
}

func TestAddChannelRequestToChannelPreservesRemark(t *testing.T) {
	t.Parallel()

	channel, err := (&AddChannelRequest{
		Type:   model.ChannelTypeOpenAI,
		Name:   "channel",
		Key:    "test-key",
		Remark: "primary **production** channel",
	}).ToChannel()
	require.NoError(t, err)
	require.Equal(t, "primary **production** channel", channel.Remark)
}

func TestUpdateChannelRequestPreservesOmittedValues(t *testing.T) {
	t.Parallel()

	current := &model.Channel{
		Name:       "existing",
		Remark:     "keep",
		Key:        "key",
		Type:       model.ChannelTypeOpenAI,
		BackupOnly: true,
	}
	updated, err := (&UpdateChannelRequest{}).Apply(current)
	require.NoError(t, err)
	require.Equal(t, "keep", updated.Remark)
	require.True(t, updated.BackupOnly)
}

func TestUpdateChannelRequestAllowsExplicitZeroValues(t *testing.T) {
	t.Parallel()

	remark := ""
	backupOnly := false
	updated, err := (&UpdateChannelRequest{Remark: &remark, BackupOnly: &backupOnly}).Apply(
		&model.Channel{Key: "key", Type: model.ChannelTypeOpenAI, Remark: "old", BackupOnly: true},
	)
	require.NoError(t, err)
	require.Empty(t, updated.Remark)
	require.False(t, updated.BackupOnly)
}

func TestChannelFilterOptionalValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		query      string
		remark     *string
		backupOnly *bool
	}{
		{query: ""},
		{query: "?remark=", remark: new("")},
		{query: "?remark=production&backup_only=true", remark: new("production"), backupOnly: new(true)},
		{query: "?backup_only=false", backupOnly: new(false)},
	} {
		t.Run(test.query, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"/"+test.query,
				nil,
			)
			filter, err := parseChannelFilter(c)
			require.NoError(t, err)
			require.Equal(t, test.remark, filter.Remark)
			require.Equal(t, test.backupOnly, filter.BackupOnly)
		})
	}
}

func TestChannelProxyOptionalUpdates(t *testing.T) {
	t.Parallel()

	current := &model.Channel{
		Type: model.ChannelTypeOpenAI, Key: "test-key", ProxyURL: "legacy-invalid-proxy",
	}
	for _, tt := range []struct {
		body string
		want string
	}{
		{body: `{}`, want: current.ProxyURL},
		{body: `{"proxy_url":null}`, want: current.ProxyURL},
		{body: `{"proxy_url":""}`, want: ""},
		{body: `{"proxy_url":"socks5://proxy.example:1080"}`, want: "socks5://proxy.example:1080"},
	} {
		var request UpdateChannelRequest
		require.NoError(t, sonic.UnmarshalString(tt.body, &request))
		updated, err := request.Apply(current)
		require.NoError(t, err)
		require.Equal(t, tt.want, updated.ProxyURL)
		require.Equal(t, "legacy-invalid-proxy", current.ProxyURL)
	}

	for _, proxyURL := range []string{"http://", "proxy.example:8080", "socks5://proxy.example:65536"} {
		_, err := (&AddChannelRequest{
			Type: model.ChannelTypeOpenAI, Key: "test-key", ProxyURL: proxyURL,
		}).ToChannel()
		require.Error(t, err)
		_, err = (&UpdateChannelRequest{ProxyURL: &proxyURL}).Apply(current)
		require.Error(t, err)
	}
}

func TestChannelFiltersRejectInvalidBooleans(t *testing.T) {
	t.Parallel()

	for _, handler := range []gin.HandlerFunc{GetChannels, SearchChannels} {
		for _, query := range []string{"?backup_only=", "?backup_only=maybe", "?backup_only=true&backup_only=false"} {
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+query, nil)
			handler(c)
			require.Equal(t, http.StatusBadRequest, response.Code)
		}
	}
}

func TestUpdateChannelRequestJSONPresence(t *testing.T) {
	t.Parallel()

	current := &model.Channel{
		Key: "key", Type: model.ChannelTypeOpenAI, Name: "keep", Remark: "keep", BackupOnly: true,
		Models: []string{"keep"}, ModelMapping: map[string]string{"keep": "keep"},
		Configs: model.ChannelConfigs{"keep": "keep"}, Sets: []string{"keep"},
	}
	for _, body := range []string{"{}", `{"remark":null,"backup_only":null,"models":null,"configs":null}`} {
		var request UpdateChannelRequest
		require.NoError(t, sonic.Unmarshal([]byte(body), &request))
		updated, err := request.Apply(current)
		require.NoError(t, err)
		require.Equal(t, current, updated)

		encoded, err := sonic.Marshal(request)
		require.NoError(t, err)
		require.JSONEq(t, "{}", string(encoded))
	}

	var request UpdateChannelRequest
	require.NoError(
		t,
		sonic.Unmarshal(
			[]byte(
				`{"remark":"","backup_only":false,"models":[],"model_mapping":{},"configs":{},"sets":[]}`,
			),
			&request,
		),
	)
	updated, err := request.Apply(current)
	require.NoError(t, err)
	require.Empty(t, updated.Remark)
	require.False(t, updated.BackupOnly)
	require.Empty(t, updated.Models)
	require.Empty(t, updated.ModelMapping)
	require.Empty(t, updated.Configs)
	require.Empty(t, updated.Sets)
	require.Equal(t, "keep", current.Remark)
	require.True(t, current.BackupOnly)

	encoded, err := sonic.Marshal(request)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{"remark":"","backup_only":false,"models":[],"model_mapping":{},"configs":{},"sets":[]}`,
		string(encoded),
	)
}

func TestRunAutoTestBannedModelsHonorsConcurrencyLimit(t *testing.T) {
	const (
		concurrency = 7
		totalJobs   = 41
	)

	channels := map[string][]int64{
		"model-a": make([]int64, totalJobs),
	}
	for i := range totalJobs {
		channels["model-a"][i] = int64(i + 1)
	}

	var (
		currentActive atomic.Int32
		maxActive     atomic.Int32
		processed     atomic.Int32
	)

	deps := autoTestBannedModelsDeps{
		tryTestChannel: func(channelID int, modelName string) bool {
			return true
		},
		loadChannelByID: func(id int) (*model.Channel, error) {
			return &model.Channel{
				ID:     id,
				Name:   "channel",
				Type:   model.ChannelTypeOpenAI,
				Status: model.ChannelStatusEnabled,
				Models: []string{"model-a"},
			}, nil
		},
		testSingleModel: func(mc *model.ModelCaches, channel *model.Channel, modelName string, saveToDB bool) (*model.ChannelTest, error) {
			active := currentActive.Add(1)
			for {
				previous := maxActive.Load()
				if active <= previous || maxActive.CompareAndSwap(previous, active) {
					break
				}
			}

			time.Sleep(15 * time.Millisecond)

			processed.Add(1)
			currentActive.Add(-1)

			return &model.ChannelTest{Success: true}, nil
		},
		clearChannelModelErrors: func(ctx context.Context, modelName string, channelID int) error {
			return nil
		},
		notifyInfo:  func(title, message string) {},
		notifyError: func(title, message string) {},
	}

	runAutoTestBannedModels(log.NewEntry(log.StandardLogger()), channels, nil, concurrency, deps)

	require.Equal(t, int32(totalJobs), processed.Load())
	require.LessOrEqual(t, maxActive.Load(), int32(concurrency))
}

func TestRunAutoTestBannedModelsClearsWhenModelRemovedFromChannel(t *testing.T) {
	var (
		cleared        atomic.Int32
		testInvoked    atomic.Bool
		clearedChannel atomic.Int64
	)

	deps := autoTestBannedModelsDeps{
		tryTestChannel: func(channelID int, modelName string) bool {
			return true
		},
		loadChannelByID: func(id int) (*model.Channel, error) {
			return &model.Channel{
				ID:     id,
				Name:   "channel",
				Type:   model.ChannelTypeOpenAI,
				Status: model.ChannelStatusEnabled,
				Models: []string{"another-model"},
			}, nil
		},
		testSingleModel: func(mc *model.ModelCaches, channel *model.Channel, modelName string, saveToDB bool) (*model.ChannelTest, error) {
			testInvoked.Store(true)
			return &model.ChannelTest{Success: true}, nil
		},
		clearChannelModelErrors: func(ctx context.Context, modelName string, channelID int) error {
			cleared.Add(1)
			clearedChannel.Store(int64(channelID))
			return nil
		},
		notifyInfo:  func(title, message string) {},
		notifyError: func(title, message string) {},
	}

	runAutoTestBannedModels(
		log.NewEntry(log.StandardLogger()),
		map[string][]int64{"removed-model": {123}},
		nil,
		1,
		deps,
	)

	require.False(t, testInvoked.Load())
	require.Equal(t, int32(1), cleared.Load())
	require.Equal(t, int64(123), clearedChannel.Load())
}
