package model_test

import (
	"encoding/json"
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
)

func TestResponseUsageCacheWriteTokensJSON(t *testing.T) {
	var usage model.ResponseUsage

	err := json.Unmarshal([]byte(`{
		"input_tokens": 2006,
		"output_tokens": 300,
		"total_tokens": 2306,
		"input_tokens_details": {
			"cached_tokens": 1920,
			"cache_write_tokens": 64
		}
	}`), &usage)
	assert.NoError(t, err)
	assert.Equal(t, coremodel.ZeroNullInt64(64), usage.ToModelUsage().CacheCreationTokens)
}

func TestChatUsageConversions(t *testing.T) {
	chatUsage := model.ChatUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: &model.PromptTokensDetails{
			AudioTokens:         5,
			ImageTokens:         6,
			VideoTokens:         9,
			CachedTokens:        20,
			CacheCreationTokens: 10,
		},
		CompletionTokensDetails: &model.CompletionTokensDetails{
			AudioTokens:     7,
			ImageTokens:     8,
			ReasoningTokens: 30,
		},
	}

	t.Run("ChatUsage to ResponseUsage", func(t *testing.T) {
		responseUsage := chatUsage.ToResponseUsage()
		assert.Equal(t, int64(100), responseUsage.InputTokens)
		assert.Equal(t, int64(50), responseUsage.OutputTokens)
		assert.Equal(t, int64(150), responseUsage.TotalTokens)
		assert.NotNil(t, responseUsage.InputTokensDetails)
		assert.Equal(t, int64(5), responseUsage.InputTokensDetails.AudioTokens)
		assert.Equal(t, int64(6), responseUsage.InputTokensDetails.ImageTokens)
		assert.Equal(t, int64(9), responseUsage.InputTokensDetails.VideoTokens)
		assert.Equal(t, int64(20), responseUsage.InputTokensDetails.CachedTokens)
		assert.Equal(t, int64(10), responseUsage.InputTokensDetails.CacheWriteTokens)
		assert.NotNil(t, responseUsage.OutputTokensDetails)
		assert.Equal(t, int64(7), responseUsage.OutputTokensDetails.AudioTokens)
		assert.Equal(t, int64(8), responseUsage.OutputTokensDetails.ImageTokens)
		assert.Equal(t, int64(30), responseUsage.OutputTokensDetails.ReasoningTokens)
	})

	t.Run("ChatUsage to model Usage", func(t *testing.T) {
		modelUsage := chatUsage.ToModelUsage()
		assert.Equal(t, coremodel.ZeroNullInt64(100), modelUsage.InputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(6), modelUsage.ImageInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(5), modelUsage.AudioInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(9), modelUsage.VideoInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(50), modelUsage.OutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(8), modelUsage.ImageOutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(7), modelUsage.AudioOutputTokens)
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
		assert.Equal(t, int64(20), geminiUsage.CandidatesTokenCount) // 50 - 30 reasoning
		assert.Equal(t, int64(150), geminiUsage.TotalTokenCount)
		assert.Equal(
			t,
			geminiUsage.TotalTokenCount,
			geminiUsage.PromptTokenCount+geminiUsage.CandidatesTokenCount+geminiUsage.ThoughtsTokenCount,
		)
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
			AudioTokens:      5,
			ImageTokens:      6,
			VideoTokens:      9,
			CachedTokens:     20,
			CacheWriteTokens: 10,
		},
		OutputTokensDetails: &model.ResponseUsageDetails{
			AudioTokens:     7,
			ImageTokens:     8,
			ReasoningTokens: 30,
		},
	}

	t.Run("ResponseUsage to ChatUsage", func(t *testing.T) {
		chatUsage := responseUsage.ToChatUsage()
		assert.Equal(t, int64(100), chatUsage.PromptTokens)
		assert.Equal(t, int64(50), chatUsage.CompletionTokens)
		assert.Equal(t, int64(150), chatUsage.TotalTokens)
		assert.NotNil(t, chatUsage.PromptTokensDetails)
		assert.Equal(t, int64(5), chatUsage.PromptTokensDetails.AudioTokens)
		assert.Equal(t, int64(6), chatUsage.PromptTokensDetails.ImageTokens)
		assert.Equal(t, int64(9), chatUsage.PromptTokensDetails.VideoTokens)
		assert.Equal(t, int64(20), chatUsage.PromptTokensDetails.CachedTokens)
		assert.Equal(t, int64(10), chatUsage.PromptTokensDetails.CacheWriteTokens)
		assert.NotNil(t, chatUsage.CompletionTokensDetails)
		assert.Equal(t, int64(7), chatUsage.CompletionTokensDetails.AudioTokens)
		assert.Equal(t, int64(8), chatUsage.CompletionTokensDetails.ImageTokens)
		assert.Equal(t, int64(30), chatUsage.CompletionTokensDetails.ReasoningTokens)
	})

	t.Run("ResponseUsage to model Usage", func(t *testing.T) {
		modelUsage := responseUsage.ToModelUsage()
		assert.Equal(t, coremodel.ZeroNullInt64(100), modelUsage.InputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(6), modelUsage.ImageInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(5), modelUsage.AudioInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(9), modelUsage.VideoInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(50), modelUsage.OutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(8), modelUsage.ImageOutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(7), modelUsage.AudioOutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(10), modelUsage.CacheCreationTokens)
	})

	t.Run("ResponseUsage to ClaudeUsage", func(t *testing.T) {
		claudeUsage := responseUsage.ToClaudeUsage()
		assert.Equal(t, int64(100), claudeUsage.InputTokens)
		assert.Equal(t, int64(50), claudeUsage.OutputTokens)
		assert.Equal(t, int64(20), claudeUsage.CacheReadInputTokens)
		assert.Equal(t, int64(10), claudeUsage.CacheCreationInputTokens)
	})

	t.Run("ResponseUsage to GeminiUsage", func(t *testing.T) {
		geminiUsage := responseUsage.ToGeminiUsage()
		assert.Equal(t, int64(100), geminiUsage.PromptTokenCount)
		assert.Equal(t, int64(20), geminiUsage.CandidatesTokenCount) // 50 - 30 reasoning
		assert.Equal(t, int64(150), geminiUsage.TotalTokenCount)
		assert.Equal(
			t,
			geminiUsage.TotalTokenCount,
			geminiUsage.PromptTokenCount+geminiUsage.CandidatesTokenCount+geminiUsage.ThoughtsTokenCount,
		)
		assert.Equal(t, int64(20), geminiUsage.CachedContentTokenCount)
		assert.Equal(t, int64(30), geminiUsage.ThoughtsTokenCount)
	})

	t.Run("ResponseUsage to GeminiUsage reasoning only", func(t *testing.T) {
		reasoningOnly := model.ResponseUsage{
			InputTokens:  268,
			OutputTokens: 192,
			TotalTokens:  460,
			OutputTokensDetails: &model.ResponseUsageDetails{
				ReasoningTokens: 192,
			},
		}

		geminiUsage := reasoningOnly.ToGeminiUsage()
		assert.Equal(t, int64(0), geminiUsage.CandidatesTokenCount)
		assert.Equal(t, int64(192), geminiUsage.ThoughtsTokenCount)
		assert.Equal(t, int64(460), geminiUsage.TotalTokenCount)
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
		PromptTokensDetails: []model.GeminiTokensDetail{
			{Modality: model.GeminiModalityImage, TokenCount: 6},
			{Modality: model.GeminiModalityAudio, TokenCount: 5},
			{Modality: model.GeminiModalityVideo, TokenCount: 9},
		},
		CandidatesTokensDetails: []model.GeminiTokensDetail{
			{Modality: model.GeminiModalityImage, TokenCount: 8},
			{Modality: model.GeminiModalityAudio, TokenCount: 7},
		},
	}

	t.Run("GeminiUsage to ChatUsage", func(t *testing.T) {
		chatUsage := geminiUsage.ToUsage()
		assert.Equal(t, int64(100), chatUsage.PromptTokens)
		assert.Equal(t, int64(80), chatUsage.CompletionTokens) // 50 + 30
		assert.Equal(t, int64(150), chatUsage.TotalTokens)
		assert.NotNil(t, chatUsage.PromptTokensDetails)
		assert.Equal(t, int64(5), chatUsage.PromptTokensDetails.AudioTokens)
		assert.Equal(t, int64(6), chatUsage.PromptTokensDetails.ImageTokens)
		assert.Equal(t, int64(9), chatUsage.PromptTokensDetails.VideoTokens)
		assert.Equal(t, int64(20), chatUsage.PromptTokensDetails.CachedTokens)
		assert.NotNil(t, chatUsage.CompletionTokensDetails)
		assert.Equal(t, int64(7), chatUsage.CompletionTokensDetails.AudioTokens)
		assert.Equal(t, int64(8), chatUsage.CompletionTokensDetails.ImageTokens)
		assert.Equal(t, int64(30), chatUsage.CompletionTokensDetails.ReasoningTokens)
	})

	t.Run("GeminiUsage to ResponseUsage", func(t *testing.T) {
		responseUsage := geminiUsage.ToResponseUsage()
		assert.Equal(t, int64(100), responseUsage.InputTokens)
		assert.Equal(t, int64(80), responseUsage.OutputTokens) // 50 candidates + 30 thoughts
		assert.Equal(t, int64(150), responseUsage.TotalTokens)
		assert.NotNil(t, responseUsage.InputTokensDetails)
		assert.Equal(t, int64(5), responseUsage.InputTokensDetails.AudioTokens)
		assert.Equal(t, int64(6), responseUsage.InputTokensDetails.ImageTokens)
		assert.Equal(t, int64(9), responseUsage.InputTokensDetails.VideoTokens)
		assert.Equal(t, int64(20), responseUsage.InputTokensDetails.CachedTokens)
		assert.NotNil(t, responseUsage.OutputTokensDetails)
		assert.Equal(t, int64(7), responseUsage.OutputTokensDetails.AudioTokens)
		assert.Equal(t, int64(8), responseUsage.OutputTokensDetails.ImageTokens)
		assert.Equal(t, int64(30), responseUsage.OutputTokensDetails.ReasoningTokens)
	})

	t.Run(
		"GeminiUsage to ChatUsage to model Usage preserves multimodal input details",
		func(t *testing.T) {
			modelUsage := geminiUsage.ToUsage().ToModelUsage()
			assert.Equal(t, coremodel.ZeroNullInt64(100), modelUsage.InputTokens)
			assert.Equal(t, coremodel.ZeroNullInt64(6), modelUsage.ImageInputTokens)
			assert.Equal(t, coremodel.ZeroNullInt64(5), modelUsage.AudioInputTokens)
			assert.Equal(t, coremodel.ZeroNullInt64(9), modelUsage.VideoInputTokens)
			assert.Equal(t, coremodel.ZeroNullInt64(80), modelUsage.OutputTokens)
			assert.Equal(t, coremodel.ZeroNullInt64(8), modelUsage.ImageOutputTokens)
			assert.Equal(t, coremodel.ZeroNullInt64(7), modelUsage.AudioOutputTokens)
			assert.Equal(t, coremodel.ZeroNullInt64(30), modelUsage.ReasoningTokens)
			assert.Equal(t, coremodel.ZeroNullInt64(150), modelUsage.TotalTokens)
		},
	)

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
		assert.Equal(t, int64(20), geminiUsage.CandidatesTokenCount)
		assert.Equal(t, int64(30), geminiUsage.ThoughtsTokenCount)

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

		restored := geminiUsage.ToUsage()
		assert.Equal(t, original.OutputTokens, restored.CompletionTokens)
		assert.Equal(
			t,
			original.OutputTokensDetails.ReasoningTokens,
			restored.CompletionTokensDetails.ReasoningTokens,
		)
	})
}

