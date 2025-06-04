package openai

import (
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/model"
)

func ResponseText2Usage(responseText, modeName string, promptTokens int64) model.Usage {
	usage := model.Usage{
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
