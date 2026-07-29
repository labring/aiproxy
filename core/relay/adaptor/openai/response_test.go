//nolint:testpackage
package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type responseTestStore struct {
	saved           []adaptor.StoreCache
	savedIfNotExist []adaptor.StoreCache
}

var responseStreamInitialBufferTimeoutTestMu sync.Mutex

func (s *responseTestStore) GetStoreByScope(
	string,
	int,
	string,
	model.ChannelScope,
) (adaptor.StoreCache, error) {
	return adaptor.StoreCache{}, nil
}

func (s *responseTestStore) SaveStore(
	cache adaptor.StoreCache,
	_ model.ChannelScope,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *responseTestStore) SaveStoreWithOption(
	cache adaptor.StoreCache,
	_ model.ChannelScope,
	_ adaptor.SaveStoreOption,
) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *responseTestStore) SaveIfNotExistStore(
	cache adaptor.StoreCache,
	_ model.ChannelScope,
) error {
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

func TestResponseStreamHandlerForwardsErrorAfterDownstreamWrite(t *testing.T) {
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

	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_123\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false}}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n" +
		"event: error\n" +
		"data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"stream failed\"}}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_123", result.UpstreamID)
	assert.Contains(t, recorder.Body.String(), `"type":"error"`)
	assert.Contains(t, recorder.Body.String(), `"message":"stream failed"`)
}

func TestResponseStreamHandlerReturnsErrorBeforeRealOutputAfterLifecycleEvents(t *testing.T) {
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

	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_123\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false}}\n\n" +
		"event: response.in_progress\n" +
		"data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_123\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false}}\n\n" +
		"event: error\n" +
		"data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"stream failed\"}}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusBadGateway, err.StatusCode())
	assert.Equal(t, "resp_123", result.UpstreamID)
	assert.Empty(t, recorder.Body.String())
}

func TestResponseStreamHandlerFailedWithoutErrorDoesNotMarkAsyncUsage(t *testing.T) {
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

	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_failed\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":true}}\n\n" +
		"event: response.in_progress\n" +
		"data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_failed\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":true}}\n\n" +
		"event: response.failed\n" +
		"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failed\",\"object\":\"response\",\"created_at\":1,\"status\":\"failed\",\"model\":\"gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":true}}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusBadGateway, err.StatusCode())
	assert.Equal(t, "resp_failed", result.UpstreamID)
	assert.False(t, result.AsyncUsage)
	assert.Empty(t, recorder.Body.String())
}

