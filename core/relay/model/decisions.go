package model

// DecisionRequest is the native OpenAI Decisions request. Input and instructions
// are unions (text or ordered content items), so relay conversion keeps their
// original JSON rather than coercing them into chat messages.
type DecisionRequest struct {
	Model              string                      `json:"model"                          binding:"required"`
	Input              any                         `json:"input"                          binding:"required"` // Text or an ordered array of messages, function/custom tool calls and outputs.
	Questions          []DecisionQuestion          `json:"questions"                      binding:"required,min=1,max=400"`
	PromptCacheKey     *string                     `json:"prompt_cache_key,omitempty"`
	PromptCacheOptions *DecisionPromptCacheOptions `json:"prompt_cache_options,omitempty"`
	SafetyIdentifier   *string                     `json:"safety_identifier,omitempty"`
}

// DecisionQuestion describes one predicate, choice, or ordered score question.
// Choice values must remain strings or booleans; false must not become "false".
type DecisionQuestion struct {
	Type         string               `json:"type"              enums:"predicate,choice,score" binding:"required"`
	Instructions any                  `json:"instructions"                                     binding:"required"`
	Name         *string              `json:"name,omitempty"`
	Choices      []DecisionChoice     `json:"choices,omitempty"                                                   validate:"omitempty,min=2,max=255"`
	Levels       []DecisionScoreLevel `json:"levels,omitempty"                                                    validate:"omitempty,min=2,max=10"`
}

type DecisionChoice struct {
	Value       any    `json:"value"                 binding:"required"`
	Description string `json:"description,omitempty"`
}

type DecisionScoreLevel struct {
	Label       string `json:"label"                 binding:"required"`
	Description string `json:"description,omitempty"`
}

type DecisionPromptCacheOptions struct {
	Mode *string `json:"mode,omitempty" enums:"implicit,explicit"`
	TTL  string  `json:"ttl,omitempty"  enums:"30m"`
}

// DecisionResponse contains answers in question order, including per-question
// refusals. These types document the API; responses are forwarded as raw JSON.
type DecisionResponse struct {
	Model   string           `json:"model"`
	Answers []DecisionAnswer `json:"answers"`
	Usage   DecisionUsage    `json:"usage"`
}

type DecisionAnswer struct {
	Type          string                `json:"type"                    enums:"predicate,choice,score,refusal"`
	Name          *string               `json:"name"`
	Probability   *float64              `json:"probability,omitempty"`
	Choice        any                   `json:"choice,omitempty"`
	Score         *float64              `json:"score,omitempty"`
	Confidence    *float64              `json:"confidence,omitempty"`
	Probabilities []DecisionProbability `json:"probabilities,omitempty"`
}

type DecisionProbability struct {
	Value       any     `json:"value"`
	Probability float64 `json:"probability"`
	Label       *string `json:"label,omitempty"`
}

type DecisionUsage struct {
	InputTokens         int64                       `json:"input_tokens"`
	OutputTokens        int64                       `json:"output_tokens"`
	TotalTokens         int64                       `json:"total_tokens"`
	InputTokensDetails  DecisionInputTokensDetails  `json:"input_tokens_details"`
	OutputTokensDetails DecisionOutputTokensDetails `json:"output_tokens_details"`
}

type DecisionInputTokensDetails struct {
	CachedTokens     int64 `json:"cached_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
}

type DecisionOutputTokensDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens"`
}
