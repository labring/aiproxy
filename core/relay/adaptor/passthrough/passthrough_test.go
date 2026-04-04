package passthrough

import (
	"bytes"
	"strings"
	"testing"
)

// ─── flushCopy tests ─────────────────────────────────────────────────────────

// testFlusherWriter implements flusherWriter for testing.
type testFlusherWriter struct {
	buf     bytes.Buffer
	flushes int
}

func (w *testFlusherWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *testFlusherWriter) Flush()                      { w.flushes++ }

func TestFlushCopy_CopiesAllBytes(t *testing.T) {
	w := &testFlusherWriter{}

	n, err := flushCopy(w, strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != 11 {
		t.Errorf("written bytes: want 11, got %d", n)
	}

	if got := w.buf.String(); got != "hello world" {
		t.Errorf("body: want %q, got %q", "hello world", got)
	}
}

func TestFlushCopy_FlushesAfterEachWrite(t *testing.T) {
	w := &testFlusherWriter{}
	payload := "data: event1\n\ndata: event2\n\ndata: [DONE]\n\n"

	_, _ = flushCopy(w, strings.NewReader(payload))

	if w.flushes == 0 {
		t.Error("flushCopy did not call Flush at all")
	}
}

// ─── ring buffer tests (B3) ──────────────────────────────────────────────────

func TestRingBuffer_SmallWrite(t *testing.T) {
	rb := newRingBuffer(8)
	rb.Write([]byte("abcd"))
	if got := string(rb.Bytes()); got != "abcd" {
		t.Fatalf("want abcd, got %q", got)
	}
}

func TestRingBuffer_ExactFill(t *testing.T) {
	rb := newRingBuffer(4)
	rb.Write([]byte("abcd"))
	if got := string(rb.Bytes()); got != "abcd" {
		t.Fatalf("want abcd, got %q", got)
	}
}

func TestRingBuffer_OverflowKeepsLastN(t *testing.T) {
	rb := newRingBuffer(4)
	rb.Write([]byte("abcdefgh")) // 8 bytes into 4-byte buffer → keep "efgh"
	if got := string(rb.Bytes()); got != "efgh" {
		t.Fatalf("want efgh (last 4), got %q", got)
	}
}

func TestRingBuffer_MultipleWrites(t *testing.T) {
	rb := newRingBuffer(8)
	rb.Write([]byte("12345678")) // fills exactly
	rb.Write([]byte("abcd"))    // wraps around, overwrites "1234"

	got := string(rb.Bytes()) // should be "5678abcd"
	if got != "5678abcd" {
		t.Fatalf("want 5678abcd, got %q", got)
	}
}

// B3: ring buffer preserves the LAST N bytes, not the first N.
func TestRingBuffer_LargeStreamKeepsLastBytes(t *testing.T) {
	// Use a ring buffer large enough to hold the usage chunk at the end of
	// the stream, but much smaller than the total stream size.
	usageChunk := `data: {"id":"x","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}` + "\n"
	rb := newRingBuffer(len(usageChunk) + 16)

	// Simulate 2KB of prior stream data that would overflow the buffer.
	rb.Write([]byte(strings.Repeat("x", 2048)))
	// The usage chunk arrives last.
	rb.Write([]byte(usageChunk))

	tail := string(rb.Bytes())
	if !strings.Contains(tail, `"usage"`) {
		t.Fatalf("ring buffer lost usage from tail; tail=%q", tail)
	}
}

// ─── usage extraction tests ──────────────────────────────────────────────────

// B2: Responses API usage is nested under response.usage, not at top level.
func TestExtractUsage_ResponsesAPI_Nested(t *testing.T) {
	// This is the actual Responses API response.completed event payload.
	payload := `data: {"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":12,"output_tokens":11}}}` + "\n\ndata: [DONE]\n"

	u := extractUsageFromTail([]byte(payload))
	if int64(u.InputTokens) != 12 {
		t.Errorf("InputTokens: want 12, got %d", u.InputTokens)
	}

	if int64(u.OutputTokens) != 11 {
		t.Errorf("OutputTokens: want 11, got %d", u.OutputTokens)
	}
}

// Standard OpenAI streaming usage (last SSE chunk before [DONE]).
func TestExtractUsage_OpenAI_Stream(t *testing.T) {
	payload := `data: {"id":"chatcmpl-1","choices":[],"usage":{"prompt_tokens":14,"completion_tokens":12,"total_tokens":26}}` + "\ndata: [DONE]\n"

	u := extractUsageFromTail([]byte(payload))
	if int64(u.InputTokens) != 14 {
		t.Errorf("InputTokens: want 14, got %d", u.InputTokens)
	}

	if int64(u.OutputTokens) != 12 {
		t.Errorf("OutputTokens: want 12, got %d", u.OutputTokens)
	}

	if int64(u.TotalTokens) != 26 {
		t.Errorf("TotalTokens: want 26, got %d", u.TotalTokens)
	}
}

// OpenAI non-streaming — usage is top-level in the JSON body.
func TestExtractUsage_OpenAI_NonStream(t *testing.T) {
	payload := `{"id":"chatcmpl-1","choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`

	u := extractUsageFromTail([]byte(payload))
	if int64(u.InputTokens) != 5 {
		t.Errorf("InputTokens: want 5, got %d", u.InputTokens)
	}

	if int64(u.OutputTokens) != 3 {
		t.Errorf("OutputTokens: want 3, got %d", u.OutputTokens)
	}
}

// Reasoning model: completion_tokens_details.reasoning_tokens.
func TestExtractUsage_ReasoningModel(t *testing.T) {
	payload := `data: {"usage":{"prompt_tokens":10,"completion_tokens":50,"total_tokens":60,"completion_tokens_details":{"reasoning_tokens":45}}}` + "\n"

	u := extractUsageFromTail([]byte(payload))
	if int64(u.ReasoningTokens) != 45 {
		t.Errorf("ReasoningTokens: want 45, got %d", u.ReasoningTokens)
	}
}

// Anthropic streaming: usage comes in message_delta event.
func TestExtractUsage_Anthropic_Stream(t *testing.T) {
	payload := "event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":12}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n"

	u := extractUsageFromTail([]byte(payload))
	if int64(u.OutputTokens) != 12 {
		t.Errorf("OutputTokens: want 12, got %d", u.OutputTokens)
	}
}

