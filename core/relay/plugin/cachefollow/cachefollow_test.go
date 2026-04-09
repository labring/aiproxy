//nolint:testpackage
package cachefollow

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingStore struct {
	stores          map[string]adaptor.StoreCache
	saved           []adaptor.StoreCache
	savedIfNotExist []adaptor.StoreCache
}

func (s *recordingStore) GetStore(_ string, _ int, id string) (adaptor.StoreCache, error) {
	if s.stores == nil {
		return adaptor.StoreCache{}, model.NotFoundError(model.ErrStoreNotFound)
	}

	store, ok := s.stores[id]
	if !ok {
		return adaptor.StoreCache{}, model.NotFoundError(model.ErrStoreNotFound)
	}

	return store, nil
}

func (s *recordingStore) SaveStore(cache adaptor.StoreCache) error {
	if s.stores == nil {
		s.stores = make(map[string]adaptor.StoreCache)
	}

	s.stores[cache.ID] = cache
	s.saved = append(s.saved, cache)

	return nil
}

func (s *recordingStore) SaveStoreWithOption(
	cache adaptor.StoreCache,
	opt adaptor.SaveStoreOption,
) error {
	if existing, ok := s.stores[cache.ID]; ok &&
		opt.MinUpdateInterval > 0 &&
		!existing.UpdatedAt.IsZero() &&
		time.Since(existing.UpdatedAt) < opt.MinUpdateInterval {
		return nil
	}

	return s.SaveStore(cache)
}

func (s *recordingStore) SaveIfNotExistStore(cache adaptor.StoreCache) error {
	if s.stores == nil {
		s.stores = make(map[string]adaptor.StoreCache)
	}

	if _, ok := s.stores[cache.ID]; ok {
		return nil
	}

	s.stores[cache.ID] = cache
	s.savedIfNotExist = append(s.savedIfNotExist, cache)

	return nil
}

type doResponseFunc struct {
	fn func(*meta.Meta, adaptor.Store, *gin.Context, *http.Response) (adaptor.DoResponseResult, adaptor.Error)
}

func (d doResponseFunc) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return d.fn(meta, store, c, resp)
}

func TestDoResponseRecordsPromptCacheStoreOnly(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/responses", nil)

	store := &recordingStore{}
	requestMeta := &meta.Meta{
		Mode:           mode.Responses,
		OriginModel:    "gpt-5",
		PromptCacheKey: "cache-key",
		ModelConfig: model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				PluginName: {"enable": true},
			},
		},
		Group:   model.GroupCache{ID: "group-1"},
		Token:   model.TokenCache{ID: 7},
		Channel: meta.ChannelMeta{ID: 9},
	}

	start := time.Now()
	result, relayErr := (&Plugin{}).DoResponse(
		requestMeta,
		store,
		c,
		&http.Response{StatusCode: http.StatusOK},
		doResponseFunc{
			fn: func(_ *meta.Meta, _ adaptor.Store, c *gin.Context, _ *http.Response) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				_, _ = c.Writer.Write(
					[]byte(
						`data: {"type":"response.created","response":{"id":"resp_123","object":"response","created_at":1,"status":"in_progress","model":"gpt-5","output":[],"parallel_tool_calls":true,"store":false,"prompt_cache_retention":"24h"}}` + "\n\n",
					),
				)
				_, _ = c.Writer.Write(
					[]byte(
						`data: {"type":"response.completed","response":{"id":"resp_123","object":"response","created_at":1,"status":"completed","model":"gpt-5","output":[],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11,"input_tokens_details":{"cached_tokens":6}}}}` + "\n\n",
					),
				)

				return adaptor.DoResponseResult{
					Usage: model.Usage{CachedTokens: 6},
				}, nil
			},
		},
	)
	end := time.Now()

	require.Nil(t, relayErr)
	assert.Equal(t, int64(6), int64(result.Usage.CachedTokens))
	require.Len(t, store.savedIfNotExist, 1)
	require.Len(t, store.saved, 1)
	assert.Equal(
		t,
		model.PromptCacheStoreID("gpt-5", "cache-key", model.CacheKeyTypeStable),
		store.savedIfNotExist[0].ID,
	)
	assert.Equal(
		t,
		model.PromptCacheStoreID("gpt-5", "cache-key", model.CacheKeyTypeLast),
		store.saved[0].ID,
	)
	assert.True(t, store.savedIfNotExist[0].ExpiresAt.After(start.Add(24*time.Hour-time.Second)))
	assert.True(t, store.savedIfNotExist[0].ExpiresAt.Before(end.Add(24*time.Hour+time.Second)))
	assert.True(t, store.saved[0].ExpiresAt.After(start.Add(24*time.Hour-time.Second)))
	assert.True(t, store.saved[0].ExpiresAt.Before(end.Add(24*time.Hour+time.Second)))
}

