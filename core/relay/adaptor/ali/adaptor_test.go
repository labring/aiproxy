//nolint:testpackage
package ali

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type aliTestStore struct {
	saved []adaptor.StoreCache
}

func TestErrorHandlerParsesAliTopLevelError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"code":"InvalidParameter","message":"duration is invalid","request_id":"req-1"}`,
		)),
	}

	err := ErrorHanlder(resp)
	if err.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.StatusCode())
	}

	body, marshalErr := err.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}

	if !strings.Contains(string(body), `"message":"duration is invalid"`) {
		t.Fatalf("expected ali message, got %s", body)
	}

	if !strings.Contains(string(body), `"code":"InvalidParameter"`) {
		t.Fatalf("expected ali code, got %s", body)
	}
}

func TestOpenAIVideoErrorHandlerParsesAliTopLevelError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"code":"InvalidParameter","message":"duration is invalid","request_id":"req-1"}`,
		)),
	}

	err := OpenAIVideoErrorHandler(resp)
	if err.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.StatusCode())
	}

	body, marshalErr := err.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}

	if string(body) != `{"detail":"duration is invalid"}` {
		t.Fatalf("expected OpenAI video error detail, got %s", body)
	}
}

func TestOpenAIVideoErrorHandlerKeepsAliStatusNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "service unavailable type",
			body:       `{"error":{"code":"ServiceUnavailable","message":"upstream overloaded","type":"ServiceUnavailable"}}`,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "request timeout type",
			body:       `{"error":{"code":"RequestTimeOut","message":"upstream timeout","type":"RequestTimeOut"}}`,
			wantStatus: http.StatusRequestTimeout,
		},
		{
			name:       "object is not iterable message",
			body:       `{"code":"InvalidParameter","message":"object is not iterable"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			err := OpenAIVideoErrorHandler(resp)
			if err.StatusCode() != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, err.StatusCode())
			}
		})
	}
}

func (s *aliTestStore) getStore(id string) (adaptor.StoreCache, error) {
	for _, cache := range s.saved {
		if cache.ID == id {
			return cache, nil
		}
	}

	return adaptor.StoreCache{}, nil
}

func (s *aliTestStore) GetStoreByScope(
	group string,
	tokenID int,
	id string,
	_ coremodel.ChannelScope,
) (adaptor.StoreCache, error) {
	return s.getStore(id)
}

func (s *aliTestStore) SaveStore(
	cache adaptor.StoreCache,
	_ coremodel.ChannelScope,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *aliTestStore) SaveStoreWithOption(
	cache adaptor.StoreCache,
	_ coremodel.ChannelScope,
	_ adaptor.SaveStoreOption,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *aliTestStore) SaveIfNotExistStore(
	cache adaptor.StoreCache,
	_ coremodel.ChannelScope,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func TestFetchAliVideoJobUsageKeepsResponseSize(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks/task-123" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output":{"task_id":"task-123","task_status":"SUCCEEDED"},
			"usage":{"duration":5,"video_count":1,"SR":720,"ratio":"16:9"}
		}`))
	}))
	defer server.Close()

	usage, usageContext, completed, err := (&Adaptor{}).fetchAliVideoJobUsage(
		t.Context(),
		&coremodel.Channel{BaseURL: server.URL},
		nil,
		&coremodel.AsyncUsageInfo{UpstreamID: "task-123"},
	)
	if err != nil {
		t.Fatalf("fetchAliVideoJobUsage returned error: %v", err)
	}

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usage.OutputTokens != coremodel.ZeroNullInt64(5) {
		t.Fatalf("expected output tokens 5, got %#v", usage.OutputTokens)
	}

	if usageContext.Resolution != "1280x720" {
		t.Fatalf(
			"expected video resolution 1280x720, got %q",
			usageContext.Resolution,
		)
	}

	if usageContext.NativeResolution != "720P" {
		t.Fatalf(
			"expected native resolution 720P, got %q",
			usageContext.NativeResolution,
		)
	}
}

func TestAdaptorSupportModeResponses(t *testing.T) {
	adaptor := &Adaptor{}

	supportedModes := []mode.Mode{
		mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems,
	}
	for _, m := range supportedModes {
		if !adaptor.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("expected mode %s to be supported", m)
		}
	}
}

func TestAdaptorGetRequestURLResponses(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{BaseURL: "https://dashscope.aliyuncs.com"}

	tests := []struct {
		name       string
		mode       mode.Mode
		responseID string
		wantMethod string
		wantURL    string
	}{
		{
			name:       "responses create",
			mode:       mode.Responses,
			wantMethod: http.MethodPost,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses",
		},
		{
			name:       "responses get",
			mode:       mode.ResponsesGet,
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses/resp_123",
		},
		{
			name:       "responses delete",
			mode:       mode.ResponsesDelete,
			responseID: "resp_123",
			wantMethod: http.MethodDelete,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses/resp_123",
		},
		{
			name:       "responses cancel",
			mode:       mode.ResponsesCancel,
			responseID: "resp_123",
			wantMethod: http.MethodPost,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses/resp_123/cancel",
		},
		{
			name:       "responses input items",
			mode:       mode.ResponsesInputItems,
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/responses/resp_123/input_items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(
				channel,
				tt.mode,
				"qwen-plus",
				coremodel.ModelConfig{},
				meta.WithResponseID(tt.responseID),
			)

			got, err := adaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != tt.wantMethod {
				t.Fatalf("expected method %s, got %s", tt.wantMethod, got.Method)
			}

			if got.URL != tt.wantURL {
				t.Fatalf("expected URL %s, got %s", tt.wantURL, got.URL)
			}
		})
	}
}

func TestAdaptorGetRequestURLMultimodalEmbeddings(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{BaseURL: "https://dashscope.aliyuncs.com"}

	tests := []struct {
		name    string
		origin  string
		actual  string
		wantURL string
	}{
		{
			name:    "vl model uses multimodal embedding endpoint",
			origin:  "qwen3-vl-embedding",
			actual:  "mapped-model",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding",
		},
		{
			name:    "multimodal model uses multimodal embedding endpoint",
			origin:  "multimodal-embedding-v1",
			actual:  "mapped-model",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding",
		},
		{
			name:    "vision model uses multimodal embedding endpoint",
			origin:  "tongyi-embedding-vision-plus",
			actual:  "mapped-model",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding",
		},
		{
			name:    "actual multimodal model uses multimodal embedding endpoint",
			origin:  "alias-model",
			actual:  "qwen3-vl-embedding",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding",
		},
		{
			name:    "text embedding model keeps compatible endpoint",
			origin:  "text-embedding-v4",
			actual:  "text-embedding-v4",
			wantURL: "https://dashscope.aliyuncs.com/compatible-mode/v1/embeddings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(channel, mode.Embeddings, tt.origin, coremodel.ModelConfig{})
			m.ActualModel = tt.actual

			got, err := adaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != http.MethodPost {
				t.Fatalf("expected method %s, got %s", http.MethodPost, got.Method)
			}

			if got.URL != tt.wantURL {
				t.Fatalf("expected URL %s, got %s", tt.wantURL, got.URL)
			}
		})
	}
}

func TestAdaptorGetRequestURLAliImage(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{BaseURL: "https://dashscope.aliyuncs.com"}

	tests := []struct {
		name    string
		mode    mode.Mode
		model   string
		wantURL string
	}{
		{
			name:    "qwen image generations uses multimodal generation endpoint",
			mode:    mode.ImagesGenerations,
			model:   "qwen-image-2.0-pro",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
		},
		{
			name:    "qwen image edits uses multimodal generation endpoint",
			mode:    mode.ImagesEdits,
			model:   "qwen-image-edit-plus",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
		},
		{
			name:    "wan image generations uses multimodal generation endpoint",
			mode:    mode.ImagesGenerations,
			model:   "wan2.7-image-pro",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
		},
		{
			name:    "wan 2.6 generations uses multimodal generation endpoint",
			mode:    mode.ImagesGenerations,
			model:   "wan2.6-t2i",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
		},
		{
			name:    "z image generations uses multimodal generation endpoint",
			mode:    mode.ImagesGenerations,
			model:   "z-image-turbo",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
		},
		{
			name:    "qwen mt image edits uses image2image endpoint",
			mode:    mode.ImagesEdits,
			model:   "qwen-mt-image",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/aigc/image2image/image-synthesis",
		},
		{
			name:    "legacy image generation keeps async endpoint",
			mode:    mode.ImagesGenerations,
			model:   "stable-diffusion-xl",
			wantURL: "https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(channel, tt.mode, tt.model, coremodel.ModelConfig{})

			got, err := adaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != http.MethodPost {
				t.Fatalf("expected method %s, got %s", http.MethodPost, got.Method)
			}

			if got.URL != tt.wantURL {
				t.Fatalf("expected URL %s, got %s", tt.wantURL, got.URL)
			}
		})
	}
}

