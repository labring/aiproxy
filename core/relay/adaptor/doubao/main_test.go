//nolint:testpackage
package doubao

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type doubaoTestStore struct {
	saved []adaptor.StoreCache
}

func (s *doubaoTestStore) GetStore(_ string, _ int, id string) (adaptor.StoreCache, error) {
	for _, cache := range s.saved {
		if cache.ID == id {
			return cache, nil
		}
	}

	return adaptor.StoreCache{}, nil
}

func (s *doubaoTestStore) SaveStore(cache adaptor.StoreCache) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *doubaoTestStore) SaveStoreWithOption(
	cache adaptor.StoreCache,
	_ adaptor.SaveStoreOption,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *doubaoTestStore) SaveIfNotExistStore(cache adaptor.StoreCache) error {
	s.saved = append(s.saved, cache)
	return nil
}

func TestErrorHandlerParsesDoubaoOpenAIError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"code":"InvalidParameter","message":"duration is invalid","type":"invalid_request_error","param":"duration"}}`,
		)),
	}

	err := ErrorHandler(resp)
	if err.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.StatusCode())
	}

	body, marshalErr := err.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}

	if !strings.Contains(string(body), `"message":"duration is invalid"`) {
		t.Fatalf("expected doubao message, got %s", body)
	}
}

func TestOpenAIVideoErrorHandlerParsesDoubaoOpenAIError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"code":"InvalidParameter","message":"duration is invalid","type":"invalid_request_error"}}`,
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

