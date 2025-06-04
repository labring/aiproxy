package anthropic

import (
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func OpenAIErrorHandler(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return relaymodel.WrapperOpenAIError(
			err,
			"read_response_failed",
			http.StatusInternalServerError,
		)
	}

	return OpenAIErrorHandlerWithBody(resp.StatusCode, respBody)
}

func OpenAIErrorHandlerWithBody(statusCode int, respBody []byte) adaptor.Error {
	statusCode, e := GetErrorWithBody(statusCode, respBody)
	return relaymodel.WrapperOpenAIErrorWithMessage(e.Message, e.Type, statusCode)
}

func ErrorHandler(resp *http.Response) adaptor.Error {
	statusCode, e := GetError(resp)
	return relaymodel.NewAnthropicError(statusCode, e)
}

func GetError(resp *http.Response) (int, relaymodel.AnthropicError) {
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, relaymodel.AnthropicError{
			Type:    "aiproxy_error",
			Message: err.Error(),
		}
	}

	return GetErrorWithBody(resp.StatusCode, respBody)
}

// status 400 {"type":"error","error":{"type":"invalid_request_error","message":"Your credit balance
// is too low to access the Anthropic API. Please go to Plans & Billing to upgrade or purchase
// credits."}}
// status 529 {Message:Overloaded Type:overloaded_error Param:}
func GetErrorWithBody(statusCode int, respBody []byte) (int, relaymodel.AnthropicError) {
	var e relaymodel.AnthropicErrorResponse
	err := sonic.Unmarshal(respBody, &e)
	if err != nil {
		return statusCode, e.Error
	}
	if strings.Contains(e.Error.Message, "balance is too low") {
		return http.StatusPaymentRequired, e.Error
	}
	if strings.Contains(e.Error.Message, "Overloaded") {
		return http.StatusTooManyRequests, e.Error
	}
	return statusCode, e.Error
}