func TestAdaptorGetRequestURLAliVideo(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{BaseURL: "https://dashscope.aliyuncs.com/custom"}

	tests := []struct {
		name       string
		mode       mode.Mode
		jobID      string
		generation string
		wantMethod string
		wantURL    string
	}{
		{
			name:       "create job uses video synthesis endpoint",
			mode:       mode.VideoGenerationsJobs,
			wantMethod: http.MethodPost,
			wantURL:    "https://dashscope.aliyuncs.com/custom/api/v1/services/aigc/video-generation/video-synthesis",
		},
		{
			name:       "get job uses task endpoint",
			mode:       mode.VideoGenerationsGetJobs,
			jobID:      "task-123",
			wantMethod: http.MethodGet,
			wantURL:    "https://dashscope.aliyuncs.com/custom/api/v1/tasks/task-123",
		},
		{
			name:       "content uses task endpoint",
			mode:       mode.VideoGenerationsContent,
			generation: "task-123",
			wantMethod: http.MethodGet,
			wantURL:    "https://dashscope.aliyuncs.com/custom/api/v1/tasks/task-123",
		},
		{
			name:       "official videos create uses video synthesis endpoint",
			mode:       mode.Videos,
			wantMethod: http.MethodPost,
			wantURL:    "https://dashscope.aliyuncs.com/custom/api/v1/services/aigc/video-generation/video-synthesis",
		},
		{
			name:       "official videos get uses task endpoint",
			mode:       mode.VideosGet,
			generation: "task-123",
			wantMethod: http.MethodGet,
			wantURL:    "https://dashscope.aliyuncs.com/custom/api/v1/tasks/task-123",
		},
		{
			name:       "official videos content uses task endpoint",
			mode:       mode.VideosContent,
			generation: "task-123",
			wantMethod: http.MethodGet,
			wantURL:    "https://dashscope.aliyuncs.com/custom/api/v1/tasks/task-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(
				channel,
				tt.mode,
				"wan2.5-t2v-preview",
				coremodel.ModelConfig{},
				meta.WithJobID(tt.jobID),
				meta.WithGenerationID(tt.generation),
				meta.WithVideoID(tt.generation),
			)

			got, err := adaptor.GetRequestURL(m, nil, nil)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			if got.Method != tt.wantMethod {
				t.Fatalf("expected method %s, got %s", tt.wantMethod, got.Method)
			}

			if got.URL != tt.wantURL {
				t.Fatalf("expected URL %s, got %s", tt.wantURL, got.URL)
			}
		})
	}
}

func TestAdaptorConvertRequestResponses(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Responses,
		"qwen-plus",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/responses",
		strings.NewReader(`{"model":"qwen-plus","input":"hello","stream":true}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var responseReq relaymodel.CreateResponseRequest
	if err := json.Unmarshal(body, &responseReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if responseReq.Model != "qwen-plus" {
		t.Fatalf("expected model qwen-plus, got %s", responseReq.Model)
	}

	if !responseReq.Stream {
		t.Fatal("expected stream to remain enabled")
	}
}

func TestAdaptorConvertRequestAliTextToVideo(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.5-t2v-preview", coremodel.ModelConfig{})
	m.ActualModel = "wan2.5-t2v-preview"

	body := `{
		"model":"wan2.5-t2v-preview",
		"prompt":"A camera moves through a quiet city street",
		"n_seconds":5,
		"width":1280,
		"height":720,
		"metadata":{
			"prompt_extend":true,
			"seed":123,
			"shot_type":"orbit"
		}
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	if result.Header.Get("X-Dashscope-Async") != "enable" {
		t.Fatalf("expected async header, got %#v", result.Header)
	}

	payload := readJSONMap(t, result.Body)
	if payload["model"] != "wan2.5-t2v-preview" {
		t.Fatalf("expected model wan2.5-t2v-preview, got %#v", payload["model"])
	}

	input := mustMap(t, payload["input"])
	if input["prompt"] != "A camera moves through a quiet city street" {
		t.Fatalf("unexpected prompt: %#v", input["prompt"])
	}

	parameters := mustMap(t, payload["parameters"])
	if int(mustFloat64(t, parameters["duration"])) != 5 {
		t.Fatalf("expected duration 5, got %#v", parameters["duration"])
	}

	if parameters["size"] != "1280*720" {
		t.Fatalf("expected size 1280*720, got %#v", parameters["size"])
	}

	if parameters["prompt_extend"] != true {
		t.Fatalf("expected prompt_extend true, got %#v", parameters["prompt_extend"])
	}

	if int64(mustFloat64(t, parameters["seed"])) != 123 {
		t.Fatalf("expected seed 123, got %#v", parameters["seed"])
	}

	if parameters["shot_type"] != "orbit" {
		t.Fatalf("expected shot_type orbit, got %#v", parameters["shot_type"])
	}
}

func TestAdaptorConvertRequestHappyHorseDimensionsUseNearestResolution(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "happyhorse-1.0-t2v", coremodel.ModelConfig{})
	m.ActualModel = "happyhorse-1.0-t2v"

	body := `{
		"model":"happyhorse-1.0-t2v",
		"prompt":"公鸡打篮球",
		"n_seconds":4,
		"width":480,
		"height":480
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	parameters := mustMap(t, payload["parameters"])

	if parameters["resolution"] != "720P" {
		t.Fatalf(
			"expected dimensions to map to nearest resolution 720P, got %#v",
			parameters["resolution"],
		)
	}

	if parameters["ratio"] != "1:1" {
		t.Fatalf("expected square dimensions to map to ratio 1:1, got %#v", parameters["ratio"])
	}

	if _, ok := parameters["size"]; ok {
		t.Fatalf("expected size not to be sent for happyhorse, got %#v", parameters["size"])
	}
}

func TestAdaptorConvertRequestAliWan27ImageToVideoUsesMedia(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	body := `{
		"model":"wan2.7-i2v",
		"prompt":"Make the scene move slowly",
		"image_url":"https://example.com/start.png",
		"last_frame_url":"https://example.com/end.png",
		"n_seconds":10,
		"width":1920,
		"height":1080,
		"audio_url":"https://example.com/music.wav"
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])

	media := mustSlice(t, input["media"])
	if len(media) != 3 {
		t.Fatalf("expected 3 media items, got %#v", media)
	}

	firstFrame := mustMap(t, media[0])
	if firstFrame["type"] != "first_frame" || firstFrame["url"] != "https://example.com/start.png" {
		t.Fatalf("unexpected first media: %#v", firstFrame)
	}

	lastFrame := mustMap(t, media[1])
	if lastFrame["type"] != "last_frame" || lastFrame["url"] != "https://example.com/end.png" {
		t.Fatalf("unexpected second media: %#v", lastFrame)
	}

	audio := mustMap(t, media[2])
	if audio["type"] != "driving_audio" || audio["url"] != "https://example.com/music.wav" {
		t.Fatalf("unexpected third media: %#v", audio)
	}

	parameters := mustMap(t, payload["parameters"])
	if parameters["duration"] != float64(10) {
		t.Fatalf("expected duration 10, got %#v", parameters["duration"])
	}

	if parameters["resolution"] != "1080P" {
		t.Fatalf(
			"expected dimensions to map to nearest resolution 1080P, got %#v",
			parameters["resolution"],
		)
	}

	if _, ok := parameters["ratio"]; ok {
		t.Fatalf(
			"expected ratio not to be auto-generated when media has first_frame, got %#v",
			parameters["ratio"],
		)
	}
}

func TestAdaptorConvertRequestAliVideoPrefersOpenAIFields(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	body := `{
		"model":"wan2.7-i2v",
		"prompt":"Animate this reference",
		"input_reference":"https://example.com/openai-reference.png",
		"image_url":"https://example.com/ali-image.png",
		"n_seconds":4,
		"size":"1080p",
		"width":1280,
		"height":720
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])
	media := mustSlice(t, input["media"])

	firstFrame := mustMap(t, media[0])
	if firstFrame["url"] != "https://example.com/openai-reference.png" {
		t.Fatalf("expected input_reference to win, got %#v", firstFrame["url"])
	}

	parameters := mustMap(t, payload["parameters"])
	if int(mustFloat64(t, parameters["duration"])) != 4 {
		t.Fatalf("expected n_seconds to win, got %#v", parameters["duration"])
	}

	if parameters["resolution"] != "720P" {
		t.Fatalf(
			"expected dimensions to map to nearest resolution, got %#v",
			parameters["resolution"],
		)
	}

	if _, ok := parameters["ratio"]; ok {
		t.Fatalf(
			"expected ratio not to be auto-generated when media has first_frame, got %#v",
			parameters["ratio"],
		)
	}

	if _, ok := parameters["size"]; ok {
		t.Fatalf("expected size not to be set for 1080p, got %#v", parameters["size"])
	}
}

func TestAdaptorConvertRequestAliVideoDimensionsSkipAutoRatioForVideoMedia(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"happyhorse-1.0-video-edit",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "happyhorse-1.0-video-edit"

	body := `{
		"model":"happyhorse-1.0-video-edit",
		"prompt":"Make the video cinematic",
		"video_url":"https://example.com/input.mp4",
		"n_seconds":4,
		"width":720,
		"height":1280
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])
	media := mustSlice(t, input["media"])

	video := mustMap(t, media[0])
	if video["type"] != "video" || video["url"] != "https://example.com/input.mp4" {
		t.Fatalf("unexpected video media: %#v", video)
	}

	parameters := mustMap(t, payload["parameters"])
	if parameters["resolution"] != "720P" {
		t.Fatalf(
			"expected dimensions to map to nearest resolution 720P, got %#v",
			parameters["resolution"],
		)
	}

	if _, ok := parameters["ratio"]; ok {
		t.Fatalf(
			"expected ratio not to be auto-generated when media has video, got %#v",
			parameters["ratio"],
		)
	}
}

