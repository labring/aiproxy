package timeout_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/plugin/timeout"
	"github.com/stretchr/testify/require"
)

func TestDecisionsTimeout(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		config model.TimeoutConfig
		want   time.Duration
	}{
		{"default synchronous timeout", model.TimeoutConfig{}, 3 * time.Minute},
		{"model override", model.TimeoutConfig{RequestTimeout: 1}, time.Second},
		{"stream override does not apply", model.TimeoutConfig{StreamRequestTimeout: 1}, 3 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requestBody := `{"model":"gpt-6-luna","input":"Hello","questions":[{"type":"predicate","instructions":"Is this a greeting?"}]}`
			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/decisions",
				strings.NewReader(requestBody),
			)
			req.Header.Set("Content-Type", "application/json")

			requestMeta := &meta.Meta{
				Mode: mode.Decisions, ActualModel: "gpt-6-luna",
				ModelConfig: model.ModelConfig{TimeoutConfig: tt.config},
			}

			result, err := timeout.NewTimeoutPlugin().
				ConvertRequest(requestMeta, nil, req, &openai.Adaptor{})
			require.NoError(t, err)
			require.Equal(t, tt.want, requestMeta.RequestTimeout)

			body, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			require.JSONEq(t, requestBody, string(body))
		})
	}
}
