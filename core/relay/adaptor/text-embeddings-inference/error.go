package textembeddingsinference

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type RerankErrorResponse struct {
	Error     string `json:"error"`
	ErrorType string `json:"error_type"`
}

func RerankErrorHanlder(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	errResp := RerankErrorResponse{}
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		return relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	return relaymodel.WrapperOpenAIErrorWithMessage(
		errResp.Error,
		errResp.ErrorType,
		resp.StatusCode,
	)
}

type EmbeddingsErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func EmbeddingsErrorHanlder(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	errResp := EmbeddingsErrorResponse{}
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		return relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	return relaymodel.WrapperOpenAIErrorWithMessage(errResp.Message, errResp.Type, resp.StatusCode)
}
