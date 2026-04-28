//nolint:testpackage
package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoResponse_MapReasoningToReasoningContent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","reasoning":"internal-thought","content":"final-answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
		)),
		Header: make(http.Header),
	}

	m := &meta.Meta{
		Mode:        mode.ChatCompletions,
		OriginModel: "gpt-4o",
		ActualModel: "gpt-4o",
		ChannelConfigs: model.ChannelConfigs{
			"map_reasoning_to_reasoning_content": true,
		},
	}

	result, err := DoResponse(m, nil, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "chatcmpl-1", result.UpstreamID)

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

	choices, ok := body["choices"].([]any)
	require.True(t, ok)
	require.Len(t, choices, 1)

	choice, ok := choices[0].(map[string]any)
	require.True(t, ok)
	message, ok := choice["message"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "internal-thought", message["reasoning_content"])
	_, exists := message["reasoning"]
	assert.False(t, exists)
}

func TestDoResponseStream_MapReasoningToReasoningContent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(
			"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"reasoning\":\"internal-thought\"}}]}\n\n" +
				"data: [DONE]\n\n",
		)),
		Header: http.Header{
			"Content-Type": {"text/event-stream"},
		},
	}

	m := &meta.Meta{
		Mode:        mode.ChatCompletions,
		OriginModel: "gpt-4o",
		ActualModel: "gpt-4o",
		ChannelConfigs: model.ChannelConfigs{
			"map_reasoning_to_reasoning_content": true,
		},
	}

	result, err := DoResponse(m, nil, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "chatcmpl-1", result.UpstreamID)

	body := recorder.Body.String()
	assert.Contains(t, body, `"reasoning_content":"internal-thought"`)
	assert.NotContains(t, body, `"reasoning":"internal-thought"`)
}
