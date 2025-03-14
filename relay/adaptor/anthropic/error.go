package anthropic

import (
	"net/http"
	"strings"

	"github.com/labring/aiproxy/relay/adaptor/openai"
	"github.com/labring/aiproxy/relay/model"
)

// status 400 {"type":"error","error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the Anthropic API. Please go to Plans & Billing to upgrade or purchase credits."}}
func ErrorHandler(resp *http.Response) *model.ErrorWithStatusCode {
	err := openai.ErrorHanlder(resp)
	if strings.Contains(err.Error.Message, "balance is too low") {
		err.StatusCode = http.StatusPaymentRequired
	}
	return err
}
