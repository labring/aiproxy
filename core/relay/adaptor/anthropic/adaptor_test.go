package anthropic_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestAdaptorConvertRequest_PurePassthroughReplacesModel(t *testing.T) {
	a := &anthropic.Adaptor{}
	channel := &model.Channel{
		Type: model.ChannelTypeAnthropic,
		Configs: model.ChannelConfigs{
			"pure_passthrough": true,
		},
	}
	m := meta.NewMeta(channel, mode.Anthropic, "client-model", model.ModelConfig{})
	m.ActualModel = "upstream-model"

	body := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hello"}]}`)
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	result, err := a.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	gotBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	if !bytes.Contains(gotBody, []byte(`"model":"upstream-model"`)) {
		t.Fatalf("pure passthrough did not replace model:\ngot: %s", gotBody)
	}

	if bytes.Contains(gotBody, []byte(`"model":"client-model"`)) {
		t.Fatalf("pure passthrough still contains original model:\ngot: %s", gotBody)
	}

	// Messages should be preserved unchanged
	if !bytes.Contains(gotBody, []byte(`"messages"`)) {
		t.Fatalf("pure passthrough lost messages field:\ngot: %s", gotBody)
	}

	if got := result.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	// Content-Length should NOT be forwarded (model replacement changes body size)
	if got := result.Header.Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length should not be set, got %q", got)
	}
}

func TestAdaptorConvertRequest_PurePassthroughNoMappingPreservesBody(t *testing.T) {
	a := &anthropic.Adaptor{}
	channel := &model.Channel{
		Type: model.ChannelTypeAnthropic,
		Configs: model.ChannelConfigs{
			"pure_passthrough": true,
		},
	}
	m := meta.NewMeta(channel, mode.Anthropic, "same-model", model.ModelConfig{})
	m.ActualModel = "same-model" // no mapping

	body := []byte(`{"model":"same-model","messages":[{"role":"user","content":"hello"}]}`)
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	result, err := a.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	gotBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	if string(gotBody) != string(body) {
		t.Fatalf("body should be unchanged when no mapping:\nwant: %s\ngot:  %s", body, gotBody)
	}
}

func TestAdaptorConvertRequest_NonPureAnthropicRewritesBody(t *testing.T) {
	a := &anthropic.Adaptor{}
	channel := &model.Channel{
		Type: model.ChannelTypeAnthropic,
	}
	m := meta.NewMeta(channel, mode.Anthropic, "client-model", model.ModelConfig{})
	m.ActualModel = "claude-sonnet-4-20250514"

	body := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hello"}]}`)
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	result, err := a.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	gotBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	if bytes.Equal(gotBody, body) {
		t.Fatalf("expected non-pure anthropic conversion to rewrite the request body")
	}

	if !bytes.Contains(gotBody, []byte(`"model":"claude-sonnet-4-20250514"`)) {
		t.Fatalf("converted body did not rewrite model: %s", gotBody)
	}

	if !bytes.Contains(gotBody, []byte(`"max_tokens":64000`)) {
		t.Fatalf("converted body did not inject default max_tokens: %s", gotBody)
	}

	if got := result.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}
