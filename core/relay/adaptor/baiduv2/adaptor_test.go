//nolint:testpackage
package baiduv2

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

func TestAdaptorSupportModeGemini(t *testing.T) {
	adaptor := &Adaptor{}

	if !adaptor.SupportMode(mode.Anthropic) {
		t.Fatal("expected Anthropic mode to be supported")
	}

	if !adaptor.SupportMode(mode.Gemini) {
		t.Fatal("expected Gemini mode to be supported")
	}
}

func TestAdaptorGetRequestURLAnthropic(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://qianfan.baidubce.com/v2"},
		mode.Anthropic,
		"ERNIE-4.0-8K",
		coremodel.ModelConfig{},
	)

	got, err := adaptor.GetRequestURL(m, nil, nil)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	if got.Method != http.MethodPost {
		t.Fatalf("expected method %s, got %s", http.MethodPost, got.Method)
	}

	wantURL := "https://qianfan.baidubce.com/v2/chat/completions"
	if got.URL != wantURL {
		t.Fatalf("expected URL %s, got %s", wantURL, got.URL)
	}
}

func TestAdaptorConvertRequestAnthropic(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Anthropic,
		"ERNIE-Character-8K",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/messages",
		strings.NewReader(
			`{"model":"ERNIE-Character-8K","messages":[{"role":"user","content":"hello"}],"max_tokens":10}`,
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if openAIReq.Model != "ernie-char-8k" {
		t.Fatalf("expected mapped model ernie-char-8k, got %s", openAIReq.Model)
	}

	if len(openAIReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(openAIReq.Messages))
	}
}

func TestAdaptorGetRequestURLGemini(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://qianfan.baidubce.com/v2"},
		mode.Gemini,
		"ERNIE-4.0-8K",
		coremodel.ModelConfig{},
	)

	got, err := adaptor.GetRequestURL(m, nil, nil)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	if got.Method != http.MethodPost {
		t.Fatalf("expected method %s, got %s", http.MethodPost, got.Method)
	}

	wantURL := "https://qianfan.baidubce.com/v2/chat/completions"
	if got.URL != wantURL {
		t.Fatalf("expected URL %s, got %s", wantURL, got.URL)
	}
}

func TestAdaptorConvertRequestGemini(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Gemini,
		"ERNIE-Character-8K",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/ERNIE-Character-8K:streamGenerateContent",
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

	if openAIReq.Model != "ernie-char-8k" {
		t.Fatalf("expected mapped model ernie-char-8k, got %s", openAIReq.Model)
	}

	if !openAIReq.Stream {
		t.Fatal("expected stream to be enabled")
	}
}
