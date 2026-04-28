//nolint:testpackage
package zhipucoding

import (
	"io"
	"net/http"
	"strings"
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
		mode.Embeddings,
		mode.AudioSpeech,
		mode.Rerank,
	}
	for _, m := range unsupportedModes {
		if adaptor.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("expected mode %s to be unsupported", m)
		}
	}
}

func TestAdaptorGetRequestURL(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{
		BaseURL: "https://open.bigmodel.cn",
	}

	tests := []struct {
		name string
		mode mode.Mode
		want string
	}{
		{
			name: "anthropic uses native anthropic endpoint",
			mode: mode.Anthropic,
			want: "https://open.bigmodel.cn/api/anthropic/v1/messages",
		},
		{
			name: "gemini uses coding chat completions",
			mode: mode.Gemini,
			want: "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
		{
			name: "chat uses coding chat completions",
			mode: mode.ChatCompletions,
			want: "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
		{
			name: "completions uses coding completions",
			mode: mode.Completions,
			want: "https://open.bigmodel.cn/api/coding/paas/v4/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(channel, tt.mode, "glm-5.1", coremodel.ModelConfig{})

			got, err := adaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != http.MethodPost {
				t.Fatalf("expected method %s, got %s", http.MethodPost, got.Method)
			}

			if got.URL != tt.want {
				t.Fatalf("expected URL %s, got %s", tt.want, got.URL)
			}

			if m.Mode != tt.mode {
				t.Fatalf("expected mode to remain %s, got %s", tt.mode, m.Mode)
			}

			if m.Channel.BaseURL != channel.BaseURL {
				t.Fatalf(
					"expected base URL to remain %s, got %s",
					channel.BaseURL,
					m.Channel.BaseURL,
				)
			}
		})
	}
}

func TestAdaptorGetRequestURLUnsupportedResponses(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://open.bigmodel.cn"},
		mode.Responses,
		"glm-5.1",
		coremodel.ModelConfig{},
	)

	if _, err := adaptor.GetRequestURL(m, nil, nil); err == nil {
		t.Fatal("expected Responses mode to be unsupported")
	}
}

func TestAdaptorDoResponseUsesZhipuErrorHandler(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://open.bigmodel.cn"},
		mode.ChatCompletions,
		"glm-5.1",
		coremodel.ModelConfig{},
	)
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body: io.NopCloser(
			//nolint:lll
			strings.NewReader(`{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`),
		),
	}

	_, err := adaptor.DoResponse(m, nil, nil, resp)
	if err == nil {
		t.Fatal("expected zhipu error")
	}

	if err.StatusCode() != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, err.StatusCode())
	}
}
