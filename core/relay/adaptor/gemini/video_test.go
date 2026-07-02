package gemini_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/consume"
	coremodel "github.com/labring/aiproxy/core/model"
	adaptorapi "github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/require"
)

type geminiVideoTestStore struct {
	saved []adaptorapi.StoreCache
}

func geminiVideoModelPriceForTest(t *testing.T, modelName string) coremodel.Price {
	t.Helper()

	for _, mc := range gemini.ModelList {
		if mc.Model == modelName {
			return mc.Price
		}
	}

	t.Fatalf("model %s not found", modelName)

	return coremodel.Price{}
}

func (s *geminiVideoTestStore) getStore(
	id string,
) (adaptorapi.StoreCache, error) {
	for _, cache := range s.saved {
		if cache.ID == id {
			return cache, nil
		}
	}

	return adaptorapi.StoreCache{}, coremodel.NotFoundError(coremodel.ErrStoreNotFound)
}

func (s *geminiVideoTestStore) GetStoreByScope(
	group string,
	tokenID int,
	id string,
	_ coremodel.ChannelScope,
) (adaptorapi.StoreCache, error) {
	return s.getStore(id)
}

func (s *geminiVideoTestStore) SaveStore(
	cache adaptorapi.StoreCache,
	_ coremodel.ChannelScope,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *geminiVideoTestStore) SaveStoreWithOption(
	cache adaptorapi.StoreCache,
	_ coremodel.ChannelScope,
	_ adaptorapi.SaveStoreOption,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *geminiVideoTestStore) SaveIfNotExistStore(
	cache adaptorapi.StoreCache,
	_ coremodel.ChannelScope,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func TestVideoContentHandlersDownloadGeneratedVideo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	videoServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/video.mp4", r.URL.Path)
			require.Equal(t, "apikey", r.Header.Get("X-Goog-Api-Key"))

			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Length", "10")
			_, _ = w.Write([]byte("video-data"))
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
			id:   "gemini_op_123-0",
			meta: meta.NewMeta(
				&coremodel.Channel{Key: "apikey"},
				mode.VideoGenerationsContent,
				"veo-3.1-generate-preview",
				coremodel.ModelConfig{},
				meta.WithGenerationID("gemini_op_123-0"),
			),
		},
		{
			name: "videos content",
			mode: mode.VideosContent,
			id:   "gemini_op_123",
			meta: meta.NewMeta(
				&coremodel.Channel{Key: "apikey"},
				mode.VideosContent,
				"veo-3.1-generate-preview",
				coremodel.ModelConfig{},
				meta.WithVideoID("gemini_op_123"),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"/content",
				nil,
			)

			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{
					"name":"models/veo-3.1-generate-preview/operations/123",
					"done":true,
					"response":{
						"generateVideoResponse":{
							"generatedSamples":[{"video":{"uri":"%s/video.mp4"}}]
						}
					}
				}`, videoServer.URL))),
			}

			var (
				result     adaptorapi.DoResponseResult
				adaptorErr adaptorapi.Error
			)
			if tt.mode == mode.VideoGenerationsContent {
				result, adaptorErr = gemini.VideoGenerationJobContentHandler(tt.meta, c, resp)
			} else {
				result, adaptorErr = gemini.VideosContentHandler(tt.meta, c, resp)
			}

			require.Nil(t, adaptorErr)
			require.Equal(t, tt.id, result.UpstreamID)
			require.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
			require.Equal(t, "video-data", recorder.Body.String())
		})
	}
}

func TestGetRequestURLGeminiVideoUsesPredictLongRunning(t *testing.T) {
	adaptor := &gemini.Adaptor{}
	m := meta.NewMeta(nil, mode.GeminiVideo, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	m.Channel.BaseURL = "https://generativelanguage.googleapis.com"

	reqURL, err := adaptor.GetRequestURL(m, nil, nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, reqURL.Method)
	require.Equal(
		t,
		"https://generativelanguage.googleapis.com/v1beta/models/veo-3.1-generate-preview:predictLongRunning",
		reqURL.URL,
	)
}

func TestGetRequestURLGeminiVideoOperationsUsesGet(t *testing.T) {
	adaptor := &gemini.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.GeminiVideoOperations,
		"public-veo",
		coremodel.ModelConfig{},
		meta.WithOperationID("video-123"),
	)
	m.ActualModel = "veo-3.1-generate-preview"
	m.OriginModel = "public-veo"
	m.Channel.BaseURL = "https://generativelanguage.googleapis.com"

	reqURL, err := adaptor.GetRequestURL(m, nil, nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, reqURL.Method)
	require.Equal(
		t,
		"https://generativelanguage.googleapis.com/v1beta/models/veo-3.1-generate-preview/operations/video-123",
		reqURL.URL,
	)
}

func TestGetRequestURLGeminiVideoOperationsUsesStoredUpstreamOperation(t *testing.T) {
	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID("video-123"),
				Metadata: "projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
			},
		},
	}
	m := meta.NewMeta(
		nil,
		mode.GeminiVideoOperations,
		"public-veo",
		coremodel.ModelConfig{},
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
		meta.WithOperationID("video-123"),
	)
	m.ActualModel = "veo-3.1-generate-preview"
	m.OriginModel = "public-veo"
	m.Channel.BaseURL = "https://generativelanguage.googleapis.com"

	reqURL, err := adaptor.GetRequestURL(m, store, nil)
	require.NoError(t, err)
	require.Equal(
		t,
		"https://generativelanguage.googleapis.com/v1beta/projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
		reqURL.URL,
	)
}

func TestGetRequestURLVideoGenerationContentUsesOperationID(t *testing.T) {
	adaptor := &gemini.Adaptor{}
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsContent,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGenerationID(operationName+":1"),
	)
	m.Channel.BaseURL = "https://generativelanguage.googleapis.com"

	reqURL, err := adaptor.GetRequestURL(m, nil, nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, reqURL.Method)
	require.Equal(
		t,
		"https://generativelanguage.googleapis.com/v1beta/models/veo-3.1-generate-preview/operations/video-123",
		reqURL.URL,
	)
}

func TestGetRequestURLVideoGenerationGetJobsDecodesLocalID(t *testing.T) {
	adaptor := &gemini.Adaptor{}
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	localID := gemini.GeminiVideoLocalIDForTest(operationName)
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID(localID),
				Metadata: operationName,
			},
		},
	}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
		meta.WithJobID(localID),
	)
	m.Channel.BaseURL = "https://generativelanguage.googleapis.com"

	reqURL, err := adaptor.GetRequestURL(m, store, nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, reqURL.Method)
	require.Equal(
		t,
		"https://generativelanguage.googleapis.com/v1beta/models/veo-3.1-generate-preview/operations/video-123",
		reqURL.URL,
	)
	require.NotContains(t, localID, "/")
}

func TestGetRequestURLVideoGenerationContentDecodesLocalGenerationID(t *testing.T) {
	adaptor := &gemini.Adaptor{}
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	localGenerationID := gemini.GeminiVideoLocalIDForTest(operationName) + ":1"
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				ID: coremodel.VideoGenerationStoreID(
					strings.TrimSuffix(localGenerationID, ":1"),
				),
				Metadata: operationName,
			},
		},
	}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsContent,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
		meta.WithGenerationID(localGenerationID),
	)
	m.Channel.BaseURL = "https://generativelanguage.googleapis.com"

	reqURL, err := adaptor.GetRequestURL(m, store, nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, reqURL.Method)
	require.Equal(
		t,
		"https://generativelanguage.googleapis.com/v1beta/models/veo-3.1-generate-preview/operations/video-123",
		reqURL.URL,
	)
}

func TestConvertVideoRequestMapsOpenAIFields(t *testing.T) {
	m := meta.NewMeta(nil, mode.Videos, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(
			`{"model":"veo","prompt":"make a video","seconds":8,"n_variants":2,"size":"1280x720","input_reference":"gs://bucket/image.png"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video","image":{"fileData":{"fileUri":"gs://bucket/image.png"}}}],
			"parameters":{"aspectRatio":"16:9","resolution":"720p","durationSeconds":8}
		}`,
		string(body),
	)
	require.Zero(t, m.RequestUsage.OutputTokens)
	require.Empty(t, m.RequestUsageContext.Resolution)
	require.Equal(
		t,
		coremodel.ZeroNullInt64(8),
		gemini.GeminiVideoRequestUsageForTest(m).OutputTokens,
	)
	require.Equal(t, "1280x720", gemini.GeminiVideoUsageContextForTest(m).Resolution)
	require.Equal(t, "720p", gemini.GeminiVideoUsageContextForTest(m).NativeResolution)
}

func TestConvertVideosEditRequestMapsVideoField(t *testing.T) {
	t.Parallel()

	m := meta.NewMeta(nil, mode.VideosEdits, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/edits",
		strings.NewReader(
			`{"model":"veo","prompt":"edit the video","seconds":4,"size":"1280x720","video":"gs://bucket/source.mp4"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"edit the video","video":{"fileData":{"fileUri":"gs://bucket/source.mp4"}}}],
			"parameters":{"aspectRatio":"16:9","resolution":"720p","durationSeconds":4}
		}`,
		string(body),
	)
}

func TestConvertVideosExtensionRequestMapsVideoField(t *testing.T) {
	t.Parallel()

	m := meta.NewMeta(
		nil,
		mode.VideosExtensions,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/extensions",
		strings.NewReader(
			`{"model":"veo","prompt":"extend the video","seconds":4,"size":"1280x720","video":"gs://bucket/source.mp4"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"extend the video","video":{"fileData":{"fileUri":"gs://bucket/source.mp4"}}}],
			"parameters":{"aspectRatio":"16:9","resolution":"720p","durationSeconds":4}
		}`,
		string(body),
	)
}

func TestConvertVideosEditRequestHydratesStoredVideoID(t *testing.T) {
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	localID := gemini.GeminiVideoLocalIDForTest(operationName)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/"+operationName, r.URL.Path)
		require.Equal(t, "apikey", r.Header.Get("X-Goog-Api-Key"))

		_, _ = w.Write([]byte(`{
			"name":"` + operationName + `",
			"done":true,
			"response":{
				"generateVideoResponse":{
					"generatedSamples":[
						{"video":{"uri":"https://example.com/generated.mp4"}}
					]
				}
			}
		}`))
	}))
	defer upstream.Close()

	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				ID:       coremodel.VideoGenerationStoreID(localID),
				Metadata: operationName,
			},
		},
	}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: upstream.URL, Key: "apikey"},
		mode.VideosEdits,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
		meta.WithVideoID(localID),
	)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos/edits",
		strings.NewReader(
			`{"prompt":"edit stored video","video":"`+localID+`","seconds":4,"size":"1280x720"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := (&gemini.Adaptor{}).ConvertRequest(m, store, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"edit stored video","video":{"fileData":{"fileUri":"https://example.com/generated.mp4"}}}],
			"parameters":{"aspectRatio":"16:9","resolution":"720p","durationSeconds":4}
		}`,
		string(body),
	)
}

func TestConvertVideosRequestIgnoresNSeconds(t *testing.T) {
	m := meta.NewMeta(nil, mode.Videos, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(
			`{"model":"veo","prompt":"make a video","n_seconds":5,"size":"1280x720"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video"}],
			"parameters":{"aspectRatio":"16:9","resolution":"720p"}
		}`,
		string(body),
	)
	require.Equal(
		t,
		coremodel.ZeroNullInt64(8),
		gemini.GeminiVideoRequestUsageForTest(m).OutputTokens,
	)
}

func TestConvertVideosRequestUsesSecondsOverNSeconds(t *testing.T) {
	m := meta.NewMeta(nil, mode.Videos, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(
			`{"model":"veo","prompt":"make a video","seconds":6,"n_seconds":5,"size":"1280x720"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video"}],
			"parameters":{"aspectRatio":"16:9","resolution":"720p","durationSeconds":6}
		}`,
		string(body),
	)
}

func TestConvertVideoRequestMissingPromptReturnsRelayError(t *testing.T) {
	t.Parallel()

	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{"model":"veo"}`),
	)

	_, err := gemini.ConvertVideoRequestForTest(m, req)
	require.Error(t, err)

	var relayErr adaptorapi.Error
	require.ErrorAs(t, err, &relayErr)
	require.Equal(t, http.StatusBadRequest, relayErr.StatusCode())

	body, marshalErr := relayErr.MarshalJSON()
	require.NoError(t, marshalErr)
	require.JSONEq(t, `{"detail":"prompt or input_reference is required"}`, string(body))
}

