//nolint:testpackage
package gemini

import "testing"

func TestBlockedFinishReasonToClaude(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"SAFETY", "refusal"},
		{"RECITATION", "refusal"},
		{"BLOCKLIST", "refusal"},
		{"PROHIBITED_CONTENT", "refusal"},
		{"SPII", "refusal"},
		{"IMAGE_SAFETY", "refusal"},
		{"IMAGE_PROHIBITED_CONTENT", "refusal"},
		{"IMAGE_RECITATION", "refusal"},
		{"STOP", "end_turn"},
		{"MAX_TOKENS", "max_tokens"},
		{"TOOL_CALLS", "tool_use"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			if got := geminiFinishReason2Claude(tt.input); got != tt.want {
				t.Errorf("stop reason = %q, want %q", got, tt.want)
			}
		})
	}
}
