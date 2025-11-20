package model_test

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
)

func TestChatUsageConversions(t *testing.T) {
	chatUsage := model.ChatUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: &model.PromptTokensDetails{
			CachedTokens:        20,
			CacheCreationTokens: 10,
		},
		CompletionTokensDetails: &model.CompletionTokensDetails{
			ReasoningTokens: 30,
		},
	}

	t.Run("ChatUsage to ResponseUsage", func(t *testing.T) {
		responseUsage := chatUsage.ToResponseUsage()
		assert.Equal(t, int64(100), responseUsage.InputTokens)
		assert.Equal(t, int64(50), responseUsage.OutputTokens)
		assert.Equal(t, int64(150), responseUsage.TotalTokens)
		assert.NotNil(t, responseUsage.InputTokensDetails)
		assert.Equal(t, int64(20), responseUsage.InputTokensDetails.CachedTokens)
		assert.NotNil(t, responseUsage.OutputTokensDetails)
		assert.Equal(t, int64(30), responseUsage.OutputTokensDetails.ReasoningTokens)
	})

	t.Run("ChatUsage to ClaudeUsage", func(t *testing.T) {
		claudeUsage := chatUsage.ToClaudeUsage()
		assert.Equal(t, int64(100), claudeUsage.InputTokens)
		assert.Equal(t, int64(50), claudeUsage.OutputTokens)
		assert.Equal(t, int64(10), claudeUsage.CacheCreationInputTokens)
		assert.Equal(t, int64(20), claudeUsage.CacheReadInputTokens)
	})

	t.Run("ChatUsage to GeminiUsage", func(t *testing.T) {
		geminiUsage := chatUsage.ToGeminiUsage()
		assert.Equal(t, int64(100), geminiUsage.PromptTokenCount)
		assert.Equal(t, int64(50), geminiUsage.CandidatesTokenCount)
		assert.Equal(t, int64(150), geminiUsage.TotalTokenCount)
		assert.Equal(t, int64(20), geminiUsage.CachedContentTokenCount)
		assert.Equal(t, int64(30), geminiUsage.ThoughtsTokenCount)
	})
}

func TestResponseUsageConversions(t *testing.T) {
	responseUsage := model.ResponseUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		InputTokensDetails: &model.ResponseUsageDetails{
			CachedTokens: 20,
		},
		OutputTokensDetails: &model.ResponseUsageDetails{
			ReasoningTokens: 30,
		},
	}

	t.Run("ResponseUsage to ChatUsage", func(t *testing.T) {
		chatUsage := responseUsage.ToChatUsage()
		assert.Equal(t, int64(100), chatUsage.PromptTokens)
		assert.Equal(t, int64(50), chatUsage.CompletionTokens)
		assert.Equal(t, int64(150), chatUsage.TotalTokens)
		assert.NotNil(t, chatUsage.PromptTokensDetails)
		assert.Equal(t, int64(20), chatUsage.PromptTokensDetails.CachedTokens)
		assert.NotNil(t, chatUsage.CompletionTokensDetails)
		assert.Equal(t, int64(30), chatUsage.CompletionTokensDetails.ReasoningTokens)
	})

	t.Run("ResponseUsage to ClaudeUsage", func(t *testing.T) {
		claudeUsage := responseUsage.ToClaudeUsage()
		assert.Equal(t, int64(100), claudeUsage.InputTokens)
		assert.Equal(t, int64(50), claudeUsage.OutputTokens)
		assert.Equal(t, int64(20), claudeUsage.CacheReadInputTokens)
	})

	t.Run("ResponseUsage to GeminiUsage", func(t *testing.T) {
		geminiUsage := responseUsage.ToGeminiUsage()
		assert.Equal(t, int64(100), geminiUsage.PromptTokenCount)
		assert.Equal(t, int64(50), geminiUsage.CandidatesTokenCount)
		assert.Equal(t, int64(150), geminiUsage.TotalTokenCount)
		assert.Equal(t, int64(20), geminiUsage.CachedContentTokenCount)
		assert.Equal(t, int64(30), geminiUsage.ThoughtsTokenCount)
	})
}

func TestClaudeUsageConversions(t *testing.T) {
	claudeUsage := model.ClaudeUsage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 10,
		CacheReadInputTokens:     20,
	}

	t.Run("ClaudeUsage to ChatUsage", func(t *testing.T) {
		chatUsage := claudeUsage.ToOpenAIUsage()
		assert.Equal(t, int64(130), chatUsage.PromptTokens) // 100 + 10 + 20
		assert.Equal(t, int64(50), chatUsage.CompletionTokens)
		assert.Equal(t, int64(180), chatUsage.TotalTokens)
		assert.NotNil(t, chatUsage.PromptTokensDetails)
		assert.Equal(t, int64(20), chatUsage.PromptTokensDetails.CachedTokens)
		assert.Equal(t, int64(10), chatUsage.PromptTokensDetails.CacheCreationTokens)
	})

	t.Run("ClaudeUsage to ResponseUsage", func(t *testing.T) {
		responseUsage := claudeUsage.ToResponseUsage()
		assert.Equal(t, int64(130), responseUsage.InputTokens) // 100 + 10 + 20
		assert.Equal(t, int64(50), responseUsage.OutputTokens)
		assert.Equal(t, int64(180), responseUsage.TotalTokens)
		assert.NotNil(t, responseUsage.InputTokensDetails)
		assert.Equal(t, int64(20), responseUsage.InputTokensDetails.CachedTokens)
	})

	t.Run("ClaudeUsage to GeminiUsage", func(t *testing.T) {
		geminiUsage := claudeUsage.ToGeminiUsage()
		assert.Equal(t, int64(130), geminiUsage.PromptTokenCount) // 100 + 10 + 20
		assert.Equal(t, int64(50), geminiUsage.CandidatesTokenCount)
		assert.Equal(t, int64(180), geminiUsage.TotalTokenCount)
		assert.Equal(t, int64(20), geminiUsage.CachedContentTokenCount)
	})
}

