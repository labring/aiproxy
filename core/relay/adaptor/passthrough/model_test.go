package passthrough_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/passthrough"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func newMeta(origin, actual string) *meta.Meta {
	ch := &model.Channel{Type: model.ChannelTypePPIO}
	m := meta.NewMeta(ch, mode.ChatCompletions, origin, model.ModelConfig{})
	m.ActualModel = actual

	return m
}

func TestReplaceModelInBody_WithMapping(t *testing.T) {
	m := newMeta("claude-sonnet-4-6", "claude-sonnet-4-20250514")
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`)

	result, replaced, err := passthrough.ReplaceModelInBody(m, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !replaced {
		t.Fatal("expected modelReplaced=true")
	}

	got, _ := io.ReadAll(result)

	if !bytes.Contains(got, []byte(`"model":"claude-sonnet-4-20250514"`)) {
		t.Fatalf("model not replaced:\ngot: %s", got)
	}

	if bytes.Contains(got, []byte(`"model":"claude-sonnet-4-6"`)) {
		t.Fatalf("original model still present:\ngot: %s", got)
	}

	if !bytes.Contains(got, []byte(`"messages"`)) {
		t.Fatalf("messages field lost:\ngot: %s", got)
	}
}

func TestReplaceModelInBody_NoMapping(t *testing.T) {
	m := newMeta("gpt-4o", "gpt-4o")
	body := []byte(`{"model":"gpt-4o","messages":[]}`)

	result, replaced, err := passthrough.ReplaceModelInBody(m, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if replaced {
		t.Fatal("expected modelReplaced=false when no mapping")
	}

	got, _ := io.ReadAll(result)

	if string(got) != string(body) {
		t.Fatalf("body should be unchanged:\nwant: %s\ngot:  %s", body, got)
	}
}

func TestReplaceModelInBody_NonJSON(t *testing.T) {
	m := newMeta("whisper-1", "whisper-1-mapped")
	body := []byte("--boundary\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\naudio data\r\n--boundary--")

	result, replaced, err := passthrough.ReplaceModelInBody(m, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if replaced {
		t.Fatal("expected modelReplaced=false for non-JSON body")
	}

	got, _ := io.ReadAll(result)

	if !bytes.Equal(got, body) {
		t.Fatalf("non-JSON body should be returned as-is:\nwant: %s\ngot:  %s", body, got)
	}
}

func TestReplaceModelInBody_NilBody(t *testing.T) {
	m := newMeta("model-a", "model-b")

	result, replaced, err := passthrough.ReplaceModelInBody(m, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if replaced {
		t.Fatal("expected modelReplaced=false for nil body")
	}

	if result != http.NoBody {
		t.Fatal("expected http.NoBody for nil input")
	}
}

func TestReplaceModelInBody_EmptyBody(t *testing.T) {
	m := newMeta("model-a", "model-b")

	result, replaced, err := passthrough.ReplaceModelInBody(m, io.NopCloser(bytes.NewReader(nil)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if replaced {
		t.Fatal("expected modelReplaced=false for empty body")
	}

	if result != http.NoBody {
		t.Fatal("expected http.NoBody for empty input")
	}
}

func TestReplaceModelInBody_NoModelField(t *testing.T) {
	m := newMeta("model-a", "model-b")
	body := []byte(`{"input":"some text","encoding_format":"float"}`)

	result, replaced, err := passthrough.ReplaceModelInBody(m, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if replaced {
		t.Fatal("expected modelReplaced=false when no model field")
	}

	got, _ := io.ReadAll(result)

	if !bytes.Equal(got, body) {
		t.Fatalf("body without model field should be returned as-is:\nwant: %s\ngot:  %s", body, got)
	}
}
