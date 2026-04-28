//nolint:testpackage
package moonshot

import (
	"context"
	"net/http"
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestAdaptorSupportMode(t *testing.T) {
	adaptor := &Adaptor{}

	supportedModes := []mode.Mode{
		mode.ChatCompletions,
		mode.Completions,
		mode.Embeddings,
		mode.Moderations,
		mode.ImagesGenerations,
		mode.ImagesEdits,
		mode.AudioSpeech,
		mode.AudioTranscription,
		mode.AudioTranslation,
		mode.Rerank,
		mode.ParsePdf,
		mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.Anthropic,
		mode.Gemini,
	}
	for _, m := range supportedModes {
		if !adaptor.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("expected mode %s to be supported", m)
		}
	}

	unsupportedModes := []mode.Mode{
		mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems,
	}
	for _, m := range unsupportedModes {
		if adaptor.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("expected mode %s to be unsupported", m)
		}
	}
}

func TestAdaptorRejectsResponsesMode(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://api.moonshot.cn/v1"},
		mode.Responses,
		"moonshot-v1-8k",
		coremodel.ModelConfig{},
	)

	if _, err := adaptor.GetRequestURL(m, nil, nil); err == nil {
		t.Fatal("expected responses mode GetRequestURL to fail")
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/responses",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if _, err := adaptor.ConvertRequest(m, nil, req); err == nil {
		t.Fatal("expected responses mode ConvertRequest to fail")
	}
}
