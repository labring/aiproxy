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
	return extractUsageFromBytes(tail, false)
}

// extractUsageFromHead scans data for the first "usage" JSON object.
// Used to capture input_tokens from Anthropic SSE message_start events,
// which appear at the beginning of a streaming response.
func extractUsageFromHead(head []byte) model.Usage {
	return extractUsageFromBytes(head, true)
}

// extractUsageFromBytes locates a "usage" JSON object in data.
// firstOccurrence=true returns the first match; false returns the last.
func extractUsageFromBytes(data []byte, firstOccurrence bool) model.Usage {
	usageKey := []byte(`"usage"`)

	var idx int
	if firstOccurrence {
		idx = bytes.Index(data, usageKey)
	} else {
		idx = bytes.LastIndex(data, usageKey)
	}

	if idx < 0 {
		return model.Usage{}
	}

	after := data[idx+len(usageKey):]

	// Advance past the colon.
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

	var raw rawUsage
	if err := sonic.Unmarshal(after[:end+1], &raw); err != nil {
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

	// Tavily format: per-request billing credits (e.g. basic search=1, advanced=2)
	Credits model.ZeroNullInt64 `json:"credits,omitempty"`
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

// cacheCreationTotal returns the total cache-creation token count,
// preferring the flat Anthropic field over the nested object sum.
func (r *rawUsage) cacheCreationTotal() model.ZeroNullInt64 {
	if r.CacheCreationInputTokens > 0 {
		return r.CacheCreationInputTokens
	}
	return model.ZeroNullInt64(r.CacheCreation.total())
}

func (r *rawUsage) toModelUsage() model.Usage {
	u := model.Usage{}

	// Input tokens: prefer OpenAI field names over Anthropic/Responses names.
	if r.PromptTokens > 0 {
		u.InputTokens = r.PromptTokens
	} else if r.InputTokens > 0 {
		// Anthropic's input_tokens excludes cached tokens, but model.Usage.InputTokens
		// must represent the total input (matching OpenAI's prompt_tokens semantics).
		// Add cache_read and cache_creation tokens to align the semantics.
		// See ClaudeUsage.ToOpenAIUsage() in relay/model/claude.go for the same logic.
		u.InputTokens = r.InputTokens + r.CacheReadInputTokens + r.cacheCreationTotal()
	}

	// Output tokens.
	if r.CompletionTokens > 0 {
		u.OutputTokens = r.CompletionTokens
	} else if r.OutputTokens > 0 {
		u.OutputTokens = r.OutputTokens
	}

	// Total tokens: use OpenAI's total_tokens if present, otherwise compute from input + output.
	if r.TotalTokens > 0 {
		u.TotalTokens = r.TotalTokens
	} else {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}

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
	// Skip if already set from OpenAI prompt_tokens_details above.
	if u.CacheCreationTokens == 0 {
		u.CacheCreationTokens = r.cacheCreationTotal()
	}

	// Tavily credits → WebSearchCount for per-request billing.
	u.WebSearchCount = r.Credits

	return u
}
