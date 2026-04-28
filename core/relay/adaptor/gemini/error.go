package gemini

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ErrorHandler(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	respBody, err := common.GetResponseBody(resp)
	if err != nil {
		return relaymodel.NewGeminiError(resp.StatusCode, relaymodel.GeminiError{
			Message: err.Error(),
			Status:  relaymodel.ErrorTypeUpstream,
			Code:    resp.StatusCode,
		})
	}

	return ErrorHandlerWithBody(resp.StatusCode, respBody)
}

func ErrorHandlerWithBody(statusCode int, respBody []byte) adaptor.Error {
	var errResponse relaymodel.GeminiErrorResponse

	err := sonic.Unmarshal(respBody, &errResponse)
	if err != nil {
		// Maybe it's not a JSON response or different format
		return relaymodel.NewGeminiError(statusCode, relaymodel.GeminiError{
			Message: string(respBody),
			Status:  relaymodel.ErrorTypeUpstream,
			Code:    statusCode,
		})
	}

	if errResponse.Error.Message == "" {
		errResponse.Error.Message = string(respBody)
	}

	if errResponse.Error.Code == 0 {
		errResponse.Error.Code = statusCode
	}

	return relaymodel.NewGeminiError(statusCode, errResponse.Error)
}
