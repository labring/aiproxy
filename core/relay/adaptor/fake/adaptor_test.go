package fake_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/fake"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopStore struct{}

func (noopStore) GetStore(string, int, string) (adaptor.StoreCache, error) {
	return adaptor.StoreCache{}, nil
}

func (noopStore) SaveStore(adaptor.StoreCache) error {
	return nil
}

func (noopStore) SaveIfNotExistStore(adaptor.StoreCache) error {
	return nil
}

type recordingStore struct {
	saved []adaptor.StoreCache
}

func (s *recordingStore) GetStore(string, int, string) (adaptor.StoreCache, error) {
	return adaptor.StoreCache{}, nil
}

func (s *recordingStore) SaveStore(cache adaptor.StoreCache) error {
	s.saved = append(s.saved, cache)
	return nil
}

func (s *recordingStore) SaveIfNotExistStore(cache adaptor.StoreCache) error {
	s.saved = append(s.saved, cache)
	return nil
}

type fakeRunResult struct {
	recorder *httptest.ResponseRecorder
	result   adaptor.DoResponseResult
	resp     *http.Response
	store    adaptor.Store
}

func TestFakeChannelTypeNameToType(t *testing.T) {
	assert.Equal(t, int(model.ChannelTypeFake), model.ChannelTypeNameToType("fake"))
}

func TestFakeAdaptorMetadataSchema(t *testing.T) {
	a := &fake.Adaptor{}
	metaInfo := a.Metadata()

	require.NotNil(t, metaInfo.ConfigSchema)
	properties, ok := metaInfo.ConfigSchema["properties"].(map[string]any)
	require.True(t, ok)

	_, hasStaticText := properties["static_text"]
	_, hasUsage := properties["usage"]
	_, hasOpenAPI := properties["openapi"]

	assert.True(t, hasStaticText)
	assert.True(t, hasUsage)
	assert.True(t, hasOpenAPI)
}

