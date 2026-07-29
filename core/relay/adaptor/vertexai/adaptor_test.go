package vertexai_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	coremodel "github.com/labring/aiproxy/core/model"
	adaptorapi "github.com/labring/aiproxy/core/relay/adaptor"
	vertexai "github.com/labring/aiproxy/core/relay/adaptor/vertexai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type vertexVideoTestStore struct {
	items map[string]adaptorapi.StoreCache
}

func (s *vertexVideoTestStore) getStore(
	id string,
) (adaptorapi.StoreCache, error) {
	if item, ok := s.items[id]; ok {
		return item, nil
	}

	return adaptorapi.StoreCache{}, coremodel.NotFoundError(coremodel.ErrStoreNotFound)
}

func (s *vertexVideoTestStore) GetStoreByScope(
	group string,
	tokenID int,
	id string,
	_ coremodel.ChannelScope,
) (adaptorapi.StoreCache, error) {
	return s.getStore(id)
}

func (s *vertexVideoTestStore) SaveStore(
	adaptorapi.StoreCache,
	coremodel.ChannelScope,
) error {
	return nil
}

func (s *vertexVideoTestStore) SaveStoreWithOption(
	adaptorapi.StoreCache,
	coremodel.ChannelScope,
	adaptorapi.SaveStoreOption,
) error {
	return nil
}

func (s *vertexVideoTestStore) SaveIfNotExistStore(
	adaptorapi.StoreCache,
	coremodel.ChannelScope,
) error {
	return nil
}

func vertexGeminiVideoLocalIDForTest(operationName string) string {
	sum := sha256.Sum256([]byte(operationName))
	return "gemini_op_" + hex.EncodeToString(sum[:])
}

func TestGetRequestURL_UsesOriginModelNameForRouting(t *testing.T) {
	adaptor := &vertexai.Adaptor{}

	t.Run("origin gemini chooses google publisher endpoint", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.ChatCompletions, "gemini-2.5-pro", coremodel.ModelConfig{})
		m.ActualModel = "mapped-model"
		m.Channel.Key = "us-central1|apikey"
		m.Set("stream", false)

		reqURL, err := adaptor.GetRequestURL(m, nil, nil)
		require.NoError(t, err)
		assert.Equal(
			t,
			http.MethodPost,
			reqURL.Method,
		)
		assert.Contains(
			t,
			reqURL.URL,
			"/publishers/google/models/mapped-model:generateContent",
		)
	})

	t.Run("origin claude chooses anthropic publisher endpoint", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.ChatCompletions, "claude-sonnet-4-5", coremodel.ModelConfig{})
		m.ActualModel = "mapped-model"
		m.Channel.Key = "us-central1|apikey"
		m.Set("stream", true)

		reqURL, err := adaptor.GetRequestURL(m, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, reqURL.Method)
		assert.Contains(
			t,
			reqURL.URL,
			"/publishers/anthropic/models/mapped-model:streamRawPredict?alt=sse",
		)
	})
}

func TestGetRequestURLGeminiVideoUsesPredictLongRunning(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	m := meta.NewMeta(nil, mode.GeminiVideo, "veo-3.1-generate-preview", coremodel.ModelConfig{})
	m.Channel.Key = "us-central1|project-1|apikey"

	reqURL, err := adaptor.GetRequestURL(m, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, reqURL.Method)
	assert.Contains(
		t,
		reqURL.URL,
		"/projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview:predictLongRunning",
	)
}

func TestGetRequestURLGeminiVideoOperationUsesGet(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.GeminiVideoOperations,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithOperationID("video-123"),
	)
	m.Channel.Key = "us-central1|project-1|apikey"

	reqURL, err := adaptor.GetRequestURL(m, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, reqURL.Method)
	assert.Contains(
		t,
		reqURL.URL,
		"/projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
	)
}

