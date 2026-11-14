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
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

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
