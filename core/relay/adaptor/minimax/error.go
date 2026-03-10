package minimax

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type BaseResp struct {
	StatusMsg  string `json:"status_msg"`
	StatusCode int    `json:"status_code"`
}

type ErrorResponse struct {
	BaseResp *BaseResp `json:"base_resp"`
}

func TryErrorHanlder(resp *http.Response) adaptor.Error {
	respBody, err := common.GetResponseBody(resp)
	_ = resp.Body.Close()
	if err != nil {
		return relaymodel.NewOpenAIError(resp.StatusCode, relaymodel.OpenAIError{
			Message: err.Error(),
			Type:    relaymodel.ErrorTypeUpstream,
			Code:    relaymodel.ErrorCodeBadResponse,
		})
	}

	var result ErrorResponse
	if err := sonic.Unmarshal(respBody, &result); err != nil {
		return relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if result.BaseResp != nil && result.BaseResp.StatusCode != 0 {
		statusCode := http.StatusInternalServerError
		switch result.BaseResp.StatusCode {
		case 1008:
			statusCode = http.StatusPaymentRequired
		case 1001:
			statusCode = http.StatusRequestTimeout
		case 1004, 2049:
			statusCode = http.StatusForbidden
		case 1026, 1027, 2013, 1042:
			statusCode = http.StatusBadRequest
		case 1002, 1039, 1041, 2056:
			statusCode = http.StatusTooManyRequests
		case 1000, 1013, 1024, 1033:
			statusCode = http.StatusInternalServerError
		}

		return relaymodel.WrapperOpenAIErrorWithMessage(
			result.BaseResp.StatusMsg,
			strconv.Itoa(result.BaseResp.StatusCode),
			statusCode,
			relaymodel.ErrorTypeUpstream,
		)
	}

	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	return nil
}
