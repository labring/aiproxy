package typesafeai

// SystemOneRequest is the native request format for the Typesafe AI SystemOne API.
type SystemOneRequest struct {
	State     string              `json:"state,omitempty"`
	Model     string              `json:"model,omitempty"`
	Questions map[string]Question `json:"questions"`
}

// Question represents a single typed question in the SystemOne request.
type Question struct {
	Type     string   `json:"type"`
	Question string   `json:"question,omitempty"`
	Choices  []Choice `json:"choices,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
}

// Choice represents a single choice option within a question.
type Choice struct {
	Label string `json:"label"`
	Text  string `json:"text,omitempty"`
}

// SystemOneResponse is the native response format from the Typesafe AI SystemOne API.
type SystemOneResponse struct {
	Answers map[string]Answer `json:"answers,omitempty"`
	Usage   SystemOneUsage    `json:"usage,omitempty"`
}

// Answer represents the answer to a single question in the SystemOne response.
type Answer struct {
	Type          string             `json:"type,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         float64            `json:"score,omitempty"`
	Noul          string             `json:"noul,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

// SystemOneUsage represents the token usage in the SystemOne response.
type SystemOneUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}
