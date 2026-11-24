package model_test

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/model"
)

// Both request types must agree with ConvertChatCompletionToResponsesRequest on
// which field wins when a client sends both caps.
func TestOutputTokenLimit(t *testing.T) {
	tests := []struct {
		name                string
		maxTokens           int
		maxCompletionTokens int
		want                int
	}{
		{name: "unset"},
		{name: "max_tokens only", maxTokens: 100, want: 100},
		{name: "max_completion_tokens only", maxCompletionTokens: 200, want: 200},
		{
			name:                "max_tokens wins when both are set",
			maxTokens:           100,
			maxCompletionTokens: 200,
			want:                100,
		},
		{
			name:                "negative max_tokens falls through",
			maxTokens:           -1,
			maxCompletionTokens: 200,
			want:                200,
		},
		{name: "negative max_completion_tokens alone is unset", maxCompletionTokens: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			general := model.GeneralOpenAIRequest{
				MaxTokens:           tc.maxTokens,
				MaxCompletionTokens: tc.maxCompletionTokens,
			}
			if got := general.OutputTokenLimit(); got != tc.want {
				t.Errorf("GeneralOpenAIRequest.OutputTokenLimit() = %d, want %d", got, tc.want)
			}

			claude := model.ClaudeOpenAIRequest{
				MaxTokens:           tc.maxTokens,
				MaxCompletionTokens: tc.maxCompletionTokens,
			}
			if got := claude.OutputTokenLimit(); got != tc.want {
				t.Errorf("ClaudeOpenAIRequest.OutputTokenLimit() = %d, want %d", got, tc.want)
			}
		})
	}
}
