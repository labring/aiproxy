//nolint:testpackage
package openai

import "testing"

func TestBlockedFinishReasonToClaude(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"content_filter", "refusal"},
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			if got := *convertFinishReasonToClaude(tt.input); got != tt.want {
				t.Errorf("stop reason = %q, want %q", got, tt.want)
			}
		})
	}
}
