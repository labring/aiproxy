//nolint:testpackage
package ali

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealtimeSTTBridge(t *testing.T) {
	const modelName = "qwen3-asr-flash-realtime"

	forwarded := make(chan []string, 1)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api-ws/v1/realtime", r.URL.Path)
		assert.Equal(t, modelName, r.URL.Query().Get("model"))
		assert.Equal(t, "Bearer provider-fixture", r.Header.Get("Authorization"))

		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()

		_ = conn.WriteJSON(
			map[string]any{
				"type":    "session.created",
				"session": map[string]string{"id": "upstream-session"},
			},
		)

		var types []string
		for {
			var event struct {
				Type  string `json:"type"`
				Audio string `json:"audio"`
			}
			if err := conn.ReadJSON(&event); err != nil {
				return
			}

			types = append(types, event.Type)
			switch event.Type {
			case "session.update":
				_ = conn.WriteJSON(map[string]string{"type": "session.updated"})
			case "input_audio_buffer.append":
				pcm, err := base64.StdEncoding.DecodeString(event.Audio)
				assert.NoError(t, err)
				assert.Len(t, pcm, 3200)

				_ = conn.WriteJSON(
					map[string]string{
						"type":    "conversation.item.input_audio_transcription.text",
						"item_id": "item-1",
						"text":    "",
						"stash":   "青问",
					},
				)
				_ = conn.WriteJSON(
					map[string]string{
						"type":    "conversation.item.input_audio_transcription.text",
						"item_id": "item-1",
						"text":    "",
						"stash":   "千问",
					},
				)
			case "session.finish":
				_ = conn.WriteJSON(
					map[string]string{
						"type":       "conversation.item.input_audio_transcription.completed",
						"item_id":    "item-1",
						"transcript": "千问识别测试。",
					},
				)
				_ = conn.WriteJSON(map[string]string{"type": "session.finished"})

				forwarded <- types

				return
			}
		}
	}))
	defer upstream.Close()

	results := make(chan adaptor.DoResponseResult, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = r
		mt := meta.NewMeta(
			&model.Channel{
				BaseURL: upstream.URL,
				Key:     "provider-fixture",
				Type:    model.ChannelTypeAli,
			},
			mode.AudioTranscription,
			modelName,
			model.ModelConfig{},
		)
		result, relayErr := RealtimeSTTHandler(mt, ctx)
		assert.Nil(t, relayErr)

		results <- result
	}))
	defer server.Close()

	conn, response, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(server.URL, "http"),
		nil,
	)
	require.NoError(t, err)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	require.NoError(
		t,
		conn.WriteJSON(
			map[string]any{
				"type":    "session.update",
				"session": map[string]any{"input_audio_format": "pcm", "sample_rate": 16000},
			},
		),
	)
	require.NoError(
		t,
		conn.WriteJSON(
			map[string]string{
				"type":  "input_audio_buffer.append",
				"audio": base64.StdEncoding.EncodeToString(make([]byte, 3200)),
			},
		),
	)
	require.NoError(t, conn.WriteJSON(map[string]string{"type": "session.finish"}))

	var types, partials []string
	for {
		var event struct {
			Type       string         `json:"type"`
			Stash      string         `json:"stash"`
			Transcript string         `json:"transcript"`
			Usage      map[string]any `json:"usage"`
		}
		require.NoError(t, conn.ReadJSON(&event))

		types = append(types, event.Type)
		if event.Stash != "" {
			partials = append(partials, event.Stash)
		}

		if event.Transcript != "" {
			assert.Equal(t, "千问识别测试。", event.Transcript)
		}

		if event.Type == "session.finished" {
			assert.Equal(t, float64(1), event.Usage["seconds"])
			assert.Equal(t, float64(1), event.Usage["total_tokens"])
			break
		}
	}

	assert.Equal(t, []string{"青问", "千问"}, partials)
	assert.Equal(
		t,
		[]string{
			"session.created",
			"session.updated",
			"conversation.item.input_audio_transcription.text",
			"conversation.item.input_audio_transcription.text",
			"conversation.item.input_audio_transcription.completed",
			"session.finished",
		},
		types,
	)
	assert.Equal(
		t,
		[]string{"session.update", "input_audio_buffer.append", "session.finish"},
		<-forwarded,
	)

	result := <-results
	assert.Equal(t, "upstream-session", result.UpstreamID)
	assert.Equal(t, model.ZeroNullInt64(1), result.Usage.TotalTokens)
}