func TestAdaptorConvertRequestAliHappyHorseVideoEditMapsImageAndVideoMedia(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"happyhorse-1.0-video-edit",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "happyhorse-1.0-video-edit"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model":"happyhorse-1.0-video-edit",
			"prompt":"Change clothes",
			"video_url":"https://example.com/input.mp4",
			"image_url":"https://example.com/reference.png",
			"n_seconds":4,
			"width":1280,
			"height":720
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])

	media := mustSlice(t, input["media"])
	if len(media) != 2 {
		t.Fatalf("expected 2 media items, got %#v", media)
	}

	referenceImage := mustMap(t, media[0])
	if referenceImage["type"] != "reference_image" ||
		referenceImage["url"] != "https://example.com/reference.png" {
		t.Fatalf("unexpected reference image media: %#v", referenceImage)
	}

	video := mustMap(t, media[1])
	if video["type"] != "video" || video["url"] != "https://example.com/input.mp4" {
		t.Fatalf("unexpected video media: %#v", video)
	}
}

func TestAdaptorConvertRequestAliVideoEditMapsFirstClipAliasToVideoMedia(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"happyhorse-1.0-video-edit",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "happyhorse-1.0-video-edit"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model":"happyhorse-1.0-video-edit",
			"prompt":"Change clothes",
			"first_clip_url":"https://example.com/input.mp4",
			"reference_urls":["https://example.com/reference.png"],
			"n_seconds":4,
			"width":1280,
			"height":720
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])

	media := mustSlice(t, input["media"])
	if len(media) != 2 {
		t.Fatalf("expected 2 media items, got %#v", media)
	}

	video := mustMap(t, media[0])
	if video["type"] != "video" || video["url"] != "https://example.com/input.mp4" {
		t.Fatalf("unexpected video media: %#v", video)
	}

	referenceImage := mustMap(t, media[1])
	if referenceImage["type"] != "reference_image" ||
		referenceImage["url"] != "https://example.com/reference.png" {
		t.Fatalf("unexpected reference image media: %#v", referenceImage)
	}
}

func TestAdaptorConvertRequestAliWanVideoEditMapsImageAndVideoMedia(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.7-videoedit", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-videoedit"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model":"wan2.7-videoedit",
			"prompt":"Change clothes",
			"video_url":"https://example.com/input.mp4",
			"input_reference":"https://example.com/reference.png",
			"n_seconds":4,
			"width":1280,
			"height":720
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])

	media := mustSlice(t, input["media"])
	if len(media) != 2 {
		t.Fatalf("expected 2 media items, got %#v", media)
	}

	referenceImage := mustMap(t, media[0])
	if referenceImage["type"] != "reference_image" ||
		referenceImage["url"] != "https://example.com/reference.png" {
		t.Fatalf("unexpected reference image media: %#v", referenceImage)
	}

	video := mustMap(t, media[1])
	if video["type"] != "video" || video["url"] != "https://example.com/input.mp4" {
		t.Fatalf("unexpected video media: %#v", video)
	}
}

func TestAdaptorConvertVideosEditMapsVideoFieldToVideoMedia(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideosEdits, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/edits",
		strings.NewReader(`{
			"model":"wan2.7-i2v",
			"prompt":"change the color",
			"video":"https://example.com/source.mp4",
			"seconds":4,
			"size":"1280x720"
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])
	media := mustSlice(t, input["media"])

	video := mustMap(t, media[0])
	if video["type"] != "video" || video["url"] != "https://example.com/source.mp4" {
		t.Fatalf("unexpected edit media: %#v", video)
	}
}

func TestAdaptorConvertVideosExtensionMapsVideoFieldToFirstClip(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideosExtensions, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/extensions",
		strings.NewReader(`{
			"model":"wan2.7-i2v",
			"prompt":"continue the shot",
			"video":"https://example.com/source.mp4",
			"seconds":4,
			"size":"1280x720"
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])
	media := mustSlice(t, input["media"])

	video := mustMap(t, media[0])
	if video["type"] != "first_clip" || video["url"] != "https://example.com/source.mp4" {
		t.Fatalf("unexpected extension media: %#v", video)
	}
}

func TestAdaptorConvertVideosEditHydratesStoredVideoID(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks/task-123" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		_, _ = w.Write([]byte(`{
			"output":{
				"task_id":"task-123",
				"task_status":"SUCCEEDED",
				"video_url":"https://example.com/source.mp4"
			}
		}`))
	}))
	defer upstream.Close()

	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: upstream.URL},
		mode.VideosEdits,
		"wan2.7-i2v",
		coremodel.ModelConfig{},
		meta.WithVideoID("task-123"),
	)
	m.ActualModel = "wan2.7-i2v"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/edits",
		strings.NewReader(`{
			"prompt":"change the color",
			"video":"task-123",
			"seconds":4,
			"size":"1280x720"
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])
	media := mustSlice(t, input["media"])

	video := mustMap(t, media[0])
	if video["type"] != "video" || video["url"] != "https://example.com/source.mp4" {
		t.Fatalf("unexpected hydrated edit media: %#v", video)
	}
}

func TestAdaptorConvertRequestAliReferenceImageKeepsAutoRatio(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "happyhorse-1.0-r2v", coremodel.ModelConfig{})
	m.ActualModel = "happyhorse-1.0-r2v"

	body := `{
		"model":"happyhorse-1.0-r2v",
		"prompt":"Animate these references",
		"reference_urls":["https://example.com/ref.png"],
		"n_seconds":4,
		"width":720,
		"height":1280
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])
	media := mustSlice(t, input["media"])

	reference := mustMap(t, media[0])
	if reference["type"] != "reference_image" || reference["url"] != "https://example.com/ref.png" {
		t.Fatalf("unexpected reference image media: %#v", reference)
	}

	parameters := mustMap(t, payload["parameters"])
	if parameters["resolution"] != "720P" {
		t.Fatalf(
			"expected dimensions to map to nearest resolution 720P, got %#v",
			parameters["resolution"],
		)
	}

	if parameters["ratio"] != "9:16" {
		t.Fatalf("expected reference_image to keep auto ratio 9:16, got %#v", parameters["ratio"])
	}
}

func TestAdaptorConvertRequestAliReferenceVideoModelMapsVideoToReferenceVideo(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.7-r2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-r2v"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model":"wan2.7-r2v",
			"prompt":"Animate these references",
			"image_url":"https://example.com/ref.png",
			"video_url":"https://example.com/ref.mp4",
			"n_seconds":4,
			"width":1280,
			"height":720
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])

	media := mustSlice(t, input["media"])
	if len(media) != 2 {
		t.Fatalf("expected 2 media items, got %#v", media)
	}

	image := mustMap(t, media[0])
	if image["type"] != "reference_image" || image["url"] != "https://example.com/ref.png" {
		t.Fatalf("unexpected reference image media: %#v", image)
	}

	video := mustMap(t, media[1])
	if video["type"] != "reference_video" || video["url"] != "https://example.com/ref.mp4" {
		t.Fatalf("unexpected reference video media: %#v", video)
	}
}

func TestAdaptorConvertRequestAliI2VModelMapsImageToFirstFrame(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "happyhorse-1.0-i2v", coremodel.ModelConfig{})
	m.ActualModel = "happyhorse-1.0-i2v"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model":"happyhorse-1.0-i2v",
			"prompt":"Animate the image",
			"image_url":"https://example.com/start.png",
			"n_seconds":4,
			"width":1280,
			"height":720
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])
	media := mustSlice(t, input["media"])

	firstFrame := mustMap(t, media[0])
	if firstFrame["type"] != "first_frame" || firstFrame["url"] != "https://example.com/start.png" {
		t.Fatalf("unexpected first frame media: %#v", firstFrame)
	}
}

func TestAdaptorConvertRequestAliVideoRejectsDangerousAliAliases(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	body := `{
		"model":"wan2.7-i2v",
		"prompt":"Animate this reference",
		"input_reference":"https://example.com/openai-reference.png",
		"duration":8,
		"resolution":"1080P"
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	_, err = adaptor.ConvertRequest(m, nil, req)
	if err == nil {
		t.Fatal("expected ConvertRequest to reject Ali aliases")
	}

	if !strings.Contains(err.Error(), "duration is not supported") {
		t.Fatalf("expected duration alias error, got %v", err)
	}
}

func TestAdaptorConvertRequestAliVideoMultipartInputReference(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "wan2.7-i2v")
	_ = writer.WriteField("prompt", "Animate the uploaded reference")
	_ = writer.WriteField("n_seconds", "3")

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="input_reference"; filename="reference.png"`)
	header.Set("Content-Type", "image/png")

	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}

	if _, err := part.Write([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}); err != nil {
		t.Fatalf("failed to write test image: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("writer close returned error: %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		&body,
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	input := mustMap(t, payload["input"])
	media := mustSlice(t, input["media"])
	firstFrame := mustMap(t, media[0])

	url, ok := firstFrame["url"].(string)
	if !ok {
		t.Fatalf("expected reference url string, got %#v", firstFrame["url"])
	}

	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("expected png data url, got %#v", url)
	}

	parameters := mustMap(t, payload["parameters"])
	if int(mustFloat64(t, parameters["duration"])) != 3 {
		t.Fatalf("expected duration 3, got %#v", parameters["duration"])
	}
}

func TestAdaptorConvertRequestQwenImageGeneration(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesGenerations, "qwen-image-plus", coremodel.ModelConfig{})
	m.ActualModel = "mapped-qwen-image"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(`{
			"model": "qwen-image-plus",
			"prompt": "draw a cat",
			"size": "1024×1024",
			"negative_prompt": "low quality",
			"prompt_extend": true,
			"watermark": false,
			"seed": 123,
			"quality": "high",
			"response_format": "b64_json"
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(body), err)
	}

	if payload["model"] != "mapped-qwen-image" {
		t.Fatalf("expected mapped model, got %#v", payload["model"])
	}

	input := mustMap(t, payload["input"])
	messages := mustSlice(t, input["messages"])
	message := mustMap(t, messages[0])
	contents := mustSlice(t, message["content"])
	assertContentString(t, contents[0], "text", "draw a cat")

	parameters := mustMap(t, payload["parameters"])
	if parameters["size"] != "1024*1024" {
		t.Fatalf("expected size 1024*1024, got %#v", parameters["size"])
	}

	if parameters["negative_prompt"] != "low quality" {
		t.Fatalf("expected negative_prompt, got %#v", parameters["negative_prompt"])
	}

	if parameters["prompt_extend"] != true {
		t.Fatalf("expected prompt_extend true, got %#v", parameters["prompt_extend"])
	}

	if parameters["watermark"] != false {
		t.Fatalf("expected watermark false, got %#v", parameters["watermark"])
	}

	if int64(mustFloat64(t, parameters["seed"])) != 123 {
		t.Fatalf("expected seed 123, got %#v", parameters["seed"])
	}

	if _, ok := parameters["quality"]; ok {
		t.Fatalf("expected quality not to be forwarded, got %#v", parameters["quality"])
	}

	if _, ok := parameters["n"]; ok {
		t.Fatalf("expected n not to be forwarded for this model, got %#v", parameters["n"])
	}
}

