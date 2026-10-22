package typesafeai

// SystemOneRequest is the native request format for the Typesafe AI SystemOne API.
type SystemOneRequest struct {
	State     any                 `json:"state"`
	Model     string              `json:"model,omitempty"`
	Questions map[string]Question `json:"questions"`
}

// Question represents a single typed question in the SystemOne request.
// Instructions and Criteria use any to support the API's flexible types:
// - Instructions: string | object | array
// - Criteria: varies by question type (map for choice, array for score, {true/false} for noul)
type Question struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions,omitempty"`
	Criteria     any    `json:"criteria,omitempty"`
}

// SystemOneResponse is the native response format from the Typesafe AI SystemOne API.
type SystemOneResponse struct {
	Model   string            `json:"model,omitempty"`
	Answers map[string]Answer `json:"answers,omitempty"`
	Usage   SystemOneUsage    `json:"usage,omitempty"`
}

// Answer represents the answer to a single question in the SystemOne response.
type Answer struct {
	Type          string             `json:"type,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         float64            `json:"score,omitempty"`
	Noul          float64            `json:"noul,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

// SystemOneUsage represents the token usage in the SystemOne response.
type SystemOneUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}
