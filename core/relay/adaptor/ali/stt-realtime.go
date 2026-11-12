package ali

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type STTMessage struct {
	Header  STTHeader  `json:"header"`
	Payload STTPayload `json:"payload"`
}

type STTHeader struct {
	Attributes   map[string]any `json:"attributes"`
	Action       string         `json:"action,omitempty"`
	TaskID       string         `json:"task_id"`
	Streaming    string         `json:"streaming,omitempty"`
	Event        string         `json:"event,omitempty"`
	ErrorCode    string         `json:"error_code,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
}

type STTPayload struct {
	Model      string        `json:"model,omitempty"`
	TaskGroup  string        `json:"task_group,omitempty"`
	Task       string        `json:"task,omitempty"`
	Function   string        `json:"function,omitempty"`
	Input      STTInput      `json:"input,omitempty"`
	Output     STTOutput     `json:"output,omitempty"`
	Parameters STTParameters `json:"parameters,omitempty"`
	Usage      STTUsage      `json:"usage,omitempty"`
}

type STTInput struct {
	AudioData []byte `json:"audio_data"`
}

type STTParameters struct {
	Format     string `json:"format,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

type STTOutput struct {
	STTSentence STTSentence `json:"sentence"`
}

type STTSentence struct {
	Text        string `json:"text"`
	EndTime     *int   `json:"end_time"`
	SentenceEnd *bool  `json:"sentence_end,omitempty"`
	Heartbeat   bool   `json:"heartbeat,omitempty"`
}

type STTUsage struct {
	Duration   float64 `json:"duration"`
	Characters int64   `json:"characters"`
}

func ConvertSTTRequest(
	meta *meta.Meta,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	audioData, format, sampleRate, err := prepareSTTAudio(meta, request, true)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	sttRequest := STTMessage{
		Header: STTHeader{
			Action:    "run-task",
			Streaming: "duplex",
			TaskID:    uuid.NewString(),
		},
		Payload: STTPayload{
			Model:     meta.ActualModel,
			Task:      "asr",
			TaskGroup: "audio",
			Function:  "recognition",
			Input:     STTInput{},
			Parameters: STTParameters{
				Format:     format,
				SampleRate: sampleRate,
			},
		},
	}

	data, err := sonic.Marshal(sttRequest)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	meta.Set("audio_data", audioData)
	meta.Set("task_id", sttRequest.Header.TaskID)

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
		Body: bytes.NewReader(data),
	}, nil
}