func TestErrorHandlerParsesDoubaoResponseMetadataError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"ResponseMetadata":{"Error":{"Code":"InvalidParameter","Message":"duration is invalid"}}}`,
		)),
	}

	err := ErrorHandler(resp)

	body, marshalErr := err.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}

	if !strings.Contains(string(body), `"message":"duration is invalid"`) {
		t.Fatalf("expected doubao response metadata message, got %s", body)
	}
}

func TestAdaptorSupportMode(t *testing.T) {
	adaptor := &Adaptor{}

	supportedModes := []mode.Mode{
		mode.ChatCompletions,
		mode.Anthropic,
		mode.Gemini,
		mode.Embeddings,
		mode.ImagesGenerations,
		mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.Videos,
		mode.VideosGet,
		mode.VideosContent,
		mode.VideosDelete,
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

	unsupportedModes := []mode.Mode{
		mode.Completions,
		mode.ImagesEdits,
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
		BaseURL: "https://ark.cn-beijing.volces.com",
	}

	tests := []struct {
		name         string
		mode         mode.Mode
		model        string
		responseID   string
		jobID        string
		videoID      string
		generationID string
		wantMethod   string
		wantURL      string
	}{
		{
			name:       "gemini uses chat completions",
			mode:       mode.Gemini,
			model:      "doubao-seed-1-6",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
		},
		{
			name:       "gemini bot uses bot chat completions",
			mode:       mode.Gemini,
			model:      "bot-123",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/bots/chat/completions",
		},
		{
			name:       "image generation",
			mode:       mode.ImagesGenerations,
			model:      "doubao-seedream-5-0-lite",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/images/generations",
		},
		{
			name:       "video job create",
			mode:       mode.VideoGenerationsJobs,
			model:      "doubao-seedance-2-0-260128",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks",
		},
		{
			name:       "video job get",
			mode:       mode.VideoGenerationsGetJobs,
			model:      "doubao-seedance-2-0-260128",
			jobID:      "task-123",
			wantMethod: http.MethodGet,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/task-123",
		},
		{
			name:         "video job content",
			mode:         mode.VideoGenerationsContent,
			model:        "doubao-seedance-2-0-260128",
			generationID: "task-456",
			wantMethod:   http.MethodGet,
			wantURL:      "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/task-456",
		},
		{
			name:       "videos get",
			mode:       mode.VideosGet,
			model:      "doubao-seedance-2-0-260128",
			videoID:    "video-123",
			wantMethod: http.MethodGet,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/video-123",
		},
		{
			name:       "videos delete",
			mode:       mode.VideosDelete,
			model:      "doubao-seedance-2-0-260128",
			videoID:    "video-123",
			wantMethod: http.MethodDelete,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/video-123",
		},
		{
			name:       "responses create",
			mode:       mode.Responses,
			model:      "doubao-seed-1-6",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses",
		},
		{
			name:       "responses get",
			mode:       mode.ResponsesGet,
			model:      "doubao-seed-1-6",
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses/resp_123",
		},
		{
			name:       "responses delete",
			mode:       mode.ResponsesDelete,
			model:      "doubao-seed-1-6",
			responseID: "resp_123",
			wantMethod: http.MethodDelete,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses/resp_123",
		},
		{
			name:       "responses cancel",
			mode:       mode.ResponsesCancel,
			model:      "doubao-seed-1-6",
			responseID: "resp_123",
			wantMethod: http.MethodPost,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses/resp_123/cancel",
		},
		{
			name:       "responses input items",
			mode:       mode.ResponsesInputItems,
			model:      "doubao-seed-1-6",
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://ark.cn-beijing.volces.com/api/v3/responses/resp_123/input_items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(
				channel,
				tt.mode,
				tt.model,
				coremodel.ModelConfig{},
				meta.WithResponseID(tt.responseID),
				meta.WithJobID(tt.jobID),
				meta.WithGenerationID(tt.generationID),
				meta.WithVideoID(tt.videoID),
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

func TestAdaptorGetRequestURL_UsesOriginModelNameFirst(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{
		BaseURL: "https://ark.cn-beijing.volces.com",
	}

	t.Run("origin bot model keeps bot endpoint", func(t *testing.T) {
		m := meta.NewMeta(channel, mode.Gemini, "bot-123", coremodel.ModelConfig{})
		m.ActualModel = "mapped-model"

		got, err := adaptor.GetRequestURL(m, nil, nil)
		if err != nil {
			t.Fatalf("GetRequestURL returned error: %v", err)
		}

		if got.URL != "https://ark.cn-beijing.volces.com/api/v3/bots/chat/completions" {
			t.Fatalf("unexpected URL: %s", got.URL)
		}
	})

	t.Run("origin vision model keeps multimodal embeddings endpoint", func(t *testing.T) {
		m := meta.NewMeta(channel, mode.Embeddings, "doubao-vision", coremodel.ModelConfig{})
		m.ActualModel = "mapped-model"

		got, err := adaptor.GetRequestURL(m, nil, nil)
		if err != nil {
			t.Fatalf("GetRequestURL returned error: %v", err)
		}

		if got.URL != "https://ark.cn-beijing.volces.com/api/v3/embeddings/multimodal" {
			t.Fatalf("unexpected URL: %s", got.URL)
		}
	})
}

func TestAdaptorConvertRequestVisionEmbeddings(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Embeddings,
		"doubao-embedding-vision-250615",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/embeddings",
		strings.NewReader(`{
			"model": "doubao-embedding-vision-250615",
			"encoding_format": "float",
			"dimensions": 1024,
			"instructions": "Represent the multimodal query",
			"input": [
				{"image": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg"},
				{"video": "https://example.com/video.mp4"},
				{"image_url": {"url": "https://example.com/image.jpg"}},
				"plain text"
			]
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

	if payload["model"] != "doubao-embedding-vision-250615" {
		t.Fatalf("expected model to be set, got %#v", payload["model"])
	}

	if payload["encoding_format"] != "float" {
		t.Fatalf("expected encoding_format to be preserved, got %#v", payload["encoding_format"])
	}

	if dimensions, ok := payload["dimensions"].(float64); !ok || int(dimensions) != 1024 {
		t.Fatalf("expected dimensions=1024, got %#v", payload["dimensions"])
	}

	input, ok := payload["input"].([]any)
	if !ok {
		t.Fatalf("expected input array, got %#v", payload["input"])
	}

	if len(input) != 4 {
		t.Fatalf("expected 4 input items, got %d", len(input))
	}

	assertDoubaoEmbeddingURLItem(
		t,
		input[0],
		"image_url",
		"data:image/png;base64,iVBORw0KGgoAAAANSUhEUg",
	)
	assertDoubaoEmbeddingURLItem(t, input[1], "video_url", "https://example.com/video.mp4")
	assertDoubaoEmbeddingURLItem(t, input[2], "image_url", "https://example.com/image.jpg")
	assertDoubaoEmbeddingTextItem(t, input[3], "plain text")
}

func TestAdaptorDoResponseVisionEmbeddings(t *testing.T) {
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
			"created": 1743575029,
			"data": {
				"embedding": [0.1, 0.2],
				"object": "embedding",
				"sparse_embedding": [{"index": 12, "value": 0.5}]
			},
			"id": "req_123",
			"model": "doubao-embedding-vision-250615",
			"object": "list",
			"usage": {
				"prompt_tokens": 528,
				"prompt_tokens_details": {
					"image_tokens": 497,
					"text_tokens": 31
				},
				"total_tokens": 528
			}
		}`)),
	}

	result, err := adaptor.DoResponse(
		&meta.Meta{
			Mode:        mode.Embeddings,
			OriginModel: "doubao-embedding-vision-250615",
			ActualModel: "doubao-embedding-vision-250615",
		},
		nil,
		ctx,
		resp,
	)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	if result.Usage.InputTokens != coremodel.ZeroNullInt64(528) {
		t.Fatalf("expected input tokens 528, got %d", result.Usage.InputTokens)
	}

	if result.Usage.ImageInputTokens != coremodel.ZeroNullInt64(497) {
		t.Fatalf("expected image input tokens 497, got %d", result.Usage.ImageInputTokens)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response body %s: %v", recorder.Body.String(), err)
	}

	data, ok := payload["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("expected single-item data array, got %#v", payload["data"])
	}

	item, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("expected data item object, got %#v", data[0])
	}

	if item["object"] != "embedding" {
		t.Fatalf("expected data.object=embedding, got %#v", item["object"])
	}

	if index, ok := item["index"].(float64); !ok || int(index) != 0 {
		t.Fatalf("expected data.index=0, got %#v", item["index"])
	}

	if _, ok := item["sparse_embedding"].([]any); !ok {
		t.Fatalf("expected sparse_embedding to be preserved, got %#v", item["sparse_embedding"])
	}
}

func TestAdaptorConvertRequestImageGenerationMapsSequentialCount(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ImagesGenerations,
		"doubao-seedream-5-0-lite",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(`{
			"model": "alias-image",
			"prompt": "Draw a quiet library",
			"n": 3,
			"size": "1024×1536",
			"response_format": "url"
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

	if payload["model"] != "doubao-seedream-5-0-lite" {
		t.Fatalf("expected actual model, got %#v", payload["model"])
	}

	if payload["size"] != "1024x1536" {
		t.Fatalf("expected normalized size 1024x1536, got %#v", payload["size"])
	}

	if _, ok := payload["n"]; ok {
		t.Fatalf("expected n to be removed, got %#v", payload["n"])
	}

	if payload["sequential_image_generation"] != "auto" {
		t.Fatalf(
			"expected sequential image generation auto, got %#v",
			payload["sequential_image_generation"],
		)
	}

	options, ok := payload["sequential_image_generation_options"].(map[string]any)
	if !ok {
		t.Fatalf(
			"expected sequential options object, got %#v",
			payload["sequential_image_generation_options"],
		)
	}

	if maxImages, ok := options["max_images"].(float64); !ok || int(maxImages) != 3 {
		t.Fatalf("expected max_images=3, got %#v", options["max_images"])
	}
}

func TestAdaptorDoResponseImageGenerationUsesDoubaoUsage(t *testing.T) {
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

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"created": 1770000000,
			"data": [{"url": "https://example.com/image.png", "size": "2048×2048"}],
			"usage": {
				"generated_images": 1,
				"output_tokens": 16384,
				"total_tokens": 16384,
				"tool_usage": {"web_search": 2}
			}
		}`)),
	}

	result, err := adaptor.DoResponse(
		&meta.Meta{Mode: mode.ImagesGenerations},
		nil,
		ctx,
		resp,
	)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	if result.Usage.OutputTokens != coremodel.ZeroNullInt64(16384) ||
		result.Usage.ImageOutputTokens != coremodel.ZeroNullInt64(1) ||
		result.Usage.TotalTokens != coremodel.ZeroNullInt64(16384) ||
		result.Usage.WebSearchCount != coremodel.ZeroNullInt64(2) {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}

	if result.UsageContext.Resolution != "2048x2048" {
		t.Fatalf("expected size context, got %#v", result.UsageContext)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response body %s: %v", recorder.Body.String(), err)
	}

	data, ok := payload["data"].([]any)
	if !ok {
		t.Fatalf("expected response data array, got %#v", payload["data"])
	}

	first, ok := data[0].(map[string]any)
	if !ok || first["url"] != "https://example.com/image.png" {
		t.Fatalf("expected OpenAI image data, got %#v", payload["data"])
	}

	if _, ok := first["size"]; ok {
		t.Fatalf("expected provider size field to be omitted, got %#v", first)
	}

	usagePayload, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAI usage object, got %#v", payload["usage"])
	}

	if _, ok := usagePayload["generated_images"]; ok {
		t.Fatalf("expected provider generated_images to be omitted, got %#v", usagePayload)
	}

	if _, ok := usagePayload["tool_usage"]; ok {
		t.Fatalf("expected provider tool_usage to be omitted, got %#v", usagePayload)
	}

	outputDetails, ok := usagePayload["output_tokens_details"].(map[string]any)
	if !ok || outputDetails["image_tokens"] != float64(1) {
		t.Fatalf("expected OpenAI output token details, got %#v", usagePayload)
	}
}

func TestAdaptorDoResponseImageGenerationStreamConvertsDoubaoEvents(t *testing.T) {
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

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": {"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: " + doubaoImageStreamEventPartialSucceeded,
			`data: {"type":"` + doubaoImageStreamEventPartialSucceeded + `","image_index":0,"url":"https://example.com/one.png","size":"2048×2048"}`,
			"",
			"event: " + relaymodel.ImageStreamEventCompleted,
			`data: {"type":"` + relaymodel.ImageStreamEventCompleted + `","usage":{"generated_images":2,"output_tokens":32768,"total_tokens":32768,"tool_usage":{"web_search":1}}}`,
			"",
		}, "\n"))),
	}

	result, err := adaptor.DoResponse(
		&meta.Meta{Mode: mode.ImagesGenerations},
		nil,
		ctx,
		resp,
	)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	if result.Usage.OutputTokens != coremodel.ZeroNullInt64(32768) ||
		result.Usage.ImageOutputTokens != coremodel.ZeroNullInt64(2) ||
		result.Usage.TotalTokens != coremodel.ZeroNullInt64(32768) ||
		result.Usage.WebSearchCount != coremodel.ZeroNullInt64(1) {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}

	if result.UsageContext.Resolution != "2048x2048" {
		t.Fatalf("expected size context, got %#v", result.UsageContext)
	}

	body := recorder.Body.String()
	if strings.Contains(body, "event: "+doubaoImageStreamEventPartialSucceeded) {
		t.Fatalf("expected upstream event names to be converted, got body: %s", body)
	}

	if !strings.Contains(body, "event: "+relaymodel.ImageStreamEventPartialImage+"\n") ||
		!strings.Contains(body, `"type":"`+relaymodel.ImageStreamEventPartialImage+`"`) ||
		!strings.Contains(body, `"partial_image_index":0`) ||
		!strings.Contains(body, "event: "+relaymodel.ImageStreamEventCompleted+"\n") ||
		!strings.Contains(body, `"type":"`+relaymodel.ImageStreamEventCompleted+`"`) ||
		!strings.Contains(body, `"url":"https://example.com/one.png"`) ||
		!strings.Contains(body, `"output_tokens":32768`) {
		t.Fatalf("unexpected stream body: %s", body)
	}

	if strings.Contains(body, "[DONE]") {
		t.Fatalf("expected DONE to be omitted, got body: %s", body)
	}

	if recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", recorder.Header().Get("Content-Type"))
	}
}

func TestAdaptorDoResponseImageGenerationStreamFallsBackToCompletedImages(t *testing.T) {
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

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": {"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: " + doubaoImageStreamEventPartialSucceeded,
			`data: {"type":"` + doubaoImageStreamEventPartialSucceeded + `","image_index":0,"url":"https://example.com/one.png","size":"2048x2048"}`,
			"",
			"event: " + doubaoImageStreamEventPartialSucceeded,
			`data: {"type":"` + doubaoImageStreamEventPartialSucceeded + `","image_index":1,"b64_json":"abc","size":"2048x2048"}`,
			"",
			"event: " + relaymodel.ImageStreamEventCompleted,
			`data: {"type":"` + relaymodel.ImageStreamEventCompleted + `"}`,
			"",
		}, "\n"))),
	}

	result, err := adaptor.DoResponse(
		&meta.Meta{
			Mode: mode.ImagesGenerations,
			RequestUsage: coremodel.Usage{
				ImageOutputTokens: 99,
			},
		},
		nil,
		ctx,
		resp,
	)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	if result.Usage.OutputTokens != coremodel.ZeroNullInt64(2) ||
		result.Usage.ImageOutputTokens != coremodel.ZeroNullInt64(2) ||
		result.Usage.TotalTokens != coremodel.ZeroNullInt64(2) {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"output_tokens":2`) ||
		strings.Contains(body, `"output_tokens":99`) {
		t.Fatalf("unexpected stream body: %s", body)
	}
}