func TestFakeAdaptorResponsesForAllModes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	boolPtr := func(v bool) *bool { return &v }

	testCases := []struct {
		name           string
		mode           mode.Mode
		modelName      string
		requestPath    string
		requestBody    any
		channelConfigs model.ChannelConfigs
		responseID     string
		store          adaptor.Store
		assertions     func(t *testing.T, result fakeRunResult)
	}{
		{
			name:        "chat completions",
			mode:        mode.ChatCompletions,
			modelName:   "fake-chat",
			requestPath: "/v1/chat/completions",
			requestBody: relaymodel.GeneralOpenAIRequest{
				Model: "fake-chat",
				Messages: []relaymodel.Message{
					{Role: relaymodel.RoleUser, Content: "hello fake"},
				},
			},
			channelConfigs: model.ChannelConfigs{
				"static_text": "fake chat payload",
				"usage": map[string]any{
					"input_tokens":  11,
					"output_tokens": 7,
				},
			},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)
				assert.Equal(t, int64(11), int64(result.result.Usage.InputTokens))
				assert.Equal(t, int64(7), int64(result.result.Usage.OutputTokens))

				var out relaymodel.TextResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Choices, 1)
				assert.Equal(t, "fake chat payload", out.Choices[0].Message.Content)
				assert.Equal(t, int64(11), out.Usage.PromptTokens)
				assert.Equal(t, int64(7), out.Usage.CompletionTokens)
				assert.NotEmpty(t, result.result.UpstreamID)
			},
		},
		{
			name:        "completions",
			mode:        mode.Completions,
			modelName:   "fake-completion",
			requestPath: "/v1/completions",
			requestBody: relaymodel.GeneralOpenAIRequest{
				Model:  "fake-completion",
				Prompt: "write a poem",
			},
			channelConfigs: model.ChannelConfigs{
				"static_text": "completion payload",
			},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)

				var out relaymodel.TextResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Choices, 1)
				assert.Equal(t, "completion payload", out.Choices[0].Text)
				assert.Equal(t, "completion payload", out.Choices[0].Message.Content)
			},
		},
		{
			name:        "embeddings",
			mode:        mode.Embeddings,
			modelName:   "fake-embedding",
			requestPath: "/v1/embeddings",
			requestBody: relaymodel.EmbeddingRequest{
				Model:      "fake-embedding",
				Input:      "embedding input",
				Dimensions: 6,
			},
			channelConfigs: model.ChannelConfigs{
				"embedding": map[string]any{
					"dimensions": 6,
					"base":       "embedding-seed",
				},
				"usage": map[string]any{
					"input_tokens": 13,
				},
			},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)

				var out relaymodel.EmbeddingResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Data, 1)
				assert.Len(t, out.Data[0].Embedding, 6)
				assert.Equal(t, int64(13), out.Usage.PromptTokens)
				assert.Equal(t, int64(25), out.Usage.TotalTokens)
			},
		},
		{
			name:        "images generations",
			mode:        mode.ImagesGenerations,
			modelName:   "fake-image",
			requestPath: "/v1/images/generations",
			requestBody: relaymodel.ImageRequest{
				Model:  "fake-image",
				Prompt: "draw a fox",
			},
			channelConfigs: model.ChannelConfigs{
				"image": map[string]any{
					"url":              "https://fake.local/custom/fox.png",
					"b64_json":         "ZmFrZS1pbWFnZQ==",
					"revised_prompt":   "fox revised",
					"image_tokens_in":  3,
					"image_tokens_out": 17,
				},
				"usage": map[string]any{
					"input_tokens":        15,
					"output_tokens":       17,
					"image_input_tokens":  3,
					"image_output_tokens": 17,
				},
			},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)

				var out relaymodel.ImageResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Data, 1)
				assert.Equal(t, "https://fake.local/custom/fox.png", out.Data[0].URL)
				assert.Equal(t, "fox revised", out.Data[0].RevisedPrompt)
				require.NotNil(t, out.Usage)
				assert.Equal(t, int64(15), out.Usage.InputTokens)
				assert.Equal(t, int64(17), out.Usage.OutputTokens)
				assert.Equal(t, int64(3), out.Usage.InputTokensDetails.ImageTokens)
				require.NotNil(t, out.Usage.OutputTokensDetails)
				assert.Equal(t, int64(17), out.Usage.OutputTokensDetails.ImageTokens)
			},
		},
		{
			name:        "images generations auto png",
			mode:        mode.ImagesGenerations,
			modelName:   "fake-image",
			requestPath: "/v1/images/generations",
			requestBody: relaymodel.ImageRequest{
				Model:          "fake-image",
				Prompt:         "draw a skyline",
				ResponseFormat: "b64_json",
				Size:           "64x64",
			},
			channelConfigs: model.ChannelConfigs{},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()

				var out relaymodel.ImageResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Data, 1)
				assert.NotEmpty(t, out.Data[0].B64Json)
				assert.Empty(t, out.Data[0].URL)

				decoder := base64.NewDecoder(
					base64.StdEncoding,
					bytes.NewBufferString(out.Data[0].B64Json),
				)
				rawPNG, err := io.ReadAll(decoder)
				require.NoError(t, err)
				assert.Greater(t, len(rawPNG), 32)
				assert.Equal(t, []byte{0x89, 'P', 'N', 'G'}, rawPNG[:4])
			},
		},
		{
			name:        "images generations url fallback",
			mode:        mode.ImagesGenerations,
			modelName:   "fake-image",
			requestPath: "/v1/images/generations",
			requestBody: relaymodel.ImageRequest{
				Model:          "fake-image",
				Prompt:         "draw a castle",
				ResponseFormat: "url",
			},
			channelConfigs: model.ChannelConfigs{},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()

				var out relaymodel.ImageResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Data, 1)
				assert.Empty(t, out.Data[0].B64Json)
				assert.Contains(t, out.Data[0].URL, "https://fake.local/images/")
			},
		},
		{
			name:        "rerank",
			mode:        mode.Rerank,
			modelName:   "fake-rerank",
			requestPath: "/v1/rerank",
			requestBody: relaymodel.RerankRequest{
				Model:     "fake-rerank",
				Query:     "best document",
				Documents: []string{"doc1", "doc2", "doc3"},
			},
			channelConfigs: model.ChannelConfigs{
				"rerank": map[string]any{
					"base_score":       0.92,
					"step":             0.07,
					"return_documents": true,
				},
			},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)

				var out relaymodel.RerankResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Results, 3)
				assert.Equal(t, 0.92, out.Results[0].RelevanceScore)
				require.NotNil(t, out.Results[0].Document)
				assert.NotEmpty(t, out.Results[0].Document.Text)
			},
		},
		{
			name:        "anthropic",
			mode:        mode.Anthropic,
			modelName:   "fake-anthropic",
			requestPath: "/v1/messages",
			requestBody: relaymodel.ClaudeAnyContentRequest{
				Model: "fake-anthropic",
				Messages: []relaymodel.ClaudeAnyContentMessage{
					{Role: relaymodel.RoleUser, Content: "hello claude"},
				},
			},
			channelConfigs: model.ChannelConfigs{
				"static_text": "anthropic payload",
				"anthropic": map[string]any{
					"stop_reason": relaymodel.ClaudeStopReasonEndTurn,
				},
			},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)

				var out relaymodel.ClaudeResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				assert.Equal(t, relaymodel.RoleAssistant, out.Role)
				assert.Equal(t, relaymodel.ClaudeStopReasonEndTurn, out.StopReason)
				require.Len(t, out.Content, 1)
				assert.Equal(t, "anthropic payload", out.Content[0].Text)
			},
		},
		{
			name:        "gemini native",
			mode:        mode.Gemini,
			modelName:   "fake-gemini",
			requestPath: "/v1beta/models/fake-gemini:generateContent",
			requestBody: relaymodel.GeminiChatRequest{
				Contents: []*relaymodel.GeminiChatContent{
					{
						Role: "user",
						Parts: []*relaymodel.GeminiPart{
							{Text: "hello gemini"},
						},
					},
				},
			},
			channelConfigs: model.ChannelConfigs{
				"static_text": "gemini payload",
				"gemini": map[string]any{
					"finish_reason": "STOP",
					"model_version": "fake-2.0",
				},
				"usage": map[string]any{
					"input_tokens":     9,
					"output_tokens":    5,
					"reasoning_tokens": 2,
				},
			},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)

				var out relaymodel.GeminiChatResponse
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Candidates, 1)
				assert.Equal(t, "gemini payload", out.Candidates[0].Content.Parts[0].Text)
				require.NotNil(t, out.UsageMetadata)
				assert.Equal(t, int64(9), out.UsageMetadata.PromptTokenCount)
				assert.Equal(t, "fake-2.0", out.ModelVersion)
			},
		},
		{
			name:        "responses",
			mode:        mode.Responses,
			modelName:   "fake-response",
			requestPath: "/v1/responses",
			requestBody: relaymodel.CreateResponseRequest{
				Model: "fake-response",
				Input: "hello responses",
			},
			channelConfigs: model.ChannelConfigs{
				"static_text":    "responses payload",
				"reasoning_text": "response reasoning",
				"metadata": map[string]any{
					"env": "test",
				},
				"response": map[string]any{
					"store":               true,
					"parallel_tool_calls": false,
				},
				"usage": map[string]any{
					"input_tokens":     10,
					"output_tokens":    6,
					"cached_tokens":    2,
					"reasoning_tokens": 1,
				},
			},
			responseID: "resp_test_create",
			store:      &recordingStore{},
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusCreated, result.resp.StatusCode)
				assert.Equal(t, "resp_test_create", result.result.UpstreamID)

				var out relaymodel.Response
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				assert.Equal(t, "resp_test_create", out.ID)
				assert.Equal(t, relaymodel.ResponseStatusCompleted, out.Status)
				assert.False(t, out.ParallelToolCalls)
				assert.True(t, out.Store)
				require.Len(t, out.Output, 1)
				assert.Equal(t, "responses payload", out.Output[0].Content[0].Text)
				require.NotNil(t, out.Usage)
				assert.Equal(t, int64(10), out.Usage.InputTokens)
				assert.Equal(t, int64(2), out.Usage.InputTokensDetails.CachedTokens)
				assert.Equal(t, int64(1), out.Usage.OutputTokensDetails.ReasoningTokens)
				assert.Equal(t, "test", out.Metadata["env"])

				store, ok := result.store.(*recordingStore)
				require.True(t, ok)
				require.Len(t, store.saved, 1)
				assert.Equal(t, model.ResponseStoreID("resp_test_create"), store.saved[0].ID)
			},
		},
		{
			name:        "responses get",
			mode:        mode.ResponsesGet,
			modelName:   "fake-response",
			requestPath: "/v1/responses/resp_get",
			requestBody: map[string]any{},
			channelConfigs: model.ChannelConfigs{
				"static_text": "responses get payload",
			},
			responseID: "resp_get",
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)
				assert.Equal(t, "resp_get", result.result.UpstreamID)

				var out relaymodel.Response
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				assert.Equal(t, "resp_get", out.ID)
				assert.Equal(t, "responses get payload", out.Output[0].Content[0].Text)
			},
		},
		{
			name:        "responses cancel",
			mode:        mode.ResponsesCancel,
			modelName:   "fake-response",
			requestPath: "/v1/responses/resp_cancel/cancel",
			requestBody: map[string]any{},
			channelConfigs: model.ChannelConfigs{
				"static_text": "responses cancel payload",
			},
			responseID: "resp_cancel",
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)
				assert.Equal(t, "resp_cancel", result.result.UpstreamID)

				var out relaymodel.Response
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				assert.Equal(t, relaymodel.ResponseStatusCancelled, out.Status)
				assert.Equal(t, "resp_cancel", out.ID)
			},
		},
		{
			name:        "responses input items",
			mode:        mode.ResponsesInputItems,
			modelName:   "fake-response",
			requestPath: "/v1/responses/resp_items/input_items",
			requestBody: map[string]any{},
			channelConfigs: model.ChannelConfigs{
				"static_text": "responses input payload",
			},
			responseID: "resp_items",
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusOK, result.resp.StatusCode)

				var out relaymodel.InputItemList
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				assert.Equal(t, "list", out.Object)
				require.Len(t, out.Data, 1)
				assert.Equal(t, relaymodel.InputItemTypeMessage, out.Data[0].Type)
				assert.Equal(t, relaymodel.RoleUser, out.Data[0].Role)
				assert.Equal(t, "fake input", out.Data[0].Content[0].Text)
			},
		},
		{
			name:        "responses delete",
			mode:        mode.ResponsesDelete,
			modelName:   "fake-response",
			requestPath: "/v1/responses/resp_delete",
			requestBody: map[string]any{},
			channelConfigs: model.ChannelConfigs{
				"static_text": "unused",
			},
			responseID: "resp_delete",
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()
				assert.Equal(t, http.StatusNoContent, result.resp.StatusCode)
				assert.Equal(t, http.StatusOK, result.recorder.Code)
				assert.Empty(t, result.recorder.Body.String())
				assert.Empty(t, result.result.UpstreamID)
			},
		},
		{
			name:        "responses input items from request",
			mode:        mode.Responses,
			modelName:   "fake-response",
			requestPath: "/v1/responses",
			requestBody: relaymodel.CreateResponseRequest{
				Model: "fake-response",
				Input: "request-driven input",
				Store: boolPtr(false),
			},
			channelConfigs: model.ChannelConfigs{
				"static_text": "request echoes request-driven input",
				"response": map[string]any{
					"store": false,
				},
			},
			responseID: "resp_from_request",
			assertions: func(t *testing.T, result fakeRunResult) {
				t.Helper()

				var out relaymodel.Response
				require.NoError(t, json.Unmarshal(result.recorder.Body.Bytes(), &out))
				require.Len(t, out.Output, 1)
				assert.Contains(t, out.Output[0].Content[0].Text, "request-driven input")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := tc.store
			if store == nil {
				store = noopStore{}
			}

			result := runFakeRequest(
				t,
				tc.mode,
				tc.modelName,
				tc.requestPath,
				tc.requestBody,
				tc.channelConfigs,
				tc.responseID,
				store,
			)
			tc.assertions(t, result)
		})
	}
}

