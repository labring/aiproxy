package model_test

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
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