func TestConvertVideoRequestKeepsProtocolAndNativeResolutionForPricing(t *testing.T) {
	m := meta.NewMeta(
		nil,
		mode.Videos,
		"veo-3.1-fast-generate-preview",
		coremodel.ModelConfig{},
		meta.WithRequestUsageContext(coremodel.UsageContext{
			Resolution: "1280x720",
		}),
	)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(
			`{"model":"veo","prompt":"make a video","seconds":8,"size":"1280x720"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	_, err = gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	usageContext := gemini.GeminiVideoUsageContextForTest(m)
	require.Equal(t, "1280x720", usageContext.Resolution)
	require.Equal(t, "720p", usageContext.NativeResolution)

	amount := consume.CalculateAmountDetail(
		200,
		coremodel.Usage{
			OutputTokens: 8,
			TotalTokens:  8,
		},
		usageContext,
		geminiVideoModelPriceForTest(t, "veo-3.1-fast-generate-preview"),
	)
	require.Equal(t, 0.8, amount.UsedAmount)
}

func TestNativeVideoConvertRequestStoresDefaultAsyncUsageMetadata(t *testing.T) {
	m := meta.NewMeta(nil, mode.GeminiVideo, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/veo-3.1-generate-preview:predictLongRunning",
		strings.NewReader(`{"instances":[{"prompt":"make a video"}]}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	_, err = gemini.NativeVideoConvertRequest(m, req)
	require.NoError(t, err)

	require.Zero(t, m.RequestUsage.OutputTokens)
	require.Equal(
		t,
		coremodel.ZeroNullInt64(8),
		gemini.GeminiVideoRequestUsageForTest(m).OutputTokens,
	)
	require.Equal(t, "720p", gemini.GeminiVideoUsageContextForTest(m).Resolution)
	require.Equal(t, "720p", gemini.GeminiVideoUsageContextForTest(m).NativeResolution)
}

func TestNativeVideoConvertRequestRemovesUnsupportedNumberOfVideos(t *testing.T) {
	m := meta.NewMeta(nil, mode.GeminiVideo, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/veo-3.1-generate-preview:predictLongRunning",
		strings.NewReader(
			`{"instances":[{"prompt":"make a video"}],"parameters":{"durationSeconds":6,"numberOfVideos":2}}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.NativeVideoConvertRequest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{"instances":[{"prompt":"make a video"}],"parameters":{"durationSeconds":6}}`,
		string(body),
	)
	require.Equal(
		t,
		coremodel.ZeroNullInt64(12),
		gemini.GeminiVideoRequestUsageForTest(m).OutputTokens,
	)
}

func TestConvertVideoGenerationJobRequestUsesJobOnlyFields(t *testing.T) {
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(
			`{"model":"veo","prompt":"make a video","n_seconds":8,"n_variants":2,"width":1280,"height":720}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video"}],
			"parameters":{"aspectRatio":"16:9","resolution":"720p","durationSeconds":8}
		}`,
		string(body),
	)
	require.Zero(t, m.RequestUsage.OutputTokens)
	require.Empty(t, m.RequestUsageContext.Resolution)
	require.Equal(
		t,
		coremodel.ZeroNullInt64(16),
		gemini.GeminiVideoRequestUsageForTest(m).OutputTokens,
	)
	require.Equal(t, "1280x720", gemini.GeminiVideoUsageContextForTest(m).Resolution)
	require.Equal(t, "720p", gemini.GeminiVideoUsageContextForTest(m).NativeResolution)
}

func TestConvertVideoGenerationJobRequestIgnoresVideosSeconds(t *testing.T) {
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(
			`{"model":"veo","prompt":"make a video","seconds":8,"n_variants":2,"width":1280,"height":720}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video"}],
			"parameters":{"aspectRatio":"16:9","resolution":"720p"}
		}`,
		string(body),
	)
}

func TestConvertVideoGenerationJobRequestStoresDefaultAsyncUsageMetadata(t *testing.T) {
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{"model":"veo","prompt":"make a video"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{"instances":[{"prompt":"make a video"}],"parameters":{}}`,
		string(body),
	)
	require.Zero(t, m.RequestUsage.OutputTokens)
	require.Equal(
		t,
		coremodel.ZeroNullInt64(8),
		gemini.GeminiVideoRequestUsageForTest(m).OutputTokens,
	)
	require.Equal(t, "1280x720", gemini.GeminiVideoUsageContextForTest(m).Resolution)
	require.Equal(t, "720p", gemini.GeminiVideoUsageContextForTest(m).NativeResolution)
}

func TestConvertVideosRequestStoresDefaultAsyncUsageMetadata(t *testing.T) {
	m := meta.NewMeta(
		nil,
		mode.Videos,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(`{"model":"veo","prompt":"make a video"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{"instances":[{"prompt":"make a video"}],"parameters":{}}`,
		string(body),
	)
	require.Zero(t, m.RequestUsage.OutputTokens)
	require.Equal(
		t,
		coremodel.ZeroNullInt64(8),
		gemini.GeminiVideoRequestUsageForTest(m).OutputTokens,
	)
	require.Equal(t, "1280x720", gemini.GeminiVideoUsageContextForTest(m).Resolution)
	require.Equal(t, "720p", gemini.GeminiVideoUsageContextForTest(m).NativeResolution)
}

func TestVideoSubmitHandlerReturnsURLSafeJobID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	m.OriginModel = "veo-3.1-generate-preview"
	m.Group.ID = "group-1"
	m.Token.ID = 7
	m.Channel.ID = 11

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"name":"models/veo-3.1-generate-preview/operations/video-123","done":false}`,
		)),
	}

	result, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)
	require.True(t, result.AsyncUsage)
	require.Equal(t, "models/veo-3.1-generate-preview/operations/video-123", result.UpstreamID)

	var job relaymodel.VideoGenerationJob
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &job))
	require.NotEmpty(t, job.ID)
	require.Equal(t, relaymodel.VideoGenerationJobStatusRunning, job.Status)
	require.NotContains(t, job.ID, "/")
	require.Equal(t, coremodel.VideoJobStoreID(job.ID), store.saved[0].ID)
	require.Equal(
		t,
		result.UpstreamID,
		gemini.GeminiVideoStoreMetadataForTest(store.saved[0].Metadata),
	)
	require.LessOrEqual(t, len(store.saved[0].ID), 128)
}