func TestRealtimeSTTURL(t *testing.T) {
	for _, tt := range []struct {
		name, model, base, want string
		bad                     bool
	}{
		{name: "default", model: "qwen3-asr-flash-realtime", want: "wss://dashscope.aliyuncs.com/api-ws/v1/realtime?model=qwen3-asr-flash-realtime"},
		{name: "mapped snapshot", model: "qwen3-asr-flash-realtime-2026-02-10", base: "https://dashscope.aliyuncs.com/compatible-mode/v1", want: "wss://dashscope.aliyuncs.com/api-ws/v1/realtime?model=qwen3-asr-flash-realtime-2026-02-10"},
		{name: "local", model: "qwen3-asr-flash-realtime", base: "http://localhost:9000/v1", want: "ws://localhost:9000/api-ws/v1/realtime?model=qwen3-asr-flash-realtime"},
		{name: "message model", model: realtimeMessageModel, want: "wss://dashscope.aliyuncs.com/api-ws/v1/inference"},
		{name: "message workspace", model: realtimeMessageModel, base: "https://workspace.cn-beijing.maas.aliyuncs.com/compatible-mode/v1?model=old", want: "wss://workspace.cn-beijing.maas.aliyuncs.com/api-ws/v1/inference"},
		{name: "batch model", model: "qwen3-asr-flash", bad: true},
		{name: "credentials", model: "qwen3-asr-flash-realtime", base: (&url.URL{Scheme: "https", Host: "example.com", User: url.UserPassword("fixture", "fixture")}).String(), bad: true},
		{name: "invalid scheme", model: "qwen3-asr-flash-realtime", base: "file://example.com", bad: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mt := meta.NewMeta(
				&model.Channel{BaseURL: tt.base},
				mode.AudioTranscription,
				tt.model,
				model.ModelConfig{},
			)

			u, err := realtimeSTTURL(mt)
			if tt.bad {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, u.String())
		})
	}
}

func TestRealtimeSTTRejectOriginBeforeDial(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://gateway/v1/audio/realtime",
		nil,
	)
	ctx.Request.Header.Set("Origin", "https://other.example")

	mt := meta.NewMeta(
		&model.Channel{BaseURL: "http://127.0.0.1:1"},
		mode.AudioTranscription,
		"qwen3-asr-flash-realtime",
		model.ModelConfig{},
	)
	_, err := RealtimeSTTHandler(mt, ctx)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusForbidden, err.StatusCode())
}

func TestRealtimeSTTRejectInvalidClientFrames(t *testing.T) {
	for _, tt := range []struct {
		name, message string
		binary        bool
	}{
		{name: "unknown event", message: `{"type":"response.create"}`},
		{name: "model override", message: `{"type":"session.update","session":{"model":"other"}}`},
		{name: "sample rate", message: `{"type":"session.update","session":{"sample_rate":48000}}`},
		{name: "compressed audio", message: `{"type":"session.update","session":{"input_audio_format":"opus"}}`},
		{name: "invalid base64", message: `{"type":"input_audio_buffer.append","audio":"!"}`},
		{name: "odd PCM byte", message: `{"type":"input_audio_buffer.append","audio":"AA=="}`},
		{name: "empty audio", message: `{"type":"input_audio_buffer.append","audio":""}`},
		{name: "binary frame", message: "audio", binary: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			results := make(chan error, 1)

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
					if !assert.NoError(t, err) {
						return
					}
					defer conn.Close()

					var count atomic.Int64
					results <- forwardRealtimeAudio(conn, nil, &count, time.Now(), nil)
				}),
			)
			defer server.Close()

			conn, response, err := websocket.DefaultDialer.Dial(
				"ws"+strings.TrimPrefix(server.URL, "http"),
				nil,
			)
			require.NoError(t, err)

			if response != nil && response.Body != nil {
				defer response.Body.Close()
			}

			defer conn.Close()

			typ := websocket.TextMessage
			if tt.binary {
				typ = websocket.BinaryMessage
			}

			require.NoError(t, conn.WriteMessage(typ, []byte(tt.message)))
			require.Error(t, <-results)
		})
	}
}

func TestRealtimeFinishedDataPreservesTokens(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{body: `{"type":"session.finished","usage":{"total_tokens":777,"vendor":{"kept":true}}}`, want: `{"type":"session.finished","usage":{"total_tokens":777,"vendor":{"kept":true},"seconds":5}}`},
		{body: `{"type":"session.finished","usage":{"total_tokens":777,"seconds":0}}`, want: `{"type":"session.finished","usage":{"total_tokens":777,"seconds":0}}`},
		{body: `{"type":"session.finished","usage":null}`, want: `{"type":"session.finished","usage":{"type":"duration","seconds":5,"total_tokens":5}}`},
	} {
		got, err := realtimeFinishedData([]byte(tt.body), 5)
		require.NoError(t, err)
		assert.JSONEq(t, tt.want, string(got))
	}
}

func TestRealtimeSTTOriginAllowed(t *testing.T) {
	for _, tt := range []struct {
		origin  string
		allowed bool
	}{
		{origin: "", allowed: true},
		{origin: "http://gateway:3810", allowed: true},
		{origin: "https://GATEWAY:3810", allowed: true},
		{origin: "http://other:3810", allowed: false},
		{origin: "http://gateway:3811", allowed: false},
		{origin: "null", allowed: false},
		{origin: "ftp://gateway:3810", allowed: false},
		{origin: "http://fixture@gateway:3810", allowed: false},
	} {
		t.Run(tt.origin, func(t *testing.T) {
			r := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"http://gateway:3810/v1/audio/realtime",
				nil,
			)
			r.Header.Set("Origin", tt.origin)
			assert.Equal(t, tt.allowed, realtimeSTTOriginAllowed(r))
		})
	}
}
