package qianfan

import (
	"net/http"

	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ErrorHandler(resp *http.Response) adaptor.Error {
	statusCode, openAIError := openai.GetError(resp)
	if openAIError.Code == "system_unsafe" ||
		openAIError.Type == "unsafe_request" {
		statusCode = http.StatusBadRequest
	}

	return relaymodel.NewOpenAIError(statusCode, openAIError)
}