func TestGetRequestURLGeminiVideoOperationUsesStoredFullVertexOperation(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	store := &vertexVideoTestStore{
		items: map[string]adaptorapi.StoreCache{
			coremodel.VideoJobStoreID("video-123"): {
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
	m.Channel.Key = "us-central1|project-1|apikey"

	reqURL, err := adaptor.GetRequestURL(m, store, nil)
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, reqURL.Method)
	assert.Equal(
		t,
		"https://us-central1-aiplatform.googleapis.com/v1/projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
		reqURL.URL,
	)
}

func TestGetRequestURLGeminiVideoGenerationContentUsesOperationID(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	operationName := "models/veo-3.1-generate-preview/operations/video-123"
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsContent,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
		meta.WithGenerationID(operationName+":1"),
	)
	m.Channel.Key = "us-central1|project-1|apikey"

	reqURL, err := adaptor.GetRequestURL(m, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, reqURL.Method)
	assert.Contains(
		t,
		reqURL.URL,
		"/projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
	)
}

func TestGetRequestURLVideoGenerationGetJobsRestoresLongVertexOperation(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	operationName := "projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/1234567890abcdef1234567890abcdef"
	localID := "gemini_op_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	store := &vertexVideoTestStore{
		items: map[string]adaptorapi.StoreCache{
			coremodel.VideoJobStoreID(localID): {
				ID:       coremodel.VideoJobStoreID(localID),
				Model:    "veo-3.1-generate-preview",
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
	m.Channel.Key = "us-central1|project-1|apikey"

	reqURL, err := adaptor.GetRequestURL(m, store, nil)
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, reqURL.Method)
	assert.Equal(
		t,
		"https://us-central1-aiplatform.googleapis.com/v1/"+operationName,
		reqURL.URL,
	)
	assert.LessOrEqual(t, len(coremodel.VideoJobStoreID(localID)), 128)
}

func TestConvertRequestVeoVideoUsesGeminiInnerAdaptor(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	m.Channel.Type = coremodel.ChannelTypeVertexAI
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		strings.NewReader(`{"model":"veo","prompt":"make a video","n_seconds":5}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video"}],
			"parameters":{"durationSeconds":5,"sampleCount":1}
		}`,
		string(body),
	)
}

func TestConvertRequestNativeGeminiVideoMapsNumberOfVideosToVertexSampleCount(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.GeminiVideo,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{},
	)
	m.Channel.Type = coremodel.ChannelTypeVertexAI
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/veo-3.1-generate-preview:predictLongRunning",
		strings.NewReader(
			`{"instances":[{"prompt":"make a video"}],"parameters":{"durationSeconds":5,"numberOfVideos":2}}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"make a video"}],
			"parameters":{"durationSeconds":5,"sampleCount":2}
		}`,
		string(body),
	)
}

func TestSetupRequestHeaderVeoVideoUsesGeminiInnerAdaptor(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.VideoGenerationsJobs,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{Type: mode.GeminiVideo},
	)
	m.Channel.Key = "us-central1|project-1|apikey"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://example.com/v1/video/generations/jobs",
		nil,
	)
	require.NoError(t, err)

	err = adaptor.SetupRequestHeader(m, nil, nil, req)
	require.NoError(t, err)
	assert.Equal(t, "apikey", req.Header.Get("X-Goog-Api-Key"))
}

func TestGeminiFileModeUsesGeminiInnerAdaptor(t *testing.T) {
	adaptor := &vertexai.Adaptor{}

	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/files/ef8j32lly0bs:download", r.URL.Path)
		require.Equal(t, "media", r.URL.Query().Get("alt"))

		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("video-data"))
	}))
	defer fileServer.Close()

	store := &vertexVideoTestStore{
		items: map[string]adaptorapi.StoreCache{
			coremodel.GeminiFileStoreID("ef8j32lly0bs"): {
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
		&coremodel.Channel{Key: "us-central1|project-1|apikey"},
		mode.GeminiFiles,
		"veo-3.1-generate-preview",
		coremodel.ModelConfig{Type: mode.GeminiVideo},
		meta.WithFileID("ef8j32lly0bs"),
		meta.WithGroup(coremodel.GroupCache{ID: "group-1"}),
		meta.WithToken(coremodel.TokenCache{ID: 7}),
	)

	require.True(t, adaptor.SupportMode(m))

	reqURL, err := adaptor.GetRequestURL(m, store, nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, reqURL.Method)
	require.Equal(t, fileServer.URL+"/v1beta/files/ef8j32lly0bs:download?alt=media", reqURL.URL)

	result, err := adaptor.ConvertRequest(
		m,
		store,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, reqURL.URL, nil),
	)
	require.NoError(t, err)
	require.Nil(t, result.Body)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"video/mp4"}},
		Body:       io.NopCloser(strings.NewReader("video-data")),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1beta/files/ef8j32lly0bs:download?alt=media",
		nil,
	)

	responseResult, relayErr := adaptor.DoResponse(m, store, c, resp)
	require.Nil(t, relayErr)
	require.Equal(t, "ef8j32lly0bs", responseResult.UpstreamID)
	require.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	require.Equal(t, "video-data", recorder.Body.String())
}

