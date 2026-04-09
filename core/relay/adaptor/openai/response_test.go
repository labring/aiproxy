package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type responseTestStore struct {
	saved           []adaptor.StoreCache
	savedIfNotExist []adaptor.StoreCache
}

func (s *responseTestStore) GetStore(string, int, string) (adaptor.StoreCache, error) {
	return adaptor.StoreCache{}, nil
}

func (s *responseTestStore) SaveStore(cache adaptor.StoreCache) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *responseTestStore) SaveIfNotExistStore(cache adaptor.StoreCache) error {
	s.savedIfNotExist = append(s.savedIfNotExist, cache)
	return nil
}

func TestGetPromptCacheStoreTTL(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 24*time.Hour, getPromptCacheStoreTTL("24h"))
	assert.Equal(t, defaultPromptCacheStoreTTL, getPromptCacheStoreTTL(""))
	assert.Equal(t, defaultPromptCacheStoreTTL, getPromptCacheStoreTTL("in-memory"))
	assert.Equal(t, defaultPromptCacheStoreTTL, getPromptCacheStoreTTL("invalid"))
}

func TestResponseHandlerPromptCacheRetention(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		body      string
		expectTTL time.Duration
	}{
		{
			name:      "default retention when upstream does not return prompt_cache_retention",
			body:      `{"id":"resp_123","object":"response","created_at":1,"status":"completed","model":"gpt-5","output":[],"parallel_tool_calls":true,"store":false}`,
			expectTTL: defaultPromptCacheStoreTTL,
		},
		{
			name:      "custom retention from upstream response",
			body:      `{"id":"resp_123","object":"response","created_at":1,"status":"completed","model":"gpt-5","output":[],"parallel_tool_calls":true,"store":false,"prompt_cache_retention":"24h"}`,
			expectTTL: 24 * time.Hour,
		},
		{
			name:      "invalid retention falls back to default",
			body:      `{"id":"resp_123","object":"response","created_at":1,"status":"completed","model":"gpt-5","output":[],"parallel_tool_calls":true,"store":false,"prompt_cache_retention":"bad-value"}`,
			expectTTL: defaultPromptCacheStoreTTL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			store := &responseTestStore{}
			meta := &meta.Meta{
				OriginModel:    "gpt-5",
				ActualModel:    "gpt-5",
				PromptCacheKey: "cache-key",
				Group:          model.GroupCache{ID: "group-1"},
				Token:          model.TokenCache{ID: 7},
				Channel:        meta.ChannelMeta{ID: 9},
			}
			resp := &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(bytes.NewBufferString(tt.body)),
				Header:     make(http.Header),
			}

			start := time.Now()
			_, err := ResponseHandler(meta, store, c, resp)
			end := time.Now()
			require.Nil(t, err)
			require.Len(t, store.savedIfNotExist, 1)

			assertPromptCacheExpiryInWindow(t, store.savedIfNotExist[0].ExpiresAt, start, end, tt.expectTTL)
			assert.Equal(
				t,
				model.PromptCacheStoreID("gpt-5", "cache-key"),
				store.savedIfNotExist[0].ID,
			)
		})
	}
}

func TestResponseStreamHandlerPromptCacheRetention(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	store := &responseTestStore{}
	meta := &meta.Meta{
		OriginModel:    "gpt-5",
		ActualModel:    "gpt-5",
		PromptCacheKey: "cache-key",
		Group:          model.GroupCache{ID: "group-1"},
		Token:          model.TokenCache{ID: 7},
		Channel:        meta.ChannelMeta{ID: 9},
	}

	body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_123\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"prompt_cache_retention\":\"24h\"}}\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	start := time.Now()
	_, err := ResponseStreamHandler(meta, store, c, resp)
	end := time.Now()
	require.Nil(t, err)
	require.Len(t, store.savedIfNotExist, 1)

	assertPromptCacheExpiryInWindow(t, store.savedIfNotExist[0].ExpiresAt, start, end, 24*time.Hour)
}

func assertPromptCacheExpiryInWindow(
	t *testing.T,
	expiresAt time.Time,
	start time.Time,
	end time.Time,
	ttl time.Duration,
) {
	t.Helper()

	assert.True(t, expiresAt.After(start.Add(ttl-time.Second)))
	assert.True(t, expiresAt.Before(end.Add(ttl+time.Second)))
}