func TestAdaptorConvertRequestVideoGenerationMapsOpenAIFields(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model": "alias-video",
			"prompt": "Animate a calm ocean",
			"width": 1280,
			"height": 720,
			"n_seconds": 5,
			"input_reference": "https://example.com/reference.png",
			"video_url": "https://example.com/reference.mp4",
			"input_audio": {"data": "https://example.com/audio.wav", "format": "wav"},
			"generate_audio": true,
			"watermark": false
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

	if payload["model"] != "doubao-seedance-2-0-260128" {
		t.Fatalf("expected actual model, got %#v", payload["model"])
	}

	if payload["resolution"] != "720p" {
		t.Fatalf("expected resolution 720p, got %#v", payload["resolution"])
	}

	if duration, ok := payload["duration"].(float64); !ok || int(duration) != 5 {
		t.Fatalf("expected duration=5, got %#v", payload["duration"])
	}

	content, ok := payload["content"].([]any)
	if !ok || len(content) != 4 {
		t.Fatalf("expected 4 content items, got %#v", payload["content"])
	}

	assertDoubaoVideoContent(t, content[0], "text", "", "Animate a calm ocean")
	assertDoubaoVideoContent(t, content[1], "image_url", "https://example.com/reference.png", "")
	assertDoubaoVideoContent(t, content[2], "video_url", "https://example.com/reference.mp4", "")
	assertDoubaoVideoContent(t, content[3], "audio_url", "https://example.com/audio.wav", "")

	usageContext := doubaoVideoRequestUsageContext(m)
	if usageContext.InputVideo == nil || !*usageContext.InputVideo ||
		usageContext.OutputAudio == nil || !*usageContext.OutputAudio {
		t.Fatalf("expected converted request media usage context, got %#v", usageContext)
	}
}

