//nolint:testpackage
package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckRelayModeAliVideo(t *testing.T) {
	t.Parallel()

	require.True(t, CheckRelayMode(mode.AliVideo, mode.AliVideo))
	require.True(t, CheckRelayMode(mode.AliVideoTasks, mode.AliVideo))
	require.True(t, CheckRelayMode(mode.VideoGenerationsJobs, mode.AliVideo))
	require.True(t, CheckRelayMode(mode.Videos, mode.AliVideo))
	require.True(t, CheckRelayMode(mode.VideosEdits, mode.AliVideo))

	require.False(t, CheckRelayMode(mode.AliVideo, mode.VideoGenerationsJobs))
	require.False(t, CheckRelayMode(mode.ChatCompletions, mode.AliVideo))
}

func TestCheckRelayModeDoubaoVideo(t *testing.T) {
	t.Parallel()

	require.True(t, CheckRelayMode(mode.DoubaoVideo, mode.DoubaoVideo))
	require.True(t, CheckRelayMode(mode.DoubaoVideoTasks, mode.DoubaoVideo))
	require.True(t, CheckRelayMode(mode.DoubaoVideoTasksDelete, mode.DoubaoVideo))
	require.True(t, CheckRelayMode(mode.VideoGenerationsJobs, mode.DoubaoVideo))
	require.True(t, CheckRelayMode(mode.Videos, mode.DoubaoVideo))
	require.True(t, CheckRelayMode(mode.VideosExtensions, mode.DoubaoVideo))

	require.False(t, CheckRelayMode(mode.DoubaoVideo, mode.VideoGenerationsJobs))
	require.False(t, CheckRelayMode(mode.ChatCompletions, mode.DoubaoVideo))
}

func TestGetRequestModelProviderVideoCreateJSON(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	for _, relayMode := range []mode.Mode{mode.AliVideo, mode.DoubaoVideo} {
		t.Run(relayMode.String(), func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/api/video",
				bytes.NewBufferString(`{"model":"provider-video","input":{"prompt":"go"}}`),
			)
			req.Header.Set("Content-Type", "application/json")

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = req

			modelName, err := getRequestModel(ctx, relayMode, "group-1", 7)
			require.NoError(t, err)
			assert.Equal(t, "provider-video", modelName)
		})
	}
}

func TestGetRequestModelProviderVideoTaskPinsStoredChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTestStoreDB(t, func() {
		require.NoError(t, model.CacheSetStoreByScope(&model.StoreCache{
			ID:        model.VideoGenerationStoreID("task-123"),
			GroupID:   "group-1",
			TokenID:   7,
			ChannelID: 42,
			Model:     "provider-video",
			ExpiresAt: time.Now().Add(time.Hour),
		}, model.ChannelScopeGroup))

		for _, relayMode := range []mode.Mode{
			mode.AliVideoTasks,
			mode.DoubaoVideoTasks,
			mode.DoubaoVideoTasksDelete,
		} {
			t.Run(relayMode.String(), func(t *testing.T) {
				req := httptest.NewRequestWithContext(
					t.Context(),
					http.MethodGet,
					"/api/tasks/task-123",
					nil,
				)

				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = req
				ctx.Params = gin.Params{{Key: "task_id", Value: "task-123"}}
				ctx.Set(GroupChannelMode, GroupChannelModeOwn)

				modelName, err := getRequestModel(ctx, relayMode, "group-1", 7)
				require.NoError(t, err)
				assert.Equal(t, "provider-video", modelName)
				assert.Equal(t, "task-123", GetVideoID(ctx))
				assert.Equal(t, 42, GetChannelID(ctx))
				assert.Equal(t, model.ChannelScopeGroup, GetChannelScope(ctx))
			})
		}
	})
}
