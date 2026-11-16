package controller_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelChatUpstreamProtocol(t *testing.T) {
	tests := []struct {
		name      string
		protocol  string
		origin    string
		actual    string
		modelType mode.Mode
		responses bool
		stream    bool
	}{
		{
			name:      "channel A converts ordinary GPT",
			protocol:  "responses",
			origin:    "gpt-4o",
			responses: true,
		},
		{
			name:      "channel A converts custom alias",
			protocol:  "responses",
			origin:    "public-alias",
			actual:    "private-deployment",
			responses: true,
		},
		{
			name:     "channel B overrides GPT name detection",
			protocol: "chat_completions",
			origin:   "gpt-5",
			actual:   "private-deployment",
		},
		{
			name:     "channel B overrides mapped name detection",
			protocol: "chat_completions",
			origin:   "public-alias",
			actual:   "gpt-5",
		},
		{
			name:      "channel B overrides model type",
			protocol:  "chat_completions",
			origin:    "public-alias",
			modelType: mode.Responses,
		},
		{name: "unset retains ordinary chat", origin: "gpt-4o"},
		{
			name:      "auto retains original name detection",
			protocol:  "auto",
			origin:    "gpt-5",
			actual:    "private-deployment",
			responses: true,
		},
		{
			name:      "auto retains mapped name detection",
			protocol:  "auto",
			origin:    "public-alias",
			actual:    "gpt-5",
			responses: true,
		},
		{
			name:      "unset retains model type detection",
			origin:    "public-alias",
			modelType: mode.Responses,
			responses: true,
		},
		{
			name:      "channel A converts Responses SSE",
			protocol:  "responses",
			origin:    "gpt-4o",
			responses: true,
			stream:    true,
		},
		{
			name:     "channel B preserves Chat SSE",
			protocol: "chat_completions",
			origin:   "gpt-5",
			stream:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &model.Channel{
				ID: 1001, Type: model.ChannelTypeOpenAI, Key: "mock-channel-key",
				Configs: model.ChannelConfigs{}, ModelMapping: map[string]string{},
			}
			if tt.protocol != "" {
				channel.Configs["chat_upstream_protocol"] = tt.protocol
			}

			if tt.actual != "" {
				channel.ModelMapping[tt.origin] = tt.actual
			}

			mt := meta.NewMeta(
				channel,
				mode.ChatCompletions,
				tt.origin,
				model.ModelConfig{Type: tt.modelType},
			)
			exerciseChannelChatProtocol(t, &openai.Adaptor{}, mt, tt.responses, tt.stream)
		})
	}
}

func TestChannelChatUpstreamProtocolSwitchChannel(t *testing.T) {
	a := &openai.Adaptor{}
	mt := meta.NewMeta(nil, mode.ChatCompletions, "gpt-5", model.ModelConfig{Type: mode.Responses})
	// Retry on the same request metadata must use the newly selected channel,
	// including its protocol and model mapping, in both directions.
	for i, protocol := range []string{"responses", "chat_completions", "responses"} {
		t.Run(fmt.Sprintf("attempt_%d_%s", i, protocol), func(t *testing.T) {
			mt.SetChannel(&model.Channel{
				ID: i + 1, Type: model.ChannelTypeOpenAI, Key: "mock-channel-key",
				Configs:      model.ChannelConfigs{"chat_upstream_protocol": protocol},
				ModelMapping: map[string]string{"gpt-5": fmt.Sprintf("deployment-%d", i)},
			})
			exerciseChannelChatProtocol(t, a, mt, protocol == "responses", false)
		})
	}
}

