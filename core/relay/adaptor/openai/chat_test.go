package openai_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertChatCompletionWithToolsRequiredField(t *testing.T) {
	tests := []struct {
		name         string
		inputRequest relaymodel.GeneralOpenAIRequest
		checkFunc    func(*testing.T, relaymodel.CreateResponseRequest)
	}{
		{
			name: "tool with null required field should be removed",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Test"},
				},
				Tools: []relaymodel.Tool{
					{
						Type: "function",
						Function: relaymodel.Function{
							Name:        "test_function",
							Description: "A test function",
							Parameters: map[string]any{
								"type": "object",
								"properties": map[string]any{
									"param1": map[string]any{
										"type": "string",
									},
								},
								"required": nil,
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.Len(t, responsesReq.Tools, 1)
				assert.Equal(t, "test_function", responsesReq.Tools[0].Name)

				// Check that required field is removed
				if params, ok := responsesReq.Tools[0].Parameters.(map[string]any); ok {
					_, hasRequired := params["required"]
					assert.False(t, hasRequired, "required field should be removed when it's null")
				}
			},
		},
		{
			name: "tool with empty required array should be removed",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Test"},
				},
				Tools: []relaymodel.Tool{
					{
						Type: "function",
						Function: relaymodel.Function{
							Name:        "test_function",
							Description: "A test function",
							Parameters: map[string]any{
								"type": "object",
								"properties": map[string]any{
									"param1": map[string]any{
										"type": "string",
									},
								},
								"required": []any{},
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.Len(t, responsesReq.Tools, 1)

				// Check that required field is removed
				if params, ok := responsesReq.Tools[0].Parameters.(map[string]any); ok {
					_, hasRequired := params["required"]
					assert.False(
						t,
						hasRequired,
						"required field should be removed when it's empty array",
					)
				}
			},
		},
		{
			name: "tool with valid required array should be kept",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Test"},
				},
				Tools: []relaymodel.Tool{
					{
						Type: "function",
						Function: relaymodel.Function{
							Name:        "test_function",
							Description: "A test function",
							Parameters: map[string]any{
								"type": "object",
								"properties": map[string]any{
									"param1": map[string]any{
										"type": "string",
									},
								},
								"required": []any{"param1"},
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.Len(t, responsesReq.Tools, 1)

				// Check that required field is kept
				if params, ok := responsesReq.Tools[0].Parameters.(map[string]any); ok {
					required, hasRequired := params["required"]
					assert.True(t, hasRequired, "required field should be kept when it has values")

					requiredArray, ok := required.([]any)
					assert.True(t, ok)
					assert.Equal(t, []any{"param1"}, requiredArray)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, err := json.Marshal(tt.inputRequest)
			require.NoError(t, err)

			req, _ := http.NewRequestWithContext(context.Background(),
				http.MethodPost,
				"/v1/chat/completions",
				bytes.NewReader(reqBody),
			)
			req.Header.Set("Content-Type", "application/json")

			m := &meta.Meta{
				ActualModel: tt.inputRequest.Model,
			}

			result, err := openai.ConvertChatCompletionToResponsesRequest(m, req)
			require.NoError(t, err)

			var responsesReq relaymodel.CreateResponseRequest

			err = json.NewDecoder(result.Body).Decode(&responsesReq)
			require.NoError(t, err)

			tt.checkFunc(t, responsesReq)
		})
	}
}

func TestIsResponsesOnlyModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "gpt-5-codex should be responses only",
			model:    "gpt-5-codex",
			expected: true,
		},
		{
			name:     "gpt-5-pro should be responses only",
			model:    "gpt-5-pro",
			expected: true,
		},
		{
			name:     "gpt-4o should not be responses only",
			model:    "gpt-4o",
			expected: false,
		},
		{
			name:     "gpt-3.5-turbo should not be responses only",
			model:    "gpt-3.5-turbo",
			expected: false,
		},
		{
			name:     "gpt-5.6 should support chat completions",
			model:    "gpt-5.6",
			expected: false,
		},
		{
			name:     "gpt-5.6 sol should support chat completions",
			model:    "gpt-5.6-sol",
			expected: false,
		},
		{
			name:     "gpt-5.6 terra should support chat completions",
			model:    "gpt-5.6-terra",
			expected: false,
		},
		{
			name:     "gpt-5.6 luna should support chat completions",
			model:    "gpt-5.6-luna",
			expected: false,
		},
		{
			name:     "empty model should not be responses only",
			model:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with nil config (fallback to model name check)
			result := openai.IsResponsesOnlyModel(nil, tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsResponsesOnlyModelAny(t *testing.T) {
	t.Run("prefers origin model match", func(t *testing.T) {
		assert.True(t, openai.IsResponsesOnlyModelAny(nil, "gpt-5-codex", "mapped-model"))
	})

	t.Run("falls back to actual model match", func(t *testing.T) {
		assert.True(t, openai.IsResponsesOnlyModelAny(nil, "custom-model", "gpt-5-codex"))
	})

	t.Run("returns false when neither matches", func(t *testing.T) {
		assert.False(t, openai.IsResponsesOnlyModelAny(nil, "custom-model", "mapped-model"))
	})
}

func TestConvertChatCompletionToResponsesRequest(t *testing.T) {
	tests := []struct {
		name         string
		inputRequest relaymodel.GeneralOpenAIRequest
		checkFunc    func(*testing.T, relaymodel.CreateResponseRequest)
	}{
		{
			name: "basic request conversion",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				assert.Equal(t, "gpt-5-codex", responsesReq.Model)
				assert.NotNil(t, responsesReq.Store)
				assert.False(t, *responsesReq.Store)
				assert.Nil(t, responsesReq.Reasoning)
			},
		},
		{
			name: "reasoning effort maps effort without enabling summary",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
				ReasoningEffort: new("medium"),
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.NotNil(t, responsesReq.Reasoning)
				require.NotNil(t, responsesReq.Reasoning.Effort)
				assert.Equal(t, "medium", *responsesReq.Reasoning.Effort)
				assert.Nil(t, responsesReq.Reasoning.Summary)
			},
		},
		{
			name: "none reasoning effort does not enable reasoning summary",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
				ReasoningEffort: new("none"),
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.NotNil(t, responsesReq.Reasoning)
				assert.Nil(t, responsesReq.Reasoning.Summary)
			},
		},
		{
			name: "system messages become developer messages",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5.5",
				Messages: []relaymodel.Message{
					{Role: "system", Content: "You are concise."},
					{Role: "user", Content: "Hello"},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()

				inputItems, ok := responsesReq.Input.([]any)
				require.True(t, ok)
				require.Len(t, inputItems, 2)

				systemItem, ok := inputItems[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "developer", systemItem["role"])
			},
		},
		{
			name: "request with temperature and max_tokens",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
				Temperature: new(0.7),
				MaxTokens:   100,
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				assert.NotNil(t, responsesReq.Temperature)
				assert.Equal(t, 0.7, *responsesReq.Temperature)
				assert.NotNil(t, responsesReq.MaxOutputTokens)
				assert.Equal(t, 100, *responsesReq.MaxOutputTokens)
			},
		},
		{
			name: "request with max_completion_tokens",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
				MaxCompletionTokens: 200,
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				assert.NotNil(t, responsesReq.MaxOutputTokens)
				assert.Equal(t, 200, *responsesReq.MaxOutputTokens)
			},
		},
		{
			name: "request with tools",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "What's the weather?"},
				},
				Tools: []relaymodel.Tool{
					{
						Type: "function",
						Function: relaymodel.Function{
							Name:        "get_weather",
							Description: "Get weather information",
							Strict:      new(true),
							Parameters: map[string]any{
								"type": "object",
								"properties": map[string]any{
									"location": map[string]any{
										"type": "string",
									},
								},
							},
						},
					},
				},
				ToolChoice: "auto",
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.Len(t, responsesReq.Tools, 1)
				assert.Equal(t, "get_weather", responsesReq.Tools[0].Name)
				require.NotNil(t, responsesReq.Tools[0].Strict)
				assert.True(t, *responsesReq.Tools[0].Strict)
				assert.Equal(t, "auto", responsesReq.ToolChoice)
			},
		},
		{
			name: "flattened tool fields take precedence",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5.6",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Find the required tool"},
				},
				Tools: []relaymodel.Tool{
					{
						Type:         "function",
						Name:         "top_level_tool",
						Description:  "Top-level description",
						Parameters:   map[string]any{"type": "object"},
						Strict:       new(true),
						DeferLoading: new(true),
						Function: relaymodel.Function{
							Name:        "nested_tool",
							Description: "Nested description",
							Parameters:  map[string]any{"type": "string"},
							Strict:      new(false),
						},
					},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.Len(t, responsesReq.Tools, 1)

				tool := responsesReq.Tools[0]
				assert.Equal(t, "top_level_tool", tool.Name)
				assert.Equal(t, "Top-level description", tool.Description)
				assert.Equal(t, map[string]any{"type": "object"}, tool.Parameters)
				require.NotNil(t, tool.Strict)
				assert.True(t, *tool.Strict)
				require.NotNil(t, tool.DeferLoading)
				assert.True(t, *tool.DeferLoading)
			},
		},
		{
			name: "request with named tool choice",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "What's the weather?"},
				},
				Tools: []relaymodel.Tool{
					{
						Type: "function",
						Function: relaymodel.Function{
							Name: "get_weather",
						},
					},
				},
				ToolChoice: map[string]any{
					"type":     "function",
					"function": map[string]any{"name": "get_weather"},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				assert.Equal(
					t,
					map[string]any{"type": "function", "name": "get_weather"},
					responsesReq.ToolChoice,
				)
			},
		},
		{
			name: "request with legacy functions",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "What's the weather?"},
				},
				Functions: []any{
					map[string]any{
						"name":        "get_weather",
						"description": "Get weather information",
						"parameters": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"location": map[string]any{
									"type": "string",
								},
							},
						},
					},
				},
				FunctionCall: map[string]any{"name": "get_weather"},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.Len(t, responsesReq.Tools, 1)
				assert.Equal(t, "function", responsesReq.Tools[0].Type)
				assert.Equal(t, "get_weather", responsesReq.Tools[0].Name)
				assert.Equal(
					t,
					map[string]any{"type": "function", "name": "get_weather"},
					responsesReq.ToolChoice,
				)
			},
		},
		{
			name: "request with image content",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5.5",
				Messages: []relaymodel.Message{
					{
						Role: "user",
						Content: []any{
							map[string]any{
								"type": "text",
								"text": "What's in this image?",
							},
							map[string]any{
								"type": "image_url",
								"image_url": map[string]any{
									"url":    "https://example.com/image.png",
									"detail": "high",
								},
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()

				inputItems, ok := responsesReq.Input.([]any)
				require.True(t, ok)
				require.Len(t, inputItems, 1)

				userItem, ok := inputItems[0].(map[string]any)
				require.True(t, ok)
				content, ok := userItem["content"].([]any)
				require.True(t, ok)
				require.Len(t, content, 2)

				imageContent, ok := content[1].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "input_image", imageContent["type"])
				assert.Equal(t, "https://example.com/image.png", imageContent["image_url"])
				assert.Equal(t, "high", imageContent["detail"])
			},
		},
		{
			name: "request with response format",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Return JSON"},
				},
				ResponseFormat: &relaymodel.ResponseFormat{
					Type: "json_schema",
					JSONSchema: &relaymodel.JSONSchema{
						Name: "answer",
						Schema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"answer": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.NotNil(t, responsesReq.Text)
				assert.Equal(t, "json_schema", responsesReq.Text.Format.Type)
				assert.Equal(t, "answer", responsesReq.Text.Format.Name)
				assert.Equal(t, map[string]any{
					"type": "object",
					"properties": map[string]any{
						"answer": map[string]any{"type": "string"},
					},
				}, responsesReq.Text.Format.Schema)
			},
		},
		{
			name: "request with logprobs",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
				Logprobs: func() *bool {
					value := true
					return &value
				}(),
				TopLogprobs: func() *int {
					value := 3
					return &value
				}(),
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.NotNil(t, responsesReq.TopLogprobs)
				assert.Equal(t, 3, *responsesReq.TopLogprobs)
				assert.Contains(t, responsesReq.Include, "message.output_text.logprobs")
			},
		},
		{
			name: "request with parallel tool calls",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
				ParallelToolCalls: new(bool),
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.NotNil(t, responsesReq.ParallelToolCalls)
				assert.False(t, *responsesReq.ParallelToolCalls)
			},
		},
		{
			name: "request with service tier",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model:       "gpt-5-codex",
				ServiceTier: "priority",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.NotNil(t, responsesReq.ServiceTier)
				assert.Equal(t, "priority", *responsesReq.ServiceTier)
			},
		},
		{
			name: "request with prompt cache key",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model:          "gpt-5-codex",
				PromptCacheKey: "cache-key-1",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.NotNil(t, responsesReq.PromptCacheKey)
				assert.Equal(t, "cache-key-1", *responsesReq.PromptCacheKey)
			},
		},
		{
			name: "request with prompt cache retention and user",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model:                "gpt-5-codex",
				PromptCacheRetention: "24h",
				User:                 "user-123",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.NotNil(t, responsesReq.PromptCacheRetention)
				assert.Equal(t, "24h", *responsesReq.PromptCacheRetention)
				require.NotNil(t, responsesReq.User)
				assert.Equal(t, "user-123", *responsesReq.User)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, err := json.Marshal(tt.inputRequest)
			require.NoError(t, err)

			req, _ := http.NewRequestWithContext(context.Background(),
				http.MethodPost,
				"/v1/chat/completions",
				bytes.NewReader(reqBody),
			)
			req.Header.Set("Content-Type", "application/json")

			m := &meta.Meta{
				ActualModel: tt.inputRequest.Model,
			}

			result, err := openai.ConvertChatCompletionToResponsesRequest(m, req)
			require.NoError(t, err)

			var responsesReq relaymodel.CreateResponseRequest

			err = json.NewDecoder(result.Body).Decode(&responsesReq)
			require.NoError(t, err)

			tt.checkFunc(t, responsesReq)
		})
	}
}