func TestDoResponseRecordsCacheFollowWhenPromptCacheKeyAbsent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	store := &recordingStore{}
	requestMeta := &meta.Meta{
		Mode:        mode.ChatCompletions,
		OriginModel: "gpt-5",
		ModelConfig: model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				PluginName: {"enable": true},
			},
		},
		Group:   model.GroupCache{ID: "group-1"},
		Token:   model.TokenCache{ID: 7},
		Channel: meta.ChannelMeta{ID: 9},
	}

	start := time.Now()
	_, relayErr := (&Plugin{}).DoResponse(
		requestMeta,
		store,
		c,
		&http.Response{StatusCode: http.StatusOK},
		doResponseFunc{
			fn: func(_ *meta.Meta, _ adaptor.Store, c *gin.Context, _ *http.Response) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				_, _ = c.Writer.Write([]byte(`{"id":"chatcmpl-1"}`))

				return adaptor.DoResponseResult{
					Usage: model.Usage{CachedTokens: 4},
				}, nil
			},
		},
	)
	end := time.Now()

	require.Nil(t, relayErr)
	require.Len(t, store.savedIfNotExist, 1)
	require.Len(t, store.saved, 1)
	assert.Equal(
		t,
		model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
		store.savedIfNotExist[0].ID,
	)
	assert.Equal(
		t,
		model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeLast),
		store.saved[0].ID,
	)
	assert.True(t, store.savedIfNotExist[0].ExpiresAt.After(start.Add(defaultStoreTTL-time.Second)))
	assert.True(t, store.savedIfNotExist[0].ExpiresAt.Before(end.Add(defaultStoreTTL+time.Second)))
	assert.True(t, store.saved[0].ExpiresAt.After(start.Add(defaultStoreTTL-time.Second)))
	assert.True(t, store.saved[0].ExpiresAt.Before(end.Add(defaultStoreTTL+time.Second)))
}

func TestDoResponseRecordsPromptCacheStoreForChatCompletions(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	store := &recordingStore{}
	requestMeta := &meta.Meta{
		Mode:           mode.ChatCompletions,
		OriginModel:    "gpt-5",
		PromptCacheKey: "cache-key",
		ModelConfig: model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				PluginName: {"enable": true},
			},
		},
		Group:   model.GroupCache{ID: "group-1"},
		Token:   model.TokenCache{ID: 7},
		Channel: meta.ChannelMeta{ID: 9},
	}

	start := time.Now()
	_, relayErr := (&Plugin{}).DoResponse(
		requestMeta,
		store,
		c,
		&http.Response{StatusCode: http.StatusOK},
		doResponseFunc{
			fn: func(_ *meta.Meta, _ adaptor.Store, c *gin.Context, _ *http.Response) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				_, _ = c.Writer.Write([]byte(`{"id":"chatcmpl-1"}`))

				return adaptor.DoResponseResult{
					Usage: model.Usage{CachedTokens: 4},
				}, nil
			},
		},
	)
	end := time.Now()

	require.Nil(t, relayErr)
	require.Len(t, store.savedIfNotExist, 1)
	require.Len(t, store.saved, 1)
	assert.Equal(
		t,
		model.PromptCacheStoreID("gpt-5", "cache-key", model.CacheKeyTypeStable),
		store.savedIfNotExist[0].ID,
	)
	assert.Equal(
		t,
		model.PromptCacheStoreID("gpt-5", "cache-key", model.CacheKeyTypeLast),
		store.saved[0].ID,
	)
	assert.True(t, store.savedIfNotExist[0].ExpiresAt.After(start.Add(defaultStoreTTL-time.Second)))
	assert.True(t, store.savedIfNotExist[0].ExpiresAt.Before(end.Add(defaultStoreTTL+time.Second)))
	assert.True(t, store.saved[0].ExpiresAt.After(start.Add(defaultStoreTTL-time.Second)))
	assert.True(t, store.saved[0].ExpiresAt.Before(end.Add(defaultStoreTTL+time.Second)))
}