func TestImageUsageToModelUsage(t *testing.T) {
	t.Run("WithOutputTokensDetails", func(t *testing.T) {
		// New format with separate text and image output tokens
		imageUsage := model.ImageUsage{
			InputTokens:  1000,
			OutputTokens: 5000, // total: 1000 text + 4000 image
			TotalTokens:  6000,
			InputTokensDetails: model.ImageInputTokensDetails{
				TextTokens:  200,
				ImageTokens: 800,
			},
			OutputTokensDetails: &model.ImageOutputTokensDetails{
				TextTokens:  1000,
				ImageTokens: 4000,
			},
		}

		usage := imageUsage.ToModelUsage()

		assert.Equal(t, coremodel.ZeroNullInt64(1000), usage.InputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(800), usage.ImageInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(5000), usage.OutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(4000), usage.ImageOutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(6000), usage.TotalTokens)
	})

	t.Run("WithoutOutputTokensDetails", func(t *testing.T) {
		// Old format where all output tokens are image tokens
		imageUsage := model.ImageUsage{
			InputTokens:  1000,
			OutputTokens: 4000, // all image tokens
			TotalTokens:  5000,
			InputTokensDetails: model.ImageInputTokensDetails{
				TextTokens:  200,
				ImageTokens: 800,
			},
			// OutputTokensDetails is nil
		}

		usage := imageUsage.ToModelUsage()

		assert.Equal(t, coremodel.ZeroNullInt64(1000), usage.InputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(800), usage.ImageInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(4000), usage.OutputTokens)
		assert.Equal(
			t,
			coremodel.ZeroNullInt64(4000),
			usage.ImageOutputTokens,
		) // same as OutputTokens
		assert.Equal(t, coremodel.ZeroNullInt64(5000), usage.TotalTokens)
	})

	t.Run("ZeroValues", func(t *testing.T) {
		imageUsage := model.ImageUsage{
			InputTokens:  0,
			OutputTokens: 0,
			TotalTokens:  0,
			InputTokensDetails: model.ImageInputTokensDetails{
				TextTokens:  0,
				ImageTokens: 0,
			},
		}

		usage := imageUsage.ToModelUsage()

		assert.Equal(t, coremodel.ZeroNullInt64(0), usage.InputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(0), usage.ImageInputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(0), usage.OutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(0), usage.ImageOutputTokens)
		assert.Equal(t, coremodel.ZeroNullInt64(0), usage.TotalTokens)
	})
}
