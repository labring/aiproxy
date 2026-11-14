//nolint:testpackage
package coze

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/adaptor/coze/constant/messagetype"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func TestStreamResponse2OpenAIFinishReason(t *testing.T) {
	t.Parallel()

	m := &meta.Meta{OriginModel: "coze-bot"}

	mid := StreamResponse2OpenAI(m, &StreamResponse{
		Message: &Message{Type: messagetype.Answer, Content: "hi"},
	})
	if mid == nil || mid.Choices[0].FinishReason != "" {
		t.Fatalf("expected no finish_reason mid-stream, got %+v", mid)
	}

	last := StreamResponse2OpenAI(m, &StreamResponse{
		Message:  &Message{Type: messagetype.Answer, Content: "bye"},
		IsFinish: true,
	})
	if last == nil || last.Choices[0].FinishReason != relaymodel.FinishReasonStop {
		t.Fatalf("expected finish_reason stop on is_finish, got %+v", last)
	}

	if last.Choices[0].Delta.Content != "bye" {
		t.Fatalf("content lost on final chunk: %+v", last.Choices[0].Delta)
	}

	followUp := StreamResponse2OpenAI(m, &StreamResponse{
		Message:  &Message{Type: messagetype.FollowUp, Content: "next?"},
		IsFinish: true,
	})
	if followUp != nil {
		t.Fatalf("expected follow_up event to be skipped, got %+v", followUp)
	}
}
