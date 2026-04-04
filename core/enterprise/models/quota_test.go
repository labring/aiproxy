//go:build enterprise

package models

import (
	"testing"
)

func TestParseBlockedModels(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []string
	}{
		{"empty string", "", nil},
		{"valid json", `["claude-opus-4*","gpt-4o"]`, []string{"claude-opus-4*", "gpt-4o"}},
		{"single model", `["gpt-4o"]`, []string{"gpt-4o"}},
		{"invalid json", `not-json`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBlockedModels(tt.raw)
			if len(got) != len(tt.expected) {
				t.Fatalf("parseBlockedModels(%q) = %v, want %v", tt.raw, got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseBlockedModels(%q)[%d] = %q, want %q", tt.raw, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestIsModelBlockedAtTier(t *testing.T) {
	policy := &QuotaPolicy{
		Tier2BlockedModels: `["claude-opus-4*","gpt-4o"]`,
		Tier3BlockedModels: `["claude-opus-4*","gpt-4o","gpt-4o-mini"]`,
	}

	tests := []struct {
		name    string
		tier    int
		model   string
		blocked bool
	}{
		// Tier 1 never blocks
		{"tier1 no block", 1, "claude-opus-4-20250101", false},

		// Tier 2: claude-opus-4* and gpt-4o blocked
		{"tier2 blocks claude-opus-4 glob", 2, "claude-opus-4-20250101", true},
		{"tier2 blocks gpt-4o exact", 2, "gpt-4o", true},
		{"tier2 allows gpt-4o-mini", 2, "gpt-4o-mini", false},
		{"tier2 allows claude-sonnet", 2, "claude-sonnet-4-20250101", false},

		// Tier 3: all three patterns blocked
		{"tier3 blocks claude-opus-4 glob", 3, "claude-opus-4-20250101", true},
		{"tier3 blocks gpt-4o", 3, "gpt-4o", true},
		{"tier3 blocks gpt-4o-mini", 3, "gpt-4o-mini", true},
		{"tier3 allows claude-sonnet", 3, "claude-sonnet-4-20250101", false},

		// Empty policy
		{"empty policy tier2", 2, "gpt-4o", false},
		{"empty policy tier3", 3, "gpt-4o", false},
	}

	emptyPolicy := &QuotaPolicy{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := policy
			if tt.name[:5] == "empty" {
				p = emptyPolicy
			}
			got := p.IsModelBlockedAtTier(tt.tier, tt.model)
			if got != tt.blocked {
				t.Errorf("IsModelBlockedAtTier(%d, %q) = %v, want %v", tt.tier, tt.model, got, tt.blocked)
			}
		})
	}
}
