package cloudflare

import (
	"fmt"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.cloudflare.com"

func (a *Adaptor) GetBaseURL() string {
	return baseURL
}

// WorkerAI cannot be used across accounts with AIGateWay
// https://developers.cloudflare.com/ai-gateway/providers/workersai/#openai-compatible-endpoints
// https://gateway.ai.cloudflare.com/v1/{account_id}/{gateway_id}/workers-ai
func isAIGateWay(baseURL string) bool {
	return strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") &&
		strings.HasSuffix(baseURL, "/workers-ai")
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
	u := meta.Channel.BaseURL
	isAIGateWay := isAIGateWay(u)
	var urlPrefix string
	if isAIGateWay {
		urlPrefix = u
	} else {
		urlPrefix = fmt.Sprintf("%s/client/v4/accounts/%s/ai", u, meta.Channel.Key)
	}

	switch meta.Mode {
	case mode.ChatCompletions:
		return urlPrefix + "/v1/chat/completions", nil
	case mode.Embeddings:
		return urlPrefix + "/v1/embeddings", nil
	default:
		if isAIGateWay {
			return fmt.Sprintf("%s/%s", urlPrefix, meta.ActualModel), nil
		}
		return fmt.Sprintf("%s/run/%s", urlPrefix, meta.ActualModel), nil
	}
}

func (a *Adaptor) GetModelList() []model.ModelConfig {
	return ModelList
}
