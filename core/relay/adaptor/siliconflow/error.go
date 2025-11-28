package siliconflow

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type errorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
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

	statusCode := resp.StatusCode

	if strings.Contains(er.Message, "System is really busy") {
		statusCode = http.StatusTooManyRequests
	}

	return relaymodel.WrapperOpenAIErrorWithMessage(
		er.Message,
		strconv.Itoa(er.Code),
		statusCode,
		relaymodel.ErrorTypeUpstream,
	)
}
