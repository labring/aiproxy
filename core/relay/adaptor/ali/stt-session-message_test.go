//nolint:testpackage
package ali

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealtimeMessageClientEvents(t *testing.T) {
	s := newRealtimeMessageSession()
	_, _, err := s.clientEvent([]byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`))
	require.Error(t, err)

	typ, data, err := s.clientEvent(
		[]byte(
			`{"type":"session.update","session":{"input_audio_transcription":{"language":"zh"},"disfluency_removal_enabled":true,"turn_detection":{"type":"server_vad","silence_duration_ms":600}}}`,
		),
	)
	require.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, typ)
	assert.JSONEq(
		t,
		`{"header":{"action":"run-task","task_id":"`+s.taskID+`","streaming":"duplex","attributes":null},"payload":{"model":"qwen-audio-3.1-asr-flash-message","task_group":"audio","task":"asr","function":"recognition","input":{},"parameters":{"format":"pcm","sample_rate":16000,"language_hints":["zh"],"intermediate_result_enabled":true,"disfluency_removal_enabled":true,"max_sentence_silence":600}}}`,
		string(data),
	)

	_, _, err = s.clientEvent([]byte(`{"type":"session.update"}`))
	require.Error(t, err)
	s.ready.Store(true)

	pcm := []byte{1, 2, 3, 4}
	typ, data, err = s.clientEvent(
		[]byte(
			`{"type":"input_audio_buffer.append","audio":"` + base64.StdEncoding.EncodeToString(
				pcm,
			) + `"}`,
		),
	)
	require.NoError(t, err)
	assert.Equal(t, websocket.BinaryMessage, typ)
	assert.Equal(t, pcm, data)

	_, data, err = s.clientEvent([]byte(`{"type":"session.finish"}`))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"action":"finish-task"`)
	assert.Contains(t, string(data), s.taskID)

	for _, event := range []string{"input_audio_buffer.clear", "input_audio_buffer.commit"} {
		_, _, err = s.clientEvent([]byte(`{"type":"` + event + `"}`))
		require.Error(t, err)
	}
}

func TestRealtimeMessageRejectManualAndInvalidVAD(t *testing.T) {
	for _, session := range []string{
		`{}`, `{"turn_detection":null}`, `{"turn_detection":{"type":"other"}}`,
		`{"turn_detection":{"type":"server_vad","silence_duration_ms":1}}`,
		`{"turn_detection":{"type":"server_vad","silence_duration_ms":6001}}`,
	} {
		s := newRealtimeMessageSession()
		_, _, err := s.clientEvent([]byte(`{"type":"session.update","session":` + session + `}`))
		require.Error(t, err)
	}
}

func TestRealtimeMessageServerEvents(t *testing.T) {
	s := newRealtimeMessageSession()
	event := func(kind, payload string) []byte {
		t.Helper()

		data, err := s.serverEvent(
			[]byte(
				`{"header":{"task_id":"` + s.taskID + `","event":"` + kind + `"},"payload":` + payload + `}`,
			),
		)
		require.NoError(t, err)

		return data
	}

	assert.False(t, s.ready.Load())
	assert.Contains(t, string(event("task-started", `{}`)), `"type":"session.updated"`)
	assert.True(t, s.ready.Load())

	for _, text := range []string{"嗯我想订明天", "嗯我想订明天不对后天的机票"} {
		encoded, err := json.Marshal(text)
		require.NoError(t, err)

		data := event(
			"result-generated",
			`{"output":{"sentence":{"sentence_id":1,"text":`+string(
				encoded,
			)+`,"sentence_end":false}}}`,
		)
		assert.JSONEq(
			t,
			`{"type":"conversation.item.input_audio_transcription.text","item_id":"`+s.taskID+`-1","text":"","stash":`+string(
				encoded,
			)+`}`,
			string(data),
		)
	}

	data := event(
		"result-generated",
		`{"output":{"sentence":{"sentence_id":1,"text":"我想订后天的机票。","sentence_end":true}},"usage":{"duration":2,"input_tokens":84,"output_tokens":4,"total_tokens":88,"vendor":{"kept":true}}}`,
	)
	assert.JSONEq(
		t,
		`{"type":"conversation.item.input_audio_transcription.completed","item_id":"`+s.taskID+`-1","transcript":"我想订后天的机票。"}`,
		string(data),
	)
	assert.Nil(t, event("result-generated", `{"output":{"sentence":{"heartbeat":true}}}`))
	data = event("task-finished", `{"usage":{"duration":2,"total_tokens":88}}`)
	assert.JSONEq(
		t,
		`{"type":"session.finished","usage":{"duration":2,"seconds":2,"type":"tokens","input_tokens":84,"output_tokens":4,"total_tokens":88,"vendor":{"kept":true}}}`,
		string(data),
	)

	usage := s.modelUsage(10)
	assert.Equal(t, model.ZeroNullInt64(84), usage.InputTokens)
	assert.Equal(t, model.ZeroNullInt64(4), usage.OutputTokens)
	assert.Equal(t, model.ZeroNullInt64(88), usage.TotalTokens)
}

func TestRealtimeMessageMissingUsageFallsBackToPCM(t *testing.T) {
	s := newRealtimeMessageSession()
	data, err := s.serverEvent(
		[]byte(`{"header":{"task_id":"` + s.taskID + `","event":"task-finished"},"payload":{}}`),
	)
	require.NoError(t, err)
	data, err = realtimeFinishedData(data, 3)
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"type":"session.finished","usage":{"type":"duration","seconds":3,"total_tokens":3}}`,
		string(data),
	)
	assert.Equal(t, model.ZeroNullInt64(3), s.modelUsage(3).TotalTokens)
}

