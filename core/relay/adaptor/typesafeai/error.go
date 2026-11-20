package typesafeai

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// maxRawErrorBodyLength bounds how much of an unrecognised error body is echoed back.
const maxRawErrorBodyLength = 200

// ErrorHanlder extracts the message from the TypeSafe error body, which reports
// failures FastAPI-style: {"detail": "<text>"} or {"detail": [{"loc": [...],
// "msg": "...", "type": "..."}]}. A body without a usable detail is reported
// with the HTTP status and a truncated copy of the body, never as an empty
// message. The message is returned in the SystemOne error shape, which is what
// clients of /v1/systemone parse.
func ErrorHanlder(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	body, err := common.GetResponseBody(resp)
	if err != nil {
		return relaymodel.NewSystemOneError(
			http.StatusInternalServerError,
			relaymodel.SystemOneError{
				Detail: "read response body failed: " + err.Error(),
			},
		)
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

	return relaymodel.NewSystemOneError(resp.StatusCode, relaymodel.SystemOneError{
		Detail: message,
	})
}

func convertRequestError(meta *meta.Meta, message string) adaptor.Error {
	return relaymodel.WrapperErrorWithMessage(
		meta.Mode,
		http.StatusBadRequest,
		message,
	)
}