func TestAdaptorConvertRequestQwenImageGenerationRejectsUnsupportedN(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesGenerations, "qwen-image-plus", coremodel.ModelConfig{})
	m.ActualModel = "qwen-image-plus"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(`{
			"model": "qwen-image-plus",
			"prompt": "draw a cat",
			"n": 2
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = adaptor.ConvertRequest(m, nil, req)
	if err == nil || !strings.Contains(err.Error(), "n must be 1") {
		t.Fatalf("expected unsupported n error, got %v", err)
	}
}

func TestAdaptorConvertRequestQwenImageGenerationSupportsN(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesGenerations, "qwen-image-2.0-pro", coremodel.ModelConfig{})
	m.ActualModel = "qwen-image-2.0-pro"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(`{
			"model": "qwen-image-2.0-pro",
			"prompt": "draw a cat",
			"n": 2
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(body), err)
	}

	parameters := mustMap(t, payload["parameters"])
	if int(mustFloat64(t, parameters["n"])) != 2 {
		t.Fatalf("expected n 2, got %#v", parameters["n"])
	}
}

func TestAdaptorConvertRequestWan27ImageGeneration(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesGenerations, "wan2.7-image-pro", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-image-pro"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(`{
			"model": "wan2.7-image-pro",
			"prompt": "create a product render",
			"size": "2K",
			"n": 4,
			"watermark": false,
			"thinking_mode": true,
			"enable_sequential": true,
			"prompt_extend": true,
			"seed": 123
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(body), err)
	}

	input := mustMap(t, payload["input"])
	messages := mustSlice(t, input["messages"])
	message := mustMap(t, messages[0])
	contents := mustSlice(t, message["content"])
	assertContentString(t, contents[0], "text", "create a product render")

	parameters := mustMap(t, payload["parameters"])
	if parameters["size"] != "2K" {
		t.Fatalf("expected size 2K, got %#v", parameters["size"])
	}

	if int(mustFloat64(t, parameters["n"])) != 4 {
		t.Fatalf("expected n 4, got %#v", parameters["n"])
	}

	if parameters["watermark"] != false {
		t.Fatalf("expected watermark false, got %#v", parameters["watermark"])
	}

	if parameters["thinking_mode"] != true {
		t.Fatalf("expected thinking_mode true, got %#v", parameters["thinking_mode"])
	}

	if parameters["enable_sequential"] != true {
		t.Fatalf("expected enable_sequential true, got %#v", parameters["enable_sequential"])
	}

	if _, ok := parameters["prompt_extend"]; ok {
		t.Fatalf("expected prompt_extend not to be forwarded, got %#v", parameters)
	}

	if _, ok := parameters["seed"]; ok {
		t.Fatalf("expected seed not to be forwarded, got %#v", parameters)
	}
}

func TestAdaptorConvertRequestWan26ImageGeneration(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesGenerations, "wan2.6-t2i", coremodel.ModelConfig{})
	m.ActualModel = "wan2.6-t2i"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(`{
			"model": "wan2.6-t2i",
			"prompt": "create a scene",
			"size": "1280x1280",
			"n": 2,
			"negative_prompt": "low quality",
			"prompt_extend": true,
			"watermark": false,
			"seed": 12345
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(body), err)
	}

	parameters := mustMap(t, payload["parameters"])
	if parameters["size"] != "1280*1280" {
		t.Fatalf("expected size 1280*1280, got %#v", parameters["size"])
	}

	if int(mustFloat64(t, parameters["n"])) != 2 {
		t.Fatalf("expected n 2, got %#v", parameters["n"])
	}

	if parameters["negative_prompt"] != "low quality" {
		t.Fatalf("expected negative_prompt, got %#v", parameters["negative_prompt"])
	}

	if parameters["prompt_extend"] != true {
		t.Fatalf("expected prompt_extend true, got %#v", parameters["prompt_extend"])
	}

	if parameters["watermark"] != false {
		t.Fatalf("expected watermark false, got %#v", parameters["watermark"])
	}

	if int64(mustFloat64(t, parameters["seed"])) != 12345 {
		t.Fatalf("expected seed 12345, got %#v", parameters["seed"])
	}
}

func TestAdaptorConvertRequestZImageGeneration(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesGenerations, "z-image-turbo", coremodel.ModelConfig{})
	m.ActualModel = "z-image-turbo"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(`{
			"model": "z-image-turbo",
			"prompt": "create a poster",
			"size": "1024x1024",
			"prompt_extend": true
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(body), err)
	}

	parameters := mustMap(t, payload["parameters"])
	if parameters["size"] != "1024*1024" {
		t.Fatalf("expected size 1024*1024, got %#v", parameters["size"])
	}

	if parameters["prompt_extend"] != true {
		t.Fatalf("expected prompt_extend true, got %#v", parameters["prompt_extend"])
	}

	if _, ok := parameters["n"]; ok {
		t.Fatalf("expected n not to be forwarded, got %#v", parameters)
	}
}