func TestVideoSubmitHandlerReturnsDefaultContextAndDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	m.OriginModel = "veo-3.1-generate-preview"
	m.Group.ID = "group-1"
	m.Token.ID = 7
	m.Channel.ID = 11

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{"model":"veo","prompt":"make a video"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	_, err = gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"name":"models/veo-3.1-generate-preview/operations/video-123","done":false}`,
		)),
	}

	result, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)
	require.True(t, result.AsyncUsage)
	require.Zero(t, result.Usage.OutputTokens)
	require.Zero(t, result.Usage.TotalTokens)
	require.Equal(t, "1280x720", result.UsageContext.Resolution)
	require.Equal(t, "720p", result.UsageContext.NativeResolution)

	var job relaymodel.VideoGenerationJob
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &job))
	require.Equal(t, 8, job.NSeconds)
	require.Equal(t, 1, job.NVariants)
	require.Equal(t, 1280, job.Width)
	require.Equal(t, 720, job.Height)
}

func TestVideoSubmitHandlerConvertsGeminiErrorToOpenAIVideoError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"code":400,"message":"Requested duration is not supported for this model.","status":"INVALID_ARGUMENT"}}`,
		)),
	}

	_, relayErr := adaptor.DoResponse(m, nil, c, resp)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadRequest, relayErr.StatusCode())

	body, err := relayErr.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{"detail":"Requested duration is not supported for this model."}`,
		string(body),
	)
}

func TestVideosSubmitHandlerReturnsDefaultContextDurationAndSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{}
	m := meta.NewMeta(nil, mode.Videos, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	m.OriginModel = "veo-3.1-generate-preview"
	m.Group.ID = "group-1"
	m.Token.ID = 7
	m.Channel.ID = 11

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(`{"model":"veo","prompt":"make a video"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	_, err = gemini.ConvertVideoRequestForTest(m, req)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/videos", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"name":"models/veo-3.1-generate-preview/operations/video-123","done":false}`,
		)),
	}

	result, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)
	require.True(t, result.AsyncUsage)
	require.Zero(t, result.Usage.OutputTokens)
	require.Zero(t, result.Usage.TotalTokens)
	require.Equal(t, "1280x720", result.UsageContext.Resolution)
	require.Equal(t, "720p", result.UsageContext.NativeResolution)

	var video relaymodel.Video
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &video))
	require.Equal(t, 8, video.Seconds)
	require.Equal(t, "1280x720", video.Size)
}

