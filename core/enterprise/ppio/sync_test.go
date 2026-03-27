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
