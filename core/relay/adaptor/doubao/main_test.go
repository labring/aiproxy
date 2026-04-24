//nolint:testpackage
package doubao

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

func TestAdaptorSupportMode(t *testing.T) {
	adaptor := &Adaptor{}

	supportedModes := []mode.Mode{
		mode.ChatCompletions,
		mode.Anthropic,
		mode.Gemini,
		mode.Embeddings,
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

	unsupportedModes := []mode.Mode{
		mode.Completions,
		mode.ImagesGenerations,
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
		BaseURL: "https://ark.cn-beijing.volces.com",
	}

	tests := []struct {
		name       string
		mode       mode.Mode
		model      string
		responseID string
		wantMethod string
		wantURL    string
	}{
		{
			name:       "gemini uses chat completions",
			mode:       mode.Gemini,
			model:      "doubao-seed-1-6",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
		},
		{
			name:       "gemini bot uses bot chat completions",
			mode:       mode.Gemini,
			model:      "bot-123",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/bots/chat/completions",
		},
		{
			name:       "responses create",
			mode:       mode.Responses,
			model:      "doubao-seed-1-6",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses",
		},
		{
			name:       "responses get",
			mode:       mode.ResponsesGet,
			model:      "doubao-seed-1-6",
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses/resp_123",
		},
		{
			name:       "responses delete",
			mode:       mode.ResponsesDelete,
			model:      "doubao-seed-1-6",
			responseID: "resp_123",
			wantMethod: http.MethodDelete,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses/resp_123",
		},
		{
			name:       "responses cancel",
			mode:       mode.ResponsesCancel,
			model:      "doubao-seed-1-6",
			responseID: "resp_123",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses/resp_123/cancel",
		},
		{
			name:       "responses input items",
			mode:       mode.ResponsesInputItems,
			model:      "doubao-seed-1-6",
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses/resp_123/input_items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(
				channel,
				tt.mode,
				tt.model,
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

func TestAdaptorConvertRequestGemini(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Gemini,
		"doubao-seed-1-6",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/doubao-seed-1-6:streamGenerateContent",
		strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`),
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if openAIReq.Model != "doubao-seed-1-6" {
		t.Fatalf("expected model doubao-seed-1-6, got %s", openAIReq.Model)
	}

	if !openAIReq.Stream {
		t.Fatal("expected stream to be enabled")
	}

	if len(openAIReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(openAIReq.Messages))
	}

	if openAIReq.Messages[0].Role != relaymodel.RoleUser {
		t.Fatalf("expected user message, got %s", openAIReq.Messages[0].Role)
	}
}