func TestAdaptorConvertVideosEditMapsVideoFieldToReferenceVideo(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideosEdits, "doubao-seedance-2-0-260128", coremodel.ModelConfig{})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/edits",
		strings.NewReader(`{
			"model": "alias-video",
			"prompt": "Change the color",
			"video": "https://example.com/source.mp4",
			"seconds": 4,
			"size": "1280x720"
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

	content, ok := payload["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected 2 content items, got %#v", payload["content"])
	}

	assertDoubaoVideoContent(t, content[1], "video_url", "https://example.com/source.mp4", "")

	item, ok := content[1].(map[string]any)
	if !ok || item["role"] != "reference_video" {
		t.Fatalf("expected reference_video role, got %#v", content[1])
	}
}

func TestAdaptorConvertVideosEditMapsStoredVideoIDToDraftTask(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(nil, mode.VideosEdits, "doubao-seedance-2-0-260128", coremodel.ModelConfig{})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/edits",
		strings.NewReader(`{
			"prompt": "Change the color",
			"video": "video_123",
			"seconds": 4
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

	content, ok := payload["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected 2 content items, got %#v", payload["content"])
	}

	item, ok := content[1].(map[string]any)
	if !ok || item["type"] != "draft_task" {
		t.Fatalf("expected draft_task content, got %#v", content[1])
	}

	draftTask, ok := item["draft_task"].(map[string]any)
	if !ok || draftTask["id"] != "video_123" {
		t.Fatalf("expected draft task video_123, got %#v", item["draft_task"])
	}

	usageContext := doubaoVideoRequestUsageContext(m)
	if usageContext.InputVideo == nil || !*usageContext.InputVideo {
		t.Fatalf("expected stored video draft task to count as input video, got %#v", usageContext)
	}
}

