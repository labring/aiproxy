package siliconflow

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type errorResponse struct {
	Message string          `json:"message"`
	Code    any             `json:"code"`
	Error   json.RawMessage `json:"error"`
}

func ErrorHandler(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	message, code, parseErr := parseErrorResponse(responseBody)
	if parseErr != nil {
		return relaymodel.WrapperOpenAIErrorWithMessage(
			parseErr.Error(),
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	statusCode, code := normalizeError(resp.StatusCode, message, code)

	return relaymodel.WrapperOpenAIErrorWithMessage(
		message,
		code,
		statusCode,
		relaymodel.ErrorTypeUpstream,
	)
}

func normalizeError(statusCode int, message string, code any) (int, any) {
	if strings.Contains(message, "System is really busy") {
		statusCode = http.StatusTooManyRequests
	}

	if isUnauthorizedMessage(message) {
		statusCode = http.StatusUnauthorized
	}

	if code == nil {
		code = relaymodel.ErrorCodeBadResponse
	}

	return statusCode, code
}

func isUnauthorizedMessage(message string) bool {
	lowerMessage := strings.ToLower(message)

	return strings.Contains(lowerMessage, "api key is invalid") ||
		strings.Contains(lowerMessage, "invalid api key") ||
		strings.Contains(lowerMessage, "api key provided is invalid") ||
		strings.Contains(lowerMessage, "unauthorized")
}

func parseErrorResponse(body []byte) (string, any, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "", nil, io.EOF
	}

	var er errorResponse
	if err := sonic.Unmarshal(body, &er); err == nil {
		if nestedMessage, nestedCode := parseNestedError(er.Error); nestedMessage != "" {
			if er.Message == "" {
				er.Message = nestedMessage
			}

			if er.Code == nil {
				er.Code = nestedCode
			}
		}

		if er.Message != "" {
			return er.Message, normalizeErrorCode(er.Code), nil
		}
	}

	var stringMessage string
	if err := sonic.Unmarshal(body, &stringMessage); err == nil && stringMessage != "" {
		return stringMessage, nil, nil
	}

	return string(body), nil, nil
}

func parseNestedError(raw json.RawMessage) (string, any) {
	if len(raw) == 0 {
		return "", nil
	}

	var message string
	if err := sonic.Unmarshal([]byte(raw), &message); err == nil {
		return message, nil
	}

	var nested struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
		Error   string `json:"error"`
	}

	if err := sonic.Unmarshal([]byte(raw), &nested); err == nil {
		if nested.Message != "" {
			return nested.Message, nested.Code
		}

		if nested.Error != "" {
			return nested.Error, nested.Code
		}
	}

	return "", nil
}

func normalizeErrorCode(code any) any {
	switch value := code.(type) {
	case nil:
		return nil
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return value
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	default:
		return value
	}
}
