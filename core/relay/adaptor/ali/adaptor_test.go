//nolint:testpackage
package ali

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
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func TestAdaptorSupportModeResponses(t *testing.T) {
	adaptor := &Adaptor{}

	supportedModes := []mode.Mode{
		mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems,
	}
	for _, m := range supportedModes {
		if !adaptor.SupportMode(m) {
			t.Fatalf("expected mode %s to be supported", m)
		}
	}
}

func TestAdaptorGetRequestURLResponses(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{BaseURL: "https://dashscope.aliyuncs.com"}

	tests := []struct {
		name       string
		mode       mode.Mode
		responseID string
		wantMethod string
		wantURL    string
	}{
		{
			name:       "responses create",
			mode:       mode.Responses,
			wantMethod: http.MethodPost,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses",
		},
		{
			name:       "responses get",
			mode:       mode.ResponsesGet,
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses/resp_123",
		},
		{
			name:       "responses delete",
			mode:       mode.ResponsesDelete,
			responseID: "resp_123",
			wantMethod: http.MethodDelete,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses/resp_123",
		},
		{
			name:       "responses cancel",
			mode:       mode.ResponsesCancel,
			responseID: "resp_123",
			wantMethod: http.MethodPost,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses/resp_123/cancel",
		},
		{
			name:       "responses input items",
			mode:       mode.ResponsesInputItems,
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses/resp_123/input_items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(
				channel,
				tt.mode,
				"qwen-plus",
				coremodel.ModelConfig{},
				meta.WithResponseID(tt.responseID),
			)

			got, err := adaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != tt.wantMethod {
				t.Fatalf("expected method %s, got %s", tt.wantMethod, got.Method)
			}

			if got.URL != tt.wantURL {
				t.Fatalf("expected URL %s, got %s", tt.wantURL, got.URL)
			}
		})
	}
}

func TestAdaptorConvertRequestResponses(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Responses,
		"qwen-plus",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/responses",
		strings.NewReader(`{"model":"qwen-plus","input":"hello","stream":true}`),
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

	var responseReq relaymodel.CreateResponseRequest
	if err := json.Unmarshal(body, &responseReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if responseReq.Model != "qwen-plus" {
		t.Fatalf("expected model qwen-plus, got %s", responseReq.Model)
	}

	if !responseReq.Stream {
		t.Fatal("expected stream to remain enabled")
	}
}

func TestAdaptorConvertGeminiRequestReasoning(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Gemini,
		"glm-4.5",
		coremodel.ModelConfig{},
	)

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
			"contents": [{"role":"user","parts":[{"text":"hello"}]}]
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

	enableThinking, ok := payload["enable_thinking"].(bool)
	if !ok || !enableThinking {
		t.Fatalf("expected enable_thinking=true, got %#v", payload["enable_thinking"])
	}

	thinkingBudget, ok := payload["thinking_budget"].(float64)
	if !ok || int(thinkingBudget) != 2048 {
		t.Fatalf("expected thinking_budget=2048, got %#v", payload["thinking_budget"])
	}

	if _, exists := payload["reasoning_effort"]; exists {
		t.Fatal("expected reasoning_effort to be removed")
	}
}

func TestAdaptorConvertChatCompletionsReasoningNotClampedByMaxTokens(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"glm-4.5",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"glm-4.5",
			"reasoning_effort":"high",
			"max_tokens":1000,
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

	enableThinking, ok := payload["enable_thinking"].(bool)
	if !ok || !enableThinking {
		t.Fatalf("expected enable_thinking=true, got %#v", payload["enable_thinking"])
	}

	thinkingBudget, ok := payload["thinking_budget"].(float64)
	if !ok || int(thinkingBudget) != 16384 {
		t.Fatalf("expected thinking_budget=16384, got %#v", payload["thinking_budget"])
	}
}

func TestAdaptorConvertChatCompletionsReasoningUsesOriginModelNameFirst(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"glm-4.5",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "mapped-upstream-model"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"glm-4.5",
			"reasoning_effort":"high",
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

	enableThinking, ok := payload["enable_thinking"].(bool)
	if !ok || !enableThinking {
		t.Fatalf("expected enable_thinking=true, got %#v", payload["enable_thinking"])
	}

	thinkingBudget, ok := payload["thinking_budget"].(float64)
	if !ok || int(thinkingBudget) != 16384 {
		t.Fatalf("expected thinking_budget=16384, got %#v", payload["thinking_budget"])
	}
}

func TestAdaptorConvertCompletionsReasoning(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Completions,
		"glm-4.5",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/completions",
		strings.NewReader(`{
			"model":"glm-4.5",
			"reasoning_effort":"low",
			"prompt":"hello"
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

	enableThinking, ok := payload["enable_thinking"].(bool)
	if !ok || !enableThinking {
		t.Fatalf("expected enable_thinking=true, got %#v", payload["enable_thinking"])
	}
}