func TestNativeVideoHandlerStoresShortOperationID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{}
	m := meta.NewMeta(
		nil,
		mode.GeminiVideo,
		"public-veo",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "veo-3.1-generate-preview"
	m.OriginModel = "public-veo"
	m.Group.ID = "group-1"
	m.Token.ID = 7
	m.Channel.ID = 11

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1beta/models/veo-3.1-generate-preview:predictLongRunning",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"name":"models/veo-3.1-generate-preview/operations/video-123","done":false}`,
		)),
	}

	result, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)
	require.True(t, result.AsyncUsage)
	require.Equal(t, "models/veo-3.1-generate-preview/operations/video-123", result.UpstreamID)
	require.Equal(t, coremodel.VideoJobStoreID("video-123"), store.saved[0].ID)
	require.Equal(
		t,
		result.UpstreamID,
		gemini.GeminiVideoStoreMetadataForTest(store.saved[0].Metadata),
	)
	require.JSONEq(
		t,
		`{"name":"models/public-veo/operations/video-123","done":false}`,
		recorder.Body.String(),
	)
}

func TestVideoSubmitHandlerStoresBoundedIDForLongVertexOperation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	m.OriginModel = "veo-3.1-generate-preview"
	m.Group.ID = "group-1"
	m.Token.ID = 7
	m.Channel.ID = 11
	m.Channel.BaseURL = "https://us-central1-aiplatform.googleapis.com"

	operationName := "projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/1234567890abcdef1234567890abcdef"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"name":"` + operationName + `","done":false}`,
		)),
	}

	_, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)

	var job relaymodel.VideoGenerationJob
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &job))
	require.NotContains(t, job.ID, "/")
	require.LessOrEqual(t, len(coremodel.VideoJobStoreID(job.ID)), 128)
	require.Equal(t, operationName, gemini.GeminiVideoStoreMetadataForTest(store.saved[0].Metadata))

	followupMeta := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(m.Group),
		meta.WithToken(m.Token),
		meta.WithJobID(job.ID),
	)
	followupMeta.Channel.BaseURL = "https://us-central1-aiplatform.googleapis.com"

	reqURL, err := adaptor.GetRequestURL(followupMeta, store, nil)
	require.NoError(t, err)
	require.Equal(
		t,
		"https://us-central1-aiplatform.googleapis.com/v1beta/"+operationName,
		reqURL.URL,
	)
}

func TestVideoStatusHandlerResolvesStoredOperationWhenResponseNameMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	localID := gemini.GeminiVideoLocalIDForTest(operationName)
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID(localID),
				Metadata: operationName,
			},
		},
	}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
		meta.WithJobID(localID),
	)
	m.OriginModel = "veo-3.1-generate-preview"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/video/generations/jobs/"+localID,
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"done":true,"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"uri":"https://example.com/video.mp4"}}]}}}`,
		)),
	}

	_, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)

	var job relaymodel.VideoGenerationJob
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &job))
	require.Equal(t, localID, job.ID)
	require.Len(t, job.Generations, 1)
	require.Equal(t, localID, job.Generations[0].ID)
	require.Equal(t, operationName, gemini.GeminiVideoStoreMetadataForTest(store.saved[1].Metadata))
}

