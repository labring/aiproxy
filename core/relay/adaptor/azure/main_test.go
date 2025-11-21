package azure_test

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/adaptor/azure"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURL(t *testing.T) {
	adaptor := &azure.Adaptor{}

	tests := []struct {
		name            string
		model           string
		mode            mode.Mode
		apiVersion      string
		expectedURL     string
		expectedContain string
	}{
		{
			name:            "gpt-5-codex with ChatCompletions should use Responses API",
			model:           "gpt-5-codex",
			mode:            mode.ChatCompletions,
			apiVersion:      "2024-02-01",
			expectedContain: "/openai/v1/responses",
		},
		{
			name:            "gpt-4o with ChatCompletions should use standard API",
			model:           "gpt-4o",
			mode:            mode.ChatCompletions,
			apiVersion:      "2024-02-01",
			expectedContain: "/openai/deployments/gpt-4o/chat/completions",
		},
		{
			name:            "gpt-35-turbo with ChatCompletions should use standard API",
			model:           "gpt-35-turbo",
			mode:            mode.ChatCompletions,
			apiVersion:      "2024-02-01",
			expectedContain: "/openai/deployments/gpt-35-turbo/chat/completions",
		},
		{
			name:            "gpt-5-codex with Anthropic mode should use Responses API",
			model:           "gpt-5-codex",
			mode:            mode.Anthropic,
			apiVersion:      "2024-02-01",
			expectedContain: "/openai/v1/responses",
		},
		{
			name:            "gpt-5-codex with Gemini mode should use Responses API",
			model:           "gpt-5-codex",
			mode:            mode.Gemini,
			apiVersion:      "2024-02-01",
			expectedContain: "/openai/v1/responses",
		},
		{
			name:            "gpt-5-codex Responses API should use preview version",
			model:           "gpt-5-codex",
			mode:            mode.ChatCompletions,
			apiVersion:      "2024-02-01",
			expectedContain: "api-version=preview",
		},
		{
			name:            "gpt-4o should use provided api-version",
			model:           "gpt-4o",
			mode:            mode.ChatCompletions,
			apiVersion:      "2024-02-01",
			expectedContain: "api-version=2024-02-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &meta.Meta{
				ActualModel: tt.model,
				Mode:        tt.mode,
			}
			m.Channel.BaseURL = "https://test.openai.azure.com"
			m.Channel.Key = "test-key|" + tt.apiVersion

			result, err := adaptor.GetRequestURL(m, nil, nil)
			require.NoError(t, err)

			assert.Contains(t, result.URL, tt.expectedContain,
				"URL should contain expected pattern for model %s with mode %s", tt.model, tt.mode)

			// Verify it's a POST request for all these modes
			assert.Equal(t, "POST", result.Method)
		})
	}
}

func TestGetRequestURL_ResponsesOnlyModels(t *testing.T) {
	adaptor := &azure.Adaptor{}

	// Test that responses-only models always use the Responses API endpoint
	responsesOnlyModels := []string{"gpt-5-codex", "gpt-5-pro"}
	modes := []mode.Mode{mode.ChatCompletions, mode.Anthropic, mode.Gemini}

	for _, model := range responsesOnlyModels {
		for _, m := range modes {
			testName := model + "_mode_" + m.String()
			t.Run(testName, func(t *testing.T) {
				meta := &meta.Meta{
					ActualModel: model,
					Mode:        m,
				}
				meta.Channel.BaseURL = "https://test.openai.azure.com"
				meta.Channel.Key = "test-key|2024-02-01"

				result, err := adaptor.GetRequestURL(meta, nil, nil)
				require.NoError(t, err)

				// Should use Responses API endpoint
				assert.Contains(t, result.URL, "/openai/v1/responses")
				// Should use preview API version
				assert.Contains(t, result.URL, "api-version=preview")
				// Should be POST
				assert.Equal(t, "POST", result.Method)
			})
		}
	}
}