func TestConvertChatCompletionToResponsesRequestPreservesClientToolSearch(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"gpt-5.6",
			"messages":[{"role":"user","content":"Find the required tool"}],
			"tools":[{
				"type":"tool_search",
				"execution":"client",
				"description":"Search deferred tool metadata",
				"parameters":{
					"type":"object",
					"properties":{"query":{"type":"string"}},
					"required":["query"],
					"additionalProperties":false
				}
			}]
		}`),
	)
	req.Header.Set("Content-Type", "application/json")

	result, err := openai.ConvertChatCompletionToResponsesRequest(&meta.Meta{
		ActualModel: "gpt-5.6",
	}, req)
	require.NoError(t, err)

	var responsesReq relaymodel.CreateResponseRequest
	require.NoError(t, json.NewDecoder(result.Body).Decode(&responsesReq))
	require.Len(t, responsesReq.Tools, 1)

	tool := responsesReq.Tools[0]
	assert.Equal(t, "tool_search", tool.Type)
	assert.Equal(t, "client", tool.Execution)
	assert.Equal(t, "Search deferred tool metadata", tool.Description)
	assert.Equal(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
		"required":             []any{"query"},
		"additionalProperties": false,
	}, tool.Parameters)
}

func TestConvertChatCompletionToResponsesRequestRejectsMultipleChoices(t *testing.T) {
	inputRequest := relaymodel.GeneralOpenAIRequest{
		Model: "gpt-5-codex",
		Messages: []relaymodel.Message{
			{Role: "user", Content: "Hello"},
		},
		N: 2,
	}
	reqBody, err := json.Marshal(inputRequest)
	require.NoError(t, err)

	req, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewReader(reqBody),
	)
	req.Header.Set("Content-Type", "application/json")

	m := &meta.Meta{
		ActualModel: inputRequest.Model,
	}

	_, err = openai.ConvertChatCompletionToResponsesRequest(m, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "n must be 1")
}

func TestConvertChatCompletionToResponsesRequestPreservesDirectFields(t *testing.T) {
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{
			"model":"gpt-5.6",
			"messages":[
				{"role":"assistant","content":"Prior answer","refusal":"Policy refusal"},
				{"role":"user","content":[{"type":"text","text":"Hello","prompt_cache_breakpoint":{"mode":"explicit"}}]}
			],
			"safety_identifier":"user_hash",
			"prompt_cache_options":{"mode":"explicit","ttl":"30m"},
			"moderation":{"model":"omni-moderation-latest","policy":{"input":{"mode":"score"}}},
			"verbosity":"high",
			"stream":true,
			"stream_options":{"include_usage":true,"include_obfuscation":false}
		}`),
	)
	req.Header.Set("Content-Type", "application/json")

	result, err := openai.ConvertChatCompletionToResponsesRequest(&meta.Meta{
		ActualModel: "gpt-5.6",
	}, req)
	require.NoError(t, err)

	var responsesReq relaymodel.CreateResponseRequest
	require.NoError(t, json.NewDecoder(result.Body).Decode(&responsesReq))

	require.NotNil(t, responsesReq.SafetyIdentifier)
	assert.Equal(t, "user_hash", *responsesReq.SafetyIdentifier)
	require.NotNil(t, responsesReq.PromptCacheOptions)
	assert.Equal(t, "explicit", responsesReq.PromptCacheOptions.Mode)
	assert.Equal(t, "30m", responsesReq.PromptCacheOptions.TTL)
	assert.Equal(t, map[string]any{
		"model": "omni-moderation-latest",
		"policy": map[string]any{
			"input": map[string]any{"mode": "score"},
		},
	}, responsesReq.Moderation)
	require.NotNil(t, responsesReq.Text)
	assert.Equal(t, "text", responsesReq.Text.Format.Type)
	assert.Equal(t, "high", responsesReq.Text.Verbosity)
	require.NotNil(t, responsesReq.StreamOptions)
	require.NotNil(t, responsesReq.StreamOptions.IncludeObfuscation)
	assert.False(t, *responsesReq.StreamOptions.IncludeObfuscation)
	require.NotNil(t, responsesReq.Store)
	assert.False(t, *responsesReq.Store)

	items, ok := responsesReq.Input.([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	assistantItem, ok := items[0].(map[string]any)
	require.True(t, ok)
	assistantContent, ok := assistantItem["content"].([]any)
	require.True(t, ok)
	require.Len(t, assistantContent, 2)
	refusalContent, ok := assistantContent[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "refusal", refusalContent["type"])
	assert.Equal(t, "Policy refusal", refusalContent["refusal"])

	userItem, ok := items[1].(map[string]any)
	require.True(t, ok)
	userContent, ok := userItem["content"].([]any)
	require.True(t, ok)
	require.Len(t, userContent, 1)
	userTextContent, ok := userContent[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"mode": "explicit"},
		userTextContent["prompt_cache_breakpoint"])
}

