package openai

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func GetError(resp *http.Response) (int, relaymodel.OpenAIError) {
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, relaymodel.OpenAIError{
			Message: err.Error(),
			Type:    relaymodel.ErrorTypeUpstream,
			Code:    relaymodel.ErrorCodeBadResponse,
		}
	}

	return GetErrorWithBody(resp.StatusCode, respBody)
}

func GetErrorWithBody(statusCode int, respBody []byte) (int, relaymodel.OpenAIError) {
	openAIError := relaymodel.OpenAIError{
		Type:  relaymodel.ErrorTypeUpstream,
		Code:  relaymodel.ErrorCodeBadResponse,
		Param: strconv.Itoa(statusCode),
	}

	var errResponse relaymodel.OpenAIErrorResponse
	err := sonic.Unmarshal(respBody, &errResponse)
	if err != nil {
		openAIError.Message = conv.BytesToString(respBody)
		return statusCode, openAIError
	}

	if errResponse.Error.Message != "" {
		// OpenAI format error, so we override the default one
		openAIError = errResponse.Error
	}

	if openAIError.Message == "" {
		openAIError.Message = fmt.Sprintf("bad response status code %d", statusCode)
	}

	if code, ok := openAIError.Code.(int64); ok && code >= 400 && code < 600 {
		statusCode = int(code)
	}

	if strings.HasPrefix(openAIError.Message, "tools is not supported in this model.") {
		statusCode = http.StatusBadRequest
	}

	return statusCode, openAIError
}

func ErrorHanlder(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return relaymodel.NewOpenAIError(resp.StatusCode, relaymodel.OpenAIError{
			Message: err.Error(),
			Type:    relaymodel.ErrorTypeUpstream,
			Code:    relaymodel.ErrorCodeBadResponse,
		})
	}

	return ErrorHanlderWithBody(resp.StatusCode, respBody)
}

func ErrorHanlderWithBody(statusCode int, respBody []byte) adaptor.Error {
	statusCode, openAIError := GetErrorWithBody(statusCode, respBody)
	return relaymodel.NewOpenAIError(statusCode, openAIError)
}

func VideoErrorHanlder(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return relaymodel.NewOpenAIVideoError(resp.StatusCode, relaymodel.OpenAIVideoError{
			Detail: err.Error(),
		})
	}

	return VideoErrorHanlderWithBody(resp.StatusCode, respBody)
}

func VideoErrorHanlderWithBody(statusCode int, respBody []byte) adaptor.Error {
	statusCode, openAIError := GetVideoErrorWithBody(statusCode, respBody)
	return relaymodel.NewOpenAIVideoError(statusCode, openAIError)
}

func GetVideoErrorWithBody(statusCode int, respBody []byte) (int, relaymodel.OpenAIVideoError) {
	openAIError := relaymodel.OpenAIVideoError{}
	err := sonic.Unmarshal(respBody, &openAIError)
	if err != nil {
		openAIError.Detail = string(respBody)
		return statusCode, openAIError
	}

	return statusCode, openAIError
}
