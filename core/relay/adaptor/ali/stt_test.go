//nolint:testpackage
package ali

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSTTProtocolSelection(t *testing.T) {
	for _, tt := range []struct {
		name, actual, protocol string
		overrides              map[string]string
		want                   string
		bad                    bool
	}{
		{name: "legacy realtime", actual: "paraformer-realtime-v2", want: sttRealtime},
		{name: "fun realtime", actual: "fun-asr-realtime", want: sttRealtime},
		{name: "Qwen ASR", actual: "qwen3-asr-flash", want: sttChatAudio},
		{name: "dated Qwen", actual: "qwen3-asr-flash-2026-02-10", want: sttChatAudio},
		{name: "channel default", actual: "private-asr", protocol: sttChatAudio, want: sttChatAudio},
		{name: "Whisper service", actual: "qwen3-asr-flash", protocol: sttTranscription, want: sttTranscription},
		{name: "mapped override", actual: "deployment", protocol: sttRealtime, overrides: map[string]string{"deployment": sttChatAudio}, want: sttChatAudio},
		{name: "public override first", actual: "deployment", overrides: map[string]string{"alias": sttTranscription, "deployment": sttChatAudio}, want: sttTranscription},
		{name: "auto overrides default", actual: "qwen3-asr-flash", protocol: sttRealtime, overrides: map[string]string{"alias": "auto"}, want: sttChatAudio},
		{name: "unsupported value", actual: "deployment", protocol: "typo", bad: true},
		{name: "async paraformer", actual: "paraformer-v2", bad: true},
		{name: "async fun", actual: "fun-asr", bad: true},
		{name: "async qwen", actual: "qwen3-asr-flash-filetrans", bad: true},
		{name: "session Qwen realtime", actual: "qwen3-asr-flash-realtime", bad: true},
		{name: "native Fun flash", actual: "fun-asr-flash-2026-06-15", bad: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mt := meta.NewMeta(
				&model.Channel{
					Configs: model.ChannelConfigs{
						"stt_upstream_protocol": tt.protocol,
						"stt_model_protocols":   tt.overrides,
					},
					ModelMapping: map[string]string{"alias": tt.actual},
				},
				mode.AudioTranscription,
				"alias",
				model.ModelConfig{},
			)

			got, err := resolveSTTProtocol(mt)
			if tt.bad {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	mt := meta.NewMeta(nil, mode.AudioTranscription, "alias", model.ModelConfig{})
	for _, protocol := range []string{sttChatAudio, sttTranscription, sttRealtime} {
		mt.SetChannel(
			&model.Channel{Configs: model.ChannelConfigs{"stt_upstream_protocol": protocol}},
		)
		got, err := resolveSTTProtocol(mt)
		require.NoError(t, err)
		assert.Equal(t, protocol, got)
	}
}

// sttWAV is a valid mono PCM16 recording with a known duration and no external fixture.
func sttWAV() []byte {
	pcm := bytes.Repeat([]byte{0, 1}, 1600)

	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(3236))
	b.WriteString("WAVEfmt ")

	for _, value := range []any{uint32(16), uint16(1), uint16(1), uint32(16000), uint32(32000), uint16(2), uint16(16)} {
		_ = binary.Write(&b, binary.LittleEndian, value)
	}

	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(3200))
	b.Write(pcm)

	return b.Bytes()
}

func sttUpload(t *testing.T, audio []byte, fields map[string]string) *http.Request {
	t.Helper()

	var b bytes.Buffer

	w := multipart.NewWriter(&b)
	require.NoError(t, w.WriteField("model", "alias"))

	for key, value := range fields {
		require.NoError(t, w.WriteField(key, value))
	}

	f, err := w.CreateFormFile("file", "misleading.mp3")
	require.NoError(t, err)
	_, err = f.Write(audio)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/audio/transcriptions",
		&b,
	)
	req.Header.Set("Content-Type", w.FormDataContentType())
	t.Cleanup(func() {
		if req.MultipartForm != nil {
			_ = req.MultipartForm.RemoveAll()
		}
	})

	return req
}

func setSTTFFmpeg(t *testing.T, enabled bool) {
	t.Helper()

	old := config.FfmpegEnabled
	config.FfmpegEnabled = enabled
	t.Cleanup(func() { config.FfmpegEnabled = old })
}