func TestConvertChatCompletionToResponsesRequestFlattensJSONSchemaTextFormat(t *testing.T) {
	strict := true
	inputRequest := relaymodel.GeneralOpenAIRequest{
		Model:     "gpt-5-codex",
		Verbosity: "low",
		Messages: []relaymodel.Message{
			{Role: "user", Content: "Return JSON"},
		},
		ResponseFormat: &relaymodel.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &relaymodel.JSONSchema{
				Name:        "answer",
				Description: "Answer payload",
				Strict:      &strict,
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"answer": map[string]any{"type": "string"},
					},
					"required": []any{"answer"},
				},
			},
		},
	}
	reqBody, err := json.Marshal(inputRequest)
	require.NoError(t, err)

	req, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewReader(reqBody),
	)
	req.Header.Set("Content-Type", "application/json")

	result, err := openai.ConvertChatCompletionToResponsesRequest(&meta.Meta{
		ActualModel: inputRequest.Model,
	}, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	assert.NotContains(t, string(body), `"json_schema":`)

	var raw map[string]any

	err = json.Unmarshal(body, &raw)
	require.NoError(t, err)

	text, ok := raw["text"].(map[string]any)
	require.True(t, ok)
	format, ok := text["format"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "json_schema", format["type"])
	assert.Equal(t, "answer", format["name"])
	assert.Equal(t, "Answer payload", format["description"])
	assert.Equal(t, true, format["strict"])
	assert.NotNil(t, format["schema"])
	assert.Equal(t, "low", text["verbosity"])
}