func TestAdaptorConvertVideosExtensionMapsVideoFieldToFirstVideo(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideosExtensions,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/extensions",
		strings.NewReader(`{
			"model": "alias-video",
			"prompt": "Continue the shot",
			"video": "https://example.com/source.mp4",
			"seconds": 4,
			"size": "1280x720"
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

	content, ok := payload["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected 2 content items, got %#v", payload["content"])
	}

	assertDoubaoVideoContent(t, content[1], "video_url", "https://example.com/source.mp4", "")

	item, ok := content[1].(map[string]any)
	if !ok || item["role"] != "first_video" {
		t.Fatalf("expected first_video role, got %#v", content[1])
	}
}

func TestAdaptorConvertRequestVideoGenerationMapsPixelSize(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model": "alias-video",
			"prompt": "Animate a calm ocean",
			"width": 1280,
			"height": 720,
			"n_seconds": 5
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

	if payload["resolution"] != "720p" {
		t.Fatalf("expected resolution 720p, got %#v", payload["resolution"])
	}

	if payload["ratio"] != "16:9" {
		t.Fatalf("expected ratio 16:9, got %#v", payload["ratio"])
	}
}

func TestAdaptorConvertRequestVideoGenerationMapsPortraitPixelSize(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model": "alias-video",
			"prompt": "Animate a calm ocean",
			"width": 720,
			"height": 1280,
			"n_seconds": 5
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

	if payload["resolution"] != "720p" {
		t.Fatalf("expected resolution 720p, got %#v", payload["resolution"])
	}

	if payload["ratio"] != "9:16" {
		t.Fatalf("expected ratio 9:16, got %#v", payload["ratio"])
	}
}

func TestAdaptorConvertRequestVideosIgnoresJobOnlyDuration(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Videos,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(`{
			"model": "alias-video",
			"prompt": "Animate a calm ocean",
			"seconds": 5,
			"n_seconds": 10
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

	if payload["duration"] != float64(5) {
		t.Fatalf("expected official videos seconds to win, got %#v", payload["duration"])
	}
}

func TestAdaptorConvertRequestVideoGenerationIgnoresVideosSeconds(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model": "alias-video",
			"prompt": "Animate a calm ocean",
			"seconds": 5
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

	if _, ok := payload["duration"]; ok {
		t.Fatalf(
			"expected videos seconds field to be ignored for jobs, got %#v",
			payload["duration"],
		)
	}
}

func TestAdaptorConvertRequestVideoGenerationMapsMultipartPixelSize(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "alias-video"); err != nil {
		t.Fatalf("failed to write model: %v", err)
	}

	if err := writer.WriteField("prompt", "Animate a calm ocean"); err != nil {
		t.Fatalf("failed to write prompt: %v", err)
	}

	if err := writer.WriteField("width", "1280"); err != nil {
		t.Fatalf("failed to write width: %v", err)
	}

	if err := writer.WriteField("height", "720"); err != nil {
		t.Fatalf("failed to write height: %v", err)
	}

	if err := writer.WriteField("n_seconds", "5"); err != nil {
		t.Fatalf("failed to write n_seconds: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		&body,
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

	if payload["resolution"] != "720p" {
		t.Fatalf("expected resolution 720p, got %#v", payload["resolution"])
	}

	if payload["ratio"] != "16:9" {
		t.Fatalf("expected ratio 16:9, got %#v", payload["ratio"])
	}
}

func TestAdaptorConvertRequestVideoGenerationIgnoresDoubaoDurationField(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model": "alias-video",
			"prompt": "Animate a calm ocean",
			"duration": 12,
			"n_seconds": 5
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

	if duration, ok := payload["duration"].(float64); !ok || int(duration) != 5 {
		t.Fatalf("expected duration from n_seconds=5, got %#v", payload["duration"])
	}
}

func TestAdaptorConvertRequestDoubaoVideoMissingContentReturnsRelayError(t *testing.T) {
	t.Parallel()

	a := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "doubao-seedance-2-0-260128"

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{"model":"alias-video"}`),
	)

	_, err := a.ConvertRequest(m, nil, req)
	if err == nil {
		t.Fatal("expected missing content error")
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

	if string(body) != `{"detail":"content is required"}` {
		t.Fatalf("expected OpenAI video detail, got %s", body)
	}
}

func TestAdaptorConvertRequestVideoGenerationKeepsNativeContentOnce(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{
			"model": "alias-video",
			"content": [
				{"type": "text", "text": "Animate a calm ocean"},
				{"type": "image_url", "image_url": {"url": "https://example.com/reference.png"}, "role": "first_frame"}
			],
			"seconds": 5
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

	content, ok := payload["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected exactly 2 native content items, got %#v", payload["content"])
	}

	assertDoubaoVideoContent(t, content[0], "text", "", "Animate a calm ocean")
	assertDoubaoVideoContent(t, content[1], "image_url", "https://example.com/reference.png", "")

	item, ok := content[1].(map[string]any)
	if !ok {
		t.Fatalf("expected image content object, got %#v", content[1])
	}

	if item["role"] != "first_frame" {
		t.Fatalf("expected first_frame role, got %#v", item["role"])
	}
}

func TestAdaptorDoResponseVideoSubmitStoresJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	store := &doubaoTestStore{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	m := meta.NewMeta(
		&coremodel.Channel{ID: 9},
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)
	m.Group.ID = "group-1"
	m.Token.ID = 7
	setDoubaoVideoMetadata(m, doubaoVideoStoreMetadata{
		Prompt:     "Animate a calm ocean",
		Resolution: "720p",
		Ratio:      "16:9",
		Duration:   5,
		InputVideo: new(false),
	})

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"id": "task-123",
			"status": "queued",
			"model": "doubao-seedance-2-0-260128",
			"created_at": 1770000000,
			"execution_expires_after": 172800
		}`)),
	}

	result, err := adaptor.DoResponse(m, store, ctx, resp)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	if !result.AsyncUsage || result.UpstreamID != "task-123" {
		t.Fatalf("unexpected result: %#v", result)
	}

	if len(store.saved) != 1 || store.saved[0].ID != coremodel.VideoJobStoreID("task-123") {
		t.Fatalf("expected video job store, got %#v", store.saved)
	}

	if store.saved[0].Metadata != `{"prompt":"Animate a calm ocean","resolution":"720p","ratio":"16:9","duration":5,"input_video":false}` {
		t.Fatalf("unexpected saved metadata: %s", store.saved[0].Metadata)
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job response %s: %v", recorder.Body.String(), err)
	}

	if job.ID != "task-123" || job.Status != relaymodel.VideoGenerationJobStatusQueued {
		t.Fatalf("unexpected job: %#v", job)
	}

	if job.Model != "doubao-seedance-2-0-260128" ||
		job.Prompt != "Animate a calm ocean" ||
		job.NSeconds != 5 ||
		job.Width != 1280 ||
		job.Height != 720 {
		t.Fatalf("expected OpenAI job fields from request metadata, got %#v", job)
	}

	if result.UsageContext.Resolution != "1280x720" ||
		result.UsageContext.NativeResolution != "720p" {
		t.Fatalf("unexpected submit usage context: %#v", result.UsageContext)
	}
}