func TestAdaptorConvertRequestLegacyWanImageGeneration(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesGenerations, "wanx-v1", coremodel.ModelConfig{})
	m.ActualModel = "wanx-v1"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(`{
			"model": "wanx-v1",
			"prompt": "create a scene",
			"size": "1024x1024",
			"n": 1,
			"negative_prompt": "low quality",
			"style": "<auto>",
			"ref_image": "https://example.com/ref.png",
			"ref_strength": 0.7,
			"ref_mode": "repaint",
			"prompt_extend": true,
			"watermark": false,
			"seed": 12345
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	if result.Header.Get("X-Dashscope-Async") != "enable" {
		t.Fatalf("expected async header, got %#v", result.Header)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(body), err)
	}

	input := mustMap(t, payload["input"])
	if input["prompt"] != "create a scene" {
		t.Fatalf("expected prompt, got %#v", input["prompt"])
	}

	if input["negative_prompt"] != "low quality" {
		t.Fatalf("expected negative_prompt, got %#v", input["negative_prompt"])
	}

	if input["ref_image"] != "https://example.com/ref.png" {
		t.Fatalf("expected ref_image, got %#v", input["ref_image"])
	}

	parameters := mustMap(t, payload["parameters"])
	if parameters["size"] != "1024*1024" {
		t.Fatalf("expected size 1024*1024, got %#v", parameters["size"])
	}

	if parameters["style"] != "<auto>" {
		t.Fatalf("expected style, got %#v", parameters["style"])
	}

	if parameters["ref_mode"] != "repaint" {
		t.Fatalf("expected ref_mode, got %#v", parameters["ref_mode"])
	}

	if parameters["prompt_extend"] != true {
		t.Fatalf("expected prompt_extend true, got %#v", parameters["prompt_extend"])
	}

	if parameters["watermark"] != false {
		t.Fatalf("expected watermark false, got %#v", parameters["watermark"])
	}

	if int64(mustFloat64(t, parameters["seed"])) != 12345 {
		t.Fatalf("expected seed 12345, got %#v", parameters["seed"])
	}
}

func TestAdaptorConvertRequestQwenImageEdit(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesEdits, "qwen-image-edit-plus", coremodel.ModelConfig{})
	m.ActualModel = "mapped-qwen-edit"

	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)
	if err := writer.WriteField("prompt", "add a hat"); err != nil {
		t.Fatalf("failed to write prompt: %v", err)
	}

	if err := writer.WriteField("size", "1024x1024"); err != nil {
		t.Fatalf("failed to write size: %v", err)
	}

	if err := writer.WriteField("negative_prompt", "blurry"); err != nil {
		t.Fatalf("failed to write negative_prompt: %v", err)
	}

	if err := writer.WriteField("prompt_extend", "true"); err != nil {
		t.Fatalf("failed to write prompt_extend: %v", err)
	}

	if err := writer.WriteField("watermark", "false"); err != nil {
		t.Fatalf("failed to write watermark: %v", err)
	}

	if err := writer.WriteField("seed", "456"); err != nil {
		t.Fatalf("failed to write seed: %v", err)
	}

	if err := writer.WriteField("response_format", "url"); err != nil {
		t.Fatalf("failed to write response_format: %v", err)
	}

	part, err := writer.CreateFormFile("image", "input.png")
	if err != nil {
		t.Fatalf("failed to create file part: %v", err)
	}

	_, _ = part.Write([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	})

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/edits",
		body,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	convertedBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(convertedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(convertedBody), err)
	}

	if payload["model"] != "mapped-qwen-edit" {
		t.Fatalf("expected mapped model, got %#v", payload["model"])
	}

	input := mustMap(t, payload["input"])
	messages := mustSlice(t, input["messages"])
	message := mustMap(t, messages[0])
	contents := mustSlice(t, message["content"])
	assertImageContentHasPNGPrefix(t, contents[0])
	assertContentString(t, contents[1], "text", "add a hat")

	parameters := mustMap(t, payload["parameters"])
	if parameters["size"] != "1024*1024" {
		t.Fatalf("expected size 1024*1024, got %#v", parameters["size"])
	}

	if parameters["negative_prompt"] != "blurry" {
		t.Fatalf("expected negative_prompt, got %#v", parameters["negative_prompt"])
	}

	if parameters["prompt_extend"] != true {
		t.Fatalf("expected prompt_extend true, got %#v", parameters["prompt_extend"])
	}

	if parameters["watermark"] != false {
		t.Fatalf("expected watermark false, got %#v", parameters["watermark"])
	}

	if int64(mustFloat64(t, parameters["seed"])) != 456 {
		t.Fatalf("expected seed 456, got %#v", parameters["seed"])
	}
}

func TestAdaptorConvertRequestQwenImageEditAcceptsImageArrayField(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesEdits, "qwen-image-edit-plus", coremodel.ModelConfig{})
	m.ActualModel = "qwen-image-edit-plus"

	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)
	if err := writer.WriteField("prompt", "blend two images"); err != nil {
		t.Fatalf("failed to write prompt: %v", err)
	}

	for _, filename := range []string{"input1.png", "input2.png"} {
		part, err := writer.CreateFormFile("image[]", filename)
		if err != nil {
			t.Fatalf("failed to create file part: %v", err)
		}

		_, _ = part.Write([]byte{
			0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
			0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		})
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/edits",
		body,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	convertedBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(convertedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(convertedBody), err)
	}

	input := mustMap(t, payload["input"])
	messages := mustSlice(t, input["messages"])
	message := mustMap(t, messages[0])
	contents := mustSlice(t, message["content"])

	if len(contents) != 3 {
		t.Fatalf("expected 2 images and 1 text content, got %d", len(contents))
	}

	assertImageContentHasPNGPrefix(t, contents[0])
	assertImageContentHasPNGPrefix(t, contents[1])
	assertContentString(t, contents[2], "text", "blend two images")
}

func TestAdaptorConvertRequestQwenImageEditBaseModelFiltersUnsupportedParams(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesEdits, "qwen-image-edit", coremodel.ModelConfig{})
	m.ActualModel = "qwen-image-edit"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("prompt", "add a hat")
	_ = writer.WriteField("size", "1024x1024")
	_ = writer.WriteField("prompt_extend", "true")

	part, err := writer.CreateFormFile("image", "input.png")
	if err != nil {
		t.Fatalf("failed to create file part: %v", err)
	}

	_, _ = part.Write([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	})

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/edits",
		body,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	convertedBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(convertedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(convertedBody), err)
	}

	if parameters, ok := payload["parameters"].(map[string]any); ok {
		if _, ok := parameters["size"]; ok {
			t.Fatalf("expected size not to be forwarded for qwen-image-edit, got %#v", parameters)
		}

		if _, ok := parameters["prompt_extend"]; ok {
			t.Fatalf(
				"expected prompt_extend not to be forwarded for qwen-image-edit, got %#v",
				parameters,
			)
		}
	}
}

func TestAdaptorConvertRequestWan27ImageEdit(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesEdits, "wan2.7-image-pro", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-image-pro"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("prompt", "edit the product image")
	_ = writer.WriteField("size", "2K")
	_ = writer.WriteField("n", "2")
	_ = writer.WriteField("watermark", "false")
	_ = writer.WriteField("bbox_list", `[[10,20,200,300]]`)

	part, err := writer.CreateFormFile("image[]", "input.png")
	if err != nil {
		t.Fatalf("failed to create file part: %v", err)
	}

	_, _ = part.Write([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	})

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/edits",
		body,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	convertedBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(convertedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(convertedBody), err)
	}

	input := mustMap(t, payload["input"])
	messages := mustSlice(t, input["messages"])
	message := mustMap(t, messages[0])
	contents := mustSlice(t, message["content"])
	assertImageContentHasPNGPrefix(t, contents[0])
	assertContentString(t, contents[1], "text", "edit the product image")

	parameters := mustMap(t, payload["parameters"])
	if parameters["size"] != "2K" {
		t.Fatalf("expected size 2K, got %#v", parameters["size"])
	}

	if int(mustFloat64(t, parameters["n"])) != 2 {
		t.Fatalf("expected n 2, got %#v", parameters["n"])
	}

	if parameters["watermark"] != false {
		t.Fatalf("expected watermark false, got %#v", parameters["watermark"])
	}

	bboxList := mustSlice(t, parameters["bbox_list"])
	if len(bboxList) != 1 {
		t.Fatalf("expected bbox_list, got %#v", parameters["bbox_list"])
	}
}

func TestAdaptorConvertRequestQwenMTImage(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesEdits, "qwen-mt-image", coremodel.ModelConfig{})
	m.ActualModel = "qwen-mt-image"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("image_url", "https://example.com/input.png")
	_ = writer.WriteField("source_lang", "en")
	_ = writer.WriteField("target_lang", "zh")
	_ = writer.WriteField("imageSegment", "false")

	_ = writer.WriteField("response_format", "url")
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/edits",
		body,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	if result.Header.Get("X-Dashscope-Async") != "enable" {
		t.Fatalf("expected async header, got %#v", result.Header)
	}

	convertedBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(convertedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(convertedBody), err)
	}

	input := mustMap(t, payload["input"])
	if input["image_url"] != "https://example.com/input.png" {
		t.Fatalf("expected image_url, got %#v", input["image_url"])
	}

	if input["source_lang"] != "en" {
		t.Fatalf("expected source_lang en, got %#v", input["source_lang"])
	}

	if input["target_lang"] != "zh" {
		t.Fatalf("expected target_lang zh, got %#v", input["target_lang"])
	}

	ext := mustMap(t, input["ext"])

	config := mustMap(t, ext["config"])
	if config["imageSegment"] != false {
		t.Fatalf("expected imageSegment false, got %#v", config["imageSegment"])
	}
}

func TestAdaptorConvertRequestQwenMTImageAcceptsUploadedImage(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.ImagesEdits, "qwen-mt-image", coremodel.ModelConfig{})
	m.ActualModel = "qwen-mt-image"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("source_lang", "en")
	_ = writer.WriteField("target_lang", "zh")

	part, err := writer.CreateFormFile("image", "input.png")
	if err != nil {
		t.Fatalf("failed to create file part: %v", err)
	}

	_, _ = part.Write([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	})

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/edits",
		body,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	convertedBody, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(convertedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(convertedBody), err)
	}

	input := mustMap(t, payload["input"])

	imageURL, ok := input["image_url"].(string)
	if !ok {
		t.Fatalf("expected image_url string, got %#v", input["image_url"])
	}

	if !strings.HasPrefix(imageURL, "data:image/png;base64,") {
		t.Fatalf("expected image_url data URL, got %#v", imageURL)
	}
}

func TestAdaptorConvertRequestMultimodalEmbeddings(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Embeddings,
		"qwen3-vl-embedding",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "mapped-vl-embedding"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/embeddings",
		strings.NewReader(`{
			"model": "qwen3-vl-embedding",
			"input": [
				{
					"type": "image_url",
					"image_url": {
						"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg"
					}
				},
				{
					"type": "text",
					"text": "describe this image"
				},
				"plain text",
				{
					"image": "https://example.com/image.jpg"
				}
			],
			"dimensions": 1024,
			"encoding_format": "float"
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body %s: %v", string(body), err)
	}

	if payload["model"] != "mapped-vl-embedding" {
		t.Fatalf("expected mapped model, got %#v", payload["model"])
	}

	if _, exists := payload["encoding_format"]; exists {
		t.Fatal("expected encoding_format to be removed")
	}

	if _, exists := payload["dimensions"]; exists {
		t.Fatal("expected dimensions to be removed")
	}

	parameters := mustMap(t, payload["parameters"])
	if dimension := mustFloat64(t, parameters["dimension"]); int(dimension) != 1024 {
		t.Fatalf("expected parameters.dimension=1024, got %#v", parameters["dimension"])
	}

	input := mustMap(t, payload["input"])
	contents := mustSlice(t, input["contents"])

	if len(contents) != 4 {
		t.Fatalf("expected 4 contents, got %d", len(contents))
	}

	assertContentString(t, contents[0], "image", "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg")
	assertContentString(t, contents[1], "text", "describe this image")
	assertContentString(t, contents[2], "text", "plain text")
	assertContentString(t, contents[3], "image", "https://example.com/image.jpg")
}

func TestAdaptorDoResponseQwenImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		nil,
	)

	m := meta.NewMeta(nil, mode.ImagesGenerations, "qwen-image-2.0-pro", coremodel.ModelConfig{})
	m.Set(MetaResponseFormat, "url")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id": "req-1",
			"output": {
				"choices": [
					{
						"message": {
							"content": [
								{"image": "https://example.com/out.png"}
							]
						}
					}
				]
			},
			"usage": {
				"height": 2048,
				"image_count": 1,
				"width": 2048
			}
		}`)),
	}

	result, adaptorErr := adaptor.DoResponse(m, nil, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	var imageResponse relaymodel.ImageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &imageResponse); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(imageResponse.Data) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imageResponse.Data))
	}

	if imageResponse.Data[0].URL != "https://example.com/out.png" {
		t.Fatalf("expected image URL, got %#v", imageResponse.Data[0].URL)
	}

	if int64(result.Usage.InputTokens) != 0 ||
		int64(result.Usage.OutputTokens) != 1 ||
		int64(result.Usage.ImageOutputTokens) != 1 ||
		int64(result.Usage.TotalTokens) != 1 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}
}

func TestAdaptorDoResponseQwenImageB64DownloadFailureFallsBackToURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	imageServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}),
	)
	defer imageServer.Close()

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		nil,
	)

	m := meta.NewMeta(nil, mode.ImagesGenerations, "qwen-image-2.0-pro", coremodel.ModelConfig{})
	m.Set(MetaResponseFormat, "b64_json")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id": "req-1",
			"output": {
				"choices": [
					{
						"message": {
							"content": [
								{"image": "` + imageServer.URL + `/missing.png"}
							]
						}
					}
				]
			},
			"usage": {
				"image_count": 1
			}
		}`)),
	}

	result, adaptorErr := adaptor.DoResponse(m, nil, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	var imageResponse relaymodel.ImageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &imageResponse); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(imageResponse.Data) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imageResponse.Data))
	}

	if imageResponse.Data[0].URL != imageServer.URL+"/missing.png" {
		t.Fatalf("expected fallback image URL, got %#v", imageResponse.Data[0].URL)
	}

	if imageResponse.Data[0].B64Json != "" {
		t.Fatalf(
			"expected empty b64_json on download failure, got %#v",
			imageResponse.Data[0].B64Json,
		)
	}

	if int64(result.Usage.OutputTokens) != 1 ||
		int64(result.Usage.ImageOutputTokens) != 1 ||
		int64(result.Usage.TotalTokens) != 1 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}
}

func TestAdaptorDoResponseQwenImageClearsURLWhenB64JSONRequested(t *testing.T) {
	gin.SetMode(gin.TestMode)

	imageServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-bytes"))
		}),
	)
	defer imageServer.Close()

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		nil,
	)

	m := meta.NewMeta(nil, mode.ImagesGenerations, "qwen-image-2.0-pro", coremodel.ModelConfig{})
	m.Set(MetaResponseFormat, "b64_json")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id": "req-1",
			"output": {
				"choices": [
					{
						"message": {
							"content": [
								{"image": "` + imageServer.URL + `/out.png"}
							]
						}
					}
				]
			},
			"usage": {
				"image_count": 1
			}
		}`)),
	}

	_, adaptorErr := adaptor.DoResponse(m, nil, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	var imageResponse relaymodel.ImageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &imageResponse); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
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

func TestAdaptorDoResponseWanImageUsesImageCountForUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		nil,
	)

	m := meta.NewMeta(nil, mode.ImagesGenerations, "wan2.7-image-pro", coremodel.ModelConfig{})
	m.Set(MetaResponseFormat, "url")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id": "req-1",
			"output": {
				"choices": [
					{
						"message": {
							"content": [
								{"image": "https://example.com/out.png"}
							]
						}
					}
				]
			},
			"usage": {
				"size": "2976*1408",
				"total_tokens": 11017,
				"image_count": 1,
				"output_tokens": 2,
				"input_tokens": 11015
			}
		}`)),
	}

	result, adaptorErr := adaptor.DoResponse(m, nil, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	if int64(result.Usage.InputTokens) != 0 ||
		int64(result.Usage.OutputTokens) != 1 ||
		int64(result.Usage.ImageOutputTokens) != 1 ||
		int64(result.Usage.TotalTokens) != 1 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}
}

func TestResponseAli2OpenAIImageUsesImageURLOutput(t *testing.T) {
	response := &TaskResponse{}
	response.Output.ImageURL = "https://example.com/translated.jpg"
	response.Usage.ImageCount = 1

	imageResponse := responseAli2OpenAIImage(context.Background(), response, "url")

	if len(imageResponse.Data) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imageResponse.Data))
	}

	if imageResponse.Data[0].URL != "https://example.com/translated.jpg" {
		t.Fatalf("expected translated image URL, got %#v", imageResponse.Data[0].URL)
	}

	if imageResponse.Usage.OutputTokens != 1 ||
		imageResponse.Usage.OutputTokensDetails == nil ||
		imageResponse.Usage.OutputTokensDetails.ImageTokens != 1 ||
		imageResponse.Usage.TotalTokens != 1 {
		t.Fatalf("unexpected usage: %#v", imageResponse.Usage)
	}
}

func TestAsyncTaskUsesBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v1/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("expected authorization header, got %#v", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"request_id": "req-1",
			"output": {
				"task_status": "SUCCEEDED",
				"image_url": "https://example.com/out.png"
			},
			"usage": {
				"image_count": 1
			}
		}`))
	}))
	defer server.Close()

	response, err := asyncTask(context.Background(), server.URL+"/custom", "task-123", "test-key")
	if err != nil {
		t.Fatalf("asyncTask returned error: %v", err)
	}

	if response.RequestID != "req-1" {
		t.Fatalf("expected request_id req-1, got %#v", response.RequestID)
	}

	if response.Output.ImageURL != "https://example.com/out.png" {
		t.Fatalf("expected image_url, got %#v", response.Output.ImageURL)
	}

	if response.Usage.ImageCount != 1 {
		t.Fatalf("expected image_count 1, got %#v", response.Usage.ImageCount)
	}
}

func TestAliVideoHandlersConvertTaskToOpenAIJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/video/generations/jobs/task-123",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"wan2.5-t2v-preview",
		coremodel.ModelConfig{},
	)
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}
	m.Set(metaAliVideoWidth, 480)
	m.Set(metaAliVideoHeight, 480)
	m.Set(metaAliVideoSize, "480x480")

	store := &aliTestStore{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id":"req-1",
			"output":{
				"task_id":"task-123",
				"task_status":"SUCCEEDED",
				"submit_time":"2026-05-20 10:00:00.000",
				"end_time":"2026-05-20 10:02:00.000",
				"video_url":"https://example.com/video.mp4",
				"orig_prompt":"A quiet street"
			},
			"usage":{
				"duration":5,
				"video_count":1,
				"SR":720,
				"ratio":"16:9"
			}
		}`)),
	}

	result, adaptorErr := adaptor.DoResponse(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	if result.UpstreamID != "task-123" {
		t.Fatalf("expected upstream task-123, got %#v", result.UpstreamID)
	}

	if len(store.saved) != 1 {
		t.Fatalf("expected 1 saved generation, got %#v", store.saved)
	}

	if store.saved[0].ID != coremodel.VideoGenerationStoreID("task-123") {
		t.Fatalf("unexpected generation store id: %#v", store.saved[0].ID)
	}

	if store.saved[0].Metadata == "" {
		t.Fatal("expected generation store metadata to be saved")
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job: %v", err)
	}

	if job.Object != relaymodel.VideoGenerationJobObject ||
		job.ID != "task-123" ||
		job.Status != relaymodel.VideoGenerationJobStatusSucceeded {
		t.Fatalf("unexpected job: %#v", job)
	}

	if len(job.Generations) != 1 || job.Generations[0].ID != "task-123" {
		t.Fatalf("unexpected generations: %#v", job.Generations)
	}

	if job.Width != 1280 || job.Height != 720 || job.NSeconds != 5 {
		t.Fatalf("unexpected dimensions or duration: %#v", job)
	}

	if job.Generations[0].Width != 1280 || job.Generations[0].Height != 720 {
		t.Fatalf("unexpected generation dimensions: %#v", job.Generations[0])
	}

	if result.UsageContext.Resolution != "1280x720" ||
		result.UsageContext.NativeResolution != "720P" {
		t.Fatalf("unexpected usage context: %#v", result.UsageContext)
	}
}

func TestAliVideoCreateJobUsesRequestMetadataFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"happyhorse-1.0-i2v",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "happyhorse-1.0-i2v"
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model":"happyhorse-1.0-i2v",
			"prompt":"Animate the horse",
			"input_reference":"https://example.com/reference.png",
			"n_seconds":5,
			"width":1280,
			"height":720
		}`),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	if _, err := adaptor.ConvertRequest(m, nil, req); err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id":"req-1",
			"output":{
				"task_id":"task-123",
				"task_status":"PENDING"
			}
		}`)),
	}

	store := &aliTestStore{}

	_, adaptorErr := adaptor.DoResponse(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	if len(store.saved) != 1 ||
		store.saved[0].Metadata != `{"prompt":"Animate the horse","seconds":5,"size":"1280x720"}` {
		t.Fatalf("expected request metadata to be stored, got %#v", store.saved)
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job: %v", err)
	}

	if job.Prompt != "Animate the horse" ||
		job.NSeconds != 5 ||
		job.Width != 1280 ||
		job.Height != 720 {
		t.Fatalf("expected request metadata fallback, got %#v", job)
	}
}

