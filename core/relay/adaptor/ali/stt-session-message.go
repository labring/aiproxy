package ali

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labring/aiproxy/core/model"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

const realtimeMessageModel = "qwen-audio-3.1-asr-flash-message"

// realtimeMessageSession adapts DashScope tasks to the gateway's live transcription events.
// Client and server conversion each have a single owner; only readiness crosses goroutines.
type realtimeMessageSession struct {
	taskID     string
	configured bool
	ready      atomic.Bool
	usage      map[string]json.RawMessage
}

func newRealtimeMessageSession() *realtimeMessageSession {
	return &realtimeMessageSession{taskID: uuid.NewString()}
}

// clientEvent preserves one task per connection. Manual commit/clear are rejected because
// this model only supports server-side sentence detection, not Qwen's audio-buffer controls.
func (s *realtimeMessageSession) clientEvent(data []byte) (int, []byte, error) {
	var event struct {
		Type    string `json:"type"`
		Audio   string `json:"audio"`
		Session struct {
			DisfluencyRemoval bool `json:"disfluency_removal_enabled"`
			Transcription     struct {
				Language string `json:"language"`
			} `json:"input_audio_transcription"`
			TurnDetection *struct {
				Type    string `json:"type"`
				Silence *int   `json:"silence_duration_ms"`
			} `json:"turn_detection"`
		} `json:"session"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return 0, nil, err
	}

	command := map[string]any{
		"header":  STTHeader{TaskID: s.taskID, Streaming: "duplex"},
		"payload": map[string]any{"input": map[string]any{}},
	}

	switch event.Type {
	case "session.update":
		if s.configured {
			return 0, nil, errors.New("message STT session can only be configured once")
		}

		if event.Session.TurnDetection == nil || event.Session.TurnDetection.Type != "server_vad" {
			return 0, nil, errors.New("message STT requires server_vad")
		}

		parameters := map[string]any{
			"format":                      "pcm",
			"sample_rate":                 realtimeSampleRate,
			"intermediate_result_enabled": true,
			"disfluency_removal_enabled":  event.Session.DisfluencyRemoval,
		}
		if language := event.Session.Transcription.Language; language != "" {
			parameters["language_hints"] = []string{language}
		}

		if silence := event.Session.TurnDetection.Silence; silence != nil {
			if *silence < 200 || *silence > 6000 {
				return 0, nil, errors.New("silence_duration_ms must be between 200 and 6000")
			}

			parameters["max_sentence_silence"] = *silence
		}

		command["header"] = STTHeader{Action: "run-task", TaskID: s.taskID, Streaming: "duplex"}
		command["payload"] = map[string]any{
			"model": realtimeMessageModel, "task_group": "audio", "task": "asr",
			"function": "recognition", "input": map[string]any{}, "parameters": parameters,
		}
		s.configured = true
	case "input_audio_buffer.append":
		if !s.ready.Load() {
			return 0, nil, errors.New("message STT task has not started")
		}

		pcm, err := base64.StdEncoding.DecodeString(event.Audio)
		if err != nil {
			return 0, nil, err
		}

		return websocket.BinaryMessage, pcm, nil
	case "session.finish":
		if !s.ready.Load() {
			return 0, nil, errors.New("message STT task has not started")
		}

		command["header"] = STTHeader{Action: "finish-task", TaskID: s.taskID, Streaming: "duplex"}
	default:
		return 0, nil, errors.New("unsupported message STT client event")
	}

	encoded, err := json.Marshal(command)

	return websocket.TextMessage, encoded, err
}

// serverEvent replaces each sentence by ID, keeping provisional text underlined until
// the model's final, optionally disfluency-free text arrives. Usage is cumulative, never summed.
func (s *realtimeMessageSession) serverEvent(data []byte) ([]byte, error) {
	var event struct {
		Header  STTHeader `json:"header"`
		Payload struct {
			Output struct {
				Sentence struct {
					ID        *int64 `json:"sentence_id"`
					Text      string `json:"text"`
					Final     bool   `json:"sentence_end"`
					Heartbeat bool   `json:"heartbeat"`
				} `json:"sentence"`
			} `json:"output"`
			Usage map[string]json.RawMessage `json:"usage"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}

	if event.Header.TaskID != s.taskID {
		return nil, errors.New("unexpected message STT task ID")
	}

	if s.usage == nil {
		s.usage = make(map[string]json.RawMessage)
	}

	maps.Copy(s.usage, event.Payload.Usage)

	result := map[string]any{}
	switch event.Header.Event {
	case "task-started":
		s.ready.Store(true)

		result["type"] = "session.updated"
		result["session"] = map[string]string{"id": s.taskID, "model": realtimeMessageModel}
	case "result-generated":
		sentence := event.Payload.Output.Sentence
		if sentence.Heartbeat {
			return nil, nil
		}

		if sentence.ID == nil {
			return nil, errors.New("message STT result is missing sentence_id")
		}

		result["item_id"] = fmt.Sprintf("%s-%d", s.taskID, *sentence.ID)
		if sentence.Final {
			result["type"] = "conversation.item.input_audio_transcription.completed"
			result["transcript"] = sentence.Text
		} else {
			result["type"] = "conversation.item.input_audio_transcription.text"
			result["text"] = ""
			result["stash"] = sentence.Text
		}
	case "task-finished":
		result["type"] = "session.finished"
		if len(s.usage) > 0 {
			// Keep upstream tokens/vendor fields and expose duration under the common seconds key.
			var duration, existingSeconds float64

			rawSeconds := s.usage["seconds"]

			hasSeconds := string(rawSeconds) != "null" &&
				json.Unmarshal(rawSeconds, &existingSeconds) == nil && existingSeconds >= 0
			if raw := s.usage["duration"]; string(raw) != "null" &&
				json.Unmarshal(
					raw,
					&duration,
				) == nil && duration >= 0 && duration < math.MaxInt64 &&
				!hasSeconds {
				seconds, err := json.Marshal(int64(math.Ceil(duration)))
				if err != nil {
					return nil, err
				}

				s.usage["seconds"] = seconds
			}

			if _, err := s.normalizeUsage(); err != nil {
				return nil, err
			}

			result["usage"] = s.usage
		}
	case "task-failed":
		result["type"] = "error"
		result["error"] = map[string]string{
			"code": event.Header.ErrorCode, "message": event.Header.ErrorMessage,
		}
	default:
		return nil, fmt.Errorf("unsupported message STT upstream event: %s", event.Header.Event)
	}

	return json.Marshal(result)
}