func TestVideoStatusHandlerReturnsStoredDefaultDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	submitStore := &geminiVideoTestStore{}
	submitMeta := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	submitMeta.OriginModel = "veo-3.1-generate-preview"
	submitMeta.Group.ID = "group-1"
	submitMeta.Token.ID = 7
	submitMeta.Channel.ID = 11

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{"model":"veo","prompt":"make a video"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	_, err = gemini.ConvertVideoRequestForTest(submitMeta, req)
	require.NoError(t, err)

	submitRecorder := httptest.NewRecorder()
	submitCtx, _ := gin.CreateTestContext(submitRecorder)
	submitCtx.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		nil,
	)
	submitResp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"name":"models/veo-3.1-generate-preview/operations/video-123","done":false}`,
		)),
	}
	_, relayErr := adaptor.DoResponse(submitMeta, submitStore, submitCtx, submitResp)
	require.Nil(t, relayErr)

	var submittedJob relaymodel.VideoGenerationJob
	require.NoError(t, json.Unmarshal(submitRecorder.Body.Bytes(), &submittedJob))

	statusMeta := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(submitMeta.Group),
		meta.WithToken(submitMeta.Token),
		meta.WithJobID(submittedJob.ID),
	)
	statusMeta.OriginModel = "veo-3.1-generate-preview"

	statusRecorder := httptest.NewRecorder()
	statusCtx, _ := gin.CreateTestContext(statusRecorder)
	statusCtx.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/video/generations/jobs/"+submittedJob.ID,
		nil,
	)
	statusResp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"done":false}`,
		)),
	}

	_, relayErr = adaptor.DoResponse(statusMeta, submitStore, statusCtx, statusResp)
	require.Nil(t, relayErr)

	var job relaymodel.VideoGenerationJob
	require.NoError(t, json.Unmarshal(statusRecorder.Body.Bytes(), &job))
	require.Equal(t, relaymodel.VideoGenerationJobStatusRunning, job.Status)
	require.Equal(t, 8, job.NSeconds)
	require.Equal(t, 1, job.NVariants)
	require.Equal(t, "make a video", job.Prompt)
	require.Equal(t, 1280, job.Width)
	require.Equal(t, 720, job.Height)
}

func TestVideoStatusHandlerTreatsRAIFilteredResponseAsFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	localID := gemini.GeminiVideoLocalIDForTest(operationName)
	reason := "Sorry, we can't create videos with real people's names or likenesses. Please remove the celebrity reference and try again."
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID(localID),
				Metadata: operationName,
			},
		},
	}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
		meta.WithJobID(localID),
	)
	m.OriginModel = "veo-3.1-generate-preview"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/video/generations/jobs/"+localID,
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"done":true,"response":{"generateVideoResponse":{"raiMediaFilteredCount":1,"raiMediaFilteredReasons":["` + reason + `"]}}}`,
		)),
	}

	_, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)

	var job relaymodel.VideoGenerationJob
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &job))
	require.Equal(t, "failed", job.Status)
	require.NotNil(t, job.FinishedAt)
	require.Empty(t, job.Generations)
	require.NotNil(t, job.FinishReason)
	require.Equal(t, reason, *job.FinishReason)
}

func TestVideoStatusHandlerTreatsRAIFilteredCountOnlyResponseAsFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	localID := gemini.GeminiVideoLocalIDForTest(operationName)
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				ID:       coremodel.VideoJobStoreID(localID),
				Metadata: operationName,
			},
		},
	}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsGetJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
		meta.WithJobID(localID),
	)
	m.OriginModel = "veo-3.1-generate-preview"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/video/generations/jobs/"+localID,
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"done":true,"response":{"generateVideoResponse":{"raiMediaFilteredCount":1}}}`,
		)),
	}

	_, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)

	var job relaymodel.VideoGenerationJob
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &job))
	require.Equal(t, "failed", job.Status)
	require.NotNil(t, job.FinishedAt)
	require.Empty(t, job.Generations)
	require.NotNil(t, job.FinishReason)
	require.NotEmpty(t, *job.FinishReason)
}

func TestVideosStatusHandlerReturnsStoredDefaultDurationAndSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	submitStore := &geminiVideoTestStore{}
	submitMeta := meta.NewMeta(
		nil,
		mode.Videos,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	submitMeta.OriginModel = "veo-3.1-generate-preview"
	submitMeta.Group.ID = "group-1"
	submitMeta.Token.ID = 7
	submitMeta.Channel.ID = 11

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(`{"model":"veo","prompt":"make a video"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	_, err = gemini.ConvertVideoRequestForTest(submitMeta, req)
	require.NoError(t, err)

	submitRecorder := httptest.NewRecorder()
	submitCtx, _ := gin.CreateTestContext(submitRecorder)
	submitCtx.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/videos",
		nil,
	)
	submitResp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"name":"models/veo-3.1-generate-preview/operations/video-123","done":false}`,
		)),
	}
	_, relayErr := adaptor.DoResponse(submitMeta, submitStore, submitCtx, submitResp)
	require.Nil(t, relayErr)

	var submittedVideo relaymodel.Video
	require.NoError(t, json.Unmarshal(submitRecorder.Body.Bytes(), &submittedVideo))

	statusMeta := meta.NewMeta(
		nil,
		mode.VideosGet,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(submitMeta.Group),
		meta.WithToken(submitMeta.Token),
		meta.WithVideoID(submittedVideo.ID),
	)
	statusMeta.OriginModel = "veo-3.1-generate-preview"

	statusRecorder := httptest.NewRecorder()
	statusCtx, _ := gin.CreateTestContext(statusRecorder)
	statusCtx.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/videos/"+submittedVideo.ID,
		nil,
	)
	statusResp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"done":false}`,
		)),
	}

	_, relayErr = adaptor.DoResponse(statusMeta, submitStore, statusCtx, statusResp)
	require.Nil(t, relayErr)

	var video relaymodel.Video
	require.NoError(t, json.Unmarshal(statusRecorder.Body.Bytes(), &video))
	require.Equal(t, relaymodel.VideoStatusInProgress, video.Status)
	require.Equal(t, 50, video.Progress)
	require.Equal(t, 8, video.Seconds)
	require.Equal(t, "1280x720", video.Size)
	require.Equal(t, "make a video", video.Prompt)
}

func TestVideosStatusHandlerTreatsRAIFilteredResponseAsFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	localID := gemini.GeminiVideoLocalIDForTest(operationName)
	reason := "Sorry, we can't create videos with real people's names or likenesses. Please remove the celebrity reference and try again."
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				ID:       coremodel.VideoGenerationStoreID(localID),
				Metadata: operationName,
			},
		},
	}
	m := meta.NewMeta(
		nil,
		mode.VideosGet,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
		meta.WithVideoID(localID),
	)
	m.OriginModel = "veo-3.1-generate-preview"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/videos/"+localID,
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"done":true,"response":{"generateVideoResponse":{"raiMediaFilteredCount":1,"raiMediaFilteredReasons":["` + reason + `"]}}}`,
		)),
	}

	_, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)

	var video relaymodel.Video
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &video))
	require.Equal(t, relaymodel.VideoStatusFailed, video.Status)
	require.Equal(t, 0, video.Progress)
	require.Equal(t, reason, video.Error["message"])
}

