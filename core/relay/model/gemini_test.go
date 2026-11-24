//nolint:testpackage
package model

import "testing"

func TestGeminiChatResponseWebSearchCountUsesGroundedPromptBeforeGemini3(t *testing.T) {
	t.Parallel()

	response := GeminiChatResponse{
		ModelVersion: "gemini-2.5-flash",
		Candidates: []*GeminiChatCandidate{
			{
				GroundingMetadata: &GeminiGroundingMetadata{
					WebSearchQueries: []string{"query one", "query two"},
				},
			},
		},
	}

	if got := response.GetWebSearchCount(); got != 1 {
		t.Fatalf("expected one grounded prompt, got %d", got)
	}
}

func TestGeminiChatResponseWebSearchCountUsesSearchQueriesForGemini3(t *testing.T) {
	t.Parallel()

	response := GeminiChatResponse{
		ModelVersion: "gemini-3-pro-preview",
		Candidates: []*GeminiChatCandidate{
			{
				GroundingMetadata: &GeminiGroundingMetadata{
					WebSearchQueries: []string{"query one", "query two", "query one"},
				},
			},
		},
	}

	if got := response.GetWebSearchCount(); got != 2 {
		t.Fatalf("expected two search queries, got %d", got)
	}
}

func TestGeminiUsageMetadataIncludesToolUsePromptTokens(t *testing.T) {
	t.Parallel()

	usage := GeminiUsageMetadata{
		PromptTokenCount:        100,
		ToolUsePromptTokenCount: 40,
		CandidatesTokenCount:    10,
		ThoughtsTokenCount:      5,
		TotalTokenCount:         155,
	}

	chatUsage := usage.ToUsage()
	if chatUsage.PromptTokens != 140 {
		t.Errorf("ToUsage PromptTokens = %d, want 140", chatUsage.PromptTokens)
	}

	// The API defines the total as prompt + tool-use prompt + thoughts + candidates.
	if got := chatUsage.PromptTokens + chatUsage.CompletionTokens; got != chatUsage.TotalTokens {
		t.Errorf("prompt + completion tokens = %d, want total %d", got, chatUsage.TotalTokens)
	}

	if got := usage.GetInputTokens(); got != 140 {
		t.Errorf("GetInputTokens = %d, want 140", got)
	}

	responseUsage := usage.ToResponseUsage()
	if responseUsage.InputTokens != 140 {
		t.Errorf("ToResponseUsage InputTokens = %d, want 140", responseUsage.InputTokens)
	}

	claudeUsage := usage.ToClaudeUsage()
	if claudeUsage.InputTokens != 140 {
		t.Errorf("ToClaudeUsage InputTokens = %d, want 140", claudeUsage.InputTokens)
	}

	modelUsage := usage.ToModelUsage()
	if int64(modelUsage.InputTokens) != 140 {
		t.Errorf("ToModelUsage InputTokens = %d, want 140", modelUsage.InputTokens)
	}
}
