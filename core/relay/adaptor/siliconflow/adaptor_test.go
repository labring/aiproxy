//nolint:testpackage
package siliconflow

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type siliconflowTestStore struct {
	saved []adaptor.StoreCache
}

func (s *siliconflowTestStore) GetStoreByScope(
	group string,
	tokenID int,
	id string,
	_ coremodel.ChannelScope,
) (adaptor.StoreCache, error) {
	for _, cache := range s.saved {
		if cache.GroupID == group && cache.TokenID == tokenID && cache.ID == id {
			return cache, nil
		}
	}

	return adaptor.StoreCache{}, nil
}

func (s *siliconflowTestStore) SaveStore(
	cache adaptor.StoreCache,
	_ coremodel.ChannelScope,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *siliconflowTestStore) SaveStoreWithOption(
	cache adaptor.StoreCache,
	_ coremodel.ChannelScope,
	_ adaptor.SaveStoreOption,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *siliconflowTestStore) SaveIfNotExistStore(
	cache adaptor.StoreCache,
	_ coremodel.ChannelScope,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func TestAdaptorSupportModeDoesNotSupportResponses(t *testing.T) {
	sfAdaptor := &Adaptor{}

	supportedModes := []mode.Mode{
		mode.ChatCompletions,
		mode.Completions,
		mode.Embeddings,
		mode.ImagesGenerations,
		mode.AudioSpeech,
		mode.AudioTranscription,
		mode.Rerank,
		mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.Videos,
		mode.VideosGet,
		mode.VideosContent,
		mode.Anthropic,
		mode.Gemini,
	}
	for _, m := range supportedModes {
		if !sfAdaptor.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("expected mode %s to be supported", m)
		}
	}

	unsupportedModes := []mode.Mode{
		mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems,
		mode.ImagesEdits,
		mode.VideosDelete,
		mode.VideosRemix,
	}
	for _, m := range unsupportedModes {
		if sfAdaptor.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("expected mode %s to be unsupported", m)
		}
	}
}

func TestAdaptorGetRequestURLUsesSiliconFlowEndpoints(t *testing.T) {
	sfAdaptor := &Adaptor{}

	tests := []struct {
		name       string
		mode       mode.Mode
		wantMethod string
		wantURL    string
	}{
		{
			name:       "chat",
			mode:       mode.ChatCompletions,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/chat/completions",
		},
		{
			name:       "image generation",
			mode:       mode.ImagesGenerations,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/images/generations",
		},
		{
			name:       "video submit",
			mode:       mode.VideoGenerationsJobs,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/video/submit",
		},
		{
			name:       "video status",
			mode:       mode.VideoGenerationsGetJobs,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/video/status",
		},
		{
			name:       "video content",
			mode:       mode.VideoGenerationsContent,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/video/status",
		},
		{
			name:       "videos create",
			mode:       mode.Videos,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/video/submit",
		},
		{
			name:       "videos get",
			mode:       mode.VideosGet,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/video/status",
		},
		{
			name:       "videos content",
			mode:       mode.VideosContent,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/video/status",
		},
		{
			name:       "anthropic",
			mode:       mode.Anthropic,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/chat/completions",
		},
		{
			name:       "gemini",
			mode:       mode.Gemini,
			wantMethod: http.MethodPost,
			wantURL:    "https://api.siliconflow.cn/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(
				&coremodel.Channel{BaseURL: "https://api.siliconflow.cn/v1"},
				tt.mode,
				"test-model",
				coremodel.ModelConfig{},
			)

			got, err := sfAdaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != tt.wantMethod || got.URL != tt.wantURL {
				t.Fatalf("unexpected request URL: %#v", got)
			}
		})
	}
}

func TestAdaptorGetRequestURLRejectsResponsesOnlyRouting(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://api.siliconflow.cn/v1"},
		mode.ChatCompletions,
		"gpt-5-codex",
		coremodel.ModelConfig{},
	)

	got, err := sfAdaptor.GetRequestURL(m, nil, nil)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	if got.URL != "https://api.siliconflow.cn/v1/chat/completions" {
		t.Fatalf("expected chat endpoint, got %q", got.URL)
	}

	m.Mode = mode.Responses
	if _, err := sfAdaptor.GetRequestURL(m, nil, nil); err == nil {
		t.Fatal("expected responses mode to be rejected")
	}
}