func TestAdaptorDoResponseVideoSubmitStoresCompletedGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	store := &doubaoTestStore{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	m := meta.NewMeta(
		&coremodel.Channel{ID: 9},
		mode.VideoGenerationsJobs,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
	)
	m.Group.ID = "group-1"
	m.Token.ID = 7
	setDoubaoVideoMetadata(m, doubaoVideoStoreMetadata{
		Prompt:     "Animate a calm ocean",
		Resolution: "720p",
		Ratio:      "9:16",
		Duration:   5,
	})

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"id": "task-123",
			"status": "succeeded",
			"model": "doubao-seedance-2-0-260128",
			"created_at": 1770000000,
			"updated_at": 1770000100,
			"execution_expires_after": 172800,
			"resolution": "720p",
			"ratio": "9:16",
			"content": {
				"video_url": "https://example.com/video.mp4"
			}
		}`)),
	}

	result, err := adaptor.DoResponse(m, store, ctx, resp)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	if !result.AsyncUsage || result.UpstreamID != "task-123" {
		t.Fatalf("unexpected result: %#v", result)
	}

	if len(store.saved) != 2 {
		t.Fatalf("expected job and generation stores, got %#v", store.saved)
	}

	if store.saved[0].ID != coremodel.VideoJobStoreID("task-123") {
		t.Fatalf("expected job store first, got %#v", store.saved[0])
	}

	if store.saved[1].ID != coremodel.VideoGenerationStoreID("task-123") {
		t.Fatalf("expected generation store second, got %#v", store.saved[1])
	}

	var job relaymodel.VideoGenerationJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to unmarshal job response %s: %v", recorder.Body.String(), err)
	}

	if job.Status != relaymodel.VideoGenerationJobStatusSucceeded ||
		len(job.Generations) != 1 ||
		job.Generations[0].ID != "task-123" {
		t.Fatalf("unexpected completed job: %#v", job)
	}

	if job.Width != 720 ||
		job.Height != 1280 ||
		job.Generations[0].Width != 720 ||
		job.Generations[0].Height != 1280 {
		t.Fatalf("expected portrait OpenAI dimensions, got %#v", job)
	}
}

func TestAdaptorDoResponseVideoStatusRestoresOpenAIFieldsFromStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	doubaoAdaptor := &Adaptor{}
	store := &doubaoTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoGenerationStoreID("video-123"),
				Metadata: `{"prompt":"A stored prompt","resolution":"720p","ratio":"9:16","duration":6}`,
			},
		},
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	m := meta.NewMeta(
		&coremodel.Channel{ID: 9},
		mode.VideosGet,
		"doubao-seedance-2-0-260128",
		coremodel.ModelConfig{},
		meta.WithVideoID("video-123"),
	)
	m.Group.ID = "group-1"
	m.Token.ID = 7

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"id": "video-123",
			"status": "succeeded",
			"created_at": 1770000000,
			"content": {
				"video_url": "https://example.com/video.mp4"
			}
		}`)),
	}

	result, err := doubaoAdaptor.DoResponse(m, store, ctx, resp)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}

	var video relaymodel.Video
	if err := json.Unmarshal(recorder.Body.Bytes(), &video); err != nil {
		t.Fatalf("failed to unmarshal video response %s: %v", recorder.Body.String(), err)
	}

	if video.ID != "video-123" ||
		video.Object != relaymodel.VideoObject ||
		video.Status != relaymodel.VideoStatusCompleted ||
		video.Model != "doubao-seedance-2-0-260128" ||
		video.Prompt != "A stored prompt" ||
		video.Seconds != 6 ||
		video.Size != "720x1280" ||
		video.Progress != 100 {
		t.Fatalf("expected OpenAI video response with stored metadata, got %#v", video)
	}

	if result.UpstreamID != "video-123" ||
		result.UsageContext.Resolution != "720x1280" ||
		result.UsageContext.NativeResolution != "720p" {
		t.Fatalf("unexpected status result: %#v", result)
	}
}

func TestAdaptorDoResponseVideoContentDownloadsGeneratedVideo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	videoServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/video.mp4" {
				t.Fatalf("expected video path, got %s", r.URL.Path)
			}

			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Length", "11")
			_, _ = w.Write([]byte("video-bytes"))
		}),
	)
	defer videoServer.Close()

	tests := []struct {
		name string
		mode mode.Mode
		id   string
		meta *meta.Meta
	}{
		{
			name: "video generation content",
			mode: mode.VideoGenerationsContent,
			id:   "generation-123",
			meta: meta.NewMeta(
				&coremodel.Channel{ID: 9},
				mode.VideoGenerationsContent,
				"doubao-seedance-2-0-260128",
				coremodel.ModelConfig{},
				meta.WithGenerationID("generation-123"),
			),
		},
		{
			name: "videos content",
			mode: mode.VideosContent,
			id:   "video-123",
			meta: meta.NewMeta(
				&coremodel.Channel{ID: 9},
				mode.VideosContent,
				"doubao-seedance-2-0-260128",
				coremodel.ModelConfig{},
				meta.WithVideoID("video-123"),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doubaoAdaptor := &Adaptor{}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"/v1/videos/"+tt.id+"/content",
				nil,
			)

			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"id": "` + tt.id + `",
					"status": "succeeded",
					"content": {
						"video_url": "` + videoServer.URL + `/video.mp4"
					}
				}`)),
			}

			result, err := doubaoAdaptor.DoResponse(tt.meta, nil, ctx, resp)
			if err != nil {
				t.Fatalf("DoResponse returned error: %v", err)
			}

			if result.UpstreamID != tt.id {
				t.Fatalf("expected upstream id %s, got %#v", tt.id, result)
			}

			if recorder.Header().Get("Content-Type") != "video/mp4" {
				t.Fatalf("expected video/mp4, got %s", recorder.Header().Get("Content-Type"))
			}

			if recorder.Body.String() != "video-bytes" {
				t.Fatalf("expected video bytes, got %q", recorder.Body.String())
			}
		})
	}
}

func TestAdaptorFetchAsyncUsageUsesDoubaoCompletionTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v3/contents/generations/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("expected authorization header, got %#v", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "task-123",
			"status": "succeeded",
			"resolution": "720p",
			"ratio": "16:9",
			"service_tier": "default",
			"generate_audio": false,
			"usage": {
				"completion_tokens": 411300,
				"total_tokens": 411300,
				"tool_usage": {"web_search": 1}
			}
		}`))
	}))
	defer server.Close()

	doubaoAdaptor := &Adaptor{}
	store := &doubaoTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID("task-123"),
				Metadata: `{"prompt":"Stored prompt","resolution":"720p","ratio":"9:16","duration":6,"input_video":true,"output_audio":true}`,
			},
		},
	}

	usage, usageContext, completed, err := doubaoAdaptor.FetchAsyncUsage(
		context.Background(),
		doubaoAsyncUsageRequest(server.URL+"/custom", "task-123", store),
	)
	if err != nil {
		t.Fatalf("FetchAsyncUsage returned error: %v", err)
	}

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usage.OutputTokens != coremodel.ZeroNullInt64(411300) ||
		usage.TotalTokens != coremodel.ZeroNullInt64(411300) ||
		usage.WebSearchCount != coremodel.ZeroNullInt64(1) {
		t.Fatalf("unexpected usage: %#v", usage)
	}

	if usageContext.Resolution != "1280x720" ||
		usageContext.NativeResolution != "720p" ||
		usageContext.ServiceTier != "default" ||
		usageContext.InputVideo == nil ||
		!*usageContext.InputVideo ||
		usageContext.OutputAudio == nil ||
		*usageContext.OutputAudio {
		t.Fatalf("unexpected usage context: %#v", usageContext)
	}
}