func TestGetRequestURL_StandardModels(t *testing.T) {
	adaptor := &azure.Adaptor{}

	// Test that standard models use the regular deployment endpoint
	standardModels := []string{"gpt-4o", "gpt-35-turbo", "gpt-4"}

	for _, model := range standardModels {
		t.Run(model, func(t *testing.T) {
			meta := &meta.Meta{
				ActualModel: model,
				Mode:        mode.ChatCompletions,
			}
			meta.Channel.BaseURL = "https://test.openai.azure.com"
			meta.Channel.Key = "test-key|2024-02-01"

			result, err := adaptor.GetRequestURL(meta, nil, nil)
			require.NoError(t, err)

			// Should use deployment endpoint
			assert.Contains(t, result.URL, "/openai/deployments/"+model)
			// Should NOT use Responses API
			assert.NotContains(t, result.URL, "/openai/v1/responses")
			// Should use provided API version
			assert.Contains(t, result.URL, "api-version=2024-02-01")
		})
	}
}

func TestGetRequestURL_DotReplacement(t *testing.T) {
	adaptor := &azure.Adaptor{}

	tests := []struct {
		name          string
		model         string
		expectedModel string
	}{
		{
			name:          "gpt-3.5-turbo should have dots removed",
			model:         "gpt-3.5-turbo",
			expectedModel: "gpt-35-turbo",
		},
		{
			name:          "gpt-4.0 should have dots removed",
			model:         "gpt-4.0",
			expectedModel: "gpt-40",
		},
		{
			name:          "model without dots should remain unchanged",
			model:         "gpt-4o",
			expectedModel: "gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &meta.Meta{
				ActualModel: tt.model,
				Mode:        mode.ChatCompletions,
			}
			meta.Channel.BaseURL = "https://test.openai.azure.com"
			meta.Channel.Key = "test-key|2024-02-01"

			result, err := adaptor.GetRequestURL(meta, nil, nil)
			require.NoError(t, err)

			// For standard models (not responses-only), check dot replacement
			if !openai.IsResponsesOnlyModel(&meta.ModelConfig, tt.model) {
				assert.Contains(t, result.URL, "/openai/deployments/"+tt.expectedModel)
			}
		})
	}
}

func TestGetRequestURL_OtherModes(t *testing.T) {
	adaptor := &azure.Adaptor{}

	tests := []struct {
		name            string
		mode            mode.Mode
		expectedContain string
	}{
		{
			name:            "Completions mode",
			mode:            mode.Completions,
			expectedContain: "/completions",
		},
		{
			name:            "Embeddings mode",
			mode:            mode.Embeddings,
			expectedContain: "/embeddings",
		},
		{
			name:            "ImagesGenerations mode",
			mode:            mode.ImagesGenerations,
			expectedContain: "/images/generations",
		},
		{
			name:            "AudioTranscription mode",
			mode:            mode.AudioTranscription,
			expectedContain: "/audio/transcriptions",
		},
		{
			name:            "AudioSpeech mode",
			mode:            mode.AudioSpeech,
			expectedContain: "/audio/speech",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &meta.Meta{
				ActualModel: "test-model",
				Mode:        tt.mode,
			}
			meta.Channel.BaseURL = "https://test.openai.azure.com"
			meta.Channel.Key = "test-key|2024-02-01"

			result, err := adaptor.GetRequestURL(meta, nil, nil)
			require.NoError(t, err)

			assert.Contains(t, result.URL, tt.expectedContain)
		})
	}
}

func TestGetRequestURL_ResponsesModeDirect(t *testing.T) {
	adaptor := &azure.Adaptor{}

	// Test direct Responses mode (not converted from another mode)
	meta := &meta.Meta{
		ActualModel: "gpt-4o",
		Mode:        mode.Responses,
	}
	meta.Channel.BaseURL = "https://test.openai.azure.com"
	meta.Channel.Key = "test-key|2024-02-01"

	result, err := adaptor.GetRequestURL(meta, nil, nil)
	require.NoError(t, err)

	// Should use Responses API endpoint
	assert.Contains(t, result.URL, "/openai/v1/responses")
	// Should use preview API version for Responses mode
	assert.Contains(t, result.URL, "api-version=preview")
	assert.Equal(t, "POST", result.Method)
}