func TestAdaptorDoResponseChatDoesNotUseResponsesOnlyConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sfAdaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := meta.NewMeta(nil, mode.ChatCompletions, "gpt-5-codex", coremodel.ModelConfig{})
	m.RequestUsage = coremodel.Usage{InputTokens: 3}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"id":"chatcmpl-123",
			"object":"chat.completion",
			"choices":[
				{
					"index":0,
					"message":{"role":"assistant","content":"Done"},
					"finish_reason":"stop"
				}
			],
			"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}
		}`))),
	}

	result, adaptorErr := sfAdaptor.DoResponse(m, nil, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	if result.UpstreamID != "chatcmpl-123" {
		t.Fatalf("expected chat upstream id, got %q", result.UpstreamID)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body["object"] != "chat.completion" {
		t.Fatalf("expected chat completion response, got %#v", body["object"])
	}
}

func TestConvertAnthropicRequestUsesChatCompletionsShape(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.Anthropic, "claude-alias", coremodel.ModelConfig{})
	m.ActualModel = "Qwen/Qwen3-Omni-30B-A3B-Instruct"

	req := newJSONRequest(t, "/v1/messages", `{
		"model":"claude-alias",
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"text","text":"Describe this image."},
					{
						"type":"image",
						"source":{
							"type":"url",
							"url":"https://example.com/image.png"
						}
					}
				]
			}
		],
		"max_tokens":128
	}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	assertMapValue(t, got, "model", "Qwen/Qwen3-Omni-30B-A3B-Instruct")
	assertMapNumber(t, got, "max_tokens", 128)

	messages, ok := got["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("expected one message, got %#v", got["messages"])
	}

	message, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("expected message object, got %#v", messages[0])
	}

	content, ok := message["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected two content parts, got %#v", message["content"])
	}

	assertSiliconFlowContentType(t, content[0], "text")
	assertSiliconFlowContentType(t, content[1], "image_url")
}

func TestConvertGeminiRequestUsesChatCompletionsShape(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.Gemini, "gemini-alias", coremodel.ModelConfig{})
	m.ActualModel = "Qwen/Qwen3-Omni-30B-A3B-Instruct"

	req := newJSONRequest(t, "/v1beta/models/gemini-alias:generateContent", `{
		"contents":[
			{
				"role":"user",
				"parts":[
					{"text":"Describe this media."},
					{"inlineData":{"mimeType":"audio/wav","data":"QUJD"}}
				]
			}
		],
		"generationConfig":{"maxOutputTokens":128}
	}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	assertMapValue(t, got, "model", "Qwen/Qwen3-Omni-30B-A3B-Instruct")

	messages, ok := got["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("expected one message, got %#v", got["messages"])
	}

	message, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("expected message object, got %#v", messages[0])
	}

	content, ok := message["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected two content parts, got %#v", message["content"])
	}

	assertSiliconFlowContentType(t, content[0], "text")
	assertSiliconFlowContentType(t, content[1], "audio_url")
}

func TestConvertGeminiRequestInfersFileDataMediaTypeFromURI(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.Gemini, "gemini-alias", coremodel.ModelConfig{})
	m.ActualModel = "Qwen/Qwen3-Omni-30B-A3B-Instruct"

	req := newJSONRequest(t, "/v1beta/models/gemini-alias:generateContent", `{
		"contents":[
			{
				"role":"user",
				"parts":[
					{"fileData":{"fileUri":"https://example.com/audio.mp3"}},
					{"fileData":{"fileUri":"https://example.com/video.mp4?token=abc"}}
				]
			}
		]
	}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)

	messages, ok := got["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("expected one message, got %#v", got["messages"])
	}

	message, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("expected message object, got %#v", messages[0])
	}

	content, ok := message["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected two content parts, got %#v", message["content"])
	}

	assertSiliconFlowContentType(t, content[0], "audio_url")
	assertSiliconFlowContentType(t, content[1], "video_url")
}

