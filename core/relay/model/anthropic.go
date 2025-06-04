package model

import "github.com/labring/aiproxy/core/relay/adaptor"

type AnthropicMessageRequest struct {
	Model    string     `json:"model,omitempty"`
	Messages []*Message `json:"messages,omitempty"`
}

type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type AnthropicErrorResponse struct {
	Type  string         `json:"type"`
	Error AnthropicError `json:"error"`
}

func NewAnthropicError(statusCode int, err AnthropicError) adaptor.Error {
	return adaptor.NewError(statusCode, AnthropicErrorResponse{
		Type:  "error",
		Error: err,
	})
}

func WrapperAnthropicError(err error, typ string, statusCode int) adaptor.Error {
	return WrapperAnthropicErrorWithMessage(err.Error(), typ, statusCode)
}

func WrapperAnthropicErrorWithMessage(message, typ string, statusCode int) adaptor.Error {
	return NewAnthropicError(statusCode, AnthropicError{
		Type:    typ,
		Message: message,
	})
}
