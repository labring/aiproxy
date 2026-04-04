//go:build enterprise

package novita

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
)

func TestNovitaResponsesBase(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "default Novita base URL",
			baseURL: "https://api.novita.ai/v3/openai",
			want:    "https://api.novita.ai/openai/v1",
		},
		{
			name:    "custom base URL with /v3/openai suffix",
			baseURL: "https://custom.example.com/v3/openai",
			want:    "https://custom.example.com/openai/v1",
		},
		{
			name:    "base URL without /v3/openai — falls back to default",
			baseURL: "https://other.example.com/api",
			want:    "https://api.novita.ai/openai/v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := novitaResponsesBase(tc.baseURL); got != tc.want {
				t.Errorf("novitaResponsesBase(%q) = %q, want %q", tc.baseURL, got, tc.want)
			}
		})
	}
}

func TestBuildConfigFromV2Model_ToolChoiceAndVision(t *testing.T) {
	tests := []struct {
		name           string
		model          NovitaModelV2
		wantToolChoice bool
		wantVision     bool
	}{
		{
			name: "Claude chat model with image input",
			model: NovitaModelV2{
				ID:              "claude-sonnet-4-20250514",
				ModelType:       "chat",
				Features:        []string{"tool_use"},
				InputModalities: []string{"text", "image"},
			},
			wantToolChoice: true,
			wantVision:     true,
		},
		{
			name: "Text-only chat model",
			model: NovitaModelV2{
				ID:              "deepseek-v3",
				ModelType:       "chat",
				InputModalities: []string{"text"},
			},
			wantToolChoice: true,
			wantVision:     false,
		},
		{
			name: "Embedding model",
			model: NovitaModelV2{
				ID:              "bge-m3",
				ModelType:       "embedding",
				InputModalities: []string{"text"},
			},
			wantToolChoice: false,
			wantVision:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildConfigFromV2Model(&tt.model)

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