func TestSTTChatAudioConversion(t *testing.T) {
	setSTTFFmpeg(t, false)
	req := sttUpload(t, sttWAV(), map[string]string{"prompt": "产品名称 FastGPT", "language": "zh"})
	mt := meta.NewMeta(
		&model.Channel{ModelMapping: map[string]string{"alias": "qwen3-asr-flash"}},
		mode.AudioTranscription,
		"alias",
		model.ModelConfig{},
	)
	converted, err := convertSTTRequest(mt, req)
	require.NoError(t, err)

	var body struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Options map[string]string `json:"asr_options"`
	}
	require.NoError(t, json.NewDecoder(converted.Body).Decode(&body))
	assert.Equal(t, "qwen3-asr-flash", body.Model)
	assert.False(t, body.Stream)
	require.Len(t, body.Messages, 2)
	assert.Equal(t, "system", body.Messages[0].Role)
	assert.JSONEq(t, `"产品名称 FastGPT"`, string(body.Messages[0].Content))
	assert.Equal(t, "zh", body.Options["language"])

	var content []struct {
		Type  string `json:"type"`
		Audio struct {
			Data string `json:"data"`
		} `json:"input_audio"`
	}
	require.NoError(t, json.Unmarshal(body.Messages[1].Content, &content))
	require.Len(t, content, 1)
	assert.Equal(t, "input_audio", content[0].Type)
	require.True(t, strings.HasPrefix(content[0].Audio.Data, "data:audio/wav;base64,"))
	data, err := base64.StdEncoding.DecodeString(
		strings.TrimPrefix(content[0].Audio.Data, "data:audio/wav;base64,"),
	)
	require.NoError(t, err)
	assert.Equal(t, sttWAV(), data)

	for _, fields := range []map[string]string{{"response_format": "verbose_json"}, {"stream": "true"}} {
		_, err := convertSTTRequest(mt, sttUpload(t, sttWAV(), fields))
		require.Error(t, err)
	}
}

func TestSTTAudioContentAndTranscoding(t *testing.T) {
	setSTTFFmpeg(t, false)

	mt := meta.NewMeta(nil, mode.AudioTranscription, "paraformer-realtime-v2", model.ModelConfig{})
	audio, format, rate, err := prepareSTTAudio(mt, sttUpload(t, sttWAV(), nil), true)
	require.NoError(t, err)
	assert.Equal(t, "pcm", format)
	assert.Equal(t, 16000, rate)
	assert.Len(t, audio, 3200)
	_, format, rate, err = prepareSTTAudio(mt, sttUpload(t, []byte{0xff, 0xf3, 0x64, 0}, nil), true)
	require.NoError(t, err)
	assert.Equal(t, "mp3", format)
	assert.Equal(t, 24000, rate)
	_, _, _, err = prepareSTTAudio(mt, sttUpload(t, []byte{0x1a, 0x45, 0xdf, 0xa3, 0}, nil), true)
	require.ErrorContains(t, err, "enable FFMPEG_ENABLED")
	_, _, _, err = prepareSTTAudio(mt, sttUpload(t, nil, nil), true)
	require.ErrorContains(t, err, "empty")
	_, _, _, err = prepareSTTAudio(
		mt,
		sttUpload(t, []byte{0, 0}, map[string]string{"format": "pcm"}),
		true,
	)
	require.Error(t, err)
	_, _, _, err = prepareSTTAudio(
		mt,
		sttUpload(t, sttWAV(), map[string]string{"sample_rate": "oops"}),
		true,
	)
	require.Error(t, err)

	corrupt := sttWAV()
	binary.LittleEndian.PutUint32(corrupt[40:44], 0xffffffff)
	_, _, _, err = prepareSTTAudio(mt, sttUpload(t, corrupt, nil), true)
	require.Error(t, err)

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Log("FFmpeg transcoding case skipped: binary not installed")
		return
	}

	cmd := exec.CommandContext(
		t.Context(),
		"ffmpeg",
		"-hide_banner",
		"-loglevel",
		"error",
		"-i",
		"pipe:0",
		"-c:a",
		"libopus",
		"-f",
		"webm",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(sttWAV())
	webm, err := cmd.Output()
	require.NoError(t, err)

	config.FfmpegEnabled = true
	audio, format, rate, err = prepareSTTAudio(mt, sttUpload(t, webm, nil), true)
	require.NoError(t, err)
	assert.Equal(t, "pcm", format)
	assert.Equal(t, 16000, rate)
	assert.NotEmpty(t, audio)
	limitedReq := sttUpload(t, webm, nil)
	oldLimit := config.MaxRequestBodySize
	config.MaxRequestBodySize = limitedReq.ContentLength + 1
	t.Cleanup(func() { config.MaxRequestBodySize = oldLimit })

	_, _, _, err = prepareSTTAudio(mt, limitedReq, true)
	require.ErrorContains(t, err, "decoded audio exceeds")

	config.MaxRequestBodySize = oldLimit
	_, _, _, err = prepareSTTAudio(mt, sttUpload(t, []byte("not audio"), nil), true)
	require.Error(t, err)
}