func TestAdaptorFetchAsyncUsageCombinesStoredRatioBeforeDerivingSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v3/contents/generations/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "task-123",
			"status": "succeeded",
			"resolution": "720p",
			"usage": {
				"completion_tokens": 411300,
				"total_tokens": 411300
			}
		}`))
	}))
	defer server.Close()

	doubaoAdaptor := &Adaptor{}
	store := &doubaoTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID("task-123"),
				Metadata: `{"prompt":"Stored prompt","resolution":"720p","ratio":"9:16","duration":6}`,
			},
		},
	}

	_, usageContext, completed, err := doubaoAdaptor.FetchAsyncUsage(
		context.Background(),
		doubaoAsyncUsageRequest(server.URL+"/custom", "task-123", store),
	)
	if err != nil {
		t.Fatalf("FetchAsyncUsage returned error: %v", err)
	}

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usageContext.Resolution != "720x1280" ||
		usageContext.NativeResolution != "720p" {
		t.Fatalf("unexpected usage context: %#v", usageContext)
	}
}

func TestAdaptorFetchAsyncUsageDoubaoNativeUsesNativeResolution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v3/contents/generations/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "task-123",
			"status": "succeeded",
			"resolution": "720p",
			"ratio": "16:9",
			"service_tier": "default",
			"usage": {
				"completion_tokens": 411300,
				"total_tokens": 411300
			}
		}`))
	}))
	defer server.Close()

	doubaoAdaptor := &Adaptor{}
	store := &doubaoTestStore{
		saved: []adaptor.StoreCache{
			{
				ID:       coremodel.VideoGenerationStoreID("task-123"),
				Metadata: `{"prompt":"Stored prompt","resolution":"1080p","ratio":"9:16","duration":6,"input_video":true,"output_audio":false}`,
			},
		},
	}

	_, usageContext, completed, err := doubaoAdaptor.FetchAsyncUsage(
		context.Background(),
		doubaoAsyncUsageRequestWithMode(
			mode.DoubaoVideo,
			server.URL+"/custom",
			"task-123",
			store,
		),
	)
	if err != nil {
		t.Fatalf("FetchAsyncUsage returned error: %v", err)
	}

	if !completed {
		t.Fatal("expected async usage to be completed")
	}

	if usageContext.Resolution != "720p" ||
		usageContext.NativeResolution != "720p" ||
		usageContext.ServiceTier != "default" ||
		usageContext.InputVideo == nil ||
		!*usageContext.InputVideo ||
		usageContext.OutputAudio == nil ||
		*usageContext.OutputAudio {
		t.Fatalf("unexpected native usage context: %#v", usageContext)
	}
}

func TestAdaptorFetchAsyncUsageDoubaoNativeUsesNativeFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/api/v3/contents/generations/tasks/task-123" {
			t.Fatalf("expected task path, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "task-123",
			"status": "succeeded",
			"usage": {
				"completion_tokens": 411300,
				"total_tokens": 411300
			}
		}`))
	}))
	defer server.Close()

	doubaoAdaptor := &Adaptor{}

	_, usageContext, completed, err := doubaoAdaptor.FetchAsyncUsage(
		context.Background(),
		adaptor.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				BaseURL: server.URL + "/fallback",
				Key:     "test-key",
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.DoubaoVideo),
				BaseURL:    server.URL + "/custom",
				UpstreamID: "task-123",
				GroupID:    "group-1",
				TokenID:    7,
				UsageContext: coremodel.UsageContext{
					Resolution:       "1080p",
					NativeResolution: "1080p",
					ServiceTier:      "priority",
					InputVideo:       new(true),
					OutputAudio:      new(false),
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

	if usageContext.Resolution != "1080p" ||
		usageContext.NativeResolution != "1080p" ||
		usageContext.ServiceTier != "priority" ||
		usageContext.InputVideo == nil ||
		!*usageContext.InputVideo ||
		usageContext.OutputAudio == nil ||
		*usageContext.OutputAudio {
		t.Fatalf("unexpected native usage context fallback: %#v", usageContext)
	}
}

func doubaoAsyncUsageRequest(
	baseURL string,
	upstreamID string,
	store adaptor.Store,
) adaptor.AsyncUsageRequest {
	return doubaoAsyncUsageRequestWithMode(
		mode.VideoGenerationsJobs,
		baseURL,
		upstreamID,
		store,
	)
}

func doubaoAsyncUsageRequestWithMode(
	relayMode mode.Mode,
	baseURL string,
	upstreamID string,
	store adaptor.Store,
) adaptor.AsyncUsageRequest {
	return adaptor.AsyncUsageRequest{
		Channel: &coremodel.Channel{
			BaseURL: baseURL + "/fallback",
			Key:     "test-key",
		},
		Info: &coremodel.AsyncUsageInfo{
			Mode:       int(relayMode),
			BaseURL:    baseURL,
			UpstreamID: upstreamID,
			GroupID:    "group-1",
			TokenID:    7,
		},
		Store: store,
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

func TestHandlerPreHandler_UsesOriginModelNameFirst(t *testing.T) {
	m := meta.NewMeta(nil, mode.ChatCompletions, "bot-123", coremodel.ModelConfig{})
	m.ActualModel = "mapped-model"

	node, err := common.GetJSONNodeNoCopy([]byte(`{
		"bot_usage": {
			"model_usage": [{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3}],
			"action_usage": [{"count": 4}]
		}
	}`))
	if err != nil {
		t.Fatalf("failed to build node: %v", err)
	}

	websearchCount := int64(0)
	if err := handlerPreHandler(m, &node, &websearchCount); err != nil {
		t.Fatalf("handlerPreHandler returned error: %v", err)
	}

	usageNode := node.Get("usage")
	if usageNode.Check() != nil {
		t.Fatal("expected usage to be copied from bot_usage.model_usage")
	}

	if websearchCount != 4 {
		t.Fatalf("expected websearchCount=4, got %d", websearchCount)
	}
}

func assertDoubaoEmbeddingURLItem(t *testing.T, got any, itemType, wantURL string) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected input item object, got %#v", got)
	}

	if gotMap["type"] != itemType {
		t.Fatalf("expected type %q, got %#v", itemType, gotMap["type"])
	}

	urlObject, ok := gotMap[itemType].(map[string]any)
	if !ok {
		t.Fatalf("expected %s object, got %#v", itemType, gotMap[itemType])
	}

	if urlObject["url"] != wantURL {
		t.Fatalf("expected %s.url=%q, got %#v", itemType, wantURL, urlObject["url"])
	}
}

func assertDoubaoEmbeddingTextItem(t *testing.T, got any, wantText string) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected input item object, got %#v", got)
	}

	if gotMap["type"] != "text" {
		t.Fatalf("expected type text, got %#v", gotMap["type"])
	}

	if gotMap["text"] != wantText {
		t.Fatalf("expected text=%q, got %#v", wantText, gotMap["text"])
	}
}

func assertDoubaoVideoContent(t *testing.T, got any, itemType, wantURL, wantText string) {
	t.Helper()

	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected content item object, got %#v", got)
	}

	if gotMap["type"] != itemType {
		t.Fatalf("expected type %q, got %#v", itemType, gotMap["type"])
	}

	if itemType == "text" {
		if gotMap["text"] != wantText {
			t.Fatalf("expected text=%q, got %#v", wantText, gotMap["text"])
		}
		return
	}

	urlObject, ok := gotMap[itemType].(map[string]any)
	if !ok {
		t.Fatalf("expected %s object, got %#v", itemType, gotMap[itemType])
	}

	if urlObject["url"] != wantURL {
		t.Fatalf("expected %s.url=%q, got %#v", itemType, wantURL, urlObject["url"])
	}
}

func TestAdaptorConvertRequestGemini(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Gemini,
		"doubao-seed-1-6",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/doubao-seed-1-6:streamGenerateContent",
		strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`),
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if openAIReq.Model != "doubao-seed-1-6" {
		t.Fatalf("expected model doubao-seed-1-6, got %s", openAIReq.Model)
	}

	if !openAIReq.Stream {
		t.Fatal("expected stream to be enabled")
	}

	if len(openAIReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(openAIReq.Messages))
	}

	if openAIReq.Messages[0].Role != relaymodel.RoleUser {
		t.Fatalf("expected user message, got %s", openAIReq.Messages[0].Role)
	}

	if openAIReq.Thinking != nil {
		t.Fatalf("expected thinking to stay unset by default, got %#v", openAIReq.Thinking)
	}
}