func TestConvertImageRequestMapsOpenAIFields(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesGenerations, "alias-image", coremodel.ModelConfig{})
	m.ActualModel = "stabilityai/stable-diffusion-3-5-large"

	req := newJSONRequest(t, "/v1/images/generations", `{
		"model":"alias-image",
		"prompt":"A city at sunset",
		"negative_prompt":"low quality",
		"size":"1024×1024",
		"n":2,
		"steps":24,
		"scale":7,
		"response_format":"url"
	}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	assertMapValue(t, got, "model", "stabilityai/stable-diffusion-3-5-large")
	assertMapValue(t, got, "prompt", "A city at sunset")
	assertMapValue(t, got, "negative_prompt", "low quality")
	assertMapValue(t, got, "image_size", "1024x1024")
	assertMapNumber(t, got, "batch_size", 2)
	assertMapNumber(t, got, "num_inference_steps", 24)
	assertMapNumber(t, got, "guidance_scale", 7)

	if _, ok := got["size"]; ok {
		t.Fatal("expected size to be removed")
	}

	if _, ok := got["n"]; ok {
		t.Fatal("expected n to be removed")
	}
}

func TestConvertImageRequestNormalizesSizeDelimiters(t *testing.T) {
	tests := []struct {
		name          string
		size          string
		wantImageSize string
	}{
		{
			name:          "asterisk",
			size:          "1024*1024",
			wantImageSize: "1024x1024",
		},
		{
			name:          "multiplication sign",
			size:          "1024×1536",
			wantImageSize: "1024x1536",
		},
		{
			name:          "native x",
			size:          "720x1280",
			wantImageSize: "720x1280",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sfAdaptor := &Adaptor{}
			m := meta.NewMeta(nil, mode.ImagesGenerations, "alias-image", coremodel.ModelConfig{})
			m.ActualModel = "stabilityai/stable-diffusion-3-5-large"

			req := newJSONRequest(t, "/v1/images/generations", `{
				"model":"alias-image",
				"prompt":"A city at sunset",
				"size":"`+tt.size+`",
				"n":3
			}`)

			result, err := sfAdaptor.ConvertRequest(m, nil, req)
			if err != nil {
				t.Fatalf("ConvertRequest returned error: %v", err)
			}

			got := readJSONBody(t, result.Body)
			assertMapValue(t, got, "image_size", tt.wantImageSize)
			assertMapNumber(t, got, "batch_size", 3)

			if _, ok := got["size"]; ok {
				t.Fatal("expected size to be removed")
			}

			if _, ok := got["n"]; ok {
				t.Fatal("expected n to be removed")
			}
		})
	}
}

func TestImageHandlerMapsSiliconFlowResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.ImagesGenerations,
		"stabilityai/stable-diffusion-3-5-large",
		coremodel.ModelConfig{},
		meta.WithRequestUsage(coremodel.Usage{
			InputTokens:  12,
			OutputTokens: 2,
		}),
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"images":[
				{"url":"https://example.com/one.png"},
				{"url":"https://example.com/two.png"}
			],
			"seed":123
		}`))),
	}

	result, adaptorErr := ImageHandler(m, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("ImageHandler returned error: %v", adaptorErr)
	}

	var imageResponse relaymodel.ImageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &imageResponse); err != nil {
		t.Fatalf("failed to unmarshal image response: %v", err)
	}

	if len(imageResponse.Data) != 2 {
		t.Fatalf("expected 2 images, got %d", len(imageResponse.Data))
	}

	if imageResponse.Data[0].URL != "https://example.com/one.png" ||
		imageResponse.Data[1].URL != "https://example.com/two.png" {
		t.Fatalf("unexpected image response data: %#v", imageResponse.Data)
	}

	var raw map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal raw image response: %v", err)
	}

	if _, ok := raw["images"]; ok {
		t.Fatalf("expected provider images field to be omitted, got %#v", raw)
	}

	if _, ok := raw["seed"]; ok {
		t.Fatalf("expected provider seed field to be omitted, got %#v", raw)
	}

	if int64(result.Usage.InputTokens) != 0 ||
		int64(result.Usage.OutputTokens) != 2 ||
		int64(result.Usage.ImageOutputTokens) != 2 ||
		int64(result.Usage.TotalTokens) != 2 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}
}

