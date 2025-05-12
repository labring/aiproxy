package textembeddingsinference

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/model"
)

type RerankErrorResponse struct {
	Error     string `json:"error"`
	ErrorType string `json:"error_type"`
}

func RerankErrorHanlder(resp *http.Response) *model.ErrorWithStatusCode {
	defer resp.Body.Close()

	errResp := RerankErrorResponse{}
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}

	return &model.ErrorWithStatusCode{
		Error: model.Error{
			Message: errResp.Error,
			Type:    errResp.ErrorType,
			Code:    resp.StatusCode,
		},
		StatusCode: resp.StatusCode,
	}
}

type EmbeddingsErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func EmbeddingsErrorHanlder(resp *http.Response) *model.ErrorWithStatusCode {
	defer resp.Body.Close()

	errResp := EmbeddingsErrorResponse{}
	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}

	return &model.ErrorWithStatusCode{
		Error: model.Error{
			Message: errResp.Message,
			Type:    errResp.Type,
			Code:    resp.StatusCode,
		},
		StatusCode: resp.StatusCode,
	}
}
