package aws

import (
	"errors"
	"net/http"
	"strings"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
)

func UnwrapInvokeError(err error) (int, string) {
	smithyErr := &smithy.OperationError{}

	ok := errors.As(err, &smithyErr)
	if !ok {
		return http.StatusInternalServerError, err.Error()
	}

	awshttpErr := &awshttp.ResponseError{}

	ok = errors.As(smithyErr.Unwrap(), &awshttpErr)
	if !ok {
		return http.StatusInternalServerError, err.Error()
	}

	code := awshttpErr.HTTPStatusCode()
	message := awshttpErr.Err.Error()

	if strings.Contains(message, "Operation not allowed") {
		code = http.StatusForbidden
	}

	return code, message
}