func TestImageHandlerClearsURLWhenB64JSONRequested(t *testing.T) {
	gin.SetMode(gin.TestMode)

	imageServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-bytes"))
		}),
	)
	defer imageServer.Close()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.ImagesGenerations,
		"stabilityai/stable-diffusion-3-5-large",
		coremodel.ModelConfig{},
	)
	m.Set("response_format", "b64_json")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"images":[{"url":"` + imageServer.URL + `/one.png"}]
		}`))),
	}

	_, adaptorErr := ImageHandler(m, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("ImageHandler returned error: %v", adaptorErr)
	}

	var imageResponse relaymodel.ImageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &imageResponse); err != nil {
		t.Fatalf("failed to unmarshal image response: %v", err)
	}

	if len(imageResponse.Data) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imageResponse.Data))
	}

	if imageResponse.Data[0].URL != "" {
		t.Fatalf(
			"expected url to be omitted for b64_json response, got %#v",
			imageResponse.Data[0].URL,
		)
	}

	if imageResponse.Data[0].B64Json == "" {
		t.Fatal("expected b64_json to be set")
	}
}

func TestConvertVideoRequestMapsJSONFields(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "alias-video", coremodel.ModelConfig{})
	m.ActualModel = "Wan-AI/Wan2.2-T2V-A14B"

	req := newJSONRequest(t, "/v1/video/generations/jobs", `{
		"model":"alias-video",
		"prompt":"A calm ocean",
		"width":1280,
		"height":720,
		"negative_prompt":"rain",
		"seed":123
	}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	assertMapValue(t, got, "model", "Wan-AI/Wan2.2-T2V-A14B")
	assertMapValue(t, got, "prompt", "A calm ocean")
	assertMapValue(t, got, "image_size", "1280x720")
	assertMapValue(t, got, "negative_prompt", "rain")
	assertMapNumber(t, got, "seed", 123)
}

func TestConvertVideoRequestMapsImageReference(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "alias-video", coremodel.ModelConfig{})
	m.ActualModel = "Wan-AI/Wan2.2-I2V-A14B"

	req := newJSONRequest(t, "/v1/video/generations/jobs", `{
		"model":"alias-video",
		"prompt":"Animate this scene",
		"width":960,
		"height":960,
		"input_reference":"https://example.com/input.png"
	}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	assertMapValue(t, got, "image", "https://example.com/input.png")
}

func TestConvertVideosRequestIgnoresJobOnlyDimensions(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.Videos, "alias-video", coremodel.ModelConfig{})
	m.ActualModel = "Wan-AI/Wan2.2-T2V-A14B"

	req := newJSONRequest(t, "/v1/videos", `{
		"model":"alias-video",
		"prompt":"A calm ocean",
		"width":1280,
		"height":720
	}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	if _, ok := got["image_size"]; ok {
		t.Fatalf("expected job-only dimensions to be ignored, got %#v", got["image_size"])
	}
}

func TestConvertVideosRequestMapsOpenAISize(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.Videos, "alias-video", coremodel.ModelConfig{})
	m.ActualModel = "Wan-AI/Wan2.2-T2V-A14B"

	req := newJSONRequest(t, "/v1/videos", `{
		"model":"alias-video",
		"prompt":"A calm ocean",
		"size":"720×1280"
	}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	assertMapValue(t, got, "image_size", "720x1280")
}

func TestConvertVideoStatusRequestUsesJobID(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"Wan-AI/Wan2.2-T2V-A14B",
		coremodel.ModelConfig{},
		meta.WithJobID("request-123"),
	)

	req := newJSONRequest(t, "/v1/video/generations/jobs/request-123", `{}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	assertMapValue(t, got, "requestId", "request-123")
}

