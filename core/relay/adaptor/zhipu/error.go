package zhipu

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ErrorHandler(resp *http.Response) adaptor.Error {
	statusCode, openAIError := openai.GetError(resp)
	statusCode, openAIError = normalizeError(statusCode, openAIError)

	return relaymodel.NewOpenAIError(statusCode, openAIError)
}

func normalizeError(
	statusCode int,
	openAIError relaymodel.OpenAIError,
) (int, relaymodel.OpenAIError) {
	if openAIError.Type == "" {
		openAIError.Type = relaymodel.ErrorTypeUpstream
	}

	if mappedStatusCode, ok := statusCodeByBusinessCode(errorCode(openAIError.Code)); ok {
		statusCode = mappedStatusCode
	} else {
		statusCode = normalizeHTTPStatusCode(statusCode)
	}

	if isNoBalanceError(openAIError) {
		statusCode = http.StatusPaymentRequired
	}

	return statusCode, openAIError
}

func statusCodeByBusinessCode(code string) (int, bool) {
	switch code {
	case "500", "1100", "1200", "1230":
		return http.StatusInternalServerError, true
	case "1000", "1001", "1002", "1003", "1004", "1111":
		return http.StatusUnauthorized, true
	case "1110", "1112", "1121", "1220", "1311":
		return http.StatusForbidden, true
	case "1113", "1304", "1308", "1309", "1310":
		return http.StatusPaymentRequired, true
	case "1120", "1234", "1312":
		return http.StatusServiceUnavailable, true
	case "1210", "1212", "1213", "1214", "1215", "1231", "1300", "1301":
		return http.StatusBadRequest, true
	case "1211", "1221", "1222":
		return http.StatusNotFound, true
	case "1261":
		return http.StatusRequestEntityTooLarge, true
	case "1302", "1303", "1305", "1313":
		return http.StatusTooManyRequests, true
	default:
		return 0, false
	}
}

func normalizeHTTPStatusCode(statusCode int) int {
	switch statusCode {
	case 434:
		return http.StatusForbidden
	case 435:
		return http.StatusRequestEntityTooLarge
	default:
		return statusCode
	}
}

func isNoBalanceError(openAIError relaymodel.OpenAIError) bool {
	if errorCode(openAIError.Code) == "1113" {
		return true
	}

	return strings.Contains(openAIError.Message, "余额不足") ||
		strings.Contains(openAIError.Message, "余额已用完") ||
		strings.Contains(openAIError.Message, "无可用资源包") ||
		strings.Contains(openAIError.Message, "请充值")
}

func errorCode(code any) string {
	if code == nil {
		return ""
	}

	return fmt.Sprint(code)
}
