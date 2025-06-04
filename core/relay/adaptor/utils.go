package adaptor

import (
	"errors"
	"fmt"
	"reflect"

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

func ValidateConfigTemplate(template ConfigTemplate) error {
	if template.Name == "" {
		return errors.New("config template is invalid: name is empty")
	}
	if template.Type == "" {
		return fmt.Errorf("config template %s is invalid: type is empty", template.Name)
	}
	if template.Example != nil {
		if err := ValidateConfigTemplateValue(template, template.Example); err != nil {
			return fmt.Errorf("config template %s is invalid: %w", template.Name, err)
		}
	}
	return nil
}

func ValidateConfigTemplateValue(template ConfigTemplate, value any) error {
	if template.Validator == nil {
		return nil
	}
	switch template.Type {
	case ConfigTypeString:
		_, ok := value.(string)
		if !ok {
			return fmt.Errorf("config template %s is invalid: value is not a string", template.Name)
		}
	case ConfigTypeNumber:
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return nil
		default:
			return fmt.Errorf("config template %s is invalid: value is not a number", template.Name)
		}
	case ConfigTypeBool:
		_, ok := value.(bool)
		if !ok {
			return fmt.Errorf("config template %s is invalid: value is not a bool", template.Name)
		}
	case ConfigTypeObject:
		if reflect.TypeOf(value).Kind() != reflect.Map &&
			reflect.TypeOf(value).Kind() != reflect.Struct {
			return fmt.Errorf("config template %s is invalid: value is not a object", template.Name)
		}
	}
	if err := template.Validator(value); err != nil {
		return fmt.Errorf(
			"config template %s(%s) is invalid: %w",
			template.Name,
			template.Name,
			err,
		)
	}
	return nil
}
