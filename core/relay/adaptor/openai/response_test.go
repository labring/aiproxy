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

	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_123\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"prompt_cache_retention\":\"24h\"}}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_123\",\"object\":\"response\",\"created_at\":1,\"status\":\"completed\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"prompt_cache_retention\":\"24h\",\"usage\":{\"input_tokens\":7,\"input_tokens_details\":{\"cached_tokens\":0},\"output_tokens\":13,\"output_tokens_details\":{\"reasoning_tokens\":0},\"total_tokens\":20}}}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(meta, store, c, resp)
	require.Nil(t, err)
	require.Empty(t, store.savedIfNotExist)
	assert.Equal(t, "resp_123", result.UpstreamID)
	assert.Equal(t, model.ZeroNullInt64(20), result.Usage.TotalTokens)
}

func TestResponseHandlerWebSearchCountFromToolUsage(t *testing.T) {
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
		OriginModel: "gpt-5.4",
		ActualModel: "gpt-5.4",
		Group:       model.GroupCache{ID: "group-1"},
		Token:       model.TokenCache{ID: 7},
		Channel:     meta.ChannelMeta{ID: 9},
	}

	body := `{
		"id":"resp_tool_usage_123",
		"object":"response",
		"created_at":1777053463,
		"status":"completed",
		"model":"gpt-5.4",
		"output":[
			{"type":"reasoning","summary":[]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}
		],
		"tool_usage":{"web_search":{"num_requests":1}},
		"usage":{
			"input_tokens":15065,
			"input_tokens_details":{"cached_tokens":10880},
			"output_tokens":256,
			"output_tokens_details":{"reasoning_tokens":81},
			"total_tokens":15321
		}
	}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseHandler(meta, store, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_tool_usage_123", result.UpstreamID)
	assert.Equal(t, model.ZeroNullInt64(15321), result.Usage.TotalTokens)
	assert.Equal(t, model.ZeroNullInt64(1), result.Usage.WebSearchCount)
}

func TestResponseStreamHandlerWebSearchCountFromToolUsage(t *testing.T) {
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
		OriginModel: "gpt-5.4",
		ActualModel: "gpt-5.4",
		Group:       model.GroupCache{ID: "group-1"},
		Token:       model.TokenCache{ID: 7},
		Channel:     meta.ChannelMeta{ID: 9},
	}

	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_ws_stream_tool_usage\",\"object\":\"response\",\"created_at\":1777053463,\"status\":\"in_progress\",\"model\":\"gpt-5.4\",\"output\":[],\"tool_usage\":{\"image_gen\":{\"input_tokens\":0,\"input_tokens_details\":{\"image_tokens\":0,\"text_tokens\":0},\"output_tokens\":0,\"output_tokens_details\":{\"image_tokens\":0,\"text_tokens\":0},\"total_tokens\":0},\"web_search\":{\"num_requests\":1}},\"parallel_tool_calls\":true,\"store\":false}}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_ws_stream_tool_usage\",\"object\":\"response\",\"created_at\":1777053474,\"status\":\"completed\",\"model\":\"gpt-5.4\",\"output\":[{\"type\":\"reasoning\",\"summary\":[]},{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"done\"}]}],\"tool_usage\":{\"image_gen\":{\"input_tokens\":0,\"input_tokens_details\":{\"image_tokens\":0,\"text_tokens\":0},\"output_tokens\":0,\"output_tokens_details\":{\"image_tokens\":0,\"text_tokens\":0},\"total_tokens\":0},\"web_search\":{\"num_requests\":1}},\"parallel_tool_calls\":true,\"store\":false,\"usage\":{\"input_tokens\":15065,\"input_tokens_details\":{\"cached_tokens\":10880},\"output_tokens\":256,\"output_tokens_details\":{\"reasoning_tokens\":81},\"total_tokens\":15321}}}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(meta, store, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_ws_stream_tool_usage", result.UpstreamID)
	assert.Equal(t, model.ZeroNullInt64(15321), result.Usage.TotalTokens)
	assert.Equal(t, model.ZeroNullInt64(1), result.Usage.WebSearchCount)
}
