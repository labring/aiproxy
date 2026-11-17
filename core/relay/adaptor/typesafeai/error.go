package typesafeai

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// maxRawErrorBodyLength bounds how much of an unrecognised error body is echoed back.
const maxRawErrorBodyLength = 200

// ErrorHanlder extracts the message from the TypeSafe error body, which reports
// failures FastAPI-style: {"detail": "<text>"} or {"detail": [{"loc": [...],
// "msg": "...", "type": "..."}]}. A body without a usable detail is reported
// with the HTTP status and a truncated copy of the body, never as an empty
// message.
func ErrorHanlder(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return relaymodel.WrapperOpenAIError(err, "read_response_body_failed", resp.StatusCode)
	}

	var parsed struct {
		Detail any `json:"detail"`
	}

	var message string

	if err := sonic.Unmarshal(body, &parsed); err == nil {
		switch detail := parsed.Detail.(type) {
		case string:
			message = detail
		case []any:
			var parts []string

			for _, item := range detail {
				if fields, ok := item.(map[string]any); ok {
					if msg, _ := fields["msg"].(string); msg != "" {
						parts = append(parts, msg)
					}
				}
			}

			message = strings.Join(parts, "; ")
		}
	}

	if message == "" {
		raw := strings.TrimSpace(string(body))
		if len(raw) > maxRawErrorBodyLength {
			raw = raw[:maxRawErrorBodyLength]
		}

		message = fmt.Sprintf("typesafe returned HTTP %d: %s", resp.StatusCode, raw)
	}

	return relaymodel.NewOpenAIError(resp.StatusCode, relaymodel.OpenAIError{
		Message: message,
		Type:    relaymodel.ErrorTypeUpstream,
		Code:    resp.StatusCode,
	})
}
