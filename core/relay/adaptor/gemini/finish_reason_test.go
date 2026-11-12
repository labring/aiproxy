package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/require"
)

func TestFinishReasonConversions(t *testing.T) {
	for _, tt := range []struct {
		reason, openai, claude string
		tool                   bool
	}{
		{"STOP", "stop", "end_turn", false},
		{"MAX_TOKENS", "length", "max_tokens", false},
		{"SAFETY", "content_filter", "end_turn", false},
		{"RECITATION", "content_filter", "end_turn", false},
		{"OTHER", "stop", "end_turn", false},
		{"STOP", "tool_calls", "tool_use", true},
		{"MAX_TOKENS", "length", "max_tokens", true},
	} {
		t.Run(tt.reason+tt.openai, func(t *testing.T) {
			parts := []*relaymodel.GeminiPart{{Text: "hello"}}
			if tt.tool {
				parts = []*relaymodel.GeminiPart{{FunctionCall: &relaymodel.GeminiFunctionCall{Name: "weather", Args: map[string]any{}}}}
			}
			response := &relaymodel.GeminiChatResponse{Candidates: []*relaymodel.GeminiChatCandidate{{FinishReason: tt.reason, Content: relaymodel.GeminiChatContent{Parts: parts}}}}
			m := &meta.Meta{OriginModel: "gemini-2.5-flash"}
			require.Equal(t, tt.openai, responseChat2OpenAI(m, response).Choices[0].FinishReason)
			require.Equal(t, tt.openai, streamResponseChat2OpenAI(m, response).Choices[0].FinishReason)
			require.Equal(t, tt.claude, geminiResponse2Claude(m, response).StopReason)
		})
	}
}

func TestClaudeStreamFinishReason(t *testing.T) {
	for _, tt := range []struct{ name, chunks, want string }{
		{"safety", `data: {"candidates":[{"finishReason":"SAFETY"}]}` + "\n\n", "end_turn"},
		{"tool in earlier chunk", `data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"weather","args":{}}}]}}]}` + "\n\ndata: " + `{"candidates":[{"finishReason":"STOP"}]}` + "\n\n", "tool_use"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			_, err := ClaudeStreamHandler(&meta.Meta{OriginModel: "gemini-2.5-flash"}, c, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tt.chunks))})
			require.Nil(t, err)
			require.Contains(t, w.Body.String(), `"stop_reason":"`+tt.want+`"`)
		})
	}
}

func TestOpenAIStreamFinishReason(t *testing.T) {
	chunks := `data: {"candidates":[{"index":0,"content":{"parts":[{"functionCall":{"name":"weather","args":{}}}]}}]}` + "\n\ndata: " + `{"candidates":[{"index":0,"finishReason":"STOP"}]}` + "\n\n"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := StreamHandler(&meta.Meta{OriginModel: "gemini-2.5-flash"}, c, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(chunks))})
	require.Nil(t, err)
	require.Contains(t, w.Body.String(), `"finish_reason":"tool_calls"`)
	require.NotContains(t, w.Body.String(), `"finish_reason":"stop"`)
}
