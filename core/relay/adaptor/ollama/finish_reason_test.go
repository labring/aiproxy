//nolint:testpackage
package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/stretchr/testify/assert"
)

func toolCallMessage() *Message {
	return &Message{
		Role: "assistant",
		ToolCalls: []*Tool{
			{Function: Function{Name: "get_weather", Arguments: map[string]any{"city": "Paris"}}},
		},
	}
}

func TestResponse2OpenAI_FinishReason(t *testing.T) {
	m := &meta.Meta{OriginModel: "llama3"}

	withTools := response2OpenAI(
		m,
		&ChatResponse{Message: toolCallMessage(), Done: true, DoneReason: "stop"},
	)
	assert.Equal(t, "tool_calls", withTools.Choices[0].FinishReason)

	plain := response2OpenAI(m, &ChatResponse{
		Message: &Message{Role: "assistant", Content: "hi"}, Done: true, DoneReason: "stop",
	})
	assert.Equal(t, "stop", plain.Choices[0].FinishReason)

	length := response2OpenAI(m, &ChatResponse{
		Message: toolCallMessage(), Done: true, DoneReason: "length",
	})
	assert.Equal(t, "length", length.Choices[0].FinishReason)
}

func TestStreamHandler_FinishReasonToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"message":{"role":"assistant","tool_calls":[{"function":{"name":"get_weather","arguments":{"city":"Paris"}}}]},"done":false}
{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":3,"eval_count":2}
`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}
	resp.Body = readCloser{strings.NewReader(body)}

	_, err := StreamHandler(&meta.Meta{OriginModel: "llama3"}, c, resp)
	assert.Nil(t, err)
	assert.Contains(t, w.Body.String(), `"finish_reason":"tool_calls"`)
	assert.NotContains(t, w.Body.String(), `"finish_reason":"stop"`)
}

type readCloser struct{ *strings.Reader }

func (readCloser) Close() error { return nil }