func TestConvertResponsesToChatCompletionResponse(t *testing.T) {
	tests := []struct {
		name           string
		responsesResp  relaymodel.Response
		checkFunc      func(*testing.T, relaymodel.TextResponse)
		expectedStatus int
	}{
		{
			name: "basic text response",
			responsesResp: relaymodel.Response{
				ID:        "resp_123",
				Model:     "gpt-5-codex",
				Status:    relaymodel.ResponseStatusCompleted,
				CreatedAt: 1234567890,
				Output: []relaymodel.OutputItem{
					{
						Type: "message",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "Hello, world!"},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  10,
					OutputTokens: 5,
					TotalTokens:  15,
				},
				ServiceTier: new("priority"),
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				assert.Equal(t, "resp_123", chatResp.ID)
				assert.Equal(t, "chat.completion", chatResp.Object)
				require.Len(t, chatResp.Choices, 1)
				assert.Contains(t, chatResp.Choices[0].Message.Content, "Hello, world!")
				assert.Equal(t, relaymodel.FinishReasonStop, chatResp.Choices[0].FinishReason)
				assert.Equal(t, int64(10), chatResp.Usage.PromptTokens)
				assert.Equal(t, int64(5), chatResp.Usage.CompletionTokens)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "response with reasoning (o1 models)",
			responsesResp: relaymodel.Response{
				ID:        "resp_456",
				Model:     "gpt-5-codex",
				Status:    relaymodel.ResponseStatusCompleted,
				CreatedAt: 1234567890,
				Output: []relaymodel.OutputItem{
					{
						Type: relaymodel.InputItemTypeReasoning,
						Summary: []relaymodel.SummaryPart{
							{Type: "summary_text", Text: "Need answer the math question."},
						},
					},
					{
						Type: "message",
						Role: "assistant",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "The answer is 42."},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  20,
					OutputTokens: 15,
					TotalTokens:  35,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				assert.Contains(t, chatResp.Choices[0].Message.Content, "The answer is 42.")
				assert.Equal(
					t,
					"Need answer the math question.",
					chatResp.Choices[0].Message.ReasoningContent,
				)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "message response without role defaults to assistant",
			responsesResp: relaymodel.Response{
				ID:        "resp_missing_role",
				Model:     "gpt-5-mini",
				Status:    relaymodel.ResponseStatusCompleted,
				CreatedAt: 1781355958,
				Output: []relaymodel.OutputItem{
					{
						Type: relaymodel.InputItemTypeMessage,
						Content: []relaymodel.OutputContent{
							{Type: "output_text", Text: "Hello"},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  4,
					OutputTokens: 2,
					TotalTokens:  6,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				assert.Equal(t, relaymodel.RoleAssistant, chatResp.Choices[0].Message.Role)
				assert.Equal(t, "Hello", chatResp.Choices[0].Message.Content)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "reasoning summary attaches only to first converted choice",
			responsesResp: relaymodel.Response{
				ID:        "resp_multi_reasoning",
				Model:     "gpt-5-mini",
				Status:    relaymodel.ResponseStatusCompleted,
				CreatedAt: 1781355958,
				Output: []relaymodel.OutputItem{
					{
						Type: relaymodel.InputItemTypeReasoning,
						Summary: []relaymodel.SummaryPart{
							{Type: "summary_text", Text: "Need compare options."},
						},
					},
					{
						Type: relaymodel.InputItemTypeMessage,
						Role: relaymodel.RoleAssistant,
						Content: []relaymodel.OutputContent{
							{Type: "output_text", Text: "Option A"},
						},
					},
					{
						Type: relaymodel.InputItemTypeMessage,
						Role: relaymodel.RoleAssistant,
						Content: []relaymodel.OutputContent{
							{Type: "output_text", Text: "Option B"},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  8,
					OutputTokens: 4,
					TotalTokens:  12,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				assert.Equal(
					t,
					"Need compare options.",
					chatResp.Choices[0].Message.ReasoningContent,
				)
				assert.Equal(t, "Option A\nOption B", chatResp.Choices[0].Message.Content)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "response fields and output items are preserved in one choice",
			responsesResp: relaymodel.Response{
				ID:          "resp_fields",
				Model:       "gpt-5.6",
				Status:      relaymodel.ResponseStatusCompleted,
				CreatedAt:   1781355958,
				ServiceTier: new("priority"),
				Moderation: map[string]any{
					"input": map[string]any{"type": "moderation_result", "flagged": false},
				},
				Output: []relaymodel.OutputItem{
					{
						Type: relaymodel.InputItemTypeMessage,
						Content: []relaymodel.OutputContent{
							{
								Type:        relaymodel.OutputContentTypeOutputText,
								Text:        "Partial answer",
								Annotations: []any{map[string]any{"type": "url_citation"}},
								Logprobs: []relaymodel.ChatCompletionTokenLogprob{
									{Token: "Partial", Bytes: []int{80}, Logprob: -0.1},
								},
							},
							{
								Type:    "refusal",
								Refusal: "Restricted detail",
								Logprobs: []relaymodel.ChatCompletionTokenLogprob{
									{Token: "Restricted", Bytes: []int{82}, Logprob: -0.2},
								},
							},
						},
					},
					{
						ID:        "fc_123",
						Type:      relaymodel.InputItemTypeFunctionCall,
						CallID:    "call_123",
						Name:      "lookup",
						Arguments: `{"id":1}`,
					},
				},
				Usage: &relaymodel.ResponseUsage{InputTokens: 4, OutputTokens: 2, TotalTokens: 6},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				choice := chatResp.Choices[0]
				assert.Equal(t, "Partial answer", choice.Message.Content)
				assert.Equal(t, "Restricted detail", choice.Message.Refusal)
				require.Len(t, choice.Message.Annotations, 1)
				require.NotNil(t, choice.Logprobs)
				require.Len(t, choice.Logprobs.Content, 1)
				assert.Equal(t, "Partial", choice.Logprobs.Content[0].Token)
				require.Len(t, choice.Logprobs.Refusal, 1)
				assert.Equal(t, "Restricted", choice.Logprobs.Refusal[0].Token)
				require.Len(t, choice.Message.ToolCalls, 1)
				assert.Equal(t, relaymodel.FinishReasonToolCalls, choice.FinishReason)
				require.NotNil(t, chatResp.ServiceTier)
				assert.Equal(t, "priority", *chatResp.ServiceTier)
				moderation, ok := chatResp.Moderation.(map[string]any)
				require.True(t, ok)
				moderationInput, ok := moderation["input"].(map[string]any)
				require.True(t, ok)
				flagged, ok := moderationInput["flagged"].(bool)
				require.True(t, ok)
				assert.False(t, flagged)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "refusal-only response preserves refusal logprobs",
			responsesResp: relaymodel.Response{
				ID:        "resp_refusal_logprobs",
				Model:     "gpt-5.6",
				Status:    relaymodel.ResponseStatusCompleted,
				CreatedAt: 1781355958,
				Output: []relaymodel.OutputItem{
					{
						Type: relaymodel.InputItemTypeMessage,
						Content: []relaymodel.OutputContent{
							{
								Type:    "refusal",
								Refusal: "Request refused",
								Logprobs: []relaymodel.ChatCompletionTokenLogprob{
									{Token: "Request", Bytes: []int{82}, Logprob: -0.3},
								},
							},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{InputTokens: 2, OutputTokens: 1, TotalTokens: 3},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				choice := chatResp.Choices[0]
				assert.Equal(t, "", choice.Message.Content)
				assert.Equal(t, "Request refused", choice.Message.Refusal)
				require.NotNil(t, choice.Logprobs)
				assert.Empty(t, choice.Logprobs.Content)
				require.Len(t, choice.Logprobs.Refusal, 1)
				assert.Equal(t, "Request", choice.Logprobs.Refusal[0].Token)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "incomplete reasoning-only response",
			responsesResp: relaymodel.Response{
				ID:        "resp_incomplete",
				Model:     "gpt-5-mini",
				Status:    relaymodel.ResponseStatusIncomplete,
				CreatedAt: 1781355958,
				Output: []relaymodel.OutputItem{
					{
						Type:    relaymodel.InputItemTypeReasoning,
						Summary: []relaymodel.SummaryPart{},
					},
				},
				IncompleteDetails: &relaymodel.IncompleteDetails{
					Reason: "max_output_tokens",
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  268,
					OutputTokens: 192,
					OutputTokensDetails: &relaymodel.ResponseUsageDetails{
						ReasoningTokens: 192,
					},
					TotalTokens: 460,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				choice := chatResp.Choices[0]
				assert.Equal(t, 0, choice.Index)
				assert.Equal(t, relaymodel.RoleAssistant, choice.Message.Role)
				assert.Equal(t, "", choice.Message.Content)
				assert.Equal(t, relaymodel.FinishReasonLength, choice.FinishReason)
				assert.Equal(t, int64(192), chatResp.Usage.CompletionTokens)
				require.NotNil(t, chatResp.Usage.CompletionTokensDetails)
				assert.Equal(
					t,
					int64(192),
					chatResp.Usage.CompletionTokensDetails.ReasoningTokens,
				)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "incomplete content filter response",
			responsesResp: relaymodel.Response{
				ID:        "resp_content_filter",
				Model:     "gpt-5-mini",
				Status:    relaymodel.ResponseStatusIncomplete,
				CreatedAt: 1781355958,
				IncompleteDetails: &relaymodel.IncompleteDetails{
					Reason: "content_filter",
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  12,
					OutputTokens: 3,
					TotalTokens:  15,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				assert.Equal(
					t,
					relaymodel.FinishReasonContentFilter,
					chatResp.Choices[0].FinishReason,
				)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "function call only response",
			responsesResp: relaymodel.Response{
				ID:        "resp_tool",
				Model:     "gpt-5-mini",
				Status:    relaymodel.ResponseStatusCompleted,
				CreatedAt: 1781355958,
				Output: []relaymodel.OutputItem{
					{
						ID:        "fc_123",
						Type:      relaymodel.InputItemTypeFunctionCall,
						CallID:    "call_123",
						Name:      "get_weather",
						Arguments: `{"location":"Boston"}`,
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  12,
					OutputTokens: 3,
					TotalTokens:  15,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)

				choice := chatResp.Choices[0]
				assert.Equal(t, relaymodel.FinishReasonToolCalls, choice.FinishReason)
				assert.Equal(t, relaymodel.RoleAssistant, choice.Message.Role)
				assert.Empty(t, choice.Message.Content)
				require.Len(t, choice.Message.ToolCalls, 1)

				toolCall := choice.Message.ToolCalls[0]
				assert.Equal(t, 0, toolCall.Index)
				assert.Equal(t, "call_123", toolCall.ID)
				assert.Equal(t, relaymodel.ToolChoiceTypeFunction, toolCall.Type)
				assert.Equal(t, "get_weather", toolCall.Function.Name)
				assert.Equal(t, `{"location":"Boston"}`, toolCall.Function.Arguments)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "multiple function calls share one choice",
			responsesResp: relaymodel.Response{
				ID:        "resp_tools",
				Model:     "gpt-5-mini",
				Status:    relaymodel.ResponseStatusCompleted,
				CreatedAt: 1781355958,
				Output: []relaymodel.OutputItem{
					{
						ID:        "fc_logs",
						Type:      relaymodel.InputItemTypeFunctionCall,
						CallID:    "call_logs",
						Name:      "command",
						Arguments: `{"command":"kubectl logs ..."}`,
					},
					{
						ID:        "fc_statefulset",
						Type:      relaymodel.InputItemTypeFunctionCall,
						CallID:    "call_statefulset",
						Name:      "command",
						Arguments: `{"command":"kubectl get statefulset ..."}`,
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  12,
					OutputTokens: 6,
					TotalTokens:  18,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)

				choice := chatResp.Choices[0]
				assert.Equal(t, relaymodel.FinishReasonToolCalls, choice.FinishReason)
				require.Len(t, choice.Message.ToolCalls, 2)
				assert.Equal(t, 0, choice.Message.ToolCalls[0].Index)
				assert.Equal(t, `{"command":"kubectl logs ..."}`,
					choice.Message.ToolCalls[0].Function.Arguments)
				assert.Equal(t, 1, choice.Message.ToolCalls[1].Index)
				assert.Equal(t, `{"command":"kubectl get statefulset ..."}`,
					choice.Message.ToolCalls[1].Function.Arguments)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "incomplete function call keeps incomplete finish reason",
			responsesResp: relaymodel.Response{
				ID:        "resp_tool_incomplete",
				Model:     "gpt-5-mini",
				Status:    relaymodel.ResponseStatusIncomplete,
				CreatedAt: 1781355958,
				Output: []relaymodel.OutputItem{
					{
						ID:        "fc_123",
						Type:      relaymodel.InputItemTypeFunctionCall,
						CallID:    "call_123",
						Name:      "get_weather",
						Arguments: `{"location":"Boston"}`,
					},
				},
				IncompleteDetails: &relaymodel.IncompleteDetails{
					Reason: "max_output_tokens",
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  12,
					OutputTokens: 3,
					TotalTokens:  15,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				assert.Equal(t, relaymodel.FinishReasonLength, chatResp.Choices[0].FinishReason)
				require.Len(t, chatResp.Choices[0].Message.ToolCalls, 1)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "incomplete unknown reason response",
			responsesResp: relaymodel.Response{
				ID:        "resp_unknown_incomplete",
				Model:     "gpt-5-mini",
				Status:    relaymodel.ResponseStatusIncomplete,
				CreatedAt: 1781355958,
				IncompleteDetails: &relaymodel.IncompleteDetails{
					Reason: "unknown_reason",
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  12,
					OutputTokens: 3,
					TotalTokens:  15,
				},
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				require.Len(t, chatResp.Choices, 1)
				assert.Equal(t, relaymodel.FinishReasonStop, chatResp.Choices[0].FinishReason)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			respBody, err := json.Marshal(tt.responsesResp)
			require.NoError(t, err)

			httpResp := &http.Response{
				StatusCode: tt.expectedStatus,
				Body:       &mockReadCloser{Reader: bytes.NewReader(respBody)},
				Header:     make(http.Header),
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			m := &meta.Meta{
				OriginModel: "client-gpt-5",
				ActualModel: tt.responsesResp.Model,
			}

			_, err = openai.ConvertResponsesToChatCompletionResponse(m, c, httpResp)
			require.Nil(t, err)

			var chatResp relaymodel.TextResponse

			err = json.Unmarshal(w.Body.Bytes(), &chatResp)
			require.NoError(t, err)

			assert.Equal(t, "client-gpt-5", chatResp.Model)
			tt.checkFunc(t, chatResp)
		})
	}
}

func TestConvertResponsesToChatCompletionStreamResponseSkipsOutputItemDoneContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_123","object":"response","created_at":1780731105,"status":"in_progress","model":"gpt-5.1","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`data: {"type":"response.output_item.added","item":{"id":"msg_123","type":"message","role":"assistant","content":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","item_id":"msg_123","output_index":0,"content_index":0,"delta":"Hello! What would you like to discuss or work on?"}`,
		"",
		`data: {"type":"response.output_item.done","item":{"id":"msg_123","type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello! What would you like to discuss or work on?"}]}}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_123","object":"response","created_at":1780731105,"status":"completed","model":"gpt-5.1","output":[{"id":"msg_123","type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello! What would you like to discuss or work on?"}]}],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":7,"output_tokens":22,"total_tokens":29}}}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5.1",
	}

	_, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.Nil(t, err)

	content := collectChatCompletionStreamContent(t, w.Body.String())
	assert.Equal(t, "Hello! What would you like to discuss or work on?", content)
	assert.Equal(
		t,
		1,
		strings.Count(w.Body.String(), "Hello! What would you like to discuss or work on?"),
	)
	assert.Equal(t, 1, strings.Count(w.Body.String(), "data: [DONE]"))
}

func TestConvertResponsesToChatCompletionStreamResponsePreservesResponseFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`data: {"type":"response.created","obfuscation":"created-pad","response":{"id":"resp_fields","object":"response","created_at":1780731105,"status":"in_progress","model":"gpt-5.6","service_tier":"priority","output":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"Hello","obfuscation":"text-pad","logprobs":[{"token":"Hello","bytes":[72],"logprob":-0.1,"top_logprobs":[]}]}`,
		"",
		`data: {"type":"response.refusal.delta","delta":"Restricted","obfuscation":"refusal-pad","logprobs":[{"token":"Restricted","bytes":[82],"logprob":-0.2,"top_logprobs":[]}]}`,
		"",
		`data: {"type":"response.completed","obfuscation":"completed-pad","response":{"id":"resp_fields","object":"response","created_at":1780731105,"status":"completed","model":"gpt-5.6","service_tier":"priority","moderation":{"input":{"type":"moderation_result","flagged":false}},"output":[],"usage":{"input_tokens":2,"output_tokens":2,"total_tokens":4}}}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	_, err := openai.ConvertResponsesToChatCompletionStreamResponse(&meta.Meta{
		ActualModel: "gpt-5.6",
	}, c, httpResp)
	require.Nil(t, err)

	chunks := collectChatCompletionStreamChunks(t, w.Body.String())
	require.Len(t, chunks, 4)

	for _, chunk := range chunks {
		require.NotNil(t, chunk.ServiceTier)
		assert.Equal(t, "priority", *chunk.ServiceTier)
	}

	assert.Equal(t, "created-pad", chunks[0].Obfuscation)
	assert.Equal(t, "text-pad", chunks[1].Obfuscation)
	assert.Equal(t, "refusal-pad", chunks[2].Obfuscation)
	assert.Equal(t, "completed-pad", chunks[3].Obfuscation)

	require.NotNil(t, chunks[1].Choices[0].Logprobs)
	require.Len(t, chunks[1].Choices[0].Logprobs.Content, 1)
	assert.Equal(t, "Hello", chunks[1].Choices[0].Logprobs.Content[0].Token)
	assert.Equal(t, "Restricted", chunks[2].Choices[0].Delta.Refusal)
	require.NotNil(t, chunks[2].Choices[0].Logprobs)
	require.Len(t, chunks[2].Choices[0].Logprobs.Refusal, 1)
	assert.Equal(t, "Restricted", chunks[2].Choices[0].Logprobs.Refusal[0].Token)
	require.NotNil(t, chunks[3].Usage)
	assert.Equal(t, int64(4), chunks[3].Usage.TotalTokens)
	moderation, ok := chunks[3].Moderation.(map[string]any)
	require.True(t, ok)
	moderationInput, ok := moderation["input"].(map[string]any)
	require.True(t, ok)
	flagged, ok := moderationInput["flagged"].(bool)
	require.True(t, ok)
	assert.False(t, flagged)
}

func TestConvertResponsesToChatCompletionStreamResponseReturnsErrorBeforeDownstreamWrite(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_123","object":"response","created_at":1781332973,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`event: response.failed`,
		`data: {"type":"response.failed","response":{"id":"resp_123","object":"response","created_at":1781332973,"status":"failed","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false},"sequence_number":1}`,
		"",
		`event: error`,
		`data: {"type":"error","error":{"type":"too_many_requests","code":"too_many_requests","message":"Too Many Requests","param":null},"sequence_number":2}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5-mini",
	}

	result, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusTooManyRequests, err.StatusCode())
	assert.Equal(t, "resp_123", result.UpstreamID)
	assert.Empty(t, w.Body.String())
}

func TestConvertResponsesToChatCompletionStreamResponsePreservesNumericStreamErrorStatus(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_429","object":"response","created_at":1781332973,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`event: error`,
		`data: {"type":"error","error":{"type":"too_many_requests","code":429,"message":"Too Many Requests","param":null},"sequence_number":1}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5-mini",
	}

	result, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusTooManyRequests, err.StatusCode())
	assert.Equal(t, "resp_429", result.UpstreamID)
	assert.Empty(t, w.Body.String())
}

func TestConvertResponsesToChatCompletionStreamResponseFailedWithoutErrorDoesNotMarkAsyncUsage(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_failed","object":"response","created_at":1781332973,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":true}}`,
		"",
		`event: response.in_progress`,
		`data: {"type":"response.in_progress","response":{"id":"resp_failed","object":"response","created_at":1781332973,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":true}}`,
		"",
		`event: response.failed`,
		`data: {"type":"response.failed","response":{"id":"resp_failed","object":"response","created_at":1781332973,"status":"failed","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":true},"sequence_number":2}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5-mini",
	}

	result, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusBadGateway, err.StatusCode())
	assert.Equal(t, "resp_failed", result.UpstreamID)
	assert.False(t, result.AsyncUsage)
	assert.Empty(t, w.Body.String())
}

func TestConvertResponsesToChatCompletionStreamResponseMapsInvalidRequestErrorToBadRequest(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: error`,
		`data: {"type":"error","error":{"type":"invalid_request_error","code":"bad_response","message":"System messages are not allowed","param":null},"sequence_number":1}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5.5",
	}

	result, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode())
	assert.Empty(t, result.UpstreamID)
	assert.Empty(t, w.Body.String())
}

func TestConvertResponsesToChatCompletionStreamResponseHandlesErrorAfterDownstreamWrite(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_123","object":"response","created_at":1781332973,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		"",
		`event: error`,
		`data: {"type":"error","error":{"type":"server_error","code":"server_error","message":"stream failed","param":null},"sequence_number":2}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"late"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_123","object":"response","created_at":1781332973,"status":"completed","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5-mini",
	}

	_, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.Nil(t, err)
	assert.Equal(t, "partial", collectChatCompletionStreamContent(t, w.Body.String()))
	assert.Equal(t, 1, strings.Count(w.Body.String(), "data: [DONE]"))
	assert.NotContains(t, w.Body.String(), "late")
}

func TestConvertResponsesToChatCompletionStreamResponseHandlesIncompleteReasoningOnly(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_incomplete","object":"response","created_at":1781355623,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item":{"id":"rs_1","type":"reasoning","summary":[]},"output_index":0,"sequence_number":2}`,
		"",
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"id":"rs_1","type":"reasoning","summary":[]},"output_index":0,"sequence_number":3}`,
		"",
		`event: response.incomplete`,
		`data: {"type":"response.incomplete","response":{"id":"resp_incomplete","object":"response","created_at":1781355623,"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"model":"gpt-5-mini","output":[{"id":"rs_1","type":"reasoning","summary":[]}],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":268,"output_tokens":192,"output_tokens_details":{"reasoning_tokens":192},"total_tokens":460}},"sequence_number":4}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5-mini",
	}

	result, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.Nil(t, err)
	assert.Equal(t, "resp_incomplete", result.UpstreamID)
	assert.Equal(t, int64(460), int64(result.Usage.TotalTokens))
	assert.Equal(t, int64(192), int64(result.Usage.ReasoningTokens))

	chunks := collectChatCompletionStreamChunks(t, w.Body.String())
	require.Len(t, chunks, 2)

	assert.Equal(t, relaymodel.RoleAssistant, chunks[0].Choices[0].Delta.Role)
	assert.Equal(t, relaymodel.FinishReasonLength, chunks[1].Choices[0].FinishReason)
	require.NotNil(t, chunks[1].Usage)
	assert.Equal(t, int64(192), chunks[1].Usage.CompletionTokens)
	assert.Equal(t, 1, strings.Count(w.Body.String(), "data: [DONE]"))
}

func TestConvertResponsesToChatCompletionStreamResponseMapsReasoningSummary(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_reasoning","object":"response","created_at":1781355623,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`event: response.reasoning_summary_text.delta`,
		`data: {"type":"response.reasoning_summary_text.delta","delta":"Checking facts","sequence_number":2}`,
		"",
		`event: response.reasoning_summary_text.delta`,
		`data: {"type":"response.reasoning_summary_text.delta","delta":" and constraints.","sequence_number":3}`,
		"",
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"Final answer","sequence_number":4}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_reasoning","object":"response","created_at":1781355623,"status":"completed","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":5,"output_tokens":6,"total_tokens":11}},"sequence_number":5}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5-mini",
	}

	_, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.Nil(t, err)

	chunks := collectChatCompletionStreamChunks(t, w.Body.String())
	require.Len(t, chunks, 5)

	assert.Equal(t, relaymodel.RoleAssistant, chunks[0].Choices[0].Delta.Role)
	assert.Equal(t, "Checking facts", chunks[1].Choices[0].Delta.ReasoningContent)
	assert.Equal(t, " and constraints.", chunks[2].Choices[0].Delta.ReasoningContent)
	assert.Equal(t, "Final answer", chunks[3].Choices[0].Delta.Content)
	assert.Equal(t, relaymodel.FinishReasonStop, chunks[4].Choices[0].FinishReason)
	assert.Contains(t, w.Body.String(), `"reasoning_content":"Checking facts"`)
}

func TestConvertResponsesToChatCompletionStreamResponseHandlesIncompleteContentFilter(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_content_filter","object":"response","created_at":1781355623,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`event: response.incomplete`,
		`data: {"type":"response.incomplete","response":{"id":"resp_content_filter","object":"response","created_at":1781355623,"status":"incomplete","incomplete_details":{"reason":"content_filter"},"model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":12,"output_tokens":3,"total_tokens":15}},"sequence_number":1}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5-mini",
	}

	_, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.Nil(t, err)

	chunks := collectChatCompletionStreamChunks(t, w.Body.String())
	require.Len(t, chunks, 2)
	assert.Equal(
		t,
		relaymodel.FinishReasonContentFilter,
		chunks[1].Choices[0].FinishReason,
	)
	assert.Equal(t, 1, strings.Count(w.Body.String(), "data: [DONE]"))
}

func TestConvertResponsesToChatCompletionStreamResponseUsesToolCallsFinishReason(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_tool","object":"response","created_at":1781355623,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item":{"id":"fc_123","type":"function_call","call_id":"call_123","name":"get_weather","arguments":"","status":"in_progress"},"output_index":0,"sequence_number":1}`,
		"",
		`event: response.function_call_arguments.delta`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_123","output_index":0,"delta":"{\"location\":\"Boston\"}","sequence_number":2}`,
		"",
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"id":"fc_123","type":"function_call","call_id":"call_123","name":"get_weather","arguments":"{\"location\":\"Boston\"}","status":"completed"},"output_index":0,"sequence_number":3}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_tool","object":"response","created_at":1781355623,"status":"completed","model":"gpt-5-mini","output":[{"id":"fc_123","type":"function_call","call_id":"call_123","name":"get_weather","arguments":"{\"location\":\"Boston\"}","status":"completed"}],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":12,"output_tokens":3,"total_tokens":15}},"sequence_number":4}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		ActualModel: "gpt-5-mini",
	}

	_, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.Nil(t, err)

	chunks := collectChatCompletionStreamChunks(t, w.Body.String())
	require.Len(t, chunks, 4)

	require.Len(t, chunks[1].Choices[0].Delta.ToolCalls, 1)
	assert.Equal(t, "call_123", chunks[1].Choices[0].Delta.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", chunks[1].Choices[0].Delta.ToolCalls[0].Function.Name)
	assert.Equal(
		t,
		`{"location":"Boston"}`,
		chunks[2].Choices[0].Delta.ToolCalls[0].Function.Arguments,
	)
	assert.Equal(t, relaymodel.FinishReasonToolCalls, chunks[3].Choices[0].FinishReason)
	assert.Equal(t, 1, strings.Count(w.Body.String(), "data: [DONE]"))
}

func TestConvertResponsesToChatCompletionStreamResponseKeepsParallelToolCallsSeparate(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_tools","object":"response","created_at":1781355623,"status":"in_progress","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item":{"id":"fc_logs","type":"function_call","call_id":"call_logs","name":"command","arguments":"","status":"in_progress"},"output_index":0,"sequence_number":1}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item":{"id":"fc_statefulset","type":"function_call","call_id":"call_statefulset","name":"command","arguments":"","status":"in_progress"},"output_index":1,"sequence_number":2}`,
		"",
		`event: response.function_call_arguments.delta`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_statefulset","output_index":1,"delta":"{\"command\":\"kubectl get statefulset ...\"}","sequence_number":3}`,
		"",
		`event: response.function_call_arguments.delta`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_logs","output_index":0,"delta":"{\"command\":\"kubectl logs ...\"}","sequence_number":4}`,
		"",
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"id":"fc_logs","type":"function_call","call_id":"call_logs","name":"command","arguments":"{\"command\":\"kubectl logs ...\"}","status":"completed"},"output_index":0,"sequence_number":5}`,
		"",
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"id":"fc_statefulset","type":"function_call","call_id":"call_statefulset","name":"command","arguments":"{\"command\":\"kubectl get statefulset ...\"}","status":"completed"},"output_index":1,"sequence_number":6}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_tools","object":"response","created_at":1781355623,"status":"completed","model":"gpt-5-mini","output":[],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":12,"output_tokens":6,"total_tokens":18}},"sequence_number":7}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	_, err := openai.ConvertResponsesToChatCompletionStreamResponse(
		&meta.Meta{ActualModel: "gpt-5-mini"},
		c,
		httpResp,
	)
	require.Nil(t, err)

	chunks := collectChatCompletionStreamChunks(t, w.Body.String())
	require.Len(t, chunks, 6)

	for _, chunk := range chunks {
		assert.Equal(t, int64(1781355623), chunk.Created)
	}

	assert.NotContains(t, w.Body.String(), `"id":""`)
	assert.NotContains(t, w.Body.String(), `"type":""`)

	argumentsByIndex := make(map[int]string)

	callIDsByIndex := make(map[int]string)
	for _, chunk := range chunks {
		for _, toolCall := range chunk.Choices[0].Delta.ToolCalls {
			argumentsByIndex[toolCall.Index] += toolCall.Function.Arguments
			if toolCall.ID != "" {
				callIDsByIndex[toolCall.Index] = toolCall.ID
			}
		}
	}

	assert.Equal(t, "call_logs", callIDsByIndex[0])
	assert.Equal(t, `{"command":"kubectl logs ..."}`, argumentsByIndex[0])
	assert.Equal(t, "call_statefulset", callIDsByIndex[1])
	assert.Equal(t, `{"command":"kubectl get statefulset ..."}`, argumentsByIndex[1])
	assert.Equal(t, relaymodel.FinishReasonToolCalls, chunks[5].Choices[0].FinishReason)
}

func TestConvertResponsesToChatCompletionStreamResponseUsesOriginModelForEveryChunk(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)

	stream := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_123","object":"response","created_at":1781332973,"status":"in_progress","model":"mapped-gpt-5","output":[],"parallel_tool_calls":true,"store":false}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_123","object":"response","created_at":1781332973,"status":"completed","model":"mapped-gpt-5","output":[],"parallel_tool_calls":true,"store":false,"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		"",
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader([]byte(stream))},
		Header:     make(http.Header),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)

	m := &meta.Meta{
		OriginModel: "gpt-5",
		ActualModel: "mapped-gpt-5",
	}

	_, err := openai.ConvertResponsesToChatCompletionStreamResponse(m, c, httpResp)
	require.Nil(t, err)
	assert.NotContains(t, w.Body.String(), "mapped-gpt-5")

	chunkCount := 0
	for line := range strings.SplitSeq(w.Body.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}

		var chunk relaymodel.ChatCompletionsStreamResponse
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk))
		assert.Equal(t, "gpt-5", chunk.Model)

		chunkCount++
	}

	assert.GreaterOrEqual(t, chunkCount, 2)
}

func collectChatCompletionStreamContent(t *testing.T, body string) string {
	t.Helper()

	var builder strings.Builder

	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}

		var chunk relaymodel.ChatCompletionsStreamResponse

		err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk)
		require.NoError(t, err)

		for _, choice := range chunk.Choices {
			if content, ok := choice.Delta.Content.(string); ok {
				builder.WriteString(content)
			}
		}
	}

	return builder.String()
}

func collectChatCompletionStreamChunks(
	t *testing.T,
	body string,
) []relaymodel.ChatCompletionsStreamResponse {
	t.Helper()

	var chunks []relaymodel.ChatCompletionsStreamResponse

	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}

		var chunk relaymodel.ChatCompletionsStreamResponse

		err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk)
		require.NoError(t, err)

		chunks = append(chunks, chunk)
	}

	return chunks
}