func TestResponseStreamHandlerFlushesLifecycleEventsOnOfficialTextStreamOrder(t *testing.T) {
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

	body := strings.Join([]string{
		"event: response.created",
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_text\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5.4\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"usage\":null}}",
		"",
		"event: response.in_progress",
		"data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_text\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5.4\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"usage\":null}}",
		"",
		"event: response.output_item.added",
		"data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"msg_text\",\"type\":\"message\",\"status\":\"in_progress\",\"role\":\"assistant\",\"content\":[]}}",
		"",
		"event: response.content_part.added",
		"data: {\"type\":\"response.content_part.added\",\"item_id\":\"msg_text\",\"output_index\":0,\"content_index\":0,\"part\":{\"type\":\"output_text\",\"text\":\"\",\"annotations\":[]}}",
		"",
		"event: response.output_text.delta",
		"data: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_text\",\"output_index\":0,\"content_index\":0,\"delta\":\"Hi\"}",
		"",
		"event: response.output_text.done",
		"data: {\"type\":\"response.output_text.done\",\"item_id\":\"msg_text\",\"output_index\":0,\"content_index\":0,\"text\":\"Hi\"}",
		"",
		"event: response.content_part.done",
		"data: {\"type\":\"response.content_part.done\",\"item_id\":\"msg_text\",\"output_index\":0,\"content_index\":0,\"part\":{\"type\":\"output_text\",\"text\":\"Hi\",\"annotations\":[]}}",
		"",
		"event: response.output_item.done",
		"data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"id\":\"msg_text\",\"type\":\"message\",\"status\":\"completed\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hi\",\"annotations\":[]}]}}",
		"",
		"event: response.completed",
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_text\",\"object\":\"response\",\"created_at\":1,\"status\":\"completed\",\"model\":\"gpt-5.4\",\"output\":[{\"id\":\"msg_text\",\"type\":\"message\",\"status\":\"completed\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hi\",\"annotations\":[]}]}],\"parallel_tool_calls\":true,\"store\":false,\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_text", result.UpstreamID)
	assert.Equal(t, model.ZeroNullInt64(2), result.Usage.TotalTokens)

	output := recorder.Body.String()
	assert.Contains(t, output, "response.created")
	assert.Contains(t, output, "response.in_progress")
	assert.Contains(t, output, "response.output_item.added")
	assert.Contains(t, output, "response.content_part.added")
	assert.Contains(t, output, "response.output_text.delta")
	assert.Contains(t, output, "response.completed")
}

func TestResponseStreamHandlerAcceptsObjectFunctionCallArguments(t *testing.T) {
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

	body := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_123","type":"function_call","call_id":"call_123","name":"search","arguments":{},"status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.done",
		`data: {"type":"response.function_call_arguments.done","item_id":"fc_123","output_index":0,"arguments":{"query":"spawn tool"},"sequence_number":1}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"fc_123","type":"function_call","call_id":"call_123","name":"search","arguments":{"query":"spawn tool"},"status":"completed"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool","object":"response","created_at":1,"status":"completed","model":"gpt-5.5","output":[{"id":"fc_123","type":"function_call","call_id":"call_123","name":"search","arguments":{"query":"spawn tool"},"status":"completed"}],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_tool", result.UpstreamID)
	assert.Equal(t, model.ZeroNullInt64(2), result.Usage.TotalTokens)

	output := recorder.Body.String()
	assert.Contains(t, output, `"arguments":{"query":"spawn tool"}`)
	assert.Contains(t, output, `"type":"response.completed"`)
}

func TestResponseStreamHandlerStartsBufferTimeoutFromFirstDelayedEvent(t *testing.T) {
	responseStreamInitialBufferTimeoutTestMu.Lock()
	defer responseStreamInitialBufferTimeoutTestMu.Unlock()

	gin.SetMode(gin.TestMode)

	oldTimeout := responseStreamInitialBufferTimeout
	responseStreamInitialBufferTimeout = time.Millisecond
	t.Cleanup(func() {
		responseStreamInitialBufferTimeout = oldTimeout
	})

	reader, writer := io.Pipe()
	defer writer.Close()

	go func() {
		_, _ = writer.Write([]byte(strings.Join([]string{
			"event: response.in_progress",
			"data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_timeout\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"gpt-5.4\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"usage\":null}}",
			"",
		}, "\n")))

		time.Sleep(20 * time.Millisecond)

		_, _ = writer.Write([]byte(strings.Join([]string{
			"event: response.completed",
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_timeout\",\"object\":\"response\",\"created_at\":1,\"status\":\"completed\",\"model\":\"gpt-5.4\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}",
			"",
		}, "\n")))
		_ = writer.Close()
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/responses",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       reader,
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_timeout", result.UpstreamID)
	assert.Contains(t, recorder.Body.String(), "response.in_progress")
	assert.Contains(t, recorder.Body.String(), "response.completed")
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

func TestResponseHandlerStoreUsesOriginModel(t *testing.T) {
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
		OriginModel: "gpt-5",
		ActualModel: "mapped-gpt-5",
		Group:       model.GroupCache{ID: "group-1"},
		Token:       model.TokenCache{ID: 7},
		Channel:     meta.ChannelMeta{ID: 9},
	}

	body := `{
		"id":"resp_store_origin",
		"object":"response",
		"created_at":1,
		"status":"completed",
		"model":"mapped-gpt-5",
		"output":[],
		"parallel_tool_calls":true,
		"store":true
	}`
	resp := &http.Response{
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseHandler(meta, store, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_store_origin", result.UpstreamID)
	require.Len(t, store.saved, 1)
	assert.Equal(t, "gpt-5", store.saved[0].Model)

	var payload relaymodel.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "gpt-5", payload.Model)
	assert.NotContains(t, recorder.Body.String(), "mapped-gpt-5")
}

func TestResponseHandlerRewritesOnlyModelField(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/responses", nil)
	m := &meta.Meta{
		OriginModel: "gpt-5",
		ActualModel: "mapped-gpt-5",
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"resp_extra",
			"object":"response",
			"created_at":1,
			"status":"completed",
			"model":"mapped-gpt-5",
			"output":[],
			"parallel_tool_calls":true,
			"store":false,
			"provider_extra":{"future_field":"kept"},
			"future_top_level":"kept"
		}`)),
		Header: make(http.Header),
	}

	_, err := ResponseHandler(m, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Contains(t, recorder.Body.String(), `"model":"gpt-5"`)
	assert.Contains(t, recorder.Body.String(), `"provider_extra":{"future_field":"kept"}`)
	assert.Contains(t, recorder.Body.String(), `"future_top_level":"kept"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-gpt-5")
}

func TestConvertResponseRequestMapsSystemInputRoleToDeveloper(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/responses",
		strings.NewReader(`{
			"model":"gpt-5.5",
			"input":[
				{"type":"message","role":"system","content":[{"type":"input_text","text":"Be concise"}]},
				{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}
			]
		}`),
	)
	req.Header.Set("Content-Type", "application/json")

	m := &meta.Meta{
		ActualModel: "mapped-gpt-5.5",
	}

	result, err := ConvertResponseRequest(m, req)
	require.NoError(t, err)

	var body map[string]any

	err = json.NewDecoder(result.Body).Decode(&body)
	require.NoError(t, err)

	input, ok := body["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "developer", first["role"])

	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", second["role"])
	assert.Equal(t, "mapped-gpt-5.5", body["model"])
}

func TestGetResponseHandlerRewritesModelToOriginModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/responses/resp_1",
		nil,
	)
	m := &meta.Meta{
		OriginModel: "gpt-5",
		ActualModel: "mapped-gpt-5",
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"resp_1",
			"object":"response",
			"created_at":1,
			"status":"completed",
			"model":"mapped-gpt-5",
			"output":[],
			"parallel_tool_calls":true,
			"store":false
		}`)),
		Header: make(http.Header),
	}

	result, err := GetResponseHandler(m, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_1", result.UpstreamID)
	assert.Zero(t, result.Usage.TotalTokens)
	assert.False(t, result.AsyncUsage)
	assert.Contains(t, recorder.Body.String(), `"model":"gpt-5"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-gpt-5")
}

func TestGetResponseHandlerDoesNotReportUsageForCompletedResponse(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/responses/resp_1",
		nil,
	)
	m := &meta.Meta{
		OriginModel: "gpt-5",
		ActualModel: "mapped-gpt-5",
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"resp_1",
			"object":"response",
			"created_at":1,
			"status":"completed",
			"model":"mapped-gpt-5",
			"output":[],
			"parallel_tool_calls":true,
			"store":true,
			"usage":{"input_tokens":7,"output_tokens":13,"total_tokens":20}
		}`)),
		Header: make(http.Header),
	}

	result, err := GetResponseHandler(m, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_1", result.UpstreamID)
	assert.Zero(t, result.Usage.TotalTokens)
	assert.False(t, result.AsyncUsage)
	assert.Contains(t, recorder.Body.String(), `"usage"`)
}

