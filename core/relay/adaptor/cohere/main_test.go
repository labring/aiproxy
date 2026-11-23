//nolint:testpackage
package cohere

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func TestStreamResponse2OpenAIMapsFinishReason(t *testing.T) {
	t.Parallel()

	reason := "COMPLETE"
	resp := StreamResponse2OpenAI(&meta.Meta{}, &StreamResponse{
		EventType: "stream-end",
		Response:  &Response{FinishReason: &reason},
	})

	if resp == nil || len(resp.Choices) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if got := resp.Choices[0].FinishReason; got != relaymodel.FinishReasonStop {
		t.Fatalf("finish reason = %q, want %q", got, relaymodel.FinishReasonStop)
	}
}

func TestStreamResponse2OpenAINilFinishReason(t *testing.T) {
	t.Parallel()

	resp := StreamResponse2OpenAI(&meta.Meta{}, &StreamResponse{
		EventType: "stream-end",
		Response:  &Response{},
	})

	if resp == nil || resp.Choices[0].FinishReason != "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestStopReasonCohere2OpenAI(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		reason *string
		want   string
	}{
		{nil, ""},
		{new("COMPLETE"), relaymodel.FinishReasonStop},
		{new("MAX_TOKENS"), relaymodel.FinishReasonLength},
		{new("ERROR"), relaymodel.FinishReasonStop},
		{new("ERROR_TOXIC"), relaymodel.FinishReasonContentFilter},
		{new("STOP_SEQUENCE"), relaymodel.FinishReasonStop},
		{new("COMPLETE_UNEXPECTED"), "COMPLETE_UNEXPECTED"},
	} {
		if got := stopReasonCohere2OpenAI(tt.reason); got != tt.want {
			t.Fatalf("stopReasonCohere2OpenAI(%v) = %q, want %q", tt.reason, got, tt.want)
		}
	}
}

func TestStreamHandlerSkipsEventsWithoutChunk(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`{"event_type":"stream-start","generation_id":"g1","is_finished":false}`,
		`{"event_type":"text-generation","text":"hi","is_finished":false}`,
		`{"event_type":"search-queries-generation","is_finished":false}`,
		`{"event_type":"stream-end","finish_reason":"COMPLETE","is_finished":true,"response":{"finish_reason":"COMPLETE","meta":{"tokens":{"input_tokens":3,"output_tokens":2}}}}`,
	}, "\n")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

	result, err := StreamHandler(&meta.Meta{}, c, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Usage.InputTokens != 3 || result.Usage.OutputTokens != 2 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}

	if !strings.Contains(w.Body.String(), `"content":"hi"`) {
		t.Fatalf("missing text chunk: %s", w.Body.String())
	}
}

func TestConvertRequestKeepsEarlierUserTurnsInHistory(t *testing.T) {
	t.Parallel()

	req := &relaymodel.GeneralOpenAIRequest{
		Model: "command-r",
		Messages: []relaymodel.Message{
			{Role: "system", Content: "be brief"},
			{Role: "user", Content: "first question"},
			{Role: "assistant", Content: "first answer"},
			{Role: "user", Content: "second question"},
		},
	}

	got := ConvertRequest(req)

	if got.Message != "second question" {
		t.Fatalf("Message = %q, want %q", got.Message, "second question")
	}

	want := []ChatMessage{
		{Role: "SYSTEM", Message: "be brief"},
		{Role: "USER", Message: "first question"},
		{Role: "CHATBOT", Message: "first answer"},
	}
	if len(got.ChatHistory) != len(want) {
		t.Fatalf("ChatHistory = %+v, want %+v", got.ChatHistory, want)
	}

	for i := range want {
		if got.ChatHistory[i] != want[i] {
			t.Fatalf("ChatHistory[%d] = %+v, want %+v", i, got.ChatHistory[i], want[i])
		}
	}
}

func TestConvertRequestArrayContent(t *testing.T) {
	t.Parallel()

	parts := func(texts ...string) []any {
		out := make([]any, 0, len(texts))
		for _, s := range texts {
			out = append(out, map[string]any{"type": "text", "text": s})
		}

		return out
	}

	req := ConvertRequest(&relaymodel.GeneralOpenAIRequest{
		Model: "command-r",
		Messages: []relaymodel.Message{
			{Role: "system", Content: parts("be brief")},
			{Role: "user", Content: parts("first question")},
			{Role: "assistant", Content: parts("first answer")},
			{Role: "user", Content: parts("Hello, ", "world")},
		},
	})

	if got, want := req.Message, "Hello, \nworld"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	if len(req.ChatHistory) != 3 {
		t.Fatalf("chat_history len = %d, want 3", len(req.ChatHistory))
	}

	for i, want := range []string{"be brief", "first question", "first answer"} {
		if got := req.ChatHistory[i].Message; got != want {
			t.Fatalf("chat_history[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestConvertRequestStopSequences(t *testing.T) {
	t.Parallel()

	convert := func(stop any) *Request {
		return ConvertRequest(&relaymodel.GeneralOpenAIRequest{
			Model: "command-r",
			Messages: []relaymodel.Message{
				{Role: "user", Content: "hi"},
			},
			Stop: stop,
		})
	}

	assertStop := func(t *testing.T, got []string, want ...string) {
		t.Helper()

		if len(got) != len(want) {
			t.Fatalf("StopSequences = %#v, want %#v", got, want)
		}

		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("StopSequences = %#v, want %#v", got, want)
			}
		}
	}

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		got := convert("END")
		if got.Message != "hi" {
			t.Fatalf("Message = %q, want %q", got.Message, "hi")
		}

		assertStop(t, got.StopSequences, "END")
	})

	t.Run("array", func(t *testing.T) {
		t.Parallel()
		assertStop(t, convert([]any{"END", "STOP"}).StopSequences, "END", "STOP")
	})

	t.Run("string slice", func(t *testing.T) {
		t.Parallel()
		assertStop(t, convert([]string{"END", "STOP"}).StopSequences, "END", "STOP")
	})

	t.Run("skip non-strings", func(t *testing.T) {
		t.Parallel()
		assertStop(t, convert([]any{"END", 3, "STOP"}).StopSequences, "END", "STOP")
	})

	t.Run("absent", func(t *testing.T) {
		t.Parallel()

		if got := convert(nil).StopSequences; got != nil {
			t.Fatalf("StopSequences = %#v, want nil", got)
		}

		if got := convert([]any{}).StopSequences; got != nil {
			t.Fatalf("StopSequences = %#v, want nil", got)
		}

		if got := convert([]any{1, true}).StopSequences; got != nil {
			t.Fatalf("StopSequences = %#v, want nil", got)
		}
	})
}

func TestConvertRequestMaxCompletionTokens(t *testing.T) {
	t.Parallel()

	req := &relaymodel.GeneralOpenAIRequest{
		Model:               "command-r",
		Messages:            []relaymodel.Message{{Role: "user", Content: "hi"}},
		MaxCompletionTokens: 123,
	}

	if got := ConvertRequest(req).MaxTokens; got != 123 {
		t.Fatalf("MaxTokens = %d, want 123", got)
	}
}
