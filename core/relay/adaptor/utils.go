package adaptor

import (
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

type BasicError[T any] struct {
	error      T
	statusCode int
}

func (e BasicError[T]) MarshalJSON() ([]byte, error) {
	return sonic.Marshal(e.error)
}

func (e BasicError[T]) StatusCode() int {
	return e.statusCode
}

func (e BasicError[T]) Error() string {
	return fmt.Sprintf("status code: %d, error: %v", e.statusCode, e.error)
}

func NewError[T any](statusCode int, err T) Error {
	return BasicError[T]{
		error:      err,
		statusCode: statusCode,
	}
}

func IsSuccessfulResponseStatus(m mode.Mode, statusCode int) bool {
	switch m {
	case mode.Responses:
		return statusCode == http.StatusOK || statusCode == http.StatusCreated
	case mode.ResponsesDelete:
		return statusCode == http.StatusOK || statusCode == http.StatusNoContent
	default:
		return statusCode == http.StatusOK
	}
}

func ModeFromMeta(meta *meta.Meta) mode.Mode {
	if meta == nil {
		return mode.Unknown
	}

	return meta.Mode
}
