package openai_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSTTResponseSeconds checks the public JSON/SSE response and the separate legacy billing result.
func TestSTTResponseSeconds(t *testing.T) {
	const upstreamFields = `"type":"tokens","input_tokens":100,"output_tokens":17,"total_tokens":777,"input_token_details":{"audio_tokens":80,"text_tokens":20},"vendor":{"kept":true},"large":9007199254740993`

	tests := []struct {
		name, upstreamUsage        string
		measured, seconds          int64
		jsonBilling, streamBilling [4]int64 // input, output, total, audio input
	}{
		{
			name:          "upstream tokens without seconds",
			upstreamUsage: `{` + upstreamFields + `}`,
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{100, 17, 777, 80},
			streamBilling: [4]int64{100, 17, 777, 80},
		},
		{
			name:          "tokens without type",
			upstreamUsage: `{"input_tokens":100,"output_tokens":17,"total_tokens":777}`,
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{100, 17, 777, 0},
			streamBilling: [4]int64{100, 17, 777, 0},
		},
		{
			name:          "preserve upstream seconds",
			upstreamUsage: `{` + upstreamFields + `,"seconds":3}`,
			measured:      5,
			seconds:       3,
			jsonBilling:   [4]int64{100, 17, 777, 80},
			streamBilling: [4]int64{100, 17, 777, 80},
		},
		{
			name:          "preserve explicit zero",
			upstreamUsage: `{` + upstreamFields + `,"seconds":0}`,
			measured:      5,
			seconds:       0,
			jsonBilling:   [4]int64{100, 17, 777, 80},
			streamBilling: [4]int64{100, 17, 777, 80},
		},
		{
			name:          "null seconds",
			upstreamUsage: `{` + upstreamFields + `,"seconds":null}`,
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{100, 17, 777, 80},
			streamBilling: [4]int64{100, 17, 777, 80},
		},
		{
			name:          "negative seconds",
			upstreamUsage: `{` + upstreamFields + `,"seconds":-1}`,
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{100, 17, 777, 80},
			streamBilling: [4]int64{100, 17, 777, 80},
		},
		{
			name:          "missing measured duration",
			upstreamUsage: `{` + upstreamFields + `}`,
			seconds:       0,
			jsonBilling:   [4]int64{100, 17, 777, 80},
			streamBilling: [4]int64{100, 17, 777, 80},
		},
		{
			name:          "no upstream usage",
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{5, 1, 6, 5},
			streamBilling: [4]int64{5, 1, 6, 5},
		},
		{
			name:          "no upstream usage or duration",
			seconds:       0,
			jsonBilling:   [4]int64{0, 1, 1, 0},
			streamBilling: [4]int64{0, 1, 1, 0},
		},
		{
			name:          "null usage",
			upstreamUsage: `null`,
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{0, 0, 6, 0},
			streamBilling: [4]int64{5, 1, 6, 5},
		},
		{
			name:          "empty usage",
			upstreamUsage: `{}`,
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{0, 0, 6, 0},
		},
		{
			name:          "duration usage without seconds",
			upstreamUsage: `{"type":"duration","total_tokens":777}`,
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{5, 0, 5, 0},
		},
		{
			name:          "duration usage with seconds",
			upstreamUsage: `{"type":"duration","seconds":3,"total_tokens":777}`,
			measured:      5,
			seconds:       3,
			jsonBilling:   [4]int64{3, 0, 3, 0},
			streamBilling: [4]int64{3, 0, 3, 0},
		},
		{
			name:          "total only does not fabricate components",
			upstreamUsage: `{"total_tokens":777}`,
			measured:      5, seconds: 5,
			jsonBilling: [4]int64{0, 0, 777, 0}, streamBilling: [4]int64{0, 0, 777, 0},
		},
		{
			name:          "chat aliases",
			upstreamUsage: `{"prompt_tokens":125,"completion_tokens":17,"total_tokens":999,"vendor":{"kept":true}}`,
			measured:      5,
			seconds:       5,
			jsonBilling:   [4]int64{125, 17, 999, 0},
			streamBilling: [4]int64{125, 17, 999, 0},
		},
	}
	for _, tt := range tests {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", tt.name, stream), func(t *testing.T) {
				payload := `{"text":"hello","vendor_request_id":"kept"`
				if stream {
					payload += `,"type":"transcript.text.done"`
				}

				if tt.upstreamUsage != "" {
					payload += `,"usage":` + tt.upstreamUsage
				}

				payload += `}`
				wire := payload

				contentType := "application/json"
				if stream {
					wire = "data: {\"type\":\"transcript.text.delta\",\"delta\":\"hello\"}\n\ndata: " + payload + "\n\ndata: [DONE]\n\n"
					contentType = "text/event-stream"
				}

				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Request = httptest.NewRequestWithContext(
					t.Context(),
					http.MethodPost,
					"/v1/audio/transcriptions",
					nil,
				)
				mt := meta.NewMeta(
					nil,
					mode.AudioTranscription,
					"whisper-1",
					model.ModelConfig{},
					meta.WithRequestUsage(
						model.Usage{
							InputTokens:      model.ZeroNullInt64(tt.measured),
							AudioInputTokens: model.ZeroNullInt64(tt.measured),
						},
					),
				)
				result, relayErr := openai.STTHandler(
					mt,
					ctx,
					&http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": {contentType}},
						Body:       io.NopCloser(strings.NewReader(wire)),
					},
				)
				require.Nil(t, relayErr)

				expectedBilling := tt.jsonBilling
				if stream {
					expectedBilling = tt.streamBilling
				}

				assert.Equal(
					t,
					model.Usage{
						InputTokens:      model.ZeroNullInt64(expectedBilling[0]),
						OutputTokens:     model.ZeroNullInt64(expectedBilling[1]),
						TotalTokens:      model.ZeroNullInt64(expectedBilling[2]),
						AudioInputTokens: model.ZeroNullInt64(expectedBilling[3]),
					},
					result.Usage,
					"actual token components and the legacy total are accounted independently",
				)

				returned := recorder.Body.String()
				if stream {
					assert.Contains(
						t,
						returned,
						"data: {\"type\":\"transcript.text.delta\",\"delta\":\"hello\"}",
					)
					assert.Equal(t, 1, strings.Count(returned, "data: [DONE]"))

					returned = ""
					for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
						if strings.HasPrefix(line, "data: ") &&
							strings.Contains(line, "transcript.text.done") {
							returned = strings.TrimPrefix(line, "data: ")
						}
					}

					require.NotEmpty(t, returned)
				} else {
					assert.Equal(
						t,
						strconv.Itoa(recorder.Body.Len()),
						recorder.Header().Get("Content-Length"),
					)
				}

				var body map[string]json.RawMessage
				require.NoError(t, json.Unmarshal([]byte(returned), &body))
				assert.JSONEq(t, `"kept"`, string(body["vendor_request_id"]))

				var usage map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(body["usage"], &usage))
				seconds, exists := usage["seconds"]
				require.True(t, exists, "seconds must be present even when zero")
				assert.JSONEq(t, strconv.FormatInt(tt.seconds, 10), string(seconds))

				switch {
				case tt.upstreamUsage != "" && tt.upstreamUsage != "null":
					var original map[string]json.RawMessage
					require.NoError(t, json.Unmarshal([]byte(tt.upstreamUsage), &original))
					delete(original, "seconds")

					if tt.name == "chat aliases" {
						original["input_tokens"] = json.RawMessage(`125`)
						original["output_tokens"] = json.RawMessage(`17`)
					}

					delete(usage, "seconds")
					assert.Equal(
						t,
						original,
						usage,
						"preserve unknown fields and exact token numbers while normalizing explicit aliases",
					)
				case tt.upstreamUsage == "" || stream:
					assert.JSONEq(t, `"duration"`, string(usage["type"]))
					assert.NotContains(t, usage, "input_tokens")
					assert.NotContains(t, usage, "output_tokens")
					assert.JSONEq(
						t,
						strconv.FormatInt(expectedBilling[2], 10),
						string(usage["total_tokens"]),
						"retain generated total_tokens",
					)
				default:
					assert.NotContains(
						t,
						usage,
						"total_tokens",
						"do not add token fields to an existing null usage",
					)
				}
			})
		}
	}
}

func TestSTTTextResponseUnchanged(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/audio/transcriptions",
		nil,
	)
	mt := meta.NewMeta(
		nil,
		mode.AudioTranscription,
		"whisper-1",
		model.ModelConfig{},
		meta.WithRequestUsage(model.Usage{InputTokens: 5, AudioInputTokens: 5}),
	)
	mt.Set(openai.MetaResponseFormat, "text")
	result, relayErr := openai.STTHandler(
		mt,
		ctx,
		&http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("hello\n")),
		},
	)
	require.Nil(t, relayErr)
	assert.Equal(t, "hello\n", recorder.Body.String())
	assert.Equal(t, model.ZeroNullInt64(6), result.Usage.TotalTokens)
}