func TestSTTChatResponseAndErrors(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
		wantError  bool
	}{
		{name: "tokens and seconds", body: `{"id":"req-1","choices":[{"message":{"content":"你好"}}],"usage":{"prompt_tokens":50,"completion_tokens":3,"total_tokens":53,"seconds":2,"prompt_tokens_details":{"audio_tokens":50}}}`, status: 200},
		{name: "silence is valid", body: `{"choices":[{"message":{"content":""}}]}`, status: 200},
		{name: "missing content", body: `{"choices":[]}`, status: 200, wantError: true},
		{name: "malformed", body: `not json`, status: 200, wantError: true},
		{name: "upstream error", body: `{"code":"InvalidParameter","message":"bad audio"}`, status: 400, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mt := meta.NewMeta(
				nil,
				mode.AudioTranscription,
				"qwen3-asr-flash",
				model.ModelConfig{},
				meta.WithRequestUsage(model.Usage{AudioInputTokens: 2}),
			)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = sttUpload(t, sttWAV(), nil)

			result, err := sttResponse(
				mt,
				c,
				&http.Response{
					StatusCode: tt.status,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				},
			)
			if tt.wantError {
				require.NotNil(t, err)

				if tt.status == 400 {
					assert.Equal(t, 400, err.StatusCode())
				}

				return
			}

			require.Nil(t, err)

			if tt.name == "tokens and seconds" {
				assert.Contains(t, recorder.Body.String(), `"seconds":2`)
				assert.Contains(t, recorder.Body.String(), `"total_tokens":53`)
				assert.EqualValues(t, 50, result.Usage.AudioInputTokens)
			}
		})
	}
}

func TestSTTRelayProtocolsAndChannelSwitch(t *testing.T) {
	setSTTFFmpeg(t, false)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer channel-key", r.Header.Get("Authorization"))

		switch r.URL.Path {
		case "/compatible-mode/v1/chat/completions":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "deployment", body["model"])
			assert.Contains(t, body, "messages")
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(
				w,
				`{"choices":[{"message":{"content":"你好"}}],"usage":{"prompt_tokens":25,"completion_tokens":2,"total_tokens":27,"seconds":1}}`,
			)
		case "/compatible-mode/v1/audio/transcriptions":
			require.NoError(t, common.ParseMultipartFormWithLimit(r))
			defer func() { _ = r.MultipartForm.RemoveAll() }()

			assert.Equal(t, "deployment", r.FormValue("model"))
			f, _, err := r.FormFile("file")
			require.NoError(t, err)

			defer f.Close()

			data, err := io.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, sttWAV(), data)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"text":"你好"}`)
		case "/api-ws/v1/inference":
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			require.NoError(t, err)

			defer conn.Close()

			var run STTMessage
			require.NoError(t, conn.ReadJSON(&run))
			assert.Equal(t, "deployment", run.Payload.Model)
			assert.Equal(t, "pcm", run.Payload.Parameters.Format)
			assert.Equal(t, 16000, run.Payload.Parameters.SampleRate)
			require.NoError(t, conn.WriteJSON(STTMessage{Header: STTHeader{Event: "task-started"}}))

			var received []byte
			for {
				typ, data, err := conn.ReadMessage()
				require.NoError(t, err)

				if typ == websocket.TextMessage {
					break
				}

				received = append(received, data...)
			}

			assert.Len(t, received, 3200)

			// Billing is cumulative on result events, not on task-finished.
			for _, event := range []string{
				`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"text":"interim","end_time":100,"sentence_end":false}},"usage":null}}`,
				`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"text":"你好","end_time":100}},"usage":{"duration":2}}}`,
				`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"text":"世界","end_time":null,"sentence_end":true}},"usage":{"duration":7}}}`,
				`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"text":"heartbeat","sentence_end":true,"heartbeat":true}},"usage":{"duration":7}}}`,
				`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"text":"","sentence_end":true}},"usage":{"duration":6}}}`,
				`{"header":{"event":"task-finished"},"payload":{"output":{},"usage":null}}`,
			} {
				require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(event)))
			}

		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	mt := meta.NewMeta(
		nil,
		mode.AudioTranscription,
		"alias",
		model.ModelConfig{},
		meta.WithRequestUsage(model.Usage{AudioInputTokens: 1}),
	)
	for _, protocol := range []string{sttChatAudio, sttTranscription, sttRealtime, sttChatAudio} {
		t.Run(protocol, func(t *testing.T) {
			mt.SetChannel(
				&model.Channel{
					BaseURL:      upstream.URL,
					Key:          "channel-key",
					Configs:      model.ChannelConfigs{"stt_upstream_protocol": protocol},
					ModelMapping: map[string]string{"alias": "deployment"},
				},
			)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = sttUpload(t, sttWAV(), nil)
			_, _, err := controller.DoHelper(&Adaptor{}, c, mt, nil)
			require.Nil(t, err)
			assert.Contains(t, recorder.Body.String(), "你好")

			if protocol == sttRealtime {
				var body struct {
					Text  string `json:"text"`
					Usage struct {
						Seconds int64 `json:"seconds"`
					} `json:"usage"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
				assert.Equal(t, "你好世界", body.Text)
				assert.EqualValues(t, 7, body.Usage.Seconds)
			}
		})
	}
}

