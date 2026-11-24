//nolint:testpackage
package baidu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func TestResponse2OpenAI_FinishReason(t *testing.T) {
	m := &meta.Meta{OriginModel: "ernie"}

	got := response2OpenAI(m, &ChatResponse{Result: "hi"})
	if fr := got.Choices[0].FinishReason; fr != relaymodel.FinishReasonStop {
		t.Fatalf("finish_reason = %q, want stop", fr)
	}

	got = response2OpenAI(m, &ChatResponse{Result: "hi", IsTruncated: true})
	if fr := got.Choices[0].FinishReason; fr != relaymodel.FinishReasonLength {
		t.Fatalf("truncated finish_reason = %q, want length", fr)
	}

	got = response2OpenAI(m, &ChatResponse{Result: "blocked", NeedClearHistory: true})
	if fr := got.Choices[0].FinishReason; fr != relaymodel.FinishReasonContentFilter {
		t.Fatalf("need_clear_history finish_reason = %q, want content_filter", fr)
	}

	got = response2OpenAI(m, &ChatResponse{
		Result:           "blocked",
		IsTruncated:      true,
		NeedClearHistory: true,
	})
	if fr := got.Choices[0].FinishReason; fr != relaymodel.FinishReasonContentFilter {
		t.Fatalf("safety+truncated finish_reason = %q, want content_filter", fr)
	}
}

func TestStreamResponse2OpenAI_FinishReason(t *testing.T) {
	m := &meta.Meta{OriginModel: "ernie"}

	got := streamResponse2OpenAI(m, &ChatStreamResponse{IsEnd: false})
	if fr := got.Choices[0].FinishReason; fr != "" {
		t.Fatalf("mid-stream finish_reason = %q, want empty", fr)
	}

	got = streamResponse2OpenAI(m, &ChatStreamResponse{IsEnd: true})
	if fr := got.Choices[0].FinishReason; fr != relaymodel.FinishReasonStop {
		t.Fatalf("finish_reason = %q, want stop", fr)
	}

	got = streamResponse2OpenAI(m, &ChatStreamResponse{
		ChatResponse: ChatResponse{IsTruncated: true},
		IsEnd:        true,
	})
	if fr := got.Choices[0].FinishReason; fr != relaymodel.FinishReasonLength {
		t.Fatalf("truncated finish_reason = %q, want length", fr)
	}

	got = streamResponse2OpenAI(m, &ChatStreamResponse{
		ChatResponse: ChatResponse{Result: "blocked", NeedClearHistory: true},
		IsEnd:        false,
	})
	if fr := got.Choices[0].FinishReason; fr != "" {
		t.Fatalf("mid-stream need_clear_history finish_reason = %q, want empty", fr)
	}

	got = streamResponse2OpenAI(m, &ChatStreamResponse{
		ChatResponse: ChatResponse{Result: "blocked", NeedClearHistory: true},
		IsEnd:        true,
	})
	if fr := got.Choices[0].FinishReason; fr != relaymodel.FinishReasonContentFilter {
		t.Fatalf("need_clear_history finish_reason = %q, want content_filter", fr)
	}
}

func TestConvertRequestMaxCompletionTokens(t *testing.T) {
	t.Parallel()

	body := `{"model":"ernie","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":123}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ConvertRequest(&meta.Meta{ActualModel: "ernie"}, req)
	if err != nil {
		t.Fatal(err)
	}

	data, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}

	var got ChatRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.MaxOutputTokens != 123 {
		t.Fatalf("MaxOutputTokens = %d, want 123", got.MaxOutputTokens)
	}
}
