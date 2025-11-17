package adaptor

import (
	"fmt"

	"github.com/bytedance/sonic"
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