// modelUsage retains native token accounting for this token-priced model, including
// the last cumulative usage on cancellation. Missing tokens fall back to forwarded PCM duration.
func (s *realtimeMessageSession) modelUsage(seconds int64) model.Usage {
	if usage, err := s.normalizeUsage(); err == nil && usage.Type == relaymodel.SttUsageTypeTokens {
		return usage.ToModelUsage()
	}

	usage := relaymodel.SttUsage{Type: relaymodel.SttUsageTypeDuration, Seconds: seconds}

	return usage.ToModelUsage()
}

// normalizeUsage retains cumulative upstream counters and accepts independently
// reported input/output tokens even when no total is present. Cancellation uses
// the same normalization as session.finished; no component is inferred.
func (s *realtimeMessageSession) normalizeUsage() (relaymodel.SttUsage, error) {
	data, err := json.Marshal(s.usage)
	if err != nil {
		return relaymodel.SttUsage{}, err
	}

	data, err = relaymodel.NormalizeSTTTokenUsage(data)
	if err != nil {
		return relaymodel.SttUsage{}, err
	}

	var usage relaymodel.SttUsage
	// Seconds is a separate wire measurement and may be fractional. Token
	// accounting does not need to decode it into the legacy integer meter.
	counters := struct {
		*relaymodel.SttUsage
		Seconds json.RawMessage `json:"seconds"`
	}{SttUsage: &usage}
	if err = json.Unmarshal(data, &counters); err != nil {
		return usage, err
	}

	if err = json.Unmarshal(data, &s.usage); err != nil {
		return usage, err
	}

	if usage.Type != relaymodel.SttUsageTypeDuration {
		for _, key := range []string{"input_tokens", "output_tokens", "total_tokens"} {
			if raw := s.usage[key]; len(raw) > 0 && string(raw) != "null" {
				usage.Type = relaymodel.SttUsageTypeTokens
				s.usage["type"] = json.RawMessage(`"tokens"`)
				break
			}
		}
	}

	return usage, nil
}
