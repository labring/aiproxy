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