func TestCancelResponseHandlerRewritesModelToOriginModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/responses/resp_1/cancel",
		nil,
	)
	m := &meta.Meta{
		OriginModel: "gpt-5",
		ActualModel: "mapped-gpt-5",
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"resp_1",
			"object":"response",
			"created_at":1,
			"status":"in_progress",
			"model":"mapped-gpt-5",
			"output":[],
			"parallel_tool_calls":true,
			"store":false
		}`)),
		Header: make(http.Header),
	}

	result, err := CancelResponseHandler(m, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_1", result.UpstreamID)
	assert.Contains(t, recorder.Body.String(), `"model":"gpt-5"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-gpt-5")
}

func TestResponseHandlerAsyncUsageInProgress(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/responses", nil)

	body := `{
		"id":"resp_async_in_progress",
		"object":"response",
		"created_at":1,
		"status":"in_progress",
		"model":"gpt-5.4",
		"output":[],
		"parallel_tool_calls":true,
		"store":true,
		"usage":null
	}`
	resp := &http.Response{
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_async_in_progress", result.UpstreamID)
	assert.True(t, result.AsyncUsage)
}

func TestResponseHandlerFailedDoesNotMarkAsyncUsage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/responses", nil)

	body := `{
		"id":"resp_async_failed",
		"object":"response",
		"created_at":1,
		"status":"failed",
		"model":"gpt-5.4",
		"output":[],
		"parallel_tool_calls":true,
		"store":true,
		"usage":null
	}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_async_failed", result.UpstreamID)
	assert.False(t, result.AsyncUsage)
}

func TestResponseStreamHandlerForegroundImageGenerationContinuesToCompleted(t *testing.T) {
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

	body := strings.Join([]string{
		"event: response.created",
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_generating_async\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"background\":false,\"model\":\"gpt-5.4\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"tool_usage\":{\"image_gen\":{\"input_tokens\":0,\"output_tokens\":0,\"total_tokens\":0},\"web_search\":{\"num_requests\":0}},\"tools\":[{\"type\":\"image_generation\",\"background\":\"auto\",\"model\":\"gpt-image-2\"}],\"usage\":null}}",
		"",
		"event: response.in_progress",
		"data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_generating_async\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"background\":false,\"model\":\"gpt-5.4\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"tool_usage\":{\"image_gen\":{\"input_tokens\":0,\"output_tokens\":0,\"total_tokens\":0},\"web_search\":{\"num_requests\":0}},\"tools\":[{\"type\":\"image_generation\",\"background\":\"auto\",\"model\":\"gpt-image-2\"}],\"usage\":null}}",
		"",
		"event: response.output_item.added",
		"data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"ig_generating_async\",\"type\":\"image_generation_call\",\"status\":\"in_progress\"},\"output_index\":0,\"sequence_number\":2}",
		"",
		"event: response.image_generation_call.generating",
		"data: {\"type\":\"response.image_generation_call.generating\",\"item_id\":\"ig_generating_async\",\"output_index\":0,\"sequence_number\":3}",
		"",
		"event: " + relaymodel.EventKeepAlive,
		"data: {\"type\":\"" + relaymodel.EventKeepAlive + "\",\"sequence_number\":4}",
		"",
		"event: response.completed",
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_generating_async\",\"object\":\"response\",\"created_at\":2,\"status\":\"completed\",\"model\":\"gpt-5.4\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"total_tokens\":3}}}",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_generating_async", result.UpstreamID)
	assert.False(t, result.AsyncUsage)
	assert.Equal(t, model.ZeroNullInt64(3), result.Usage.TotalTokens)
	assert.Contains(t, recorder.Body.String(), "response.image_generation_call.generating")
	assert.Contains(t, recorder.Body.String(), "keepalive")
	assert.Contains(t, recorder.Body.String(), "response.completed")
}

func TestResponseStreamHandlerUsesLastResponseForAsyncUsage(t *testing.T) {
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

	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_async\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"background\":false,\"model\":\"gpt-5.4\",\"output\":[],\"parallel_tool_calls\":true,\"store\":false,\"tool_usage\":{\"image_gen\":{\"input_tokens\":0,\"output_tokens\":0,\"total_tokens\":0},\"web_search\":{\"num_requests\":0}},\"tools\":[{\"type\":\"image_generation\",\"background\":\"auto\",\"model\":\"gpt-image-2\"}],\"usage\":null}}\n\n" +
		"event: response.output_item.added\n" +
		"data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"ig_stream_async\",\"type\":\"image_generation_call\",\"status\":\"in_progress\"},\"output_index\":0,\"sequence_number\":2}\n\n" +
		"event: response.image_generation_call.generating\n" +
		"data: {\"type\":\"response.image_generation_call.generating\",\"item_id\":\"ig_stream_async\",\"output_index\":0,\"sequence_number\":3}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(&meta.Meta{}, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_stream_async", result.UpstreamID)
	assert.True(t, result.AsyncUsage)
	assert.Equal(t, model.ZeroNullInt64(0), result.Usage.TotalTokens)
	assert.Contains(t, recorder.Body.String(), "response.image_generation_call.generating")
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

func TestResponseStreamHandlerStoreUsesOriginModel(t *testing.T) {
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
		OriginModel: "gpt-5",
		ActualModel: "mapped-gpt-5",
		Group:       model.GroupCache{ID: "group-1"},
		Token:       model.TokenCache{ID: 7},
		Channel:     meta.ChannelMeta{ID: 9},
	}

	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_store_origin\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"mapped-gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":true}}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_store_origin\",\"object\":\"response\",\"created_at\":2,\"status\":\"completed\",\"model\":\"mapped-gpt-5\",\"output\":[],\"parallel_tool_calls\":true,\"store\":true}}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	result, err := ResponseStreamHandler(meta, store, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "resp_stream_store_origin", result.UpstreamID)
	require.Len(t, store.saved, 1)
	assert.Equal(t, "gpt-5", store.saved[0].Model)
	assert.Contains(t, recorder.Body.String(), `"model":"gpt-5"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-gpt-5")
}