func TestFakeAdaptorStreamingResponses(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		mode           mode.Mode
		modelName      string
		requestPath    string
		requestBody    any
		channelConfigs model.ChannelConfigs
		responseID     string
		expectedParts  []string
		expectedType   string
	}{
		{
			name:        "openai chat stream",
			mode:        mode.ChatCompletions,
			modelName:   "fake-chat",
			requestPath: "/v1/chat/completions",
			requestBody: relaymodel.GeneralOpenAIRequest{
				Model:  "fake-chat",
				Stream: true,
				Messages: []relaymodel.Message{
					{Role: relaymodel.RoleUser, Content: "stream please"},
				},
			},
			channelConfigs: model.ChannelConfigs{
				"static_text":       "streamed openai payload",
				"stream_chunks":     2,
				"stream_chunk_size": 8,
			},
			expectedParts: []string{
				"data: ",
				"\"object\":\"chat.completion.chunk\"",
				"streamed",
				"[DONE]",
			},
			expectedType: "text/event-stream",
		},
		{
			name:        "anthropic stream",
			mode:        mode.Anthropic,
			modelName:   "fake-anthropic",
			requestPath: "/v1/messages",
			requestBody: relaymodel.ClaudeAnyContentRequest{
				Model:  "fake-anthropic",
				Stream: true,
				Messages: []relaymodel.ClaudeAnyContentMessage{
					{Role: relaymodel.RoleUser, Content: "stream anthropic"},
				},
			},
			channelConfigs: model.ChannelConfigs{
				"static_text":       "streamed anthropic payload",
				"stream_chunks":     2,
				"stream_chunk_size": 10,
			},
			expectedParts: []string{
				"event: message_start",
				"event: content_block_delta",
				"streamed a",
				"event: message_stop",
			},
			expectedType: "text/event-stream",
		},
		{
			name:        "gemini stream",
			mode:        mode.Gemini,
			modelName:   "fake-gemini",
			requestPath: "/v1beta/models/fake-gemini:streamGenerateContent",
			requestBody: relaymodel.GeminiChatRequest{
				Contents: []*relaymodel.GeminiChatContent{
					{
						Role: "user",
						Parts: []*relaymodel.GeminiPart{
							{Text: "stream gemini"},
						},
					},
				},
			},
			channelConfigs: model.ChannelConfigs{
				"static_text":       "streamed gemini payload",
				"stream_chunks":     2,
				"stream_chunk_size": 7,
			},
			expectedParts: []string{
				"data: ",
				"streame",
				"\"modelVersion\":\"fake-1.0\"",
			},
			expectedType: "text/event-stream",
		},
		{
			name:        "responses stream",
			mode:        mode.Responses,
			modelName:   "fake-response",
			requestPath: "/v1/responses",
			requestBody: relaymodel.CreateResponseRequest{
				Model:  "fake-response",
				Input:  "stream responses",
				Stream: true,
			},
			channelConfigs: model.ChannelConfigs{
				"static_text":       "streamed responses payload",
				"stream_chunks":     2,
				"stream_chunk_size": 9,
			},
			responseID: "resp_stream",
			expectedParts: []string{
				"event: response.created",
				"event: response.output_text.delta",
				"event: response.output_text.done",
				"event: response.done",
				"streamed ",
			},
			expectedType: "text/event-stream",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := runFakeRequest(
				t,
				tc.mode,
				tc.modelName,
				tc.requestPath,
				tc.requestBody,
				tc.channelConfigs,
				tc.responseID,
				noopStore{},
			)

			assert.Equal(t, tc.expectedType, result.recorder.Header().Get("Content-Type"))

			body := result.recorder.Body.String()
			for _, part := range tc.expectedParts {
				assert.Contains(t, body, part)
			}
		})
	}
}

