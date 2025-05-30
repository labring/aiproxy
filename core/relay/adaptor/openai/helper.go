package openai

import (
	"fmt"
	"strings"

	"github.com/labring/aiproxy/core/relay/model"
)

func ResponseText2Usage(responseText, modeName string, promptTokens int64) *model.Usage {
	usage := &model.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: CountTokenText(responseText, modeName),
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage
}

func GetFullRequestURL(baseURL, requestURL string) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		fullRequestURL = fmt.Sprintf(
			"%s%s",
			baseURL,
			strings.TrimPrefix(requestURL, "/openai/deployments"),
		)
	}
	return fullRequestURL
}