// exerciseChannelChatProtocol uses a real TCP listener as the upstream and the
// production relay helper to verify URL, body, headers, transport and responses.
func exerciseChannelChatProtocol(
	t *testing.T,
	a *openai.Adaptor,
	mt *meta.Meta,
	responses, stream bool,
) {
	t.Helper()

	responseJSON := fmt.Sprintf(
		`{"id":"resp_mock","object":"response","created_at":1700000000,"status":"completed","model":%q,"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"mock answer"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`,
		mt.ActualModel,
	)
	chatJSON := fmt.Sprintf(
		`{"id":"chat_mock","object":"chat.completion","created":1700000000,"model":%q,"choices":[{"index":0,"message":{"role":"assistant","content":"mock answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
		mt.ActualModel,
	)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/v1/chat/completions"
		if responses {
			expectedPath = "/v1/responses"
		}

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, "Bearer mock-channel-key", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		assert.Equal(t, mt.ActualModel, body["model"])

		if responses {
			assert.Contains(t, body, "input")
			assert.NotContains(t, body, "messages")
			assert.Equal(t, false, body["store"])
		} else {
			assert.Contains(t, body, "messages")
			assert.NotContains(t, body, "input")
		}

		if stream {
			assert.Equal(t, true, body["stream"])
			w.Header().Set("Content-Type", "text/event-stream")

			if responses {
				_, _ = fmt.Fprintf(
					w,
					"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_mock\",\"created_at\":1700000000}}\n\n",
				)
				_, _ = fmt.Fprintf(
					w,
					"data: {\"type\":\"response.output_text.delta\",\"delta\":\"mock answer\"}\n\n",
				)
				_, _ = fmt.Fprintf(
					w,
					"data: {\"type\":\"response.completed\",\"response\":%s}\n\n",
					responseJSON,
				)
			} else {
				_, _ = fmt.Fprintf(
					w,
					"data: {\"id\":\"chat_mock\",\"object\":\"chat.completion.chunk\",\"model\":%q,\"choices\":[{\"index\":0,\"delta\":{\"content\":\"mock answer\"},\"finish_reason\":null}]}\n\n",
					mt.ActualModel,
				)
				_, _ = fmt.Fprintf(
					w,
					"data: {\"id\":\"chat_mock\",\"object\":\"chat.completion.chunk\",\"model\":%q,\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n",
					mt.ActualModel,
				)
			}

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if responses {
			_, _ = w.Write([]byte(responseJSON))
		} else {
			_, _ = w.Write([]byte(chatJSON))
		}
	}))
	defer upstream.Close()

	t.Logf("Mock upstream listener: %s (responses=%t, stream=%t)", upstream.URL, responses, stream)
	mt.Channel.BaseURL = upstream.URL + "/v1"
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := fmt.Sprintf(
		`{"model":%q,"messages":[{"role":"user","content":"hello"}],"stream":%t,"stream_options":{"include_usage":true}}`,
		mt.OriginModel,
		stream,
	)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(body),
	)
	c.Request.Header.Set("Content-Type", "application/json")
	result, _, relayErr := controller.DoHelper(a, c, mt, nil)
	require.Nil(t, relayErr)
	require.Equal(t, http.StatusOK, recorder.Code)

	if stream {
		assert.Contains(t, recorder.Body.String(), "data: [DONE]")
		assert.NotContains(t, recorder.Body.String(), "response.output_text.delta")

		var answer strings.Builder
		for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
			if !strings.HasPrefix(line, "data: {") {
				continue
			}

			var chunk relaymodel.ChatCompletionsStreamResponse
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk))
			assert.Equal(t, "chat.completion.chunk", chunk.Object)

			for _, choice := range chunk.Choices {
				if content, ok := choice.Delta.Content.(string); ok {
					answer.WriteString(content)
				}
			}
		}

		assert.Equal(t, "mock answer", answer.String())
	} else {
		var response relaymodel.TextResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		assert.Equal(t, "chat.completion", response.Object)
		require.Len(t, response.Choices, 1)
		assert.Equal(t, "mock answer", response.Choices[0].Message.Content)
	}

	assert.Equal(t, int64(3), int64(result.Usage.InputTokens))
	assert.Equal(t, int64(2), int64(result.Usage.OutputTokens))
}
