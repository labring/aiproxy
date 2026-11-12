package ali

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

const (
	realtimeSampleRate    = 16000
	realtimeMaxAudioBytes = realtimeSampleRate * 2 * 300
)

// realtimeSTTOriginAllowed permits same-origin browsers and authenticated non-browser clients.
func realtimeSTTOriginAllowed(req *http.Request) bool {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)

	return err == nil && u.User == nil && (u.Scheme == "http" || u.Scheme == "https") &&
		strings.EqualFold(u.Host, req.Host)
}

// realtimeSTTURL selects the channel's native Qwen ASR session endpoint, preserving model mapping.
func realtimeSTTURL(mt *meta.Meta) (*url.URL, error) {
	if !strings.HasPrefix(mt.ActualModel, "qwen3-asr-flash-realtime") {
		return nil, errors.New(
			"realtime STT requires qwen3-asr-flash-realtime or its dated version",
		)
	}

	base := mt.Channel.BaseURL
	if base == "" {
		base = "https://dashscope.aliyuncs.com"
	}

	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil {
		return nil, errors.New("invalid realtime STT channel base URL")
	}

	switch u.Scheme {
	case "https", "wss":
		u.Scheme = "wss"
	case "http", "ws":
		u.Scheme = "ws"
	default:
		return nil, errors.New("unsupported realtime STT URL scheme")
	}

	if !strings.HasSuffix(u.Path, "/api-ws/v1/realtime") {
		u.Path = "/api-ws/v1/realtime"
	}

	q := u.Query()
	q.Set("model", mt.ActualModel)
	u.RawQuery = q.Encode()
	u.Fragment = ""

	return u, nil
}

// RealtimeSTTHandler relays one bounded PCM session. Audio is sent once, partial/final recognition
// events are passed through, and accounting uses forwarded PCM frames rather than client duration.
func RealtimeSTTHandler(mt *meta.Meta, c *gin.Context) (adaptor.DoResponseResult, adaptor.Error) {
	fail := func(err error, status int) (adaptor.DoResponseResult, adaptor.Error) {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"realtime_stt_error",
			status,
		)
	}

	endpoint, err := realtimeSTTURL(mt)
	if err != nil {
		return fail(err, http.StatusBadRequest)
	}

	if !realtimeSTTOriginAllowed(c.Request) {
		return fail(errors.New("realtime STT requires a same-origin client"), http.StatusForbidden)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fail(err, http.StatusBadRequest)
	}

	req.Header.Set("Authorization", "Bearer "+mt.Channel.Key)

	upstream, resp, err := dialSTTWebSocket(ctx, mt, req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			return adaptor.DoResponseResult{}, ErrorHanlder(resp)
		}
		return fail(err, http.StatusBadGateway)
	}
	defer upstream.Close()

	upstream.SetReadLimit(1024 * 1024)
	_ = upstream.SetReadDeadline(time.Now().Add(30 * time.Second))

	upgrader := websocket.Upgrader{
		Subprotocols: []string{"aiproxy-realtime"}, HandshakeTimeout: 10 * time.Second,
		CheckOrigin: realtimeSTTOriginAllowed,
	}

	downstream, err := upgrader.Upgrade(
		c.Writer,
		c.Request,
		nil,
	)
	if err != nil {
		return fail(err, http.StatusBadRequest)
	}
	defer downstream.Close()

	downstream.SetReadLimit(64 * 1024)

	stop := context.AfterFunc(ctx, func() { _ = upstream.Close(); _ = downstream.Close() })
	defer stop()

	var audioBytes atomic.Int64

	finished := make(chan error, 1)

	started := time.Now()
	go func() {
		err := forwardRealtimeAudio(downstream, upstream, &audioBytes, started)
		if err != nil {
			_ = upstream.Close()
		}

		finished <- err
	}()

	var (
		upstreamID string
		sessionErr error
	)
	for {
		_ = upstream.SetReadDeadline(time.Now().Add(30 * time.Second))

		messageType, data, readErr := upstream.ReadMessage()
		if readErr != nil {
			sessionErr = readErr
			break
		}

		if messageType != websocket.TextMessage {
			continue
		}

		var event struct {
			Type    string `json:"type"`
			Session struct {
				ID string `json:"id"`
			} `json:"session"`
		}
		if json.Unmarshal(data, &event) != nil {
			sessionErr = errors.New("invalid realtime STT upstream event")
			break
		}

		if event.Session.ID != "" {
			upstreamID = event.Session.ID
		}

		if event.Type == "session.finished" {
			seconds := int64(math.Ceil(float64(audioBytes.Load()) / float64(realtimeSampleRate*2)))

			data, err = realtimeFinishedData(data, seconds)
			if err != nil {
				sessionErr = err
				break
			}
		}

		_ = downstream.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := downstream.WriteMessage(websocket.TextMessage, data); err != nil {
			sessionErr = err
			break
		}

		if event.Type == "session.finished" {
			break
		}

		if event.Type == "error" ||
			event.Type == "conversation.item.input_audio_transcription.failed" {
			sessionErr = errors.New("upstream realtime recognition failed")
			break
		}
	}

	_ = downstream.Close()
	clientErr := <-finished
	seconds := int64(math.Ceil(float64(audioBytes.Load()) / float64(realtimeSampleRate*2)))
	usage := relaymodel.SttUsage{Type: relaymodel.SttUsageTypeDuration, Seconds: seconds}
	result := adaptor.DoResponseResult{Usage: usage.ToModelUsage(), UpstreamID: upstreamID}
	// A normal client cancellation still consumes the audio already accepted by upstream.
	if sessionErr != nil && clientErr == nil {
		return result, relaymodel.WrapperOpenAIError(
			sessionErr,
			"realtime_stt_error",
			http.StatusBadGateway,
		)
	}

	return result, nil
}

