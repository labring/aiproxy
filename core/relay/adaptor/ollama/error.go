package ollama

import (
	"net/http"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type errorResponse struct {
	Error string `json:"error"`
}

// responseError converts an error reported inside a 200 response body.
func responseError(message string) adaptor.Error {
	return relaymodel.WrapperOpenAIErrorWithMessage(
		message,
		relaymodel.ErrorCodeBadResponse,
		http.StatusBadGateway,
		relaymodel.ErrorTypeUpstream,
	)
}

func ErrorHandler(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	var er errorResponse

	err := common.UnmarshalResponse(resp, &er)
	if err != nil {
		return relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	return relaymodel.WrapperOpenAIErrorWithMessage(er.Error, nil, resp.StatusCode)
}