func runFakeRequest(
	t *testing.T,
	m mode.Mode,
	modelName string,
	requestPath string,
	requestBody any,
	channelConfigs model.ChannelConfigs,
	responseID string,
	store adaptor.Store,
) fakeRunResult {
	t.Helper()

	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		requestPath,
		bytes.NewReader(bodyBytes),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	metaOpts := []meta.Option{
		meta.WithRequestID("req_test_id"),
		meta.WithGroup(model.GroupCache{ID: "group-test"}),
		meta.WithToken(model.TokenCache{ID: 42}),
	}
	if responseID != "" {
		metaOpts = append(metaOpts, meta.WithResponseID(responseID))
	}

	mm := meta.NewMeta(
		&model.Channel{
			ID:      7,
			Type:    model.ChannelTypeFake,
			BaseURL: "https://fake.local/v1",
			Configs: channelConfigs,
		},
		m,
		modelName,
		model.ModelConfig{},
		metaOpts...,
	)

	a := &fake.Adaptor{}

	_, err = a.ConvertRequest(mm, store, req)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	resp, err := a.DoRequest(mm, store, c, req)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	rawBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	resp.Body = io.NopCloser(bytes.NewReader(rawBody))

	result, relayErr := a.DoResponse(mm, store, c, resp)
	require.Nil(t, relayErr)

	return fakeRunResult{
		recorder: rr,
		result:   result,
		resp:     resp,
		store:    store,
	}
}
