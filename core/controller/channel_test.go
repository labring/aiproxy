//nolint:testpackage
package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

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
