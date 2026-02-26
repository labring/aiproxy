package ali

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	Text    string `json:"text"`
	EndTime *int   `json:"end_time"`
}

type STTUsage struct {
	Characters int64 `json:"characters"`
}

func ConvertSTTRequest(
	meta *meta.Meta,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	err := request.ParseMultipartForm(1024 * 1024 * 4)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	audioFile, _, err := request.FormFile("file")
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	audioData, err := io.ReadAll(audioFile)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	format := "mp3"
	if request.FormValue("format") != "" {
		format = request.FormValue("format")
	}

	sampleRate := 24000
	if request.FormValue("sample_rate") != "" {
		sampleRate, err = strconv.Atoi(request.FormValue("sample_rate"))
		if err != nil {
			return adaptor.ConvertResult{}, err
		}
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
			"X-DashScope-DataInspection": {"enable"},
		},
		Body: bytes.NewReader(data),
	}, nil
}

func STTDoRequest(meta *meta.Meta, req *http.Request) (*http.Response, error) {
	wsURL := req.URL
	wsURL.Scheme = "wss"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), req.Header)
	if err != nil {
		return nil, err
	}

	meta.Set("ws_conn", conn)

	jsonWriter, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return nil, err
	}
	defer jsonWriter.Close()

	_, err = io.Copy(jsonWriter, req.Body)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(nil),
	}, nil
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

	output := strings.Builder{}

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

		switch msg.Header.Event {
		case "task-started":
			chunkSize := 3 * 1024
			for i := 0; i < len(audioData); i += chunkSize {
				end := min(i+chunkSize, len(audioData))

				chunk := audioData[i:end]

				err = conn.WriteMessage(websocket.BinaryMessage, chunk)
				if err != nil {
					return adaptor.DoResponseResult{
							Usage: usage,
						}, relaymodel.WrapperOpenAIErrorWithMessage(
							"ali_wss_write_msg_failed",
							nil,
							http.StatusInternalServerError,
						)
				}
			}

			finishMsg := STTMessage{
				Header: STTHeader{
					Action:    "finish-task",
					TaskID:    taskID,
					Streaming: "duplex",
				},
				Payload: STTPayload{
					Input: STTInput{},
				},
			}

			finishData, err := sonic.Marshal(finishMsg)
			if err != nil {
				return adaptor.DoResponseResult{
						Usage: usage,
					}, relaymodel.WrapperOpenAIErrorWithMessage(
						"ali_wss_write_msg_failed",
						nil,
						http.StatusInternalServerError,
					)
			}

			err = conn.WriteMessage(websocket.TextMessage, finishData)
			if err != nil {
				return adaptor.DoResponseResult{
						Usage: usage,
					}, relaymodel.WrapperOpenAIErrorWithMessage(
						"ali_wss_write_msg_failed",
						nil,
						http.StatusInternalServerError,
					)
			}
		case "result-generated":
			if msg.Payload.Output.STTSentence.EndTime != nil &&
				msg.Payload.Output.STTSentence.Text != "" {
				output.WriteString(msg.Payload.Output.STTSentence.Text)
			}

			continue
		case "task-finished":
			sttUsage := relaymodel.SttUsage{
				Type:         relaymodel.SttUsageTypeTokens,
				Seconds:      int64(meta.RequestUsage.AudioInputTokens),
				InputTokens:  int64(meta.RequestUsage.AudioInputTokens),
				OutputTokens: msg.Payload.Usage.Characters,
				InputTokenDetails: &relaymodel.SttUsageInputTokenDetails{
					AudioTokens: int64(meta.RequestUsage.AudioInputTokens),
				},
				TotalTokens: int64(
					meta.RequestUsage.AudioInputTokens,
				) + msg.Payload.Usage.Characters,
			}
			c.JSON(http.StatusOK, gin.H{
				"text":  output.String(),
				"usage": sttUsage,
			})

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