// mockReadCloser is a helper to create a ReadCloser from a Reader
type mockReadCloser struct {
	*bytes.Reader
}

func (m *mockReadCloser) Close() error {
	return nil
}

func TestConvertChatCompletionsRequest_WithToolsRequiredField(t *testing.T) {
	tests := []struct {
		name      string
		request   string
		checkFunc func(*testing.T, relaymodel.GeneralOpenAIRequest)
	}{
		{
			name: "null required field should be removed",
			request: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "Hello"}],
				"tools": [{
					"type": "function",
					"function": {
						"name": "get_weather",
						"description": "Get weather info",
						"parameters": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": null
						}
					}
				}]
			}`,
			checkFunc: func(t *testing.T, openAIReq relaymodel.GeneralOpenAIRequest) {
				t.Helper()
				require.Len(t, openAIReq.Tools, 1)

				// Check that required field is removed
				if params, ok := openAIReq.Tools[0].Function.Parameters.(map[string]any); ok {
					_, hasRequired := params["required"]
					assert.False(t, hasRequired, "required field should be removed when it's null")
				} else {
					t.Errorf(
						"Parameters should be a map, got %T",
						openAIReq.Tools[0].Function.Parameters,
					)
				}
			},
		},
		{
			name: "empty required array should be removed",
			request: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "Hello"}],
				"tools": [{
					"type": "function",
					"function": {
						"name": "get_weather",
						"description": "Get weather info",
						"parameters": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": []
						}
					}
				}]
			}`,
			checkFunc: func(t *testing.T, openAIReq relaymodel.GeneralOpenAIRequest) {
				t.Helper()
				require.Len(t, openAIReq.Tools, 1)

				// Check that required field is removed
				if params, ok := openAIReq.Tools[0].Function.Parameters.(map[string]any); ok {
					_, hasRequired := params["required"]
					assert.False(
						t,
						hasRequired,
						"required field should be removed when it's empty array",
					)
				}
			},
		},
		{
			name: "valid required array should be kept",
			request: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "Hello"}],
				"tools": [{
					"type": "function",
					"function": {
						"name": "get_weather",
						"description": "Get weather info",
						"parameters": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": ["location"]
						}
					}
				}]
			}`,
			checkFunc: func(t *testing.T, openAIReq relaymodel.GeneralOpenAIRequest) {
				t.Helper()
				require.Len(t, openAIReq.Tools, 1)

				// Check that required field is kept
				if params, ok := openAIReq.Tools[0].Function.Parameters.(map[string]any); ok {
					required, hasRequired := params["required"]
					assert.True(t, hasRequired, "required field should be kept when it has values")

					if reqArray, ok := required.([]any); ok {
						assert.Equal(t, 1, len(reqArray))
						assert.Equal(t, "location", reqArray[0])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq, _ := http.NewRequestWithContext(context.Background(),
				http.MethodPost,
				"/v1/chat/completions",
				bytes.NewReader([]byte(tt.request)),
			)
			httpReq.Header.Set("Content-Type", "application/json")

			m := &meta.Meta{
				ActualModel: "gpt-4",
			}

			result, err := openai.ConvertChatCompletionsRequest(m, httpReq, false)
			require.NoError(t, err)

			var openAIReq relaymodel.GeneralOpenAIRequest

			err = json.NewDecoder(result.Body).Decode(&openAIReq)
			require.NoError(t, err)

			tt.checkFunc(t, openAIReq)
		})
	}
}