func TestConvertRequestGeminiImageUsesGeminiInnerAdaptor(t *testing.T) {
	adaptor := &vertexai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ImagesGenerations,
		"gemini-3-pro-image-preview",
		coremodel.ModelConfig{Type: mode.GeminiImage},
	)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/generations",
		strings.NewReader(
			`{"model":"gemini-3-pro-image-preview","prompt":"draw a cat","size":"1024x1024"}`,
		),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
				"contents":[{"role":"user","parts":[{"text":"draw a cat"}]}],
				"generationConfig":{
					"responseModalities":["IMAGE"],
					"imageConfig":{"aspectRatio":"1:1","imageSize":"1K"}
				}
			}`,
		string(body),
	)
}

func TestFetchAsyncUsageGeminiVideoBuildsUsageFromStoredMetadata(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			"/v1/projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
			r.URL.Path,
		)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
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

	operationName := "projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123"
	localID := vertexGeminiVideoLocalIDForTest(operationName)
	store := &vertexVideoTestStore{
		items: map[string]adaptorapi.StoreCache{
			coremodel.VideoJobStoreID(localID): {
				GroupID:   "group-1",
				TokenID:   7,
				ChannelID: 11,
				Model:     "veo-3.1-generate-preview",
				ID:        coremodel.VideoJobStoreID(localID),
				Metadata:  `{"operation_name":"projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123","seconds":5,"variants":2,"resolution":"1080p"}`,
				ExpiresAt: time.Now().Add(time.Hour),
			},
		},
	}

	adaptor := &vertexai.Adaptor{}
	usage, usageContext, done, err := adaptor.FetchAsyncUsage(
		t.Context(),
		adaptorapi.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				Key:     "us-central1|project-1|apikey",
				BaseURL: ts.URL,
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				Model:      "veo-3.1-generate-preview",
				BaseURL:    ts.URL,
				GroupID:    "group-1",
				TokenID:    7,
				UpstreamID: operationName,
				UsageContext: coremodel.UsageContext{
					Resolution:       "1280x720",
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
	require.Equal(t, "1280x720", usageContext.Resolution)
	require.Equal(t, "1080p", usageContext.NativeResolution)
}

func TestFetchAsyncUsageGeminiVideoTreatsHTTP200ErrorAsCompletedFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			"/v1/projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
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

	adaptor := &vertexai.Adaptor{}
	usage, usageContext, done, err := adaptor.FetchAsyncUsage(
		t.Context(),
		adaptorapi.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				Key:     "us-central1|project-1|apikey",
				BaseURL: ts.URL,
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				Model:      "veo-3.1-generate-preview",
				BaseURL:    ts.URL,
				UpstreamID: "projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
			},
		},
	)
	require.ErrorContains(t, err, "Requested duration is not supported for this model.")
	require.True(t, done)
	require.Zero(t, usage)
	require.Zero(t, usageContext)
}

func TestFetchAsyncUsageGeminiVideoTreatsPartialRAIFilterAsSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			"/v1/projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
			r.URL.Path,
		)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
			"done":true,
			"response":{
				"generateVideoResponse":{
					"generatedSamples":[{"video":{"uri":"https://example.com/video.mp4"}}],
					"raiMediaFilteredCount":1,
					"raiMediaFilteredReasons":["one requested video was filtered"]
				}
			}
		}`))
	}))
	defer ts.Close()

	adaptor := &vertexai.Adaptor{}
	usage, _, done, err := adaptor.FetchAsyncUsage(
		t.Context(),
		adaptorapi.AsyncUsageRequest{
			Channel: &coremodel.Channel{
				Key:     "us-central1|project-1|apikey",
				BaseURL: ts.URL,
			},
			Info: &coremodel.AsyncUsageInfo{
				Mode:       int(mode.VideoGenerationsJobs),
				Model:      "veo-3.1-generate-preview",
				BaseURL:    ts.URL,
				UpstreamID: "projects/project-1/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/video-123",
			},
		},
	)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, coremodel.ZeroNullInt64(8), usage.OutputTokens)
	require.Equal(t, coremodel.ZeroNullInt64(8), usage.TotalTokens)
}