func TestGeminiUsageConversions(t *testing.T) {
	geminiUsage := model.GeminiUsageMetadata{
		PromptTokenCount:        100,
		CandidatesTokenCount:    50,
		TotalTokenCount:         150,
		ThoughtsTokenCount:      30,
		CachedContentTokenCount: 20,
	}

	t.Run("GeminiUsage to ChatUsage", func(t *testing.T) {
		chatUsage := geminiUsage.ToUsage()
		assert.Equal(t, int64(100), chatUsage.PromptTokens)
		assert.Equal(t, int64(80), chatUsage.CompletionTokens) // 50 + 30
		assert.Equal(t, int64(150), chatUsage.TotalTokens)
		assert.NotNil(t, chatUsage.PromptTokensDetails)
		assert.Equal(t, int64(20), chatUsage.PromptTokensDetails.CachedTokens)
		assert.NotNil(t, chatUsage.CompletionTokensDetails)
		assert.Equal(t, int64(30), chatUsage.CompletionTokensDetails.ReasoningTokens)
	})

	t.Run("GeminiUsage to ResponseUsage", func(t *testing.T) {
		responseUsage := geminiUsage.ToResponseUsage()
		assert.Equal(t, int64(100), responseUsage.InputTokens)
		assert.Equal(t, int64(50), responseUsage.OutputTokens)
		assert.Equal(t, int64(150), responseUsage.TotalTokens)
		assert.NotNil(t, responseUsage.InputTokensDetails)
		assert.Equal(t, int64(20), responseUsage.InputTokensDetails.CachedTokens)
		assert.NotNil(t, responseUsage.OutputTokensDetails)
		assert.Equal(t, int64(30), responseUsage.OutputTokensDetails.ReasoningTokens)
	})

	t.Run("GeminiUsage to ClaudeUsage", func(t *testing.T) {
		claudeUsage := geminiUsage.ToClaudeUsage()
		assert.Equal(t, int64(100), claudeUsage.InputTokens)
		assert.Equal(t, int64(50), claudeUsage.OutputTokens)
		assert.Equal(t, int64(20), claudeUsage.CacheReadInputTokens)
	})
}

func TestRoundTripConversions(t *testing.T) {
	t.Run("ChatUsage -> ResponseUsage -> ChatUsage", func(t *testing.T) {
		original := model.ChatUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			PromptTokensDetails: &model.PromptTokensDetails{
				CachedTokens: 20,
			},
			CompletionTokensDetails: &model.CompletionTokensDetails{
				ReasoningTokens: 30,
			},
		}

		responseUsage := original.ToResponseUsage()
		converted := responseUsage.ToChatUsage()
		assert.Equal(t, original.PromptTokens, converted.PromptTokens)
		assert.Equal(t, original.CompletionTokens, converted.CompletionTokens)
		assert.Equal(t, original.TotalTokens, converted.TotalTokens)
		assert.Equal(
			t,
			original.PromptTokensDetails.CachedTokens,
			converted.PromptTokensDetails.CachedTokens,
		)
		assert.Equal(
			t,
			original.CompletionTokensDetails.ReasoningTokens,
			converted.CompletionTokensDetails.ReasoningTokens,
		)
	})

	t.Run("ResponseUsage -> GeminiUsage -> ResponseUsage", func(t *testing.T) {
		original := model.ResponseUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
			InputTokensDetails: &model.ResponseUsageDetails{
				CachedTokens: 20,
			},
			OutputTokensDetails: &model.ResponseUsageDetails{
				ReasoningTokens: 30,
			},
		}

		geminiUsage := original.ToGeminiUsage()
		converted := geminiUsage.ToResponseUsage()
		assert.Equal(t, original.InputTokens, converted.InputTokens)
		assert.Equal(t, original.OutputTokens, converted.OutputTokens)
		assert.Equal(t, original.TotalTokens, converted.TotalTokens)
		assert.Equal(
			t,
			original.InputTokensDetails.CachedTokens,
			converted.InputTokensDetails.CachedTokens,
		)
		assert.Equal(
			t,
			original.OutputTokensDetails.ReasoningTokens,
			converted.OutputTokensDetails.ReasoningTokens,
		)
	})
}
