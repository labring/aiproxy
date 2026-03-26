package model_test

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestModelConfigShouldSummaryServiceTier(t *testing.T) {
	t.Run("default false does not record summary service tier", func(t *testing.T) {
		cfg := &model.ModelConfig{}
		if cfg.ShouldSummaryServiceTier() {
			t.Fatal("expected zero-value summary_service_tier to default to false")
		}
	})

	t.Run("explicit false disables summary service tier recording", func(t *testing.T) {
		cfg := &model.ModelConfig{SummaryServiceTier: false}
		if cfg.ShouldSummaryServiceTier() {
			t.Fatal("expected false summary_service_tier to disable recording")
		}
	})
}

func TestModelConfigLoadFromGroupModelConfigSummaryServiceTier(t *testing.T) {
	base := (&model.ModelConfig{
		SummaryServiceTier: true,
	}).LoadFromGroupModelConfig(model.GroupModelConfig{
		OverrideSummaryServiceTier: true,
		SummaryServiceTier:         false,
	})

	if base.SummaryServiceTier {
		t.Fatal("expected override to set summary_service_tier to false")
	}
}

func TestModelConfigShouldSummaryClaudeLongContext(t *testing.T) {
	t.Run("default false does not record claude long context", func(t *testing.T) {
		cfg := &model.ModelConfig{}
		if cfg.ShouldSummaryClaudeLongContext() {
			t.Fatal("expected zero-value summary_claude_long_context to default to false")
		}
	})

	t.Run("explicit true enables claude long context", func(t *testing.T) {
		cfg := &model.ModelConfig{SummaryClaudeLongContext: true}
		if !cfg.ShouldSummaryClaudeLongContext() {
			t.Fatal("expected true summary_claude_long_context to enable recording")
		}
	})
}

func TestModelConfigLoadFromGroupModelConfigSummaryClaudeLongContext(t *testing.T) {
	base := (&model.ModelConfig{
		SummaryClaudeLongContext: false,
	}).LoadFromGroupModelConfig(
		model.GroupModelConfig{
			OverrideSummaryClaudeLongContext: true,
			SummaryClaudeLongContext:         true,
		},
	)

	if !base.SummaryClaudeLongContext {
		t.Fatal("expected override to set summary_claude_long_context to true")
	}
}

func TestModelConfigSupportStreamTimeout(t *testing.T) {
	t.Run("supports stream timeout for stream-capable modes", func(t *testing.T) {
		modes := []mode.Mode{
			mode.ChatCompletions,
			mode.Completions,
			mode.Anthropic,
			mode.Responses,
			mode.Gemini,
		}

		for _, m := range modes {
			cfg := &model.ModelConfig{Type: m}
			if !cfg.SupportStreamTimeout() {
				t.Fatalf("expected mode %s to support stream timeout", m.String())
			}
		}
	})

	t.Run("does not support stream timeout for non-stream modes", func(t *testing.T) {
		modes := []mode.Mode{
			mode.Embeddings,
			mode.Moderations,
			mode.ImagesGenerations,
			mode.ImagesEdits,
			mode.AudioSpeech,
			mode.AudioTranscription,
			mode.AudioTranslation,
			mode.Rerank,
			mode.ParsePdf,
			mode.VideoGenerationsJobs,
		}

		for _, m := range modes {
			cfg := &model.ModelConfig{Type: m}
			if cfg.SupportStreamTimeout() {
				t.Fatalf("expected mode %s to not support stream timeout", m.String())
			}
		}
	})
}

func TestModelConfigBeforeSaveClearsUnsupportedStreamTimeout(t *testing.T) {
	cfg := &model.ModelConfig{
		Model: "test-embedding",
		Type:  mode.Embeddings,
		TimeoutConfig: model.TimeoutConfig{
			RequestTimeout:       30,
			StreamRequestTimeout: 120,
		},
	}

	if err := cfg.BeforeSave(nil); err != nil {
		t.Fatalf("expected BeforeSave to succeed, got error: %v", err)
	}

	if cfg.TimeoutConfig.RequestTimeout != 30 {
		t.Fatalf("expected request timeout to remain unchanged, got %d", cfg.TimeoutConfig.RequestTimeout)
	}

	if cfg.TimeoutConfig.StreamRequestTimeout != 0 {
		t.Fatalf("expected unsupported stream timeout to be cleared, got %d", cfg.TimeoutConfig.StreamRequestTimeout)
	}
}
