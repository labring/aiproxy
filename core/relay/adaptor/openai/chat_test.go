package openai_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

			req := httptest.NewRequest(
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
				assert.Equal(t, "auto", responsesReq.ToolChoice)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, err := json.Marshal(tt.inputRequest)
			require.NoError(t, err)

			req := httptest.NewRequest(
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
			},
			checkFunc: func(t *testing.T, chatResp relaymodel.TextResponse) {
				t.Helper()
				assert.Equal(t, "resp_123", chatResp.ID)
				assert.Equal(t, "gpt-5-codex", chatResp.Model)
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
						Type: "reasoning",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "Let me think about this..."},
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
				// Current implementation creates one choice per output item
				require.Len(t, chatResp.Choices, 2)
				// First choice is reasoning
				assert.Contains(
					t,
					chatResp.Choices[0].Message.Content,
					"Let me think about this...",
				)
				// Second choice is the message
				assert.Contains(t, chatResp.Choices[1].Message.Content, "The answer is 42.")
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
				ActualModel: tt.responsesResp.Model,
			}

			_, err = openai.ConvertResponsesToChatCompletionResponse(m, c, httpResp)
			require.Nil(t, err)

			var chatResp relaymodel.TextResponse

			err = json.Unmarshal(w.Body.Bytes(), &chatResp)
			require.NoError(t, err)

			tt.checkFunc(t, chatResp)
		})
	}
}

// Helper function
//
//go:fix inline
func floatPtr(f float64) *float64 {
	return new(f)
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
			httpReq := httptest.NewRequest(
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
