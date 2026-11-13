//nolint:testpackage
package cohere

import (
	"testing"

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