func TestDoResponseSkipsGenericCacheFollowWhenPromptCacheKeyExistsOnUnsupportedPromptStoreMode(
	t *testing.T,
) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1beta/models/gemini:generateContent",
		nil,
	)

	store := &recordingStore{}
	requestMeta := &meta.Meta{
		Mode:           mode.Gemini,
		OriginModel:    "gemini-2.5-pro",
		PromptCacheKey: "cache-key",
		ModelConfig: model.ModelConfig{
			Model: "gemini-2.5-pro",
			Plugin: map[string]map[string]any{
				PluginName: {"enable": true},
			},
		},
		Group:   model.GroupCache{ID: "group-1"},
		Token:   model.TokenCache{ID: 7},
		Channel: meta.ChannelMeta{ID: 9},
	}

	_, relayErr := (&Plugin{}).DoResponse(
		requestMeta,
		store,
		c,
		&http.Response{StatusCode: http.StatusOK},
		doResponseFunc{
			fn: func(_ *meta.Meta, _ adaptor.Store, c *gin.Context, _ *http.Response) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				_, _ = c.Writer.Write([]byte(`{"candidates":[]}`))

				return adaptor.DoResponseResult{
					Usage: model.Usage{CachedTokens: 4},
				}, nil
			},
		},
	)

	require.Nil(t, relayErr)
	assert.Empty(t, store.savedIfNotExist)
	assert.Empty(t, store.saved)
}

func TestDoResponseRecordsWhenOnlyCacheCreationTokensExist(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	store := &recordingStore{}
	requestMeta := &meta.Meta{
		Mode:        mode.ChatCompletions,
		OriginModel: "gpt-5",
		ModelConfig: model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				PluginName: {"enable": true},
			},
		},
		Group:   model.GroupCache{ID: "group-1"},
		Token:   model.TokenCache{ID: 7},
		Channel: meta.ChannelMeta{ID: 9},
	}

	_, relayErr := (&Plugin{}).DoResponse(
		requestMeta,
		store,
		c,
		&http.Response{StatusCode: http.StatusOK},
		doResponseFunc{
			fn: func(_ *meta.Meta, _ adaptor.Store, c *gin.Context, _ *http.Response) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				_, _ = c.Writer.Write([]byte(`{"id":"chatcmpl-1"}`))

				return adaptor.DoResponseResult{
					Usage: model.Usage{CacheCreationTokens: 8},
				}, nil
			},
		},
	)

	require.Nil(t, relayErr)
	require.Len(t, store.savedIfNotExist, 1)
	require.Len(t, store.saved, 1)
	assert.Equal(
		t,
		model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeStable),
		store.savedIfNotExist[0].ID,
	)
	assert.Equal(
		t,
		model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeLast),
		store.saved[0].ID,
	)
}

func TestDoResponseSkipsWhenResponseNotSuccessfulOrNotWritten(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		fn   func(*gin.Context) (adaptor.DoResponseResult, adaptor.Error)
	}{
		{
			name: "no cached tokens",
			fn: func(c *gin.Context) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				_, _ = c.Writer.Write([]byte(`{"ok":true}`))
				return adaptor.DoResponseResult{Usage: model.Usage{}}, nil
			},
		},
		{
			name: "writer has no body",
			fn: func(c *gin.Context) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				return adaptor.DoResponseResult{Usage: model.Usage{CachedTokens: 1}}, nil
			},
		},
		{
			name: "non 2xx status",
			fn: func(c *gin.Context) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusBadGateway)
				_, _ = c.Writer.Write([]byte(`{"ok":false}`))
				return adaptor.DoResponseResult{Usage: model.Usage{CachedTokens: 1}}, nil
			},
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
				"/v1/chat/completions",
				nil,
			)

			store := &recordingStore{}
			requestMeta := &meta.Meta{
				Mode:        mode.ChatCompletions,
				OriginModel: "gpt-5",
				ModelConfig: model.ModelConfig{
					Model: "gpt-5",
					Plugin: map[string]map[string]any{
						PluginName: {"enable": true},
					},
				},
				Group:   model.GroupCache{ID: "group-1"},
				Token:   model.TokenCache{ID: 7},
				Channel: meta.ChannelMeta{ID: 9},
			}

			_, relayErr := (&Plugin{}).DoResponse(
				requestMeta,
				store,
				c,
				&http.Response{StatusCode: http.StatusOK},
				doResponseFunc{
					fn: func(_ *meta.Meta, _ adaptor.Store, c *gin.Context, _ *http.Response) (adaptor.DoResponseResult, adaptor.Error) {
						return tt.fn(c)
					},
				},
			)

			require.Nil(t, relayErr)
			assert.Empty(t, store.savedIfNotExist)
			assert.Empty(t, store.saved)
		})
	}
}

