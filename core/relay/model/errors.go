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

func WrapperError(m mode.Mode, statusCode int, err error, typ ...string) adaptor.Error {
	return WrapperErrorWithMessage(m, statusCode, err.Error(), typ...)
}

func WrapperErrorWithMessage(
	m mode.Mode,
	statusCode int,
	message string,
	typ ...string,
) adaptor.Error {
	respType := ErrorTypeAIPROXY
	if len(typ) > 0 {
		respType = typ[0]
	}
	switch m {
	case mode.Anthropic:
		return NewAnthropicError(statusCode, AnthropicError{
			Message: message,
			Type:    respType,
		})
	case mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent:
		return NewOpenAIVideoError(statusCode, OpenAIVideoError{
			Detail: message,
		})
	default:
		return NewOpenAIError(statusCode, OpenAIError{
			Message: message,
			Type:    respType,
		})
	}
}
