package typesafeai

import (
	"errors"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

// SystemOneError represents an error returned by the Typesafe AI SystemOne API.
type SystemOneError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// SystemOneErrorResponse is the full error response structure.
type SystemOneErrorResponse struct {
	Error SystemOneError `json:"error"`
}

// NewSystemOneError creates a new SystemOneError wrapped in SystemOneErrorResponse.
func NewSystemOneError(statusCode int, err SystemOneError) adaptor.Error {
	return adaptor.NewError(statusCode, SystemOneErrorResponse{
		Error: err,
	})
}

// WrapperSystemOneError wraps a generic error into a SystemOneError.
func WrapperSystemOneError(err error, typ string, statusCode int) adaptor.Error {
	return WrapperSystemOneErrorWithMessage(err.Error(), typ, statusCode)
}

// WrapperSystemOneErrorWithMessage wraps a message string into a SystemOneError.
func WrapperSystemOneErrorWithMessage(message, typ string, statusCode int) adaptor.Error {
	return NewSystemOneError(statusCode, SystemOneError{
		Type:    typ,
		Message: message,
	})
}

// ErrorHanlder reads an HTTP response and converts it to a SystemOneError.
func ErrorHanlder(resp *http.Response) adaptor.Error {
	if resp == nil {
		return WrapperSystemOneErrorWithMessage("response is nil", "invalid_response", http.StatusInternalServerError)
	}

	body, err := common.GetResponseBody(resp)
	if err != nil {
		return WrapperSystemOneError(err, "read_response_body_failed", http.StatusInternalServerError)
	}

	var errResp SystemOneErrorResponse
	if err := sonic.Unmarshal(body, &errResp); err != nil {
		return WrapperSystemOneError(errors.New(string(body)), "bad_response", resp.StatusCode)
	}

	return NewSystemOneError(resp.StatusCode, errResp.Error)
}

// IsSuccessfulResponseStatus checks if the response status code indicates success
// for the SystemOne mode.
func IsSuccessfulResponseStatus(statusCode int) bool {
	return statusCode == http.StatusOK
}