func TestTryParseRetentionNullMarksParsed(t *testing.T) {
	t.Parallel()

	rw := &retentionResponseWriter{}
	rw.tryParseRetention(
		[]byte(`data: {"type":"response.created","response":{"prompt_cache_retention":null}}`),
	)

	assert.True(t, rw.parsed)
	assert.Empty(t, rw.retention)
}

func TestDoResponseSkipsWhenPluginDisabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	store := &recordingStore{}
	requestMeta := &meta.Meta{
		Mode:        mode.ChatCompletions,
		OriginModel: "gpt-5",
		ModelConfig: model.ModelConfig{Model: "gpt-5"},
		Group:       model.GroupCache{ID: "group-1"},
		Token:       model.TokenCache{ID: 7},
		Channel:     meta.ChannelMeta{ID: 9},
	}

	_, relayErr := (&Plugin{}).DoResponse(
		requestMeta,
		store,
		c,
		&http.Response{StatusCode: http.StatusOK},
		doResponseFunc{
			fn: func(_ *meta.Meta, _ adaptor.Store, c *gin.Context, _ *http.Response) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				_, _ = c.Writer.Write([]byte(`{"id":"chatcmpl-1"}`))

				return adaptor.DoResponseResult{
					Usage: model.Usage{CachedTokens: 4},
				}, nil
			},
		},
	)

	require.Nil(t, relayErr)
	assert.Empty(t, store.savedIfNotExist)
	assert.Empty(t, store.saved)
}

func TestDoResponseSkipsUpdatingLastStoreWithinWindow(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	lastID := model.CacheFollowStoreID("gpt-5", model.CacheKeyTypeLast)
	store := &recordingStore{
		stores: map[string]adaptor.StoreCache{
			lastID: {
				ID:        lastID,
				GroupID:   "group-1",
				TokenID:   7,
				ChannelID: 5,
				Model:     "gpt-5",
				CreatedAt: time.Now().Add(-5 * time.Second),
				UpdatedAt: time.Now().Add(-5 * time.Second),
				ExpiresAt: time.Now().Add(time.Minute),
			},
		},
	}
	requestMeta := &meta.Meta{
		Mode:        mode.ChatCompletions,
		OriginModel: "gpt-5",
		ModelConfig: model.ModelConfig{
			Model: "gpt-5",
			Plugin: map[string]map[string]any{
				PluginName: {"enable": true},
			},
		},
		Group:   model.GroupCache{ID: "group-1"},
		Token:   model.TokenCache{ID: 7},
		Channel: meta.ChannelMeta{ID: 9},
	}

	_, relayErr := (&Plugin{}).DoResponse(
		requestMeta,
		store,
		c,
		&http.Response{StatusCode: http.StatusOK},
		doResponseFunc{
			fn: func(_ *meta.Meta, _ adaptor.Store, c *gin.Context, _ *http.Response) (adaptor.DoResponseResult, adaptor.Error) {
				c.Status(http.StatusOK)
				_, _ = c.Writer.Write([]byte(`{"id":"chatcmpl-1"}`))

				return adaptor.DoResponseResult{
					Usage: model.Usage{CachedTokens: 4},
				}, nil
			},
		},
	)

	require.Nil(t, relayErr)
	require.Len(t, store.savedIfNotExist, 1)
	assert.Empty(t, store.saved)
}
