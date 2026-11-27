//nolint:testpackage
package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/stretchr/testify/require"
)

func TestPromptBlockedResponse(t *testing.T) {
	const body = `{"promptFeedback":{"blockReason":"PROHIBITED_CONTENT"},"usageMetadata":{"promptTokenCount":10,"totalTokenCount":10},"modelVersion":"gemini-2.5-flash"}`

	type handler func(*meta.Meta, *gin.Context, *http.Response) (adaptor.DoResponseResult, adaptor.Error)

	for _, tt := range []struct {
		name, body, want string
		handle           handler
	}{
		{"openai", body, `"finish_reason":"content_filter"`, Handler},
		{"openai stream", "data: " + body + "\n\n", `"finish_reason":"content_filter"`, StreamHandler},
		{"claude", body, `"stop_reason":"refusal"`, ClaudeHandler},
		{"claude stream", "data: " + body + "\n\n", `"stop_reason":"refusal"`, ClaudeStreamHandler},
		{"native", body, `"blockReason":"PROHIBITED_CONTENT"`, NativeHandler},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

			result, err := tt.handle(
				&meta.Meta{OriginModel: "gemini-2.5-flash"},
				c,
				&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				},
			)
			require.Nil(t, err)
			require.Contains(t, w.Body.String(), tt.want)
			require.EqualValues(t, 10, result.Usage.InputTokens)
		})
	}
}
