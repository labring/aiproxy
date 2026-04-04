package passthrough

import (
	"bytes"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
)

// extractUsageFromTail scans the tail bytes for the last "usage" JSON object
// and returns the parsed model.Usage.
//
// Three upstream usage formats are handled:
//
//	OpenAI SSE:      "usage":{"prompt_tokens":N,"completion_tokens":N,...}
//	Anthropic:       "usage":{"input_tokens":N,"output_tokens":N,...}
//	Responses API:   "response":{"usage":{"input_tokens":N,"output_tokens":N,...}}
//
// The function performs a backward scan for the last occurrence of "usage" so
// that intermediate usage chunks (e.g. message_start in Anthropic streaming) do
// not shadow the final, complete usage figure.
func extractUsageFromTail(tail []byte) model.Usage {
	usageKey := []byte(`"usage"`)

	// Find the last occurrence of "usage" in the tail.
	idx := bytes.LastIndex(tail, usageKey)
	if idx < 0 {
		return model.Usage{}
	}

	// Advance past the key to the colon.
	after := tail[idx+len(usageKey):]
	colon := bytes.IndexByte(after, ':')
	if colon < 0 {
		return model.Usage{}
	}

	after = after[colon+1:]

	// Skip whitespace.
	for len(after) > 0 && (after[0] == ' ' || after[0] == '\t' || after[0] == '\n' || after[0] == '\r') {
		after = after[1:]
	}

	if len(after) == 0 || after[0] != '{' {
		return model.Usage{}
	}

	// Find the matching closing brace.
	depth := 0
	end := -1

	for i, b := range after {
		switch b {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}

		if end >= 0 {
			break
		}
	}

	if end < 0 {
		return model.Usage{}
	}

	usageJSON := after[:end+1]

	var raw rawUsage
	if err := sonic.Unmarshal(usageJSON, &raw); err != nil {
		return model.Usage{}
	}

	return raw.toModelUsage()
}

// rawUsage covers the union of all usage field names returned by PPIO/Novita
// across the OpenAI, Anthropic, and Responses API protocols.
type rawUsage struct {
	// OpenAI format
	PromptTokens     model.ZeroNullInt64 `json:"prompt_tokens,omitempty"`
	CompletionTokens model.ZeroNullInt64 `json:"completion_tokens,omitempty"`
	TotalTokens      model.ZeroNullInt64 `json:"total_tokens,omitempty"`

	// OpenAI: reasoning model breakdown
	CompletionTokensDetails *completionTokensDetails `json:"completion_tokens_details,omitempty"`

	// OpenAI: prompt cache breakdown
	PromptTokensDetails *promptTokensDetails `json:"prompt_tokens_details,omitempty"`

	// Anthropic / Responses API format
	InputTokens  model.ZeroNullInt64 `json:"input_tokens,omitempty"`
	OutputTokens model.ZeroNullInt64 `json:"output_tokens,omitempty"`

	// Anthropic: prompt cache (flat top-level fields)
	CacheReadInputTokens     model.ZeroNullInt64 `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens model.ZeroNullInt64 `json:"cache_creation_input_tokens,omitempty"`

	// Anthropic: prompt cache creation breakdown (nested object form)
	// e.g. "cache_creation": {"ephemeral_5m_input_tokens": 0, "ephemeral_1h_input_tokens": 0}
	CacheCreation *cacheCreation `json:"cache_creation,omitempty"`
}

type completionTokensDetails struct {
	ReasoningTokens model.ZeroNullInt64 `json:"reasoning_tokens,omitempty"`
}

type promptTokensDetails struct {
	CachedTokens             model.ZeroNullInt64 `json:"cached_tokens,omitempty"`
	CacheCreationInputTokens model.ZeroNullInt64 `json:"cache_creation_input_tokens,omitempty"`
}

// cacheCreation handles Anthropic's nested cache_creation object.
// The sum of all sub-fields is treated as the total cache-creation token count.
type cacheCreation struct {
	Ephemeral5mInputTokens model.ZeroNullInt64 `json:"ephemeral_5m_input_tokens,omitempty"`
	Ephemeral1hInputTokens model.ZeroNullInt64 `json:"ephemeral_1h_input_tokens,omitempty"`
}

func (cc *cacheCreation) total() int64 {
	if cc == nil {
		return 0
	}

	return int64(cc.Ephemeral5mInputTokens) + int64(cc.Ephemeral1hInputTokens)
}

func (r *rawUsage) toModelUsage() model.Usage {
	u := model.Usage{}

	// Input tokens: prefer OpenAI field names over Anthropic/Responses names.
	if r.PromptTokens > 0 {
		u.InputTokens = r.PromptTokens
	} else if r.InputTokens > 0 {
		u.InputTokens = r.InputTokens
	}

	// Output tokens.
	if r.CompletionTokens > 0 {
		u.OutputTokens = r.CompletionTokens
	} else if r.OutputTokens > 0 {
		u.OutputTokens = r.OutputTokens
	}

	// Total tokens (OpenAI only; Anthropic/Responses API don't return it).
	u.TotalTokens = r.TotalTokens

	// Reasoning tokens (OpenAI format: completion_tokens_details.reasoning_tokens).
	if r.CompletionTokensDetails != nil {
		u.ReasoningTokens = r.CompletionTokensDetails.ReasoningTokens
	}

	// Cached tokens (OpenAI format: prompt_tokens_details.cached_tokens).
	if r.PromptTokensDetails != nil {
		u.CachedTokens = r.PromptTokensDetails.CachedTokens

		// Cache-creation tokens in OpenAI format are top-level inside prompt_tokens_details.
		if r.PromptTokensDetails.CacheCreationInputTokens > 0 {
			u.CacheCreationTokens = r.PromptTokensDetails.CacheCreationInputTokens
		}
	}

	// Cached tokens (Anthropic flat format: cache_read_input_tokens).
	if r.CacheReadInputTokens > 0 {
		u.CachedTokens = r.CacheReadInputTokens
	}

	// Cache-creation tokens: prefer the flat Anthropic top-level field, then
	// fall back to the nested cacheCreation object (sum of ephemeral tiers).
	if r.CacheCreationInputTokens > 0 {
		u.CacheCreationTokens = r.CacheCreationInputTokens
	} else if total := r.CacheCreation.total(); total > 0 {
		u.CacheCreationTokens = model.ZeroNullInt64(total)
	}

	return u
}