func TestAliVideoGetUsesStoredRequestMetadataFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	aliAdaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/video/generations/jobs/task-123",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"happyhorse-1.0-i2v",
		coremodel.ModelConfig{},
		meta.WithJobID("task-123"),
	)
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}

	store := &aliTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID("task-123"),
				Metadata: `{"prompt":"Stored horse","seconds":5,"size":"1280x720"}`,
			},
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id":"req-1",
			"output":{
				"task_id":"task-123",
				"task_status":"PENDING"
			}
		}`)),
	}

	_, adaptorErr := aliAdaptor.DoResponse(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job: %v", err)
	}

	if job.Prompt != "Stored horse" ||
		job.NSeconds != 5 ||
		job.Width != 1280 ||
		job.Height != 720 {
		t.Fatalf("expected stored metadata fallback, got %#v", job)
	}
}

func TestAliVideoGetUsesStoredRequestSizeWhenUpstreamRatioMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	aliAdaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/video/generations/jobs/task-123",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"happyhorse-1.0-i2v",
		coremodel.ModelConfig{},
		meta.WithJobID("task-123"),
	)
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}

	store := &aliTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID("task-123"),
				Metadata: `{"prompt":"Stored horse","seconds":5,"size":"720x1280"}`,
			},
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id":"req-1",
			"output":{
				"task_id":"task-123",
				"task_status":"SUCCEEDED",
				"video_url":"https://example.com/video.mp4"
			},
			"usage":{
				"duration":5,
				"video_count":1,
				"SR":720
			}
		}`)),
	}

	result, adaptorErr := aliAdaptor.DoResponse(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	if result.UsageContext.Resolution != "720x1280" {
		t.Fatalf("expected stored usage context resolution, got %#v", result.UsageContext)
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job: %v", err)
	}

	if job.Width != 720 || job.Height != 1280 {
		t.Fatalf("expected stored metadata dimensions, got %#v", job)
	}

	if len(job.Generations) != 1 ||
		job.Generations[0].Width != 720 ||
		job.Generations[0].Height != 1280 {
		t.Fatalf("expected stored generation dimensions, got %#v", job.Generations)
	}
}

func TestAliVideosHandlerConvertsTaskToOpenAIVideo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		nil,
	)

	m := meta.NewMeta(
		nil,
		mode.Videos,
		"wan2.5-t2v-preview",
		coremodel.ModelConfig{},
	)
	m.Channel.ID = 9
	m.Group = coremodel.GroupCache{ID: "group-1"}
	m.Token = coremodel.TokenCache{ID: 7}

	store := &aliTestStore{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"request_id":"req-1",
			"output":{
				"task_id":"task-123",
				"task_status":"PENDING",
				"submit_time":"2026-05-20 10:00:00.000",
				"orig_prompt":"A quiet street"
			},
			"usage":{
				"duration":5,
				"video_count":1,
				"SR":720,
				"ratio":"16:9"
			}
		}`)),
	}

	result, adaptorErr := adaptor.DoResponse(m, store, ctx, resp)
	if adaptorErr != nil {
		t.Fatalf("DoResponse returned error: %v", adaptorErr)
	}

	if result.UpstreamID != "task-123" || !result.AsyncUsage {
		t.Fatalf("unexpected result: %#v", result)
	}

	if result.UsageContext.Resolution != "1280x720" {
		t.Fatalf("expected usage context resolution, got %#v", result.UsageContext)
	}

	if len(store.saved) != 1 {
		t.Fatalf("expected 1 saved video, got %#v", store.saved)
	}

	if store.saved[0].ID != coremodel.VideoGenerationStoreID("task-123") {
		t.Fatalf("unexpected video store id: %#v", store.saved[0].ID)
	}

	var video relaymodel.Video
	if err := json.Unmarshal(recorder.Body.Bytes(), &video); err != nil {
		t.Fatalf("failed to unmarshal video: %v", err)
	}

	if video.Object != relaymodel.VideoObject ||
		video.ID != "task-123" ||
		video.Status != relaymodel.VideoStatusQueued ||
		video.Model != "wan2.5-t2v-preview" {
		t.Fatalf("unexpected video: %#v", video)
	}

	if video.Seconds != 5 || video.Size != "1280x720" {
		t.Fatalf("unexpected video usage fields: %#v", video)
	}
}

func TestAdaptorConvertRequestAliVideosIgnoresJobOnlyFields(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.Videos, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	body := `{
		"model":"wan2.7-i2v",
		"prompt":"Animate this reference",
		"seconds":4,
		"n_seconds":10,
		"n_variants":2,
		"width":1920,
		"height":1080,
		"input_reference":"https://example.com/openai-reference.png"
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	parameters := mustMap(t, payload["parameters"])

	if int(mustFloat64(t, parameters["duration"])) != 4 {
		t.Fatalf("expected official videos seconds to win, got %#v", parameters["duration"])
	}

	if _, ok := parameters["size"]; ok {
		t.Fatalf("expected job-only dimensions to be ignored, got size %#v", parameters["size"])
	}

	if _, ok := parameters["resolution"]; ok {
		t.Fatalf(
			"expected job-only dimensions to be ignored, got resolution %#v",
			parameters["resolution"],
		)
	}
}

func TestAdaptorConvertRequestAliVideoInvalidJobVariantReturnsRelayError(t *testing.T) {
	t.Parallel()

	a := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model":"wan2.7-i2v",
			"prompt":"Animate this reference",
			"n_variants":2
		}`),
	)

	_, err := a.ConvertRequest(m, nil, req)
	if err == nil {
		t.Fatal("expected n_variants error")
	}

	var relayErr adaptor.Error

	ok := errors.As(err, &relayErr)
	if !ok {
		t.Fatalf("expected adaptor.Error, got %T: %v", err, err)
	}

	if relayErr.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", relayErr.StatusCode())
	}

	body, marshalErr := relayErr.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}

	if string(body) != `{"detail":"n_variants must be 1 for Ali video models"}` {
		t.Fatalf("expected OpenAI video detail, got %s", body)
	}
}

func TestAdaptorConvertRequestAliVideoGenerationIgnoresVideosSeconds(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideoGenerationsJobs, "wan2.7-i2v", coremodel.ModelConfig{})
	m.ActualModel = "wan2.7-i2v"

	body := `{
		"model":"wan2.7-i2v",
		"prompt":"Animate this reference",
		"seconds":4,
		"input_reference":"https://example.com/openai-reference.png"
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload := readJSONMap(t, result.Body)
	parameters, _ := payload["parameters"].(map[string]any)

	if _, ok := parameters["duration"]; ok {
		t.Fatalf(
			"expected videos seconds field to be ignored for jobs, got %#v",
			parameters["duration"],
		)
	}
}

func TestAliVideoAsyncUsageUsesBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v1/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("expected authorization header, got %#v", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": {
				"task_id": "task-123",
				"task_status": "SUCCEEDED"
			},
			"usage": {
				"duration": 8,
				"input_video_duration": 3,
				"output_video_duration": 5,
				"SR": 720,
				"ratio": "9:16"
			}
		}`))
	}))
	defer server.Close()

	aliAdaptor := &Adaptor{}
	store := &aliTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID("task-123"),
				Metadata: `{"prompt":"Stored prompt","seconds":5,"size":"1280x720"}`,
			},
		},
	}

	usage, usageContext, completed, err := aliAdaptor.FetchAsyncUsage(
		context.Background(),
		adaptor.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				BaseURL: server.URL + "/fallback",
				Key:     "test-key",
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				BaseURL:    server.URL + "/custom",
				UpstreamID: "task-123",
				GroupID:    "group-1",
				TokenID:    7,
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

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usageContext.Resolution != "720x1280" {
		t.Fatalf("unexpected usage context: %#v", usageContext)
	}

	if usage.VideoInputTokens != coremodel.ZeroNullInt64(3) ||
		usage.OutputTokens != coremodel.ZeroNullInt64(5) ||
		usage.TotalTokens != coremodel.ZeroNullInt64(8) {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestAliVideoAsyncUsageUsesStoredSizeWhenUpstreamRatioMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v1/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": {
				"task_id": "task-123",
				"task_status": "SUCCEEDED"
			},
			"usage": {
				"duration": 8,
				"input_video_duration": 3,
				"output_video_duration": 5,
				"SR": 720
			}
		}`))
	}))
	defer server.Close()

	aliAdaptor := &Adaptor{}
	store := &aliTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID("task-123"),
				Metadata: `{"prompt":"Stored prompt","seconds":5,"size":"720x1280"}`,
			},
		},
	}

	_, usageContext, completed, err := aliAdaptor.FetchAsyncUsage(
		context.Background(),
		adaptor.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				BaseURL: server.URL + "/fallback",
				Key:     "test-key",
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				BaseURL:    server.URL + "/custom",
				UpstreamID: "task-123",
				GroupID:    "group-1",
				TokenID:    7,
			},
			Store: store,
		},
	)
	if err != nil {
		t.Fatalf("FetchAsyncUsage returned error: %v", err)
	}

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usageContext.Resolution != "720x1280" {
		t.Fatalf("unexpected usage context: %#v", usageContext)
	}

	if usageContext.NativeResolution != "720P" {
		t.Fatalf("expected native resolution 720P, got %#v", usageContext)
	}
}

