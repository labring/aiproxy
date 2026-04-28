//nolint:testpackage
package zhipu

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/require"
)

func TestErrorHandlerNoBalanceError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`,
		)),
	}

	err := ErrorHandler(resp)
	require.Equal(t, http.StatusPaymentRequired, err.StatusCode())

	data, marshalErr := err.MarshalJSON()
	require.NoError(t, marshalErr)

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	require.NoError(t, json.Unmarshal(data, &body))
	require.Equal(t, "1113", body.Error.Code)
	require.Equal(t, "余额不足或无可用资源包,请充值。", body.Error.Message)
	require.Equal(t, "upstream_error", body.Error.Type)
}

func TestNormalizeErrorStatusCodeByBusinessCode(t *testing.T) {
	tests := []struct {
		name       string
		code       any
		statusCode int
		want       int
	}{
		{
			name:       "auth error",
			code:       "1002",
			statusCode: http.StatusBadRequest,
			want:       http.StatusUnauthorized,
		},
		{
			name:       "no api permission",
			code:       "1220",
			statusCode: http.StatusBadRequest,
			want:       http.StatusForbidden,
		},
		{
			name:       "plan no model permission",
			code:       "1311",
			statusCode: http.StatusBadRequest,
			want:       http.StatusForbidden,
		},
		{
			name:       "quota exceeded",
			code:       "1308",
			statusCode: http.StatusTooManyRequests,
			want:       http.StatusPaymentRequired,
		},
		{
			name:       "rate limited",
			code:       "1303",
			statusCode: http.StatusBadRequest,
			want:       http.StatusTooManyRequests,
		},
		{
			name:       "prompt too long",
			code:       "1261",
			statusCode: http.StatusBadRequest,
			want:       http.StatusRequestEntityTooLarge,
		},
		{
			name:       "http permission status",
			code:       "unknown",
			statusCode: 434,
			want:       http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode, openAIError := normalizeError(tt.statusCode, relaymodel.OpenAIError{
				Code:    tt.code,
				Message: "test message",
			})

			require.Equal(t, tt.want, statusCode)
			require.Equal(t, "upstream_error", openAIError.Type)
		})
	}
}