func TestResponseStreamHandlerRewritesOnlyResponseModelField(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/responses", nil)
	m := &meta.Meta{
		OriginModel: "gpt-5",
		ActualModel: "mapped-gpt-5",
	}

	body := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_stream_extra","object":"response","created_at":1,"status":"in_progress","model":"mapped-gpt-5","output":[],"parallel_tool_calls":true,"store":false,"provider_extra":{"future_field":"kept"},"future_response_field":"kept"},"future_event_field":"kept"}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"hi","future_event_field":"kept"}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	_, err := ResponseStreamHandler(m, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Contains(t, recorder.Body.String(), `"model":"gpt-5"`)
	assert.Contains(t, recorder.Body.String(), `"provider_extra":{"future_field":"kept"}`)
	assert.Contains(t, recorder.Body.String(), `"future_response_field":"kept"`)
	assert.Contains(t, recorder.Body.String(), `"future_event_field":"kept"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-gpt-5")
}

func TestVideoHandlerMarksAsyncUsage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/video/generations/jobs",
		nil,
	)

	m := &meta.Meta{
		OriginModel: "sora-2",
		ActualModel: "mapped-sora-2",
		Group:       model.GroupCache{ID: "group-1"},
		Token:       model.TokenCache{ID: 7},
		Channel:     meta.ChannelMeta{ID: 9},
	}
	resp := &http.Response{
		StatusCode: http.StatusCreated,
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"video_job_async",
			"object":"video.generation.job",
			"status":"queued",
			"model":"mapped-sora-2"
		}`)),
		Header: make(http.Header),
	}

	result, err := VideoHandler(m, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "video_job_async", result.UpstreamID)
	assert.True(t, result.AsyncUsage)
	assert.Contains(t, recorder.Body.String(), `"model":"sora-2"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-sora-2")
}

func TestVideosHandlerStoresVideoAndMarksAsyncUsage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/videos",
		nil,
	)

	m := &meta.Meta{
		OriginModel: "sora-2",
		ActualModel: "mapped-sora-2",
		Group:       model.GroupCache{ID: "group-1"},
		Token:       model.TokenCache{ID: 7},
		Channel:     meta.ChannelMeta{ID: 9},
	}
	store := &responseTestStore{}
	resp := &http.Response{
		StatusCode: http.StatusCreated,
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"video_async",
			"object":"video",
			"status":"queued",
			"model":"mapped-sora-2"
		}`)),
		Header: make(http.Header),
	}

	result, err := VideosHandler(m, store, c, resp)
	require.Nil(t, err)
	assert.Equal(t, "video_async", result.UpstreamID)
	assert.True(t, result.AsyncUsage)
	require.Len(t, store.saved, 1)
	assert.Equal(t, model.VideoGenerationStoreID("video_async"), store.saved[0].ID)
	assert.Contains(t, recorder.Body.String(), `"model":"sora-2"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-sora-2")
}

func TestVideoGetJobsHandlerRewritesModelToOriginModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/video/generations/jobs/job_1",
		nil,
	)

	m := &meta.Meta{
		OriginModel: "sora-2",
		ActualModel: "mapped-sora-2",
		Group:       model.GroupCache{ID: "group-1"},
		Token:       model.TokenCache{ID: 7},
		Channel:     meta.ChannelMeta{ID: 9},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"job_1",
			"object":"video.generation.job",
			"status":"succeeded",
			"model":"mapped-sora-2",
			"expires_at":1780000000,
			"generations":[{"id":"gen_1","object":"video.generation","job_id":"job_1"}]
		}`)),
		Header: make(http.Header),
	}

	_, err := VideoGetJobsHandler(m, &responseTestStore{}, c, resp)
	require.Nil(t, err)
	assert.Contains(t, recorder.Body.String(), `"model":"sora-2"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-sora-2")
}

func TestVideosGetHandlerRewritesModelToOriginModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/videos/video_1",
		nil,
	)

	m := &meta.Meta{
		OriginModel: "sora-2",
		ActualModel: "mapped-sora-2",
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"video_1",
			"object":"video",
			"status":"completed",
			"model":"mapped-sora-2"
		}`)),
		Header: make(http.Header),
	}

	_, err := VideosGetHandler(m, c, resp)
	require.Nil(t, err)
	assert.Contains(t, recorder.Body.String(), `"model":"sora-2"`)
	assert.NotContains(t, recorder.Body.String(), "mapped-sora-2")
}