func TestRealtimeMessageDurationFallback(t *testing.T) {
	for _, tt := range []struct {
		usage string
		want  int
	}{
		{usage: `{"duration":null,"total_tokens":7}`, want: 3},
		{usage: `{"duration":"invalid","total_tokens":7}`, want: 3},
		{usage: `{"duration":1.1,"total_tokens":7}`, want: 2},
		{usage: `{"duration":1.1,"seconds":null,"total_tokens":7}`, want: 2},
		{usage: `{"duration":1.1,"seconds":0,"total_tokens":7}`, want: 0},
		{usage: `{"duration":1.1,"seconds":5,"total_tokens":7}`, want: 5},
	} {
		s := newRealtimeMessageSession()
		data, err := s.serverEvent(
			[]byte(
				`{"header":{"task_id":"` + s.taskID + `","event":"task-finished"},"payload":{"usage":` + tt.usage + `}}`,
			),
		)
		require.NoError(t, err)
		data, err = realtimeFinishedData(data, 3)
		require.NoError(t, err)

		var result struct {
			Usage struct {
				Seconds int `json:"seconds"`
				Total   int `json:"total_tokens"`
			} `json:"usage"`
		}
		require.NoError(t, json.Unmarshal(data, &result))
		assert.Equal(t, tt.want, result.Usage.Seconds)
		assert.Equal(t, 7, result.Usage.Total)
	}
}

func TestRealtimeMessageFailures(t *testing.T) {
	s := newRealtimeMessageSession()
	data, err := s.serverEvent(
		[]byte(
			`{"header":{"task_id":"` + s.taskID + `","event":"task-failed","error_code":"CLIENT_ERROR","error_message":"model unavailable"},"payload":{}}`,
		),
	)
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"type":"error","error":{"code":"CLIENT_ERROR","message":"model unavailable"}}`,
		string(data),
	)

	for _, data := range []string{
		`not json`, `{"header":{"task_id":"other","event":"task-finished"}}`,
		`{"header":{"task_id":"` + s.taskID + `","event":"result-generated"},"payload":{"output":{"sentence":{"text":"missing ID"}}}}`,
	} {
		_, err := s.serverEvent([]byte(data))
		require.Error(t, err)
	}
}