// Anthropic non-streaming: usage is top-level.
func TestExtractUsage_Anthropic_NonStream(t *testing.T) {
	payload := `{"id":"msg_1","usage":{"input_tokens":14,"output_tokens":12,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`

	u := extractUsageFromTail([]byte(payload))
	if int64(u.InputTokens) != 14 {
		t.Errorf("InputTokens: want 14, got %d", u.InputTokens)
	}

	if int64(u.OutputTokens) != 12 {
		t.Errorf("OutputTokens: want 12, got %d", u.OutputTokens)
	}
}

// B5: Anthropic cache_creation nested object form.
func TestExtractUsage_Anthropic_CacheCreation_Nested(t *testing.T) {
	// In the real pa/claude-sonnet-4-6 response, cache_creation is a nested object.
	payload := `{"id":"msg_1","usage":{"input_tokens":20,"output_tokens":5,"cache_read_input_tokens":10,"cache_creation_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":8,"ephemeral_1h_input_tokens":3}}}`

	u := extractUsageFromTail([]byte(payload))
	if int64(u.CachedTokens) != 10 {
		t.Errorf("CachedTokens: want 10 (cache_read_input_tokens), got %d", u.CachedTokens)
	}

	// cache_creation_input_tokens=0 so falls back to the nested object total (8+3=11).
	if int64(u.CacheCreationTokens) != 11 {
		t.Errorf("CacheCreationTokens: want 11 (nested sum 8+3), got %d", u.CacheCreationTokens)
	}
}

// Null details fields should not panic (deepseek-v3 non-streaming behavior).
func TestExtractUsage_NullDetails(t *testing.T) {
	payload := `{"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":null,"completion_tokens_details":null}}`

	u := extractUsageFromTail([]byte(payload))
	if int64(u.InputTokens) != 10 {
		t.Errorf("InputTokens: want 10, got %d", u.InputTokens)
	}
}

// Empty / missing usage should return zero Usage without panic.
func TestExtractUsage_Missing(t *testing.T) {
	u := extractUsageFromTail([]byte(`data: {"choices":[]}`))
	if int64(u.InputTokens) != 0 || int64(u.OutputTokens) != 0 {
		t.Errorf("expected zero usage, got %+v", u)
	}
}

// ─── URL building tests ───────────────────────────────────────────────────────

func TestStripV1Prefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/v1/chat/completions", "/chat/completions"},
		{"/v1/embeddings", "/embeddings"},
		{"/v1/responses/resp_abc", "/responses/resp_abc"},
		{"/v1", "/"},
		{"/v2/something", "/v2/something"},
		{"/v1beta/models/gemini", "/v1beta/models/gemini"}, // not /v1 exactly
	}

	for _, tc := range cases {
		got := stripV1Prefix(tc.in)
		if got != tc.want {
			t.Errorf("stripV1Prefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMatchPathBaseMap_LongestPrefixWins(t *testing.T) {
	pbm := map[string]string{
		"/v1/responses":  "https://api.ppinfra.com/openai/v1",
		"/v1/web-search": "https://api.ppinfra.com/v3",
	}

	cases := []struct {
		path       string
		wantBase   string
		wantSuffix string
		wantOK     bool
	}{
		{
			"/v1/responses",
			"https://api.ppinfra.com/openai/v1", "/responses", true,
		},
		{
			"/v1/responses/resp_abc",
			"https://api.ppinfra.com/openai/v1", "/responses/resp_abc", true,
		},
		{
			"/v1/responses/resp_abc/input_items",
			"https://api.ppinfra.com/openai/v1", "/responses/resp_abc/input_items", true,
		},
		{
			"/v1/web-search",
			"https://api.ppinfra.com/v3", "/web-search", true,
		},
		{
			"/v1/chat/completions",
			"", "", false,
		},
	}

	for _, tc := range cases {
		base, suffix, ok := matchPathBaseMap(pbm, tc.path)
		if ok != tc.wantOK {
			t.Errorf("matchPathBaseMap(%q) ok=%v, want %v", tc.path, ok, tc.wantOK)
			continue
		}

		if ok && (base != tc.wantBase || suffix != tc.wantSuffix) {
			t.Errorf("matchPathBaseMap(%q) = (%q, %q), want (%q, %q)",
				tc.path, base, suffix, tc.wantBase, tc.wantSuffix)
		}
	}
}
