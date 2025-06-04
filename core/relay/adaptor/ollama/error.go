package ollama

import (
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type errorResponse struct {
	Error string `json:"error"`
}

func ErrorHandler(resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return relaymodel.WrapperOpenAIErrorWithMessage(
			"read response body error: "+err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	var er errorResponse
	err = sonic.Unmarshal(data, &er)
	if err != nil {
		return relaymodel.WrapperOpenAIErrorWithMessage(
			conv.BytesToString(data),
			nil,
			http.StatusInternalServerError,
		)
	}
	return relaymodel.WrapperOpenAIErrorWithMessage(er.Error, nil, resp.StatusCode)
}