func TestAdaptorConvertRequestGeminiReasoning(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Gemini,
		"doubao-seed-1-6",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/doubao-seed-1-6:generateContent",
		strings.NewReader(`{
			"generationConfig":{"thinkingConfig":{"thinkingBudget":2048,"includeThoughts":true}},
			"contents":[{"role":"user","parts":[{"text":"hello"}]}]
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if openAIReq.Thinking == nil {
		t.Fatal("expected thinking to be set")
	}

	if openAIReq.Thinking.Type != relaymodel.ClaudeThinkingTypeEnabled {
		t.Fatalf("expected thinking.type enabled, got %s", openAIReq.Thinking.Type)
	}

	if openAIReq.ReasoningEffort != nil {
		t.Fatal("expected reasoning_effort to be removed")
	}
}

func TestAdaptorConvertRequestChatReasoning(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"doubao-seed-1-6",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"doubao-seed-1-6",
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if openAIReq.Thinking == nil {
		t.Fatal("expected thinking to be set")
	}

	if openAIReq.Thinking.Type != relaymodel.ClaudeThinkingTypeEnabled {
		t.Fatalf("expected thinking.type enabled, got %s", openAIReq.Thinking.Type)
	}
}

func TestAdaptorConvertRequestChatReasoningDisabled(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"doubao-seed-1-6",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"doubao-seed-1-6",
			"reasoning_effort":"none",
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if openAIReq.Thinking == nil {
		t.Fatal("expected thinking to be set")
	}

	if openAIReq.Thinking.Type != relaymodel.ClaudeThinkingTypeDisabled {
		t.Fatalf("expected thinking.type disabled, got %s", openAIReq.Thinking.Type)
	}
}

func TestAdaptorConvertRequestChatDeepseekReasonerPrompt(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"doubao-seed-1-6",
		coremodel.ModelConfig{},
	)
	m.OriginModel = "deepseek-reasoner"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"doubao-seed-1-6",
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if len(openAIReq.Messages) < 2 {
		t.Fatalf("expected injected system prompt, got %d messages", len(openAIReq.Messages))
	}

	if openAIReq.Messages[0].Role != relaymodel.RoleSystem {
		t.Fatalf("expected first message to be system, got %s", openAIReq.Messages[0].Role)
	}
}

func TestAdaptorConvertRequestChatDeepseekReasonerPrompt_UsesActualModelFallback(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		"alias-model",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "deepseek-reasoner"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"alias-model",
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if len(openAIReq.Messages) < 2 {
		t.Fatalf("expected injected system prompt, got %d messages", len(openAIReq.Messages))
	}

	if openAIReq.Messages[0].Role != relaymodel.RoleSystem {
		t.Fatalf("expected first message to be system, got %s", openAIReq.Messages[0].Role)
	}
}

func TestAdaptorConvertRequestAnthropicReasoning(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Anthropic,
		"doubao-seed-1-6",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/messages",
		strings.NewReader(`{
			"model":"doubao-seed-1-6",
			"thinking":{"type":"enabled","budget_tokens":2048},
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

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if openAIReq.Thinking == nil {
		t.Fatal("expected thinking to be set")
	}

	if openAIReq.Thinking.Type != relaymodel.ClaudeThinkingTypeEnabled {
		t.Fatalf("expected thinking.type enabled, got %s", openAIReq.Thinking.Type)
	}

	if openAIReq.ReasoningEffort != nil {
		t.Fatal("expected reasoning_effort to be removed")
	}
}
