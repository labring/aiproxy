package baidu

import (
	"net/http"
	"strconv"

	"github.com/labring/aiproxy/core/relay/adaptor"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/tlmyncueh

func ErrorHandler(baiduError *Error) adaptor.Error {
	switch baiduError.ErrorCode {
	case 13, 14, 100, 110:
		return relaymodel.WrapperOpenAIErrorWithMessage(
			baiduError.ErrorMsg,
			"upstream_"+strconv.Itoa(baiduError.ErrorCode),
			http.StatusUnauthorized,
		)
	case 17, 19, 111:
		return relaymodel.WrapperOpenAIErrorWithMessage(
			baiduError.ErrorMsg,
			"upstream_"+strconv.Itoa(baiduError.ErrorCode),
			http.StatusForbidden,
		)
	case 336001, 336002, 336003,
		336005, 336006, 336007,
		336008, 336103, 336104,
		336106, 336118, 336122,
		336123, 336221, 337006,
		337008, 337009:
		return relaymodel.WrapperOpenAIErrorWithMessage(
			baiduError.ErrorMsg,
			"upstream_"+strconv.Itoa(baiduError.ErrorCode),
			http.StatusBadRequest,
		)
	case 4, 18, 336117, 336501, 336502,
		336503, 336504, 336505,
		336507:
		return relaymodel.WrapperOpenAIErrorWithMessage(
			baiduError.ErrorMsg,
			"upstream_"+strconv.Itoa(baiduError.ErrorCode),
			http.StatusTooManyRequests,
		)
	}
	return relaymodel.WrapperOpenAIErrorWithMessage(
		baiduError.ErrorMsg,
		"upstream_"+strconv.Itoa(baiduError.ErrorCode),
		http.StatusInternalServerError,
	)
}