func TestConvertVideoContentStatusRequestUsesGenerationID(t *testing.T) {
	sfAdaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsContent,
		"Wan-AI/Wan2.2-T2V-A14B",
		coremodel.ModelConfig{},
		meta.WithGenerationID("request-123"),
	)

	req := newJSONRequest(t, "/v1/video/generations/request-123/content/video", `{}`)

	result, err := sfAdaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	got := readJSONBody(t, result.Body)
	assertMapValue(t, got, "requestId", "request-123")
}

func TestVideoSubmitHandlerMapsRequestIDToJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"Wan-AI/Wan2.2-T2V-A14B",
		coremodel.ModelConfig{},
	)
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}
	m.Set(metaVideoRequest, videoSubmitRequest{
		Prompt:    "A calm ocean",
		ImageSize: "1280x720",
	})

	store := &siliconflowTestStore{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"requestId":"request-123"}`))),
	}

	result, adaptorErr := VideoGenerationJobSubmitHandler(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("VideoSubmitHandler returned error: %v", adaptorErr)
	}

	if result.UpstreamID != "request-123" || !result.AsyncUsage {
		t.Fatalf("unexpected result: %#v", result)
	}

	if result.UsageContext.Resolution != "1280x720" {
		t.Fatalf("expected usage context resolution, got %#v", result.UsageContext)
	}

	if m.RequestUsage.OutputTokens != 0 || m.RequestUsage.TotalTokens != 0 {
		t.Fatalf("expected submit handler not to mutate request usage, got %#v", m.RequestUsage)
	}

	if len(store.saved) != 1 || store.saved[0].ID != coremodel.VideoJobStoreID("request-123") {
		t.Fatalf("unexpected saved stores: %#v", store.saved)
	}

	if store.saved[0].Metadata != `{"prompt":"A calm ocean","image_size":"1280x720"}` {
		t.Fatalf("unexpected saved metadata: %s", store.saved[0].Metadata)
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job: %v", err)
	}

	if job.ID != "request-123" ||
		job.Status != relaymodel.VideoGenerationJobStatusQueued ||
		job.Prompt != "A calm ocean" ||
		job.Width != 1280 ||
		job.Height != 720 {
		t.Fatalf("unexpected job: %#v", job)
	}
}

func TestVideoStatusHandlerRestoresRequestMetadataFromStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs/request-123",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"Wan-AI/Wan2.2-T2V-A14B",
		coremodel.ModelConfig{},
		meta.WithJobID("request-123"),
	)
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}

	store := &siliconflowTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:        coremodel.VideoJobStoreID("request-123"),
				GroupID:   "group-1",
				TokenID:   7,
				ChannelID: 9,
				Model:     "Wan-AI/Wan2.2-T2V-A14B",
				Metadata:  `{"prompt":"A calm ocean","image_size":"1280x720"}`,
			},
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"status":"Succeed",
			"results":{"videos":[{"url":"https://example.com/video.mp4"}]}
		}`))),
	}

	_, adaptorErr := VideoGenerationJobStatusHandler(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("VideoGenerationJobStatusHandler returned error: %v", adaptorErr)
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job: %v", err)
	}

	if job.Prompt != "A calm ocean" || job.Width != 1280 || job.Height != 720 ||
		len(job.Generations) != 1 ||
		job.Generations[0].Width != 1280 ||
		job.Generations[0].Height != 720 {
		t.Fatalf("expected stored request metadata to be restored, got %#v", job)
	}
}

func TestVideoStatusHandlerMapsSucceededResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs/request-123",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"Wan-AI/Wan2.2-T2V-A14B",
		coremodel.ModelConfig{},
		meta.WithJobID("request-123"),
	)
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}
	m.Set(metaVideoRequest, videoSubmitRequest{
		Prompt:    "A calm ocean",
		ImageSize: "1280x720",
	})

	store := &siliconflowTestStore{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"status":"Succeed",
			"results":{"videos":[{"url":"https://example.com/video.mp4"}]}
		}`))),
	}

	_, adaptorErr := VideoGenerationJobStatusHandler(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("VideoGenerationJobStatusHandler returned error: %v", adaptorErr)
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job: %v", err)
	}

	if job.ID != "request-123" ||
		job.Status != relaymodel.VideoGenerationJobStatusSucceeded ||
		len(job.Generations) != 1 {
		t.Fatalf("unexpected job: %#v", job)
	}

	if len(store.saved) != 1 ||
		store.saved[0].ID != coremodel.VideoGenerationStoreID(job.Generations[0].ID) {
		t.Fatalf("unexpected saved stores: %#v", store.saved)
	}

	if job.Generations[0].ID != "request-123" {
		t.Fatalf("expected generation id to use upstream request id, got %q", job.Generations[0].ID)
	}
}

func TestVideosHandlerMapsSubmitResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		nil,
	)

	m := meta.NewMeta(nil, mode.Videos, "Wan-AI/Wan2.2-T2V-A14B", coremodel.ModelConfig{})
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}
	m.Set(metaVideoRequest, videoSubmitRequest{
		Prompt:    "A calm ocean",
		ImageSize: "1280x720",
	})

	store := &siliconflowTestStore{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"requestId":"request-123"}`))),
	}

	result, adaptorErr := VideosSubmitHandler(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("VideoSubmitHandler returned error: %v", adaptorErr)
	}

	if result.UpstreamID != "request-123" || !result.AsyncUsage {
		t.Fatalf("unexpected result: %#v", result)
	}

	if len(store.saved) != 1 ||
		store.saved[0].ID != coremodel.VideoGenerationStoreID("request-123") {
		t.Fatalf("unexpected saved stores: %#v", store.saved)
	}

	if result.UsageContext.Resolution != "1280x720" {
		t.Fatalf("expected usage context resolution, got %#v", result.UsageContext)
	}

	var video relaymodel.Video
	if err := json.Unmarshal(recorder.Body.Bytes(), &video); err != nil {
		t.Fatalf("failed to unmarshal video: %v", err)
	}

	if video.ID != "request-123" ||
		video.Object != relaymodel.VideoObject ||
		video.Status != relaymodel.VideoStatusQueued ||
		video.Prompt != "A calm ocean" ||
		video.Size != "1280x720" {
		t.Fatalf("unexpected video: %#v", video)
	}
}

func TestVideosGetHandlerMapsStatusResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/request-123",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.VideosGet,
		"Wan-AI/Wan2.2-T2V-A14B",
		coremodel.ModelConfig{},
		meta.WithVideoID("request-123"),
	)
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}
	m.Set(metaVideoRequest, videoSubmitRequest{
		Prompt:    "A calm ocean",
		ImageSize: "1280x720",
	})

	store := &siliconflowTestStore{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"status":"Succeed",
			"results":{"videos":[{"url":"https://example.com/video.mp4"}]}
		}`))),
	}

	result, adaptorErr := VideosStatusHandler(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("VideosStatusHandler returned error: %v", adaptorErr)
	}

	if result.UpstreamID != "request-123" {
		t.Fatalf("expected upstream id, got %q", result.UpstreamID)
	}

	if result.UsageContext.Resolution != "1280x720" {
		t.Fatalf("expected usage context resolution, got %#v", result.UsageContext)
	}

	var video relaymodel.Video
	if err := json.Unmarshal(recorder.Body.Bytes(), &video); err != nil {
		t.Fatalf("failed to unmarshal video: %v", err)
	}

	if video.ID != "request-123" ||
		video.Object != relaymodel.VideoObject ||
		video.Status != relaymodel.VideoStatusCompleted ||
		video.Progress != 100 {
		t.Fatalf("unexpected video: %#v", video)
	}
}

func TestVideoContentHandlerDownloadsGeneratedVideo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	videoServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("video-data"))
		}),
	)
	defer videoServer.Close()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/video/generations/request-123/content/video",
		nil,
	)

	m := meta.NewMeta(
		&coremodel.Channel{},
		mode.VideoGenerationsContent,
		"Wan-AI/Wan2.2-T2V-A14B",
		coremodel.ModelConfig{},
		meta.WithGenerationID("request-123"),
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"status":"Succeed",
			"results":{"videos":[{"url":"` + videoServer.URL + `"}]}
		}`))),
	}

	result, adaptorErr := VideoGenerationJobContentHandler(m, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("VideoGenerationJobContentHandler returned error: %v", adaptorErr)
	}

	if result.UpstreamID != "request-123" {
		t.Fatalf("expected upstream id, got %q", result.UpstreamID)
	}

	if recorder.Body.String() != "video-data" {
		t.Fatalf("unexpected video body: %q", recorder.Body.String())
	}

	if recorder.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("unexpected content type: %q", recorder.Header().Get("Content-Type"))
	}
}

