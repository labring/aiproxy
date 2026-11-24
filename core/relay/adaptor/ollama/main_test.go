package ollama_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/ollama"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
)

func TestConvertRequest_JsonObject(t *testing.T) {
	// Setup metadata
	meta := meta.NewMeta(
		&model.Channel{
			Type: model.ChannelTypeOllama,
		},
		0,
		"llama2",
		model.ModelConfig{},
	)

	// Create OpenAI request with response_format: {"type": "json_object"}
	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "llama2",
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "Hello, give me JSON",
			},
		},
		ResponseFormat: &relaymodel.ResponseFormat{
			Type: "json_object",
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost:11434/api/chat",
		bytes.NewBuffer(jsonData),
	)

	// Convert request
	result, err := ollama.ConvertRequest(meta, req)
	assert.NoError(t, err)

	// Parse body to check format field
	bodyBytes, _ := io.ReadAll(result.Body)

	var ollamaReq ollama.ChatRequest

	err = json.Unmarshal(bodyBytes, &ollamaReq)
	assert.NoError(t, err)

	// Verify format is "json"
	assert.Equal(t, "json", ollamaReq.Format)
}

func TestConvertRequest_MultiPartTextContent(t *testing.T) {
	meta := meta.NewMeta(
		&model.Channel{
			Type: model.ChannelTypeOllama,
		},
		0,
		"llama2",
		model.ModelConfig{},
	)

	body := `{"model":"llama2","messages":[{"role":"user","content":[` +
		`{"type":"text","text":"Hello, "},{"type":"text","text":"world"}]}]}`

	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost:11434/api/chat",
		strings.NewReader(body),
	)

	result, err := ollama.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var ollamaReq ollama.ChatRequest

	err = json.Unmarshal(bodyBytes, &ollamaReq)
	assert.NoError(t, err)
	assert.Len(t, ollamaReq.Messages, 1)
	assert.Equal(t, "Hello, world", ollamaReq.Messages[0].Content)
}

func TestStreamHandler_ToolCallIndexes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeOllama},
		0,
		"llama3",
		model.ModelConfig{},
	)

	// Two tool calls in the first chunk and one more in a later chunk: the
	// indexes must be distinct across the whole stream, because OpenAI clients
	// merge streamed tool call deltas by index.
	body := `{"message":{"role":"assistant","tool_calls":[` +
		`{"function":{"name":"a","arguments":{}}},` +
		`{"function":{"name":"b","arguments":{}}}]},"done":false}` + "\n" +
		`{"message":{"role":"assistant","tool_calls":[` +
		`{"function":{"name":"c","arguments":{}}}]},"done":false}` + "\n" +
		`{"message":{"role":"assistant"},"done":true,"done_reason":"stop"}` + "\n"

	_, err := ollama.StreamHandler(m, c, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	})
	assert.Nil(t, err)

	got := map[string]int{}

	for line := range strings.SplitSeq(w.Body.String(), "\n") {
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok || strings.TrimSpace(payload) == "[DONE]" {
			continue
		}

		var chunk relaymodel.ChatCompletionsStreamResponse

		assert.NoError(t, json.Unmarshal([]byte(payload), &chunk))

		for _, choice := range chunk.Choices {
			for _, tc := range choice.Delta.ToolCalls {
				got[tc.Function.Name] = tc.Index
			}
		}
	}

	assert.Equal(t, map[string]int{"a": 0, "b": 1, "c": 2}, got)
}

func TestConvertRequest_ReasoningEffortToThink(t *testing.T) {
	cases := []struct {
		effort string
		want   any
	}{
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"none", false},
		{"off", false},
		{"bogus", nil},
	}
	for _, tc := range cases {
		t.Run(tc.effort, func(t *testing.T) {
			m := meta.NewMeta(
				&model.Channel{Type: model.ChannelTypeOllama},
				0,
				"gpt-oss:20b",
				model.ModelConfig{},
			)
			effort := tc.effort
			data, _ := sonic.Marshal(relaymodel.GeneralOpenAIRequest{
				Model:           "gpt-oss:20b",
				Messages:        []relaymodel.Message{{Role: "user", Content: "hi"}},
				ReasoningEffort: &effort,
			})
			req, _ := http.NewRequestWithContext(
				t.Context(), http.MethodPost, "http://localhost:11434/api/chat",
				bytes.NewBuffer(data),
			)

			result, err := ollama.ConvertRequest(m, req)
			assert.NoError(t, err)

			body, _ := io.ReadAll(result.Body)

			var got map[string]any

			assert.NoError(t, json.Unmarshal(body, &got))

			think, ok := got["think"]
			if tc.want == nil {
				assert.False(t, ok)
				return
			}

			assert.Equal(t, tc.want, think)
		})
	}
}

func TestConvertRequest_MaxCompletionTokens(t *testing.T) {
	m := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeOllama},
		0,
		"llama2",
		model.ModelConfig{},
	)

	jsonData, _ := sonic.Marshal(relaymodel.GeneralOpenAIRequest{
		Model:               "llama2",
		Messages:            []relaymodel.Message{{Role: "user", Content: "hi"}},
		MaxCompletionTokens: 55,
	})
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost:11434/api/chat",
		bytes.NewBuffer(jsonData),
	)

	result, err := ollama.ConvertRequest(m, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var ollamaReq ollama.ChatRequest

	assert.NoError(t, json.Unmarshal(bodyBytes, &ollamaReq))
	assert.NotNil(t, ollamaReq.Options)
	assert.Equal(t, 55, ollamaReq.Options.NumPredict)
}

func TestConvertRequest_StopSequences(t *testing.T) {
	tests := []struct {
		name string
		stop any
		want []any
	}{
		{name: "string", stop: "END", want: []any{"END"}},
		{name: "array", stop: []string{"a", "b"}, want: []any{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(
				&model.Channel{Type: model.ChannelTypeOllama},
				0,
				"llama2",
				model.ModelConfig{},
			)

			jsonData, _ := sonic.Marshal(relaymodel.GeneralOpenAIRequest{
				Model:    "llama2",
				Messages: []relaymodel.Message{{Role: "user", Content: "hi"}},
				Stop:     tt.stop,
			})
			req, _ := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://localhost:11434/api/chat",
				bytes.NewBuffer(jsonData),
			)

			result, err := ollama.ConvertRequest(m, req)
			assert.NoError(t, err)

			bodyBytes, _ := io.ReadAll(result.Body)

			var ollamaReq ollama.ChatRequest

			assert.NoError(t, json.Unmarshal(bodyBytes, &ollamaReq))
			assert.Equal(t, tt.want, ollamaReq.Options.Stop)
		})
	}
}

func TestConvertRequest_ForwardsTopK(t *testing.T) {
	m := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeOllama},
		0,
		"llama2",
		model.ModelConfig{},
	)

	body := `{"model":"llama2","messages":[{"role":"user","content":"hi"}],"top_k":40}`
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost:11434/api/chat",
		strings.NewReader(body),
	)

	result, err := ollama.ConvertRequest(m, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var got map[string]any
	assert.NoError(t, json.Unmarshal(bodyBytes, &got))

	options, ok := got["options"].(map[string]any)
	assert.True(t, ok)
	assert.EqualValues(t, 40, options["top_k"])
}
