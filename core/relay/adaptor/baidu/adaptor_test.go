//nolint:testpackage
package baidu

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

func TestAdaptorSupportModeAnthropicAndGemini(t *testing.T) {
	adaptor := &Adaptor{}

	if !adaptor.SupportMode(mode.Anthropic) {
		t.Fatal("expected Anthropic mode to be supported")
	}

	if !adaptor.SupportMode(mode.Gemini) {
		t.Fatal("expected Gemini mode to be supported")
	}
}

func TestAdaptorGetRequestURLAnthropicAndGemini(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{BaseURL: "https://aip.baidubce.com"}

	tests := []struct {
		name string
		mode mode.Mode
	}{
		{name: "anthropic", mode: mode.Anthropic},
		{name: "gemini", mode: mode.Gemini},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(channel, tt.mode, "ERNIE-4.0-8K", coremodel.ModelConfig{})

			got, err := adaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != http.MethodPost {
				t.Fatalf("expected method %s, got %s", http.MethodPost, got.Method)
			}

			wantURL := "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/completions_pro"
			if got.URL != wantURL {
				t.Fatalf("expected URL %s, got %s", wantURL, got.URL)
			}
		})
	}
}

func TestAdaptorConvertRequestAnthropic(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.Anthropic, "ERNIE-4.0-8K", coremodel.ModelConfig{})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/messages",
		strings.NewReader(
			`{"model":"ERNIE-4.0-8K","messages":[{"role":"user","content":"hello"}],"max_tokens":10}`,
		),
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

	var baiduReq ChatRequest
	if err := json.Unmarshal(body, &baiduReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if len(baiduReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(baiduReq.Messages))
	}

	if baiduReq.Messages[0].Role != "user" {
		t.Fatalf("expected user message, got %s", baiduReq.Messages[0].Role)
	}
}

func TestAdaptorConvertRequestGemini(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.Gemini, "ERNIE-4.0-8K", coremodel.ModelConfig{})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/ERNIE-4.0-8K:streamGenerateContent",
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

	var baiduReq ChatRequest
	if err := json.Unmarshal(body, &baiduReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if !baiduReq.Stream {
		t.Fatal("expected stream to be enabled")
	}

	if len(baiduReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(baiduReq.Messages))
	}
}