// realtimeFinishedData adds gateway duration without replacing upstream token/vendor fields.
func realtimeFinishedData(data []byte, seconds int64) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}

	var usage map[string]json.RawMessage
	if err := json.Unmarshal(body["usage"], &usage); err != nil || usage == nil {
		encoded, err := json.Marshal(
			relaymodel.SttUsage{
				Type:        relaymodel.SttUsageTypeDuration,
				Seconds:     seconds,
				TotalTokens: seconds,
			},
		)
		if err != nil {
			return nil, err
		}

		body["usage"] = encoded
	} else {
		var existing float64
		if raw := usage["seconds"]; len(raw) == 0 || string(raw) == "null" ||
			json.Unmarshal(raw, &existing) != nil ||
			existing < 0 {
			encoded, err := json.Marshal(seconds)
			if err != nil {
				return nil, err
			}

			usage["seconds"] = encoded
		}

		encoded, err := json.Marshal(usage)
		if err != nil {
			return nil, err
		}

		body["usage"] = encoded
	}

	return json.Marshal(body)
}

// forwardRealtimeAudio validates the fixed PCM contract and keeps a single upstream writer.
// Returning after session.finish drains the final upstream events instead of closing recognition early.
func forwardRealtimeAudio(
	client, upstream *websocket.Conn,
	total *atomic.Int64,
	started time.Time,
) error {
	for {
		_ = client.SetReadDeadline(time.Now().Add(30 * time.Second))

		typ, data, err := client.ReadMessage()
		if err != nil {
			return err
		}

		if typ != websocket.TextMessage {
			return errors.New("realtime STT expects JSON text frames")
		}

		var event struct {
			Type    string `json:"type"`
			Audio   string `json:"audio"`
			Session struct {
				Model  string `json:"model"`
				Format string `json:"input_audio_format"`
				Rate   int    `json:"sample_rate"`
			} `json:"session"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return fmt.Errorf("invalid realtime STT client event: %w", err)
		}

		var bytes int64
		switch event.Type {
		case "session.update":
			if event.Session.Model != "" ||
				(event.Session.Format != "" && event.Session.Format != "pcm") ||
				(event.Session.Rate != 0 && event.Session.Rate != realtimeSampleRate) {
				return errors.New("session requires fixed model and mono PCM16 at 16000 Hz")
			}
		case "input_audio_buffer.append":
			pcm, err := base64.StdEncoding.DecodeString(event.Audio)
			if err != nil || len(pcm) == 0 || len(pcm)%2 != 0 {
				return errors.New("invalid PCM16 audio")
			}

			bytes = int64(len(pcm))
			if total.Load()+bytes > realtimeMaxAudioBytes ||
				float64(
					total.Load()+bytes,
				) > float64(
					realtimeSampleRate*2,
				)*(time.Since(started).Seconds()*4+2) {
				return errors.New("realtime audio limit exceeded")
			}
		case "input_audio_buffer.commit", "input_audio_buffer.clear", "session.finish":
		default:
			return errors.New("unsupported realtime STT client event")
		}

		_ = upstream.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := upstream.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}

		total.Add(bytes)

		if event.Type == "session.finish" {
			_ = upstream.SetReadDeadline(time.Now().Add(10 * time.Second))
			return nil
		}
	}
}
