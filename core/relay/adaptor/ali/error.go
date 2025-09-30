package ali

import (
	"net/http"
	"strings"

	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// https://help.aliyun.com/zh/model-studio/error-code?userCode=okjhlpr5

func ErrorHanlder(resp *http.Response) adaptor.Error {
	statusCode, openAIError := openai.GetError(resp)

	// {"error":{"code":"ServiceUnavailable","message":"<503> InternalError.Algo: An error occurred in model serving, error message is: [Too many requests. Your requests are being throttled due to system capacity limits. Please try again later.]","type":"ServiceUnavailable"}}
	switch openAIError.Type {
	case "ServiceUnavailable":
		statusCode = http.StatusServiceUnavailable
		openAIError.Type = relaymodel.ErrorTypeUpstream
	case "RequestTimeOut":
		statusCode = http.StatusRequestTimeout
		openAIError.Type = relaymodel.ErrorTypeUpstream
	}

	if strings.Contains(openAIError.Message, "object is not iterable") {
		statusCode = http.StatusBadRequest
	}

	return relaymodel.NewOpenAIError(statusCode, openAIError)
}
