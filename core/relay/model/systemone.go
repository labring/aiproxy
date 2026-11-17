package model

// SystemOneRequest is the native TypeSafe SystemOne request. The state is a
// string, object or array and instructions and criteria are unions, so relay
// conversion keeps their original JSON rather than coercing them.
type SystemOneRequest struct {
	Model     string                       `json:"model,omitempty"`
	State     any                          `json:"state"           binding:"required"`
	Questions map[string]SystemOneQuestion `json:"questions"       binding:"required,min=1"`
}

// SystemOneQuestion describes one noul, choice, score or box question. Noul
// criteria map true/false to descriptions, choice criteria map option names to
// descriptions, score criteria are an ordered rubric and box has no criteria.
type SystemOneQuestion struct {
	Type         string `json:"type"               binding:"required"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// SystemOneResponse contains the answer to each named question. These types
// document the API; responses are forwarded as raw JSON.
type SystemOneResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]SystemOneAnswer `json:"answers"`
	Usage   SystemOneUsage             `json:"usage"`
}

type SystemOneAnswer struct {
	Type          string             `json:"type"                    enums:"noul,choice,score,box"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        any                `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Box           any                `json:"box,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

// SystemOneUsage bills input tokens only; output tokens are currently free.
type SystemOneUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}
