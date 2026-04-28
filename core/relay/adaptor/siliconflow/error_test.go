//nolint:testpackage
package siliconflow

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorHandlerParsesObjectError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body: io.NopCloser(strings.NewReader(
			`{"message":"System is really busy, please try again later","code":20015}`,
		)),
	}

	err := ErrorHandler(resp)
	require.Equal(t, http.StatusTooManyRequests, err.StatusCode())

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
	require.Equal(t, "20015", body.Error.Code)
	require.Equal(t, "System is really busy, please try again later", body.Error.Message)
	require.Equal(t, "upstream_error", body.Error.Type)
}

func TestErrorHandlerParsesJSONStringError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(`"Api key is invalid"`)),
	}

	err := ErrorHandler(resp)
	require.Equal(t, http.StatusUnauthorized, err.StatusCode())

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
	require.Equal(t, "bad_response", body.Error.Code)
	require.Equal(t, "Api key is invalid", body.Error.Message)
	require.Equal(t, "upstream_error", body.Error.Type)
}

func TestErrorHandlerParsesNestedErrorObject(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"message":"Api key is invalid","code":"invalid_api_key"}}`,
		)),
	}

	err := ErrorHandler(resp)
	require.Equal(t, http.StatusUnauthorized, err.StatusCode())

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
	require.Equal(t, "invalid_api_key", body.Error.Code)
	require.Equal(t, "Api key is invalid", body.Error.Message)
	require.Equal(t, "upstream_error", body.Error.Type)
}