func TestVideosContentHandlerDownloadsGeneratedVideo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	videoServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("video-data"))
		}),
	)
	defer videoServer.Close()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/videos/request-123/content",
		nil,
	)

	m := meta.NewMeta(
		&coremodel.Channel{},
		mode.VideosContent,
		"Wan-AI/Wan2.2-T2V-A14B",
		coremodel.ModelConfig{},
		meta.WithVideoID("request-123"),
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"status":"Succeed",
			"results":{"videos":[{"url":"` + videoServer.URL + `"}]}
		}`))),
	}

	result, adaptorErr := VideosContentHandler(m, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("VideosContentHandler returned error: %v", adaptorErr)
	}

	if result.UpstreamID != "request-123" {
		t.Fatalf("expected upstream id, got %q", result.UpstreamID)
	}

	if recorder.Body.String() != "video-data" {
		t.Fatalf("unexpected video body: %q", recorder.Body.String())
	}
}

func TestFetchAsyncUsageUsesReturnedVideoCount(t *testing.T) {
	statusServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
			"status":"Succeed",
			"results":{
				"videos":[
					{"url":"https://example.com/one.mp4"},
					{"url":"https://example.com/two.mp4"}
				]
			}
		}`))
		}),
	)
	defer statusServer.Close()

	sfAdaptor := &Adaptor{}
	store := &siliconflowTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoGenerationStoreID("request-123"),
				GroupID:  "group-1",
				TokenID:  7,
				Metadata: `{"prompt":"Stored prompt","image_size":"720x1280"}`,
			},
		},
	}

	usage, usageContext, done, err := sfAdaptor.FetchAsyncUsage(
		context.Background(),
		adaptor.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				BaseURL: statusServer.URL,
				Key:     "test-key",
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.Videos),
				UpstreamID: "request-123",
				GroupID:    "group-1",
				TokenID:    7,
				Usage: coremodel.Usage{
					OutputTokens: 8,
					TotalTokens:  8,
				},
				UsageContext: coremodel.UsageContext{
					Resolution: "640x480",
				},
			},
			Store: store,
		},
	)
	if err != nil {
		t.Fatalf("FetchAsyncUsage returned error: %v", err)
	}

	if !done {
		t.Fatalf("expected async usage to be done")
	}

	if usage.OutputTokens != 2 || usage.TotalTokens != 2 {
		t.Fatalf("expected usage to count returned videos, got %#v", usage)
	}

	if usageContext.Resolution != "720x1280" {
		t.Fatalf("expected async usage context resolution, got %#v", usageContext)
	}
}

func newJSONRequest(t *testing.T, path, body string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		path,
		bytes.NewReader([]byte(body)),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req
}

func readJSONBody(t *testing.T, body io.Reader) map[string]any {
	t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal body %s: %v", string(data), err)
	}

	return got
}

func assertMapValue(t *testing.T, got map[string]any, key, want string) {
	t.Helper()

	if got[key] != want {
		t.Fatalf("expected %s=%q, got %#v", key, want, got[key])
	}
}

func assertMapNumber(t *testing.T, got map[string]any, key string, want int) {
	t.Helper()

	value, ok := got[key].(float64)
	if !ok || int(value) != want {
		t.Fatalf("expected %s=%d, got %#v", key, want, got[key])
	}
}

func assertSiliconFlowContentType(t *testing.T, got any, wantType string) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected content object, got %T", got)
	}

	if gotMap["type"] != wantType {
		t.Fatalf("expected content type %q, got %#v", wantType, gotMap["type"])
	}
}
