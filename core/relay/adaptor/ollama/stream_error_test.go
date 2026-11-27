package ollama_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/ollama"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runOllamaHandler(t *testing.T, stream bool, body string) (*httptest.ResponseRecorder, error) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

	m := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeOllama},
		0,
		"llama3",
		model.ModelConfig{},
	)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	handler := ollama.Handler
	if stream {
		handler = ollama.StreamHandler
	}

	_, relayErr := handler(m, c, resp)
	if relayErr != nil {
		return recorder, relayErr
	}

	return recorder, nil
}

func TestHandlersReturnUpstreamErrorBody(t *testing.T) {
	for name, stream := range map[string]bool{"stream": true, "non-stream": false} {
		t.Run(name, func(t *testing.T) {
			recorder, err := runOllamaHandler(t, stream, `{"error":"model runner stopped"}`+"\n")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "model runner stopped")
			assert.Empty(t, recorder.Body.String())
		})
	}
}

func TestStreamHandlerStopsOnMidStreamError(t *testing.T) {
	recorder, err := runOllamaHandler(t, true, strings.Join([]string{
		`{"model":"llama3","message":{"role":"assistant","content":"Hel"},"done":false}`,
		`{"error":"model runner stopped"}`,
		`{"model":"llama3","message":{"role":"assistant","content":"after"},"done":false}`,
	}, "\n"))

	require.NoError(t, err)

	body := recorder.Body.String()
	assert.Equal(t, 1, strings.Count(body, `"chat.completion.chunk"`))
	assert.NotContains(t, body, "after")
}
