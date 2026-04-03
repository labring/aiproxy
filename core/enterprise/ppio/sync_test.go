//go:build enterprise

package ppio

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
)

func TestInferToolChoice(t *testing.T) {
	tests := []struct {
		name      string
		modelType string
		features  []string
		want      bool
	}{
		{
			name:      "chat model with no features",
			modelType: "chat",
			features:  nil,
			want:      true,
		},
		{
			name:      "chat model with empty features",
			modelType: "chat",
			features:  []string{},
			want:      true,
		},
		{
			name:      "chat model with tool_use feature",
			modelType: "chat",
			features:  []string{"tool_use", "streaming"},
			want:      true,
		},
		{
			name:      "chat model with function_calling feature",
			modelType: "chat",
			features:  []string{"function_calling"},
			want:      true,
		},
		{
			name:      "chat model with tools feature",
			modelType: "chat",
			features:  []string{"tools"},
			want:      true,
		},
		{
			name:      "embedding model with no features",
			modelType: "embedding",
			features:  nil,
			want:      false,
		},
		{
			name:      "image model with no features",
			modelType: "image",
			features:  nil,
			want:      false,
		},
		{
			name:      "non-chat model with tool_use feature",
			modelType: "embedding",
			features:  []string{"tool_use"},
			want:      true,
		},
		{
			name:      "rerank model",
			modelType: "rerank",
			features:  []string{},
			want:      false,
		},
		{
			name:      "empty model type with no features",
			modelType: "",
			features:  nil,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferToolChoice(tt.modelType, tt.features)
			if got != tt.want {
				t.Errorf("inferToolChoice(%q, %v) = %v, want %v",
					tt.modelType, tt.features, got, tt.want)
			}
		})
	}
}

func TestPPIOURLHelpers(t *testing.T) {
	cases := []struct {
		name         string
		baseURL      string
		wantResp     string
		wantWebSearch string
	}{
		{
			name:          "default PPIO base URL",
			baseURL:       "https://api.ppinfra.com/v3/openai",
			wantResp:      "https://api.ppinfra.com/openai/v1",
			wantWebSearch: "https://api.ppinfra.com/v3",
		},
		{
			name:          "custom base URL with /v3/openai suffix",
			baseURL:       "https://custom.example.com/v3/openai",
			wantResp:      "https://custom.example.com/openai/v1",
			wantWebSearch: "https://custom.example.com/v3",
		},
		{
			name:          "base URL without /v3/openai — falls back to default",
			baseURL:       "https://other.example.com/api",
			wantResp:      "https://api.ppinfra.com/openai/v1",
			wantWebSearch: "https://api.ppinfra.com/v3",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ppioResponsesBase(tc.baseURL); got != tc.wantResp {
				t.Errorf("ppioResponsesBase(%q) = %q, want %q", tc.baseURL, got, tc.wantResp)
			}
			if got := ppioWebSearchBase(tc.baseURL); got != tc.wantWebSearch {
				t.Errorf("ppioWebSearchBase(%q) = %q, want %q", tc.baseURL, got, tc.wantWebSearch)
			}
		})
	}
}

func TestBuildConfigFromPPIOModelV2_ToolChoiceAndVision(t *testing.T) {
	tests := []struct {
		name           string
		model          PPIOModelV2
		wantToolChoice bool
		wantVision     bool
	}{
		{
			name: "Claude chat model with image input",
			model: PPIOModelV2{
				ID:              "claude-sonnet-4-20250514",
				ModelType:       "chat",
				Features:        []string{"tool_use", "streaming"},
				InputModalities: []string{"text", "image"},
			},
			wantToolChoice: true,
			wantVision:     true,
		},
		{
			name: "Chat model text only",
			model: PPIOModelV2{
				ID:              "deepseek-v3",
				ModelType:       "chat",
				Features:        []string{},
				InputModalities: []string{"text"},
			},
			wantToolChoice: true,
			wantVision:     false,
		},
		{
			name: "Embedding model",
			model: PPIOModelV2{
				ID:              "bge-m3",
				ModelType:       "embedding",
				Features:        []string{},
				InputModalities: []string{"text"},
			},
			wantToolChoice: false,
			wantVision:     false,
		},
		{
			name: "Image generation model",
			model: PPIOModelV2{
				ID:              "flux-1",
				ModelType:       "image",
				Features:        []string{},
				InputModalities: []string{"text"},
			},
			wantToolChoice: false,
			wantVision:     false,
		},
		{
			name: "Chat model with image modality but no tool features",
			model: PPIOModelV2{
				ID:              "llava-v1.6",
				ModelType:       "chat",
				Features:        []string{"streaming"},
				InputModalities: []string{"text", "image"},
			},
			wantToolChoice: true,
			wantVision:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildConfigFromPPIOModelV2(&tt.model)

			gotToolChoice, _ := cfg[string(model.ModelConfigToolChoiceKey)].(bool)
			if gotToolChoice != tt.wantToolChoice {
				t.Errorf("tool_choice = %v, want %v", gotToolChoice, tt.wantToolChoice)
			}

			gotVision, _ := cfg[string(model.ModelConfigVisionKey)].(bool)
			if gotVision != tt.wantVision {
				t.Errorf("vision = %v, want %v", gotVision, tt.wantVision)
			}
		})
	}
}