func TestConvertVideoRequestInjectsPersonGenerationWhenEnabled(t *testing.T) {
	m := meta.NewMeta(nil, mode.Videos, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(`{"model":"veo","prompt":"make a video"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestWithConfigForTest(
		m,
		req,
		gemini.Config{EnablePersonGenerationAllowAll: true},
	)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video"}],
			"parameters":{"personGeneration":"allow_all"}
		}`,
		string(body),
	)
}

func TestConvertVideoRequestKeepsRequestPersonGeneration(t *testing.T) {
	m := meta.NewMeta(nil, mode.Videos, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/videos",
		strings.NewReader(
			`{"model":"veo","prompt":"make a video","person_generation":"allow_adult"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := gemini.ConvertVideoRequestWithConfigForTest(
		m,
		req,
		gemini.Config{EnablePersonGenerationAllowAll: true},
	)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video"}],
			"parameters":{"personGeneration":"allow_adult"}
		}`,
		string(body),
	)
}

func TestGeminiVideoURLByIDSelectsVariantSuffix(t *testing.T) {
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	operation := &relaymodel.GeminiVideoOperation{
		Response: relaymodel.GeminiVideoOperationResponse{
			GenerateVideoResponse: relaymodel.GeminiGenerateVideoResponse{
				GeneratedSamples: []relaymodel.GeminiGeneratedSample{
					{Video: relaymodel.GeminiGeneratedVideo{URI: "https://example.com/first.mp4"}},
					{Video: relaymodel.GeminiGeneratedVideo{URI: "https://example.com/second.mp4"}},
				},
			},
		},
	}

	require.Equal(
		t,
		"https://example.com/first.mp4",
		gemini.GeminiVideoURLByIDForTest(operation, operationName),
	)
	require.Equal(
		t,
		"https://example.com/second.mp4",
		gemini.GeminiVideoURLByIDForTest(operation, operationName+":1"),
	)
	require.Empty(t, gemini.GeminiVideoURLByIDForTest(operation, operationName+":9"))
}

func TestGeminiVideoOperationResponseExtractsGeneratedSampleURI(t *testing.T) {
	body := `{
		"name": "models/veo-3.1-generate-preview/operations/9p5ug42emn2z",
		"done": true,
		"response": {
			"@type": "type.googleapis.com/google.ai.generativelanguage.v1beta.PredictLongRunningResponse",
			"generateVideoResponse": {
				"generatedSamples": [
					{
						"video": {
							"uri": "https://generativelanguage.googleapis.com/v1beta/files/04pmwg9vlfwb:download?alt=media"
						}
					}
				]
			}
		}
	}`

	var operation relaymodel.GeminiVideoOperation
	require.NoError(t, json.Unmarshal([]byte(body), &operation))

	require.Equal(t, "models/veo-3.1-generate-preview/operations/9p5ug42emn2z", operation.Name)
	require.True(t, operation.Done)
	require.Equal(
		t,
		"https://generativelanguage.googleapis.com/v1beta/files/04pmwg9vlfwb:download?alt=media",
		gemini.GeminiVideoURLByIDForTest(&operation, operation.Name),
	)
}

func TestNativeVideoOperationHandlerRewritesGeneratedFileURI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{}
	m := meta.NewMeta(
		nil,
		mode.GeminiVideoOperations,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithOperationID("3b12t6snoaee"),
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
	)
	m.OriginModel = "veo-3.1-generate-preview"
	m.Channel.ID = 11

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1beta/models/veo-3.1-generate-preview/operations/3b12t6snoaee",
		nil,
	)
	c.Request.Host = "proxy.example.com"
	c.Request.TLS = &tls.ConnectionState{}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"name":"models/veo-3.1-generate-preview/operations/3b12t6snoaee",
			"done":true,
			"response":{
				"@type":"type.googleapis.com/google.ai.generativelanguage.v1beta.PredictLongRunningResponse",
				"generateVideoResponse":{
					"generatedSamples":[
						{"video":{"uri":"https://generativelanguage.googleapis.com/v1beta/files/ef8j32lly0bs:download?alt=media"}}
					]
				}
			}
		}`)),
	}

	_, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)
	require.Len(t, store.saved, 1)
	require.Equal(t, coremodel.GeminiFileStoreID("ef8j32lly0bs"), store.saved[0].ID)
	require.JSONEq(
		t,
		`{
			"name":"models/veo-3.1-generate-preview/operations/3b12t6snoaee",
			"done":true,
			"response":{
				"@type":"type.googleapis.com/google.ai.generativelanguage.v1beta.PredictLongRunningResponse",
				"generateVideoResponse":{
					"generatedSamples":[
						{"video":{"uri":"https://proxy.example.com/v1beta/files/ef8j32lly0bs:download?alt=media"}}
					]
				}
			}
		}`,
		recorder.Body.String(),
	)
	require.JSONEq(
		t,
		`{"uri":"https://generativelanguage.googleapis.com/v1beta/files/ef8j32lly0bs:download?alt=media"}`,
		store.saved[0].Metadata,
	)
}

func TestNativeVideoOperationHandlerUsesDefaultHostForGeneratedFileURI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Setenv("DEFAULT_HOST", "")

	oldDefaultHost := config.GetDefaultHost()

	config.SetDefaultHost("public.example.com")
	defer config.SetDefaultHost(oldDefaultHost)

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{}
	m := meta.NewMeta(
		nil,
		mode.GeminiVideoOperations,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithOperationID("3b12t6snoaee"),
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
	)
	m.OriginModel = "veo-3.1-generate-preview"
	m.Channel.ID = 11

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1beta/models/veo-3.1-generate-preview/operations/3b12t6snoaee",
		nil,
	)
	c.Request.Host = "internal.example.local"
	c.Request.TLS = &tls.ConnectionState{}
	c.Request.Header.Set("X-Forwarded-Host", "forwarded.example.com")
	c.Request.Header.Set("X-Forwarded-Proto", "https")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"name":"models/veo-3.1-generate-preview/operations/3b12t6snoaee",
			"done":true,
			"response":{
				"generateVideoResponse":{
					"generatedSamples":[
						{"video":{"uri":"https://generativelanguage.googleapis.com/v1beta/files/ef8j32lly0bs:download?alt=media"}}
					]
				}
			}
		}`)),
	}

	_, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)

	require.Contains(
		t,
		recorder.Body.String(),
		`https://public.example.com/v1beta/files/ef8j32lly0bs:download?alt=media`,
	)
}

