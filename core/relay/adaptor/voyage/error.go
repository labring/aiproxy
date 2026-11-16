package voyage

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// maxRawErrorBodyLength bounds how much of an unrecognised error body is echoed back.
const maxRawErrorBodyLength = 200

// ErrorHandler extracts the message from the Voyage error body, which is always
// {"detail": "<text>"}. A body without a string `detail` is reported as an
// upstream error carrying the HTTP status and a truncated copy of the body,
// never as an empty message.
func ErrorHandler(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return relaymodel.WrapperOpenAIError(err, "read_response_body_failed", resp.StatusCode)
	}

	var parsed struct {
		Detail string `json:"detail"`
	}

	var message string
	if err := sonic.Unmarshal(body, &parsed); err == nil && parsed.Detail != "" {
		message = parsed.Detail
	} else {
		raw := strings.TrimSpace(string(body))
		if len(raw) > maxRawErrorBodyLength {
			raw = raw[:maxRawErrorBodyLength]
		}

		message = fmt.Sprintf("voyage returned HTTP %d: %s", resp.StatusCode, raw)
	}

	return relaymodel.NewOpenAIError(resp.StatusCode, relaymodel.OpenAIError{
		Message: message,
		Type:    relaymodel.ErrorTypeUpstream,
		Code:    resp.StatusCode,
	})
}

func convertRequestError(message string) adaptor.Error {
	return relaymodel.WrapperOpenAIErrorWithMessage(
		message,
		"invalid_request_error",
		http.StatusBadRequest,
	)
}

func unsupportedModeError(m mode.Mode) adaptor.Error {
	return relaymodel.WrapperOpenAIErrorWithMessage(
		"voyage channel does not support mode "+m.String(),
		"unsupported_mode",
		http.StatusBadRequest,
	)
}

func missingUsageError(what string) adaptor.Error {
	return relaymodel.WrapperOpenAIErrorWithMessage(
		"voyage response has no usage.total_tokens for "+what+"; refusing to bill zero",
		"missing_usage",
		http.StatusBadGateway,
	)
}
