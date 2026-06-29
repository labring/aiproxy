package siliconflow_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/siliconflow"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestConvertRequestChatPatchesInputAudioToAudioURL(t *testing.T) {
	adaptor := &siliconflow.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"Qwen/Qwen3-Omni-30B-A3B-Instruct",
		coremodel.ModelConfig{},
	)

	req := newChatRequest(t, []byte(`{
		"model": "Qwen/Qwen3-Omni-30B-A3B-Instruct",
		"messages": [
			{
				"role": "user",
				"content": [
					{"type":"text","text":"Transcribe this audio."},
					{"type":"input_audio","input_audio":{"data":"QUJD","format":"wav"}},
					{"type":"input_audio","input_audio":{"url":"https://example.com/audio.mp3"}},
					{"type":"input_audio","input_audio":{"data":"https://example.com/audio.wav","format":"wav"}},
					{
						"type":"video_url",
						"video_url":{
							"url":"https://example.com/video.mp4",
							"detail":"low",
							"max_frames":8,
							"fps":1
						}
					}
				]
			}
		],
		"stream": true
	}`))

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readConvertResultBody(t, result.Body)
	if got["model"] != "Qwen/Qwen3-Omni-30B-A3B-Instruct" {
		t.Fatalf("expected actual model, got %#v", got["model"])
	}

	messages, ok := got["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("expected one message, got %#v", got["messages"])
	}

	message, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("expected message object, got %#v", messages[0])
	}

	content, ok := message["content"].([]any)
	if !ok || len(content) != 5 {
		t.Fatalf("expected five content items, got %#v", message["content"])
	}

	assertSiliconFlowTextContent(t, content[0], "Transcribe this audio.")
	assertSiliconFlowAudioURL(t, content[1], "data:audio/wav;base64,QUJD")
	assertSiliconFlowAudioURL(t, content[2], "https://example.com/audio.mp3")
	assertSiliconFlowAudioURL(t, content[3], "https://example.com/audio.wav")
	assertSiliconFlowVideoURL(t, content[4])

	streamOptions, ok := got["stream_options"].(map[string]any)
	if !ok || streamOptions["include_usage"] != true {
		t.Fatalf("expected include_usage stream_options, got %#v", got["stream_options"])
	}
}

func newChatRequest(t *testing.T, body []byte) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	return req
}

func assertSiliconFlowAudioURL(t *testing.T, got any, wantURL string) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected audio content object, got %T", got)
	}

	if gotMap["type"] != "audio_url" {
		t.Fatalf("expected type=audio_url, got %#v", gotMap["type"])
	}

	audioURL, ok := gotMap["audio_url"].(map[string]any)
	if !ok {
		t.Fatalf("expected audio_url object, got %#v", gotMap["audio_url"])
	}

	if audioURL["url"] != wantURL {
		t.Fatalf("expected audio url %q, got %#v", wantURL, audioURL["url"])
	}
}

func assertSiliconFlowTextContent(t *testing.T, got any, wantText string) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected text content object, got %T", got)
	}

	if gotMap["type"] != "text" {
		t.Fatalf("expected type=text, got %#v", gotMap["type"])
	}

	if gotMap["text"] != wantText {
		t.Fatalf("expected text %q, got %#v", wantText, gotMap["text"])
	}
}

func assertSiliconFlowVideoURL(t *testing.T, got any) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected video content object, got %T", got)
	}

	if gotMap["type"] != "video_url" {
		t.Fatalf("expected type=video_url, got %#v", gotMap["type"])
	}

	videoURL, ok := gotMap["video_url"].(map[string]any)
	if !ok {
		t.Fatalf("expected video_url object, got %#v", gotMap["video_url"])
	}

	if videoURL["url"] != "https://example.com/video.mp4" {
		t.Fatalf("expected video url, got %#v", videoURL["url"])
	}

	if videoURL["detail"] != "low" {
		t.Fatalf("expected video detail, got %#v", videoURL["detail"])
	}

	if videoURL["max_frames"] != float64(8) {
		t.Fatalf("expected max_frames=8, got %#v", videoURL["max_frames"])
	}

	if videoURL["fps"] != float64(1) {
		t.Fatalf("expected fps=1, got %#v", videoURL["fps"])
	}
}