func TestNativeVideoOperationHandlerIgnoresForwardedHostWithoutDefaultHost(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Setenv("DEFAULT_HOST", "")

	oldDefaultHost := config.GetDefaultHost()

	config.SetDefaultHost("")
	defer config.SetDefaultHost(oldDefaultHost)

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{}
	m := meta.NewMeta(
		nil,
		mode.GeminiVideoOperations,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithOperationID("3b12t6snoaee"),
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
	)
	m.OriginModel = "veo-3.1-generate-preview"
	m.Channel.ID = 11

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1beta/models/veo-3.1-generate-preview/operations/3b12t6snoaee",
		nil,
	)
	c.Request.Host = "proxy.example.com"
	c.Request.Header.Set("X-Forwarded-Host", "attacker.example.com")
	c.Request.Header.Set("X-Forwarded-Proto", "https")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"name":"models/veo-3.1-generate-preview/operations/3b12t6snoaee",
			"done":true,
			"response":{
				"generateVideoResponse":{
					"generatedSamples":[
						{"video":{"uri":"https://generativelanguage.googleapis.com/v1beta/files/ef8j32lly0bs:download?alt=media"}}
					]
				}
			}
		}`)),
	}

	_, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)

	require.Contains(
		t,
		recorder.Body.String(),
		`http://proxy.example.com/v1beta/files/ef8j32lly0bs:download?alt=media`,
	)
	require.NotContains(t, recorder.Body.String(), "attacker.example.com")
}

func TestGeminiFileModeDownloadsStoredFileWithAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/files/ef8j32lly0bs:download", r.URL.Path)
		require.Equal(t, "media", r.URL.Query().Get("alt"))
		require.Equal(t, "apikey", r.Header.Get("X-Goog-Api-Key"))

		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "10")
		_, _ = w.Write([]byte("video-data"))
	}))
	defer fileServer.Close()

	adaptor := &gemini.Adaptor{}
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				GroupID:   "group-1",
				TokenID:   7,
				ChannelID: 11,
				Model:     "veo-3.1-generate-preview",
				ID:        coremodel.GeminiFileStoreID("ef8j32lly0bs"),
				Metadata:  `{"uri":"` + fileServer.URL + `/v1beta/files/ef8j32lly0bs:download?alt=media"}`,
				ExpiresAt: time.Now().Add(time.Hour),
			},
		},
	}
	m := meta.NewMeta(
		&coremodel.Channel{Key: "apikey"},
		mode.GeminiFiles,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithFileID("ef8j32lly0bs"),
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
	)

	reqURL, err := adaptor.GetRequestURL(m, store, nil)
	require.NoError(t, err)
	require.Equal(t, fileServer.URL+"/v1beta/files/ef8j32lly0bs:download?alt=media", reqURL.URL)

	req, err := http.NewRequestWithContext(t.Context(), reqURL.Method, reqURL.URL, nil)
	require.NoError(t, err)
	require.NoError(t, adaptor.SetupRequestHeader(m, store, nil, req))

	resp, err := adaptor.DoRequest(m, store, nil, req)
	require.NoError(t, err)

	defer resp.Body.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1beta/files/ef8j32lly0bs:download?alt=media",
		nil,
	)

	result, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)
	require.Equal(t, "ef8j32lly0bs", result.UpstreamID)
	require.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	require.Equal(t, "video-data", recorder.Body.String())
}

