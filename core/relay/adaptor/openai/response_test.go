//nolint:testpackage
package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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

func (s *responseTestStore) SaveStoreWithOption(
	cache adaptor.StoreCache,
	_ adaptor.SaveStoreOption,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *responseTestStore) SaveIfNotExistStore(cache adaptor.StoreCache) error {
	s.savedIfNotExist = append(s.savedIfNotExist, cache)
	return nil
}

func TestResponseHandlerPromptCacheRetention(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		body              string
		expectStoreWrites int
	}{
		{
			name:              "empty retention when upstream does not return prompt_cache_retention",
			body:              `{"id":"resp_123","object":"response","created_at":1,"status":"completed","model":"gpt-5","output":[],"parallel_tool_calls":true,"store":false}`,
			expectStoreWrites: 0,
		},
		{
			name:              "custom retention from upstream response",
			body:              `{"id":"resp_123","object":"response","created_at":1,"status":"completed","model":"gpt-5","output":[],"parallel_tool_calls":true,"store":false,"prompt_cache_retention":"24h"}`,
			expectStoreWrites: 0,
		},
		{
			name:              "invalid retention is still passed through to plugin layer",
			body:              `{"id":"resp_123","object":"response","created_at":1,"status":"completed","model":"gpt-5","output":[],"parallel_tool_calls":true,"store":false,"prompt_cache_retention":"bad-value"}`,
			expectStoreWrites: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/responses",
				nil,
			)
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

			result, err := ResponseHandler(meta, store, c, resp)
			require.Nil(t, err)
			require.Len(t, store.savedIfNotExist, tt.expectStoreWrites)
			assert.Equal(t, "resp_123", result.UpstreamID)
		})
	}
}

func TestResponseStreamHandlerPromptCacheRetention(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/responses",
		nil,
	)
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

	result, err := ResponseStreamHandler(meta, store, c, resp)
	require.Nil(t, err)
	require.Empty(t, store.savedIfNotExist)
	assert.Equal(t, "resp_123", result.UpstreamID)
}