// STTDoRequest opens the selected channel's WebSocket using its proxy/TLS
// settings and a bounded deadline. Failed setup never leaves a connection open.
func STTDoRequest(meta *meta.Meta, req *http.Request) (*http.Response, error) {
	timeout := meta.RequestTimeout
	if timeout <= 0 {
		timeout = time.Minute
	}

	// Start the request budget before dialing so slow TCP/TLS/upgrade setup
	// cannot exceed the model timeout or extend the recognition deadline.
	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	defer cancel()

	deadline, _ := ctx.Deadline()

	conn, resp, err := dialSTTWebSocket(ctx, meta, req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			// Preserve upstream auth/rate-limit errors for channel monitoring
			// and retry selection; the handler also closes the rejected body.
			return nil, ErrorHanlder(resp)
		}

		return nil, err
	}

	_ = conn.SetReadDeadline(deadline)
	_ = conn.SetWriteDeadline(deadline)
	conn.SetReadLimit(4 * 1024 * 1024)

	data, err := io.ReadAll(req.Body)
	if err == nil {
		err = conn.WriteMessage(websocket.TextMessage, data)
	}

	if err != nil {
		conn.Close()
		return nil, err
	}

	meta.Set("ws_conn", conn)

	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func STTDoResponse(
	meta *meta.Meta,
	c *gin.Context,
	_ *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	audioData, ok := meta.MustGet("audio_data").([]byte)
	if !ok {
		panic(fmt.Sprintf("audio data type error: %T, %v", audioData, audioData))
	}

	taskID, ok := meta.MustGet("task_id").(string)
	if !ok {
		panic(fmt.Sprintf("task id type error: %T, %v", taskID, taskID))
	}

	conn, ok := meta.MustGet("ws_conn").(*websocket.Conn)
	if !ok {
		panic(fmt.Sprintf("ws conn type error: %T, %v", conn, conn))
	}
	defer conn.Close()

	stop := context.AfterFunc(c.Request.Context(), func() { _ = conn.Close() })
	defer stop()

	output := strings.Builder{}
	started := false
	billedDuration := float64(0)

	usage := model.Usage{
		InputTokens:      meta.RequestUsage.InputTokens,
		AudioInputTokens: meta.RequestUsage.AudioInputTokens,
		TotalTokens:      meta.RequestUsage.TotalTokens,
	}

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIErrorWithMessage(
				"ali_wss_read_msg_failed",
				nil,
				http.StatusInternalServerError,
			)
		}

		if messageType != websocket.TextMessage {
			return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIErrorWithMessage(
				"expect text message, but got binary message",
				nil,
				http.StatusInternalServerError,
			)
		}

		var msg STTMessage

		err = sonic.Unmarshal(data, &msg)
		if err != nil {
			return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIErrorWithMessage(
				"ali_wss_read_msg_failed",
				nil,
				http.StatusInternalServerError,
			)
		}
		// DashScope reports cumulative task billing on result-generated, and
		// task-finished usually has usage=null. Retain the latest maximum;
		// summing sentence events would charge the same audio multiple times.
		if msg.Payload.Usage.Duration > billedDuration {
			billedDuration = msg.Payload.Usage.Duration
		}

		switch msg.Header.Event {
		case "task-started":
			if started {
				continue
			}

			started = true
			// Read results while writing audio: upstream buffers may otherwise
			// fill and deadlock both peers on longer recordings.
			go func() {
				if err := sendSTTAudio(conn, taskID, audioData); err != nil {
					_ = conn.Close()
				}
			}()
		case "result-generated":
			sentence := msg.Payload.Output.STTSentence

			final := sentence.SentenceEnd != nil && *sentence.SentenceEnd
			if sentence.SentenceEnd == nil {
				final = sentence.EndTime != nil
			}

			if final && !sentence.Heartbeat && sentence.Text != "" {
				output.WriteString(sentence.Text)
			}

			continue
		case "task-finished":
			seconds := int64(meta.RequestUsage.AudioInputTokens)
			if value, ok := meta.Get("stt_audio_seconds"); ok {
				if duration, ok := value.(float64); ok {
					seconds = int64(math.Ceil(duration))
				}
			}

			if billedDuration > 0 {
				seconds = int64(math.Ceil(billedDuration))
			}

			sttUsage := relaymodel.SttUsage{
				Type:        relaymodel.SttUsageTypeDuration,
				Seconds:     seconds,
				TotalTokens: seconds,
			}
			writeSTTResponse(c, output.String(), sttUsage)

			return adaptor.DoResponseResult{Usage: sttUsage.ToModelUsage()}, nil
		case "task-failed":
			return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIErrorWithMessage(
				msg.Header.ErrorMessage,
				msg.Header.ErrorCode,
				http.StatusInternalServerError,
			)
		}
	}
}

// sendSTTAudio has exclusive ownership of WebSocket writes after run-task.
// Closing the connection on return from the reader also unblocks this writer.
func sendSTTAudio(conn *websocket.Conn, taskID string, audio []byte) error {
	const chunkSize = 3 * 1024
	for offset := 0; offset < len(audio); offset += chunkSize {
		if err := conn.WriteMessage(
			websocket.BinaryMessage,
			audio[offset:min(offset+chunkSize, len(audio))],
		); err != nil {
			return err
		}
	}

	return conn.WriteJSON(STTMessage{
		Header:  STTHeader{Action: "finish-task", TaskID: taskID, Streaming: "duplex"},
		Payload: STTPayload{Input: STTInput{}},
	})
}