func TestFetchAsyncUsageBuildsFinalUsageFromStoredMetadata(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			"/v1beta/models/veo-3.1-generate-preview/operations/video-123",
			r.URL.Path,
		)
		require.Equal(t, "apikey", r.Header.Get("X-Goog-Api-Key"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"models/veo-3.1-generate-preview/operations/video-123",
			"done":true,
			"response":{
				"generateVideoResponse":{
					"generatedSamples":[{"video":{"uri":"https://example.com/video.mp4"}}]
				},
				"usageMetadata":{
					"promptTokenCount":1,
					"candidatesTokenCount":999,
					"totalTokenCount":1000,
					"promptTokensDetails":[],
					"candidatesTokensDetails":[]
				}
			}
		}`))
	}))
	defer ts.Close()

	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	localID := gemini.GeminiVideoLocalIDForTest(operationName)
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				GroupID:   "group-1",
				TokenID:   7,
				ChannelID: 11,
				Model:     "veo-3.1-generate-preview",
				ID:        coremodel.VideoJobStoreID(localID),
				Metadata:  `{"operation_name":"models/veo-3.1-generate-preview/operations/video-123","seconds":6,"variants":2,"resolution":"1080p","width":1920,"height":1080}`,
				ExpiresAt: time.Now().Add(time.Hour),
			},
		},
	}

	adaptor := &gemini.Adaptor{}
	usage, usageContext, done, err := adaptor.FetchAsyncUsage(
		t.Context(),
		adaptorapi.AsyncUsageRequest{
			Channel: &coremodel.Channel{Key: "apikey", BaseURL: ts.URL},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				Model:      "veo-3.1-generate-preview",
				BaseURL:    ts.URL,
				GroupID:    "group-1",
				TokenID:    7,
				UpstreamID: operationName,
				Usage: coremodel.Usage{
					OutputTokens: 99,
					TotalTokens:  99,
				},
				UsageContext: coremodel.UsageContext{
					Resolution:       "1920x1080",
					NativeResolution: "720p",
				},
			},
			Store: store,
		},
	)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, coremodel.ZeroNullInt64(12), usage.OutputTokens)
	require.Equal(t, coremodel.ZeroNullInt64(12), usage.TotalTokens)
	require.Equal(t, "1920x1080", usageContext.Resolution)
	require.Equal(t, "1080p", usageContext.NativeResolution)
}

func TestFetchAsyncUsageTreatsRAIFilteredResponseAsCompletedFailure(t *testing.T) {
	reason := "Sorry, we can't create videos with real people's names or likenesses."

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			"/v1beta/models/veo-3.1-generate-preview/operations/video-123",
			r.URL.Path,
		)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"models/veo-3.1-generate-preview/operations/video-123",
			"done":true,
			"response":{
				"generateVideoResponse":{
					"raiMediaFilteredCount":1,
					"raiMediaFilteredReasons":["` + reason + `"]
				}
			}
		}`))
	}))
	defer ts.Close()

	adaptor := &gemini.Adaptor{}
	usage, usageContext, done, err := adaptor.FetchAsyncUsage(
		t.Context(),
		adaptorapi.AsyncUsageRequest{
			Channel: &coremodel.Channel{Key: "apikey", BaseURL: ts.URL},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				Model:      "veo-3.1-generate-preview",
				BaseURL:    ts.URL,
				UpstreamID: "models/veo-3.1-generate-preview/operations/video-123",
			},
		},
	)
	require.ErrorContains(t, err, reason)
	require.True(t, done)
	require.Zero(t, usage)
	require.Zero(t, usageContext)
}

func TestFetchAsyncUsageTreatsPartialRAIFilteredResponseAsSuccess(t *testing.T) {
	reason := "One requested video was filtered by responsible AI policy."

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			"/v1beta/models/veo-3.1-generate-preview/operations/video-123",
			r.URL.Path,
		)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"models/veo-3.1-generate-preview/operations/video-123",
			"done":true,
			"response":{
				"generateVideoResponse":{
					"generatedSamples":[{"video":{"uri":"https://example.com/video.mp4"}}],
					"raiMediaFilteredCount":1,
					"raiMediaFilteredReasons":["` + reason + `"]
				}
			}
		}`))
	}))
	defer ts.Close()

	adaptor := &gemini.Adaptor{}
	usage, _, done, err := adaptor.FetchAsyncUsage(
		t.Context(),
		adaptorapi.AsyncUsageRequest{
			Channel: &coremodel.Channel{Key: "apikey", BaseURL: ts.URL},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				Model:      "veo-3.1-generate-preview",
				BaseURL:    ts.URL,
				UpstreamID: "models/veo-3.1-generate-preview/operations/video-123",
			},
		},
	)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, coremodel.ZeroNullInt64(8), usage.OutputTokens)
	require.Equal(t, coremodel.ZeroNullInt64(8), usage.TotalTokens)
}

func TestFetchAsyncUsageTreatsHTTP200GeminiErrorAsCompletedFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			"/v1beta/models/veo-3.1-generate-preview/operations/video-123",
			r.URL.Path,
		)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"error":{
				"code":400,
				"message":"Requested duration is not supported for this model.",
				"status":"INVALID_ARGUMENT"
			}
		}`))
	}))
	defer ts.Close()

	adaptor := &gemini.Adaptor{}
	usage, usageContext, done, err := adaptor.FetchAsyncUsage(
		t.Context(),
		adaptorapi.AsyncUsageRequest{
			Channel: &coremodel.Channel{Key: "apikey", BaseURL: ts.URL},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				Model:      "veo-3.1-generate-preview",
				BaseURL:    ts.URL,
				UpstreamID: "models/veo-3.1-generate-preview/operations/video-123",
			},
		},
	)
	require.ErrorContains(t, err, "Requested duration is not supported for this model.")
	require.True(t, done)
	require.Zero(t, usage)
	require.Zero(t, usageContext)
}

func TestFetchAsyncUsageNativeGeminiVideoUsesNativeStoreID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			"/v1beta/models/veo-3.1-generate-preview/operations/video-123",
			r.URL.Path,
		)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"models/veo-3.1-generate-preview/operations/video-123",
			"done":true,
			"response":{
				"generateVideoResponse":{
					"generatedSamples":[
						{"video":{"uri":"https://example.com/one.mp4"}},
						{"video":{"uri":"https://example.com/two.mp4"}}
					]
				}
			}
		}`))
	}))
	defer ts.Close()

	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	store := &geminiVideoTestStore{
		saved: []adaptorapi.StoreCache{
			{
				GroupID:   "group-1",
				TokenID:   7,
				ChannelID: 11,
				Model:     "veo-3.1-generate-preview",
				ID:        coremodel.VideoJobStoreID("video-123"),
				Metadata:  `{"operation_name":"models/veo-3.1-generate-preview/operations/video-123","seconds":5,"variants":2,"resolution":"1080p"}`,
				ExpiresAt: time.Now().Add(time.Hour),
			},
		},
	}

	adaptor := &gemini.Adaptor{}
	usage, usageContext, done, err := adaptor.FetchAsyncUsage(
		t.Context(),
		adaptorapi.AsyncUsageRequest{
			Channel: &coremodel.Channel{Key: "apikey", BaseURL: ts.URL},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.GeminiVideo),
				Model:      "veo-3.1-generate-preview",
				BaseURL:    ts.URL,
				GroupID:    "group-1",
				TokenID:    7,
				UpstreamID: operationName,
				UsageContext: coremodel.UsageContext{
					Resolution:       "720p",
					NativeResolution: "720p",
				},
			},
			Store: store,
		},
	)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, coremodel.ZeroNullInt64(10), usage.OutputTokens)
	require.Equal(t, coremodel.ZeroNullInt64(10), usage.TotalTokens)
	require.Equal(t, "1080p", usageContext.Resolution)
	require.Equal(t, "1080p", usageContext.NativeResolution)
}
