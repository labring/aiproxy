//nolint:testpackage
package baidu

import (
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
}
