//nolint:testpackage
package zhipu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestAdaptorSupportMode(t *testing.T) {
	adaptor := &Adaptor{}

	supportedModes := []mode.Mode{
		mode.ChatCompletions,
		mode.Completions,
		mode.AudioTranscription,
		mode.AudioSpeech,
		mode.Embeddings,
		mode.Rerank,
		mode.Anthropic,
		mode.Gemini,
	}
	for _, m := range supportedModes {
		if !adaptor.SupportMode(m) {
			t.Fatalf("expected mode %s to be supported", m)
		}
	}

	unsupportedModes := []mode.Mode{
		mode.Responses,
		mode.ResponsesGet,
		mode.ImagesGenerations,
		mode.Moderations,
		mode.AudioTranslation,
	}
	for _, m := range unsupportedModes {
		if adaptor.SupportMode(m) {
			t.Fatalf("expected mode %s to be unsupported", m)
		}
	}
}

func TestAdaptorGetRequestURL(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{
		BaseURL: "https://open.bigmodel.cn/api/paas/v4",
	}

	tests := []struct {
		name string
		mode mode.Mode
		want string
	}{
		{
			name: "anthropic uses chat completions",
			mode: mode.Anthropic,
			want: "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name: "gemini uses chat completions",
			mode: mode.Gemini,
			want: "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name: "completions uses completions",
			mode: mode.Completions,
			want: "https://open.bigmodel.cn/api/paas/v4/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(channel, tt.mode, "glm-5.1", coremodel.ModelConfig{})

			got, err := adaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != http.MethodPost {
				t.Fatalf("expected method %s, got %s", http.MethodPost, got.Method)
			}

			if got.URL != tt.want {
				t.Fatalf("expected URL %s, got %s", tt.want, got.URL)
			}

			if m.Mode != tt.mode {
				t.Fatalf("expected mode to remain %s, got %s", tt.mode, m.Mode)
			}
		})
	}
}

func TestAdaptorGetRequestURLUnsupportedResponses(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://open.bigmodel.cn/api/paas/v4"},
		mode.Responses,
		"glm-5.1",
		coremodel.ModelConfig{},
	)

	if _, err := adaptor.GetRequestURL(m, nil, nil); err == nil {
		t.Fatal("expected Responses mode to be unsupported")
	}
}

func TestAdaptorConvertRequestReasoning(t *testing.T) {
	adaptor := &Adaptor{}

	t.Run("chat reasoning_effort maps to zhipu thinking", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.ChatCompletions, "glm-5.1", coremodel.ModelConfig{})

		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/v1/chat/completions",
			strings.NewReader(`{
				"model":"glm-5.1",
				"reasoning_effort":"none",
				"messages":[{"role":"user","content":"hello"}]
			}`),
		)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		result, err := adaptor.ConvertRequest(m, nil, req)
		if err != nil {
			t.Fatalf("ConvertRequest returned error: %v", err)
		}

		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("failed to read converted body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal converted body: %v", err)
		}

		thinking, ok := payload["thinking"].(map[string]any)
		if !ok {
			t.Fatalf("expected thinking object, got %#v", payload["thinking"])
		}

		if thinking["type"] != "disabled" {
			t.Fatalf("expected thinking.type=disabled, got %#v", thinking["type"])
		}
	})

	t.Run("gemini thinking maps to zhipu thinking", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.Gemini, "glm-5.1", coremodel.ModelConfig{})

		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/v1beta/models/gemini-pro:generateContent",
			strings.NewReader(`{
				"generationConfig": {
					"thinkingConfig": {
						"thinkingBudget": 2048,
						"includeThoughts": true
					}
				},
				"contents":[{"role":"user","parts":[{"text":"hello"}]}]
			}`),
		)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		result, err := adaptor.ConvertRequest(m, nil, req)
		if err != nil {
			t.Fatalf("ConvertRequest returned error: %v", err)
		}

		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("failed to read converted body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal converted body: %v", err)
		}

		thinking, ok := payload["thinking"].(map[string]any)
		if !ok {
			t.Fatalf("expected thinking object, got %#v", payload["thinking"])
		}

		if thinking["type"] != "enabled" {
			t.Fatalf("expected thinking.type=enabled, got %#v", thinking["type"])
		}

		if _, exists := payload["reasoning_effort"]; exists {
			t.Fatal("expected reasoning_effort to be removed")
		}
	})
}