func TestAliNativeVideoAsyncUsageUsesNativeResolution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v1/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": {
				"task_id": "task-123",
				"task_status": "SUCCEEDED"
			},
			"usage": {
				"duration": 8,
				"input_video_duration": 3,
				"output_video_duration": 5,
				"SR": 720,
				"ratio": "9:16"
			}
		}`))
	}))
	defer server.Close()

	aliAdaptor := &Adaptor{}

	usage, usageContext, completed, err := aliAdaptor.FetchAsyncUsage(
		context.Background(),
		adaptor.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				BaseURL: server.URL + "/fallback",
				Key:     "test-key",
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.AliVideo),
				BaseURL:    server.URL + "/custom",
				UpstreamID: "task-123",
				GroupID:    "group-1",
				TokenID:    7,
			},
		},
	)
	if err != nil {
		t.Fatalf("FetchAsyncUsage returned error: %v", err)
	}

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usageContext.Resolution != "720P" ||
		usageContext.NativeResolution != "720P" {
		t.Fatalf("unexpected native usage context: %#v", usageContext)
	}

	if usage.VideoInputTokens != coremodel.ZeroNullInt64(3) ||
		usage.OutputTokens != coremodel.ZeroNullInt64(5) ||
		usage.TotalTokens != coremodel.ZeroNullInt64(8) {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestAliNativeVideoAsyncUsageUsesNativeFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v1/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": {
				"task_id": "task-123",
				"task_status": "SUCCEEDED"
			},
			"usage": {
				"duration": 8,
				"input_video_duration": 3,
				"output_video_duration": 5
			}
		}`))
	}))
	defer server.Close()

	aliAdaptor := &Adaptor{}

	_, usageContext, completed, err := aliAdaptor.FetchAsyncUsage(
		context.Background(),
		adaptor.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				BaseURL: server.URL + "/fallback",
				Key:     "test-key",
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.AliVideo),
				BaseURL:    server.URL + "/custom",
				UpstreamID: "task-123",
				GroupID:    "group-1",
				TokenID:    7,
				UsageContext: coremodel.UsageContext{
					Resolution:       "1080P",
					NativeResolution: "1080P",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("FetchAsyncUsage returned error: %v", err)
	}

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usageContext.Resolution != "1080P" ||
		usageContext.NativeResolution != "1080P" {
		t.Fatalf("unexpected native usage context fallback: %#v", usageContext)
	}
}

func TestAliNativeVideoAsyncUsageUsesStoredNativeFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v1/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": {
				"task_id": "task-123",
				"task_status": "SUCCEEDED"
			},
			"usage": {
				"duration": 8,
				"input_video_duration": 3,
				"output_video_duration": 5
			}
		}`))
	}))
	defer server.Close()

	aliAdaptor := &Adaptor{}
	store := &aliTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoGenerationStoreID("task-123"),
				Metadata: `{"prompt":"Stored prompt","seconds":5,"size":"1080P"}`,
			},
		},
	}

	_, usageContext, completed, err := aliAdaptor.FetchAsyncUsage(
		context.Background(),
		adaptor.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				BaseURL: server.URL + "/fallback",
				Key:     "test-key",
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.AliVideo),
				BaseURL:    server.URL + "/custom",
				UpstreamID: "task-123",
				GroupID:    "group-1",
				TokenID:    7,
			},
			Store: store,
		},
	)
	if err != nil {
		t.Fatalf("FetchAsyncUsage returned error: %v", err)
	}

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usageContext.Resolution != "1080P" ||
		usageContext.NativeResolution != "1080P" {
		t.Fatalf("unexpected native usage context fallback: %#v", usageContext)
	}
}

func TestAdaptorDoResponseResponsesDeleteNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	resp := &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}

	_, err := adaptor.DoResponse(
		&meta.Meta{Mode: mode.ResponsesDelete},
		nil,
		ctx,
		resp,
	)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	if ctx.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, ctx.Writer.Status())
	}
}

func TestAdaptorDoResponseMultimodalEmbeddings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/embeddings",
		nil,
	)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"output": {
				"embeddings": [
					{
						"index": 0,
						"embedding": [0.1, 0.2],
						"type": "text"
					},
					{
						"index": 1,
						"embedding": [0.3, 0.4],
						"type": "image"
					}
				]
			},
			"usage": {
				"input_tokens": 10,
				"input_tokens_details": {
					"image_tokens": 8,
					"text_tokens": 2
				},
				"output_tokens": 2,
				"total_tokens": 12
			},
			"request_id": "req_123"
		}`)),
	}

	result, err := adaptor.DoResponse(
		&meta.Meta{
			Mode:        mode.Embeddings,
			OriginModel: "qwen3-vl-embedding",
			ActualModel: "qwen3-vl-embedding",
		},
		nil,
		ctx,
		resp,
	)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	if result.Usage.InputTokens != coremodel.ZeroNullInt64(10) {
		t.Fatalf("expected input tokens 10, got %d", result.Usage.InputTokens)
	}

	if result.Usage.ImageInputTokens != coremodel.ZeroNullInt64(8) {
		t.Fatalf("expected image input tokens 8, got %d", result.Usage.ImageInputTokens)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response body %s: %v", recorder.Body.String(), err)
	}

	if payload["object"] != "list" {
		t.Fatalf("expected object=list, got %#v", payload["object"])
	}

	if payload["model"] != "qwen3-vl-embedding" {
		t.Fatalf("expected model qwen3-vl-embedding, got %#v", payload["model"])
	}

	data, ok := payload["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("expected 2 data items, got %#v", payload["data"])
	}

	first, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first data object, got %#v", data[0])
	}

	if first["object"] != "embedding" {
		t.Fatalf("expected first object=embedding, got %#v", first["object"])
	}

	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("expected usage object, got %#v", payload["usage"])
	}

	if promptTokens, ok := usage["prompt_tokens"].(float64); !ok || int(promptTokens) != 10 {
		t.Fatalf("expected usage.prompt_tokens=10, got %#v", usage["prompt_tokens"])
	}

	details, ok := usage["prompt_tokens_details"].(map[string]any)
	if !ok {
		t.Fatalf(
			"expected usage.prompt_tokens_details object, got %#v",
			usage["prompt_tokens_details"],
		)
	}

	if imageTokens, ok := details["image_tokens"].(float64); !ok || int(imageTokens) != 8 {
		t.Fatalf("expected image_tokens=8, got %#v", details["image_tokens"])
	}
}

func TestAdaptorConvertGeminiRequestReasoning(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Gemini,
		"glm-4.5",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/gemini-pro:generateContent",
		strings.NewReader(`{
			"generationConfig": {
				"thinkingConfig": {
					"thinkingBudget": 2048,
					"includeThoughts": true
				}
			},
			"contents": [{"role":"user","parts":[{"text":"hello"}]}]
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	enableThinking, ok := payload["enable_thinking"].(bool)
	if !ok || !enableThinking {
		t.Fatalf("expected enable_thinking=true, got %#v", payload["enable_thinking"])
	}

	thinkingBudget, ok := payload["thinking_budget"].(float64)
	if !ok || int(thinkingBudget) != 2048 {
		t.Fatalf("expected thinking_budget=2048, got %#v", payload["thinking_budget"])
	}

	if _, exists := payload["reasoning_effort"]; exists {
		t.Fatal("expected reasoning_effort to be removed")
	}
}

func assertContentString(t *testing.T, got any, key, want string) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected content object, got %T", got)
	}

	if len(gotMap) != 1 {
		t.Fatalf("expected content to have 1 key, got %#v", gotMap)
	}

	if gotMap[key] != want {
		t.Fatalf("expected %s=%q, got %#v", key, want, gotMap[key])
	}
}

func assertImageContentHasPNGPrefix(t *testing.T, got any) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected content object, got %T", got)
	}

	if len(gotMap) != 1 {
		t.Fatalf("expected content to have 1 key, got %#v", gotMap)
	}

	value, ok := gotMap["image"].(string)
	if !ok {
		t.Fatalf("expected image string, got %#v", gotMap["image"])
	}

	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(value, prefix) {
		t.Fatalf("expected image prefix %q, got %#v", prefix, value)
	}
}

func mustMap(t *testing.T, value any) map[string]any {
	t.Helper()

	got, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T: %#v", value, value)
	}

	return got
}

func readJSONMap(t *testing.T, r io.Reader) map[string]any {
	t.Helper()

	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal body %s: %v", string(body), err)
	}

	return payload
}

func mustSlice(t *testing.T, value any) []any {
	t.Helper()

	got, ok := value.([]any)
	if !ok {
		t.Fatalf("expected array, got %T: %#v", value, value)
	}

	return got
}

func mustFloat64(t *testing.T, value any) float64 {
	t.Helper()

	got, ok := value.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T: %#v", value, value)
	}

	return got
}

func TestAdaptorConvertChatCompletionsReasoningNotClampedByMaxTokens(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"glm-4.5",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"glm-4.5",
			"reasoning_effort":"high",
			"max_tokens":1000,
			"messages":[{"role":"user","content":"hello"}]
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	enableThinking, ok := payload["enable_thinking"].(bool)
	if !ok || !enableThinking {
		t.Fatalf("expected enable_thinking=true, got %#v", payload["enable_thinking"])
	}

	thinkingBudget, ok := payload["thinking_budget"].(float64)
	if !ok || int(thinkingBudget) != 16384 {
		t.Fatalf("expected thinking_budget=16384, got %#v", payload["thinking_budget"])
	}
}

func TestAdaptorConvertChatCompletionsReasoningUsesOriginModelNameFirst(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"glm-4.5",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "mapped-upstream-model"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"glm-4.5",
			"reasoning_effort":"high",
			"messages":[{"role":"user","content":"hello"}]
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	enableThinking, ok := payload["enable_thinking"].(bool)
	if !ok || !enableThinking {
		t.Fatalf("expected enable_thinking=true, got %#v", payload["enable_thinking"])
	}

	thinkingBudget, ok := payload["thinking_budget"].(float64)
	if !ok || int(thinkingBudget) != 16384 {
		t.Fatalf("expected thinking_budget=16384, got %#v", payload["thinking_budget"])
	}
}

func TestAdaptorConvertCompletionsReasoning(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Completions,
		"glm-4.5",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/completions",
		strings.NewReader(`{
			"model":"glm-4.5",
			"reasoning_effort":"low",
			"prompt":"hello"
		}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	enableThinking, ok := payload["enable_thinking"].(bool)
	if !ok || !enableThinking {
		t.Fatalf("expected enable_thinking=true, got %#v", payload["enable_thinking"])
	}
}