func TestSTTWebSocketCancellation(t *testing.T) {
	setSTTFFmpeg(t, false)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer upstream.Close()

	mt := meta.NewMeta(
		&model.Channel{BaseURL: upstream.URL},
		mode.AudioTranscription,
		"paraformer-realtime-v2",
		model.ModelConfig{},
	)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := sttUpload(t, sttWAV(), nil)

	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()

	c.Request = req.WithContext(ctx)
	start := time.Now()
	_, _, err := controller.DoHelper(&Adaptor{}, c, mt, nil)
	require.NotNil(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestSTTWebSocketTLSDoesNotShareHTTPALPN(t *testing.T) {
	upstream := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/warmup" {
				w.WriteHeader(http.StatusOK)
				return
			}

			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			_, _, _ = conn.ReadMessage()
		}),
	)
	upstream.EnableHTTP2 = true

	upstream.StartTLS()
	defer upstream.Close()

	mt := meta.NewMeta(
		&model.Channel{SkipTLSVerify: true},
		mode.AudioTranscription,
		"paraformer-realtime-v2",
		model.ModelConfig{},
	)
	mt.RequestTimeout = time.Second
	client, err := utils.LoadHTTPClientWithTLSConfigE(mt.RequestTimeout, "", true)
	require.NoError(t, err)
	warmup, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		upstream.URL+"/warmup",
		nil,
	)
	require.NoError(t, err)
	resp, err := client.Do(warmup)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	before := append([]string(nil), transport.TLSClientConfig.NextProtos...)
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		upstream.URL+"/api-ws/v1/inference",
		strings.NewReader(`{"header":{"action":"run-task"}}`),
	)
	require.NoError(t, err)
	resp, err = STTDoRequest(mt, req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	conn, ok := mt.MustGet("ws_conn").(*websocket.Conn)
	require.True(t, ok)
	require.NoError(t, conn.Close())
	assert.Equal(t, before, transport.TLSClientConfig.NextProtos)
}

func TestSTTFullOpenAIBaseURL(t *testing.T) {
	for _, tt := range []struct{ protocol, path string }{
		{sttChatAudio, "/v1/chat/completions"},
		{sttTranscription, "/v1/audio/transcriptions"},
	} {
		mt := meta.NewMeta(
			&model.Channel{
				BaseURL: "https://upstream.example/v1/",
				Configs: model.ChannelConfigs{"stt_upstream_protocol": tt.protocol},
			},
			mode.AudioTranscription,
			"deployment",
			model.ModelConfig{},
		)
		target, err := sttRequestURL(mt)
		require.NoError(t, err)
		assert.Equal(t, "https://upstream.example"+tt.path, target.URL)
	}
}

func TestSTTPlainTextResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = sttUpload(t, sttWAV(), map[string]string{"response_format": "text"})
	mt := meta.NewMeta(nil, mode.AudioTranscription, "qwen3-asr-flash", model.ModelConfig{})
	_, err := sttResponse(
		mt,
		c,
		&http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(
				strings.NewReader(
					`{"choices":[{"message":{"content":"你好"}}],"usage":{"seconds":1}}`,
				),
			),
		},
	)
	require.Nil(t, err)
	assert.Equal(t, "你好", recorder.Body.String())
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/plain")
}
