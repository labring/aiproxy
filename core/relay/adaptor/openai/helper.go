package openai

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/model"
)

// ResponsesURL builds a RequestURL for any Responses API sub-mode.
// Used by OpenAI, PPIO, and Novita adaptors.
func ResponsesURL(base string, m mode.Mode, responseID string) (adaptor.RequestURL, error) {
	switch m {
	case mode.Responses:
		u, err := url.JoinPath(base, "/responses")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodPost, URL: u}, nil

	case mode.ResponsesGet:
		u, err := url.JoinPath(base, "/responses", responseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodGet, URL: u}, nil

	case mode.ResponsesDelete:
		u, err := url.JoinPath(base, "/responses", responseID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodDelete, URL: u}, nil

	case mode.ResponsesCancel:
		u, err := url.JoinPath(base, "/responses", responseID, "cancel")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodPost, URL: u}, nil

	case mode.ResponsesInputItems:
		u, err := url.JoinPath(base, "/responses", responseID, "input_items")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{Method: http.MethodGet, URL: u}, nil

	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported responses mode: %s", m)
	}
}

func ResponseText2Usage(responseText, modeName string, promptTokens int64) model.ChatUsage {
	usage := model.ChatUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: CountTokenText(responseText, modeName),
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	return usage
}

func ChatCompletionID() string {
	return "chatcmpl-" + common.ShortUUID()
}

func CallID() string {
	return "call_" + common.ShortUUID()
}
