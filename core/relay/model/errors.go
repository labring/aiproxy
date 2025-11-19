package model

import (
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/mode"
)

const (
	ErrorTypeAIPROXY     = "aiproxy_error"
	ErrorTypeUpstream    = "upstream_error"
	ErrorCodeBadResponse = "bad_response"
)

func WrapperError(
	m mode.Mode,
	statusCode int,
	err error,
	opts ...WrapperErrorOptionFunc,
) adaptor.Error {
	return WrapperErrorWithMessage(m, statusCode, err.Error(), opts...)
}

type WrapperErrorOption struct {
	Type string
	Code any
}

type WrapperErrorOptionFunc func(o *WrapperErrorOption)

func WithType(typ string) WrapperErrorOptionFunc {
	return func(o *WrapperErrorOption) {
		o.Type = typ
	}
}

func WithCode(code any) WrapperErrorOptionFunc {
	return func(o *WrapperErrorOption) {
		o.Code = code
	}
}

func DefaultWrapperErrorOption() WrapperErrorOption {
	return WrapperErrorOption{
		Type: ErrorTypeAIPROXY,
	}
}

func WrapperErrorWithMessage(
	m mode.Mode,
	statusCode int,
	message string,
	opts ...WrapperErrorOptionFunc,
) adaptor.Error {
	opt := DefaultWrapperErrorOption()
	for _, o := range opts {
		if o == nil {
			continue
		}

		o(&opt)
	}

	switch m {
	case mode.Anthropic:
		return NewAnthropicError(statusCode, AnthropicError{
			Message: message,
			Type:    opt.Type,
		})
	case mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent:
		return NewOpenAIVideoError(statusCode, OpenAIVideoError{
			Detail: message,
		})
	case mode.Gemini:
		return NewGeminiError(statusCode, GeminiError{
			Message: message,
			Status:  opt.Type,
			Code:    statusCode,
		})
	default:
		return NewOpenAIError(statusCode, OpenAIError{
			Message: message,
			Type:    opt.Type,
			Code:    opt.Code,
		})
	}
}
