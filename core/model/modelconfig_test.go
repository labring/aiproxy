package model_test

import (
	"path/filepath"
	"testing"

	"github.com/labring/aiproxy/core/common"
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

func TestModelConfigLoadFromGroupModelConfigBodyStorageMaxSize(t *testing.T) {
	base := (&model.ModelConfig{
		RequestBodyStorageMaxSize:  1024,
		ResponseBodyStorageMaxSize: 2048,
	}).LoadFromGroupModelConfig(
		model.GroupModelConfig{
			OverrideRequestBodyStorageMaxSize:  true,
			RequestBodyStorageMaxSize:          512,
			OverrideResponseBodyStorageMaxSize: true,
			ResponseBodyStorageMaxSize:         1536,
		},
	)

	if base.RequestBodyStorageMaxSize != 512 {
		t.Fatalf(
			"expected request_body_storage_max_size to be overridden to 512, got %d",
			base.RequestBodyStorageMaxSize,
		)
	}

	if base.ResponseBodyStorageMaxSize != 1536 {
		t.Fatalf(
			"expected response_body_storage_max_size to be overridden to 1536, got %d",
			base.ResponseBodyStorageMaxSize,
		)
	}
}

func TestModelConfigLoadFromGroupModelConfigTimeoutConfig(t *testing.T) {
	base := (&model.ModelConfig{
		Type: mode.ChatCompletions,
		TimeoutConfig: model.TimeoutConfig{
			RequestTimeout:       30,
			StreamRequestTimeout: 60,
		},
	}).LoadFromGroupModelConfig(
		model.GroupModelConfig{
			OverrideTimeoutConfig: true,
			TimeoutConfig: model.TimeoutConfig{
				RequestTimeout:       45,
				StreamRequestTimeout: 90,
			},
		},
	)

	if base.TimeoutConfig.RequestTimeout != 45 {
		t.Fatalf(
			"expected request timeout to be overridden to 45, got %d",
			base.TimeoutConfig.RequestTimeout,
		)
	}

	if base.TimeoutConfig.StreamRequestTimeout != 90 {
		t.Fatalf(
			"expected stream request timeout to be overridden to 90, got %d",
			base.TimeoutConfig.StreamRequestTimeout,
		)
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
		t.Fatalf(
			"expected request timeout to remain unchanged, got %d",
			cfg.TimeoutConfig.RequestTimeout,
		)
	}

	if cfg.TimeoutConfig.StreamRequestTimeout != 0 {
		t.Fatalf(
			"expected unsupported stream timeout to be cleared, got %d",
			cfg.TimeoutConfig.StreamRequestTimeout,
		)
	}
}

func TestGetModelConfigLoadsFastJSONFields(t *testing.T) {
	prevDB := model.DB
	prevUsingSQLite := common.UsingSQLite

	dbPath := filepath.Join(t.TempDir(), "model-config.db")

	testDB, err := model.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = testDB
	common.UsingSQLite = true
	t.Cleanup(func() {
		model.DB = prevDB
		common.UsingSQLite = prevUsingSQLite
	})

	if err := testDB.AutoMigrate(&model.ModelConfig{}); err != nil {
		t.Fatalf("failed to migrate model config: %v", err)
	}

	expected := model.ModelConfig{
		Model: "provider:model:v1",
		Owner: "owner",
		Type:  mode.ChatCompletions,
		Config: map[model.ModelConfigKey]any{
			model.ModelConfigSupportFormatsKey: []string{"json", "text"},
		},
		Plugin: map[string]map[string]any{
			"cache": {
				"enable": true,
			},
		},
		ImagePrices: map[string]float64{
			"1024x1024": 0.12,
		},
		ImageQualityPrices: map[string]map[string]float64{
			"1024x1024": {
				"hd": 0.34,
			},
		},
	}

	if err := testDB.Create(&expected).Error; err != nil {
		t.Fatalf("failed to create model config: %v", err)
	}

	got, err := model.GetModelConfig(expected.Model)
	if err != nil {
		t.Fatalf("expected GetModelConfig to succeed, got error: %v", err)
	}

	if got.Model != expected.Model {
		t.Fatalf("expected model %q, got %q", expected.Model, got.Model)
	}

	if got.Plugin["cache"]["enable"] != true {
		t.Fatalf("expected plugin cache.enable to be true, got %#v", got.Plugin)
	}

	if got.ImagePrices["1024x1024"] != 0.12 {
		t.Fatalf("expected image price 0.12, got %#v", got.ImagePrices)
	}

	if got.ImageQualityPrices["1024x1024"]["hd"] != 0.34 {
		t.Fatalf("expected image quality price 0.34, got %#v", got.ImageQualityPrices)
	}
}
