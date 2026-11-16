//nolint:testpackage
package ali

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

// TestAliSTTLive is opt-in: it reads credentials in memory from an existing dev
// channel and runs the local production relay against that channel's upstream.
// It never saves or changes the dev gateway's channels/models and never logs keys.
func TestAliSTTLive(t *testing.T) {
	envFile := os.Getenv("AIPROXY_STT_DEV_ENV_FILE")

	audioFile := os.Getenv("AIPROXY_STT_LIVE_AUDIO_FILE")
	if envFile == "" || audioFile == "" {
		t.Skip(
			"set AIPROXY_STT_DEV_ENV_FILE and AIPROXY_STT_LIVE_AUDIO_FILE for paid live STT checks",
		)
	}

	setSTTFFmpeg(t, true)

	env, err := godotenv.Read(envFile)
	require.NoError(t, err)

	channelID := os.Getenv("AIPROXY_STT_LIVE_CHANNEL_ID")
	if channelID == "" {
		t.Fatal("AIPROXY_STT_LIVE_CHANNEL_ID is required")
	}

	_, err = strconv.Atoi(channelID)
	require.NoError(t, err)
	// #nosec G704 -- This opt-in test uses an operator-provided dev API endpoint.
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		strings.TrimRight(env["AIPROXY_API_ENDPOINT"], "/")+"/api/channel/"+channelID,
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+env["AIPROXY_API_TOKEN"])
	// #nosec G704 -- Only the operator-provided dev endpoint is requested.
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var channel struct {
		Success bool `json:"success"`
		Data    struct {
			Key           string            `json:"key"`
			BaseURL       string            `json:"base_url"`
			ProxyURL      string            `json:"proxy_url"`
			Type          model.ChannelType `json:"type"`
			SkipTLSVerify bool              `json:"skip_tls_verify"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&channel))
	require.True(t, channel.Success)
	require.NotEmpty(t, channel.Data.Key)
	require.Equal(t, model.ChannelTypeAli, channel.Data.Type)

	// #nosec G703 -- Explicit local fixture path selected by the operator.
	audio, err := os.ReadFile(audioFile)
	require.NoError(t, err)

	for _, modelName := range []string{"paraformer-realtime-v2", "fun-asr-realtime", "qwen3-asr-flash"} {
		t.Run(modelName, func(t *testing.T) {
			ch := model.Channel{
				Key:           channel.Data.Key,
				BaseURL:       channel.Data.BaseURL,
				ProxyURL:      channel.Data.ProxyURL,
				Type:          channel.Data.Type,
				SkipTLSVerify: channel.Data.SkipTLSVerify,
				Configs:       model.ChannelConfigs{"stt_upstream_protocol": "auto"},
			}
			mt := meta.NewMeta(&ch, mode.AudioTranscription, modelName, model.ModelConfig{})
			mt.RequestTimeout = 30 * time.Second
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = sttUpload(t, audio, nil)
			start := time.Now()
			_, _, relayErr := controller.DoHelper(&Adaptor{}, c, mt, nil)
			require.Nil(t, relayErr)
			require.Equal(t, http.StatusOK, recorder.Code)

			var body struct {
				Text  string `json:"text"`
				Usage struct {
					Seconds int64 `json:"seconds"`
					Total   int64 `json:"total_tokens"`
				} `json:"usage"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.NotEmpty(t, strings.TrimSpace(body.Text))
			require.Positive(t, body.Usage.Seconds)
			t.Logf(
				"model=%s transcript=%q seconds=%d tokens=%d elapsed=%s",
				modelName,
				body.Text,
				body.Usage.Seconds,
				body.Usage.Total,
				time.Since(start).Round(time.Millisecond),
			)
		})
	}
}
