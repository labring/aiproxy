package anthropic_test

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestAdaptorConvertRequest_PurePassthroughPreservesAnthropicBody(t *testing.T) {
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
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))

	result, err := a.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	gotBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	if string(gotBody) != string(body) {
		t.Fatalf("pure passthrough body changed:\nwant: %s\ngot:  %s", body, gotBody)
	}

	if got := result.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	if got := result.Header.Get("Content-Length"); got != strconv.Itoa(len(body)) {
		t.Fatalf("Content-Length = %q, want %d", got, len(body))
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
