//nolint:testpackage
package moonshot

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
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.Anthropic,
		mode.Gemini,
	}
	for _, m := range supportedModes {
		if !adaptor.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("expected mode %s to be supported", m)
		}
	}

	unsupportedModes := []mode.Mode{
		mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems,
	}
	for _, m := range unsupportedModes {
		if adaptor.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("expected mode %s to be unsupported", m)
		}
	}
}

func TestAdaptorRejectsResponsesMode(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://api.moonshot.cn/v1"},
		mode.Responses,
		"moonshot-v1-8k",
		coremodel.ModelConfig{},
	)

	if _, err := adaptor.GetRequestURL(m, nil, nil); err == nil {
		t.Fatal("expected responses mode GetRequestURL to fail")
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/responses",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if _, err := adaptor.ConvertRequest(m, nil, req); err == nil {
		t.Fatal("expected responses mode ConvertRequest to fail")
	}
}

func TestConvertChatCompletionsRequestMapsReasoningToKimiThinking(t *testing.T) {
	adaptor := &Adaptor{}
	m := &meta.Meta{
		OriginModel: "moonshot-v1-8k",
		Mode:        mode.ChatCompletions,
		ActualModel: "kimi-k2.6",
	}
	req := newJSONRequest(t, "/v1/chat/completions", `{
		"model":"moonshot-v1-8k",
		"messages":[{"role":"user","content":"hello"}],
		"reasoning_effort":"high"
	}`)

	payload := convertAndDecodePayload(t, adaptor, m, req)

	if _, exists := payload["reasoning_effort"]; exists {
		t.Fatal("expected reasoning_effort to be removed")
	}

	assertThinkingType(t, payload, "enabled")
}

func TestConvertGeminiRequestMapsThinkingToKimiThinking(t *testing.T) {
	adaptor := &Adaptor{}
	m := &meta.Meta{
		Mode:        mode.Gemini,
		ActualModel: "kimi-k2.6",
	}
	req := newJSONRequest(t, "/v1beta/models/gemini-2.5-pro:generateContent", `{
		"generationConfig": {
			"thinkingConfig": {
				"thinkingBudget": 2048,
				"includeThoughts": true
			}
		},
		"contents": [{"role":"user","parts":[{"text":"hello"}]}]
	}`)

	payload := convertAndDecodePayload(t, adaptor, m, req)

	if _, exists := payload["reasoning_effort"]; exists {
		t.Fatal("expected reasoning_effort to be removed")
	}

	assertThinkingType(t, payload, "enabled")
}

func TestConvertAnthropicRequestMapsThinkingToKimiThinking(t *testing.T) {
	adaptor := &Adaptor{}
	m := &meta.Meta{
		Mode:        mode.Anthropic,
		ActualModel: "kimi-k2.6",
	}
	req := newJSONRequest(t, "/v1/messages", `{
		"model":"claude-sonnet-4-5",
		"max_tokens":4096,
		"thinking":{"type":"disabled"},
		"messages":[{"role":"user","content":"hello"}]
	}`)

	payload := convertAndDecodePayload(t, adaptor, m, req)

	if _, exists := payload["reasoning_effort"]; exists {
		t.Fatal("expected reasoning_effort to be removed")
	}

	assertThinkingType(t, payload, "disabled")
}

func TestConvertChatCompletionsRequestDropsReasoningForNonToggleKimiModel(t *testing.T) {
	adaptor := &Adaptor{}
	m := &meta.Meta{
		Mode:        mode.ChatCompletions,
		ActualModel: "kimi-k2-thinking",
	}
	req := newJSONRequest(t, "/v1/chat/completions", `{
		"model":"kimi-k2-thinking",
		"messages":[{"role":"user","content":"hello"}],
		"reasoning_effort":"none"
	}`)

	payload := convertAndDecodePayload(t, adaptor, m, req)

	if _, exists := payload["reasoning_effort"]; exists {
		t.Fatal("expected reasoning_effort to be removed")
	}

	if _, exists := payload["thinking"]; exists {
		t.Fatal("expected thinking to be omitted for non-toggle Kimi model")
	}
}

func newJSONRequest(t *testing.T, path, body string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		path,
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req
}

func convertAndDecodePayload(
	t *testing.T,
	adaptor *Adaptor,
	m *meta.Meta,
	req *http.Request,
) map[string]any {
	t.Helper()

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	bodyBytes, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	return payload
}

func assertThinkingType(t *testing.T, payload map[string]any, expected string) {
	t.Helper()

	thinking, ok := payload["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("expected thinking object, got %#v", payload["thinking"])
	}

	if thinking["type"] != expected {
		t.Fatalf("expected thinking.type=%s, got %#v", expected, thinking["type"])
	}
}
