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

					if reqArray, ok := required.([]any); ok {
						assert.Len(t, reqArray, 1)
						assert.Equal(t, "param1", reqArray[0])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, err := json.Marshal(tt.inputRequest)
			require.NoError(t, err)

			httpReq := httptest.NewRequest(
				http.MethodPost,
				"/v1/chat/completions",
				bytes.NewReader(reqBody),
			)
			httpReq.Header.Set("Content-Type", "application/json")

			m := &meta.Meta{
				ActualModel: "gpt-5-codex",
			}

			result, err := openai.ConvertChatCompletionToResponsesRequest(m, httpReq)
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
		name           string
		inputRequest   relaymodel.GeneralOpenAIRequest
		expectedModel  string
		expectedStream bool
		expectedStore  bool
	}{
		{
			name: "basic chat completion request",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
				Stream: false,
			},
			expectedModel:  "gpt-5-codex",
			expectedStream: false,
			expectedStore:  false,
		},
		{
			name: "streaming chat completion request",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Hello"},
				},
				Stream: true,
			},
			expectedModel:  "gpt-5-codex",
			expectedStream: true,
			expectedStore:  false,
		},
		{
			name: "chat completion with tools",
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
				Stream: false,
			},
			expectedModel:  "gpt-5-codex",
			expectedStream: false,
			expectedStore:  false,
		},
		{
			name: "chat completion with tool role messages",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "user", Content: "Run a command"},
					{Role: "tool", Content: `{"result": "success"}`},
				},
				Stream: false,
			},
			expectedModel:  "gpt-5-codex",
			expectedStream: false,
			expectedStore:  false,
		},
		{
			name: "chat completion with assistant messages",
			inputRequest: relaymodel.GeneralOpenAIRequest{
				Model: "gpt-5-codex",
				Messages: []relaymodel.Message{
					{Role: "system", Content: "You are a helpful assistant"},
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi! How can I help you?"},
					{Role: "user", Content: "Tell me about Go"},
				},
				Stream: false,
			},
			expectedModel:  "gpt-5-codex",
			expectedStream: false,
			expectedStore:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare request
			reqBody, err := json.Marshal(tt.inputRequest)
			require.NoError(t, err)

			httpReq := httptest.NewRequest(
				http.MethodPost,
				"/v1/chat/completions",
				bytes.NewReader(reqBody),
			)
			httpReq.Header.Set("Content-Type", "application/json")

			// Create meta
			m := &meta.Meta{
				ActualModel: tt.expectedModel,
			}

			// Convert
			result, err := openai.ConvertChatCompletionToResponsesRequest(m, httpReq)
			require.NoError(t, err)

			// Parse result
			var responsesReq relaymodel.CreateResponseRequest

			err = json.NewDecoder(result.Body).Decode(&responsesReq)
			require.NoError(t, err)

			// Verify
			assert.Equal(t, tt.expectedModel, responsesReq.Model)
			assert.Equal(t, tt.expectedStream, responsesReq.Stream)
			assert.NotNil(t, responsesReq.Store)
			assert.Equal(t, tt.expectedStore, *responsesReq.Store)

			// Verify input is an array
			if inputArray, ok := responsesReq.Input.([]any); ok {
				assert.Equal(t, len(tt.inputRequest.Messages), len(inputArray))

				// Verify role mapping and content types
				for i, msg := range tt.inputRequest.Messages {
					inputItem, ok := inputArray[i].(map[string]any)
					require.True(t, ok, "Input item should be a map")

					// Check tool role mapping
					if msg.Role == "tool" {
						assert.Equal(
							t,
							"user",
							inputItem["role"],
							"Tool role should be mapped to user",
						)
					}

					// Check content type based on role
					if content, ok := inputItem["content"].([]any); ok && len(content) > 0 {
						if contentItem, ok := content[0].(map[string]any); ok {
							expectedType := "input_text"
							if msg.Role == "assistant" {
								expectedType = "output_text"
							}

							assert.Equal(
								t,
								expectedType,
								contentItem["type"],
								"Message with role %s should use content type %s",
								msg.Role,
								expectedType,
							)
						}
					}
				}
			} else {
				t.Errorf("Input is not an array, got type %T", responsesReq.Input)
			}

			// Verify tools if present
			if len(tt.inputRequest.Tools) > 0 {
				assert.Equal(t, len(tt.inputRequest.Tools), len(responsesReq.Tools))
				assert.Equal(t, tt.inputRequest.Tools[0].Function.Name, responsesReq.Tools[0].Name)
			}
		})
	}
}

func TestConvertClaudeToResponsesRequest(t *testing.T) {
	tests := []struct {
		name             string
		inputRequest     string
		expectedModel    string
		expectedMessages int
		validateContent  bool
	}{
		{
			name: "basic claude request",
			inputRequest: `{
				"model": "gpt-5-codex",
				"messages": [
					{"role": "user", "content": "Hello"}
				],
				"max_tokens": 1024
			}`,
			expectedModel:    "gpt-5-codex",
			expectedMessages: 1,
			validateContent:  true,
		},
		{
			name: "claude request with multiple content blocks",
			inputRequest: `{
				"model": "gpt-5-codex",
				"messages": [
					{
						"role": "user",
						"content": [
							{"type": "text", "text": "First part of message"},
							{"type": "text", "text": "Second part of message"},
							{"type": "text", "text": "Third part of message"}
						]
					}
				],
				"max_tokens": 1024
			}`,
			expectedModel:    "gpt-5-codex",
			expectedMessages: 1,
			validateContent:  true,
		},
		{
			name: "claude request with system and user messages",
			inputRequest: `{
				"model": "gpt-5-codex",
				"system": [
					{"type": "text", "text": "You are a helpful assistant."}
				],
				"messages": [
					{
						"role": "user",
						"content": [
							{"type": "text", "text": "Hello, how are you?"}
						]
					}
				],
				"max_tokens": 1024
			}`,
			expectedModel:    "gpt-5-codex",
			expectedMessages: 2,
			validateContent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq := httptest.NewRequest(
				http.MethodPost,
				"/v1/messages",
				bytes.NewReader([]byte(tt.inputRequest)),
			)
			httpReq.Header.Set("Content-Type", "application/json")

			m := &meta.Meta{
				ActualModel: tt.expectedModel,
			}

			result, err := openai.ConvertClaudeToResponsesRequest(m, httpReq)
			require.NoError(t, err)

			var responsesReq relaymodel.CreateResponseRequest

			err = json.NewDecoder(result.Body).Decode(&responsesReq)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedModel, responsesReq.Model)
			assert.NotNil(t, responsesReq.Store)
			assert.False(t, *responsesReq.Store)

			// Verify input structure
			if inputArray, ok := responsesReq.Input.([]any); ok {
				assert.Equal(
					t,
					tt.expectedMessages,
					len(inputArray),
					"Should have expected number of messages",
				)

				// Validate that all messages have content
				if tt.validateContent {
					for i, item := range inputArray {
						inputItem, ok := item.(map[string]any)
						require.True(t, ok, "Input item %d should be a map", i)

						// Every message should have a content field
						content, hasContent := inputItem["content"]
						assert.True(t, hasContent, "Message %d should have content field", i)

						// Content should be a non-empty array
						if contentArray, ok := content.([]any); ok {
							assert.NotEmpty(
								t,
								contentArray,
								"Message %d content should not be empty",
								i,
							)

							// Each content item should have text
							for j, contentItem := range contentArray {
								if contentMap, ok := contentItem.(map[string]any); ok {
									text, hasText := contentMap["text"]
									assert.True(
										t,
										hasText,
										"Message %d content item %d should have text",
										i,
										j,
									)
									assert.NotEmpty(
										t,
										text,
										"Message %d content item %d text should not be empty",
										i,
										j,
									)
								}
							}
						} else {
							t.Errorf(
								"Message %d content is not an array, got type %T",
								i,
								content,
							)
						}
					}
				}
			} else {
				t.Errorf("Input is not an array, got type %T", responsesReq.Input)
			}
		})
	}
}

func TestConvertResponsesToChatCompletionResponse(t *testing.T) {
	tests := []struct {
		name            string
		responsesResp   relaymodel.Response
		expectedObject  string
		expectedContent string
		expectedUsage   bool
		expectedFinish  relaymodel.FinishReason
	}{
		{
			name: "completed response",
			responsesResp: relaymodel.Response{
				ID:        "resp_123",
				Model:     "gpt-5-codex",
				CreatedAt: 1234567890,
				Status:    relaymodel.ResponseStatusCompleted,
				Output: []relaymodel.OutputItem{
					{
						Role: "assistant",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "Hello! How can I help you?"},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  10,
					OutputTokens: 20,
					TotalTokens:  30,
				},
			},
			expectedObject:  "chat.completion",
			expectedContent: "Hello! How can I help you?",
			expectedUsage:   true,
			expectedFinish:  relaymodel.FinishReasonStop,
		},
		{
			name: "incomplete response",
			responsesResp: relaymodel.Response{
				ID:        "resp_456",
				Model:     "gpt-5-codex",
				CreatedAt: 1234567890,
				Status:    relaymodel.ResponseStatusIncomplete,
				Output: []relaymodel.OutputItem{
					{
						Role: "assistant",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "This is a long response that was cut off..."},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  10,
					OutputTokens: 100,
					TotalTokens:  110,
				},
			},
			expectedObject:  "chat.completion",
			expectedContent: "This is a long response that was cut off...",
			expectedUsage:   true,
			expectedFinish:  relaymodel.FinishReasonLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock response
			respBody, err := json.Marshal(tt.responsesResp)
			require.NoError(t, err)

			httpResp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
			}
			httpResp.Body = &mockReadCloser{Reader: bytes.NewReader(respBody)}

			// Create gin context
			gin.SetMode(gin.TestMode)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			m := &meta.Meta{
				ActualModel: tt.responsesResp.Model,
			}

			// Convert
			usage, err := openai.ConvertResponsesToChatCompletionResponse(m, c, httpResp)
			require.Nil(t, err)

			// Parse response
			var chatResp relaymodel.TextResponse

			err = json.Unmarshal(w.Body.Bytes(), &chatResp)
			require.NoError(t, err)

			// Verify
			assert.Equal(t, tt.expectedObject, chatResp.Object)
			assert.Equal(t, tt.responsesResp.ID, chatResp.ID)
			assert.Equal(t, tt.responsesResp.Model, chatResp.Model)
			assert.NotEmpty(t, chatResp.Choices)

			choice := chatResp.Choices[0]
			assert.Equal(t, tt.expectedFinish, choice.FinishReason)

			if tt.expectedContent != "" {
				assert.Contains(t, choice.Message.Content, tt.expectedContent)
			}

			if tt.expectedUsage {
				assert.NotNil(t, usage)
				assert.Equal(t, tt.responsesResp.Usage.InputTokens, int64(usage.InputTokens))
				assert.Equal(t, tt.responsesResp.Usage.OutputTokens, int64(usage.OutputTokens))
			}
		})
	}
}

func TestConvertResponsesToClaudeResponse(t *testing.T) {
	tests := []struct {
		name             string
		responsesResp    relaymodel.Response
		expectedType     string
		expectedRole     string
		expectedContent  string
		hasReasoning     bool
		expectedThinking string
	}{
		{
			name: "basic claude response",
			responsesResp: relaymodel.Response{
				ID:        "resp_789",
				Model:     "gpt-5-codex",
				CreatedAt: 1234567890,
				Status:    relaymodel.ResponseStatusCompleted,
				Output: []relaymodel.OutputItem{
					{
						Role: "assistant",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "I'm Claude, how can I help?"},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  15,
					OutputTokens: 25,
					TotalTokens:  40,
				},
			},
			expectedType:    "message",
			expectedRole:    "assistant",
			expectedContent: "I'm Claude, how can I help?",
			hasReasoning:    false,
		},
		{
			name: "claude response with reasoning",
			responsesResp: relaymodel.Response{
				ID:        "resp_reasoning_123",
				Model:     "gpt-5-codex",
				CreatedAt: 1234567890,
				Status:    relaymodel.ResponseStatusCompleted,
				Output: []relaymodel.OutputItem{
					{
						Type: "reasoning",
						Content: []relaymodel.OutputContent{
							{Type: "output_text", Text: "Let me think about this carefully..."},
						},
					},
					{
						Role: "assistant",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "Here's my answer!"},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  20,
					OutputTokens: 35,
					TotalTokens:  55,
				},
			},
			expectedType:     "message",
			expectedRole:     "assistant",
			expectedContent:  "Here's my answer!",
			hasReasoning:     true,
			expectedThinking: "Let me think about this carefully...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock response
			respBody, err := json.Marshal(tt.responsesResp)
			require.NoError(t, err)

			httpResp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
			}
			httpResp.Body = &mockReadCloser{Reader: bytes.NewReader(respBody)}

			// Create gin context
			gin.SetMode(gin.TestMode)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			m := &meta.Meta{
				ActualModel: tt.responsesResp.Model,
			}

			// Convert
			usage, err := openai.ConvertResponsesToClaudeResponse(m, c, httpResp)
			require.Nil(t, err)

			// Parse response
			var claudeResp relaymodel.ClaudeResponse

			err = json.Unmarshal(w.Body.Bytes(), &claudeResp)
			require.NoError(t, err)

			// Verify
			assert.Equal(t, tt.expectedType, claudeResp.Type)
			assert.Equal(t, tt.expectedRole, claudeResp.Role)
			assert.NotEmpty(t, claudeResp.Content)

			if tt.hasReasoning {
				// Should have at least 2 content blocks (thinking + text)
				assert.GreaterOrEqual(t, len(claudeResp.Content), 2)

				// First block should be thinking
				thinkingBlock := claudeResp.Content[0]
				assert.Equal(t, "thinking", thinkingBlock.Type)
				assert.Equal(t, tt.expectedThinking, thinkingBlock.Thinking)

				// Second block should be text
				textBlock := claudeResp.Content[1]
				assert.Equal(t, "text", textBlock.Type)
				assert.Equal(t, tt.expectedContent, textBlock.Text)
			} else {
				assert.Equal(t, "text", claudeResp.Content[0].Type)
				assert.Equal(t, tt.expectedContent, claudeResp.Content[0].Text)
			}

			assert.NotNil(t, usage)
			assert.Equal(t, tt.responsesResp.Usage.InputTokens, int64(usage.InputTokens))
		})
	}
}

func TestConvertResponsesToGeminiResponse(t *testing.T) {
	tests := []struct {
		name            string
		responsesResp   relaymodel.Response
		expectedContent string
		expectedFinish  string
		hasReasoning    bool
	}{
		{
			name: "basic gemini response",
			responsesResp: relaymodel.Response{
				ID:        "resp_gemini_123",
				Model:     "gpt-5-codex",
				CreatedAt: 1234567890,
				Status:    relaymodel.ResponseStatusCompleted,
				Output: []relaymodel.OutputItem{
					{
						Role: "assistant",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "Hello from Gemini!"},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  12,
					OutputTokens: 18,
					TotalTokens:  30,
				},
			},
			expectedContent: "Hello from Gemini!",
			expectedFinish:  "STOP",
			hasReasoning:    false,
		},
		{
			name: "gemini response with reasoning",
			responsesResp: relaymodel.Response{
				ID:        "resp_gemini_reasoning",
				Model:     "gpt-5-codex",
				CreatedAt: 1234567890,
				Status:    relaymodel.ResponseStatusCompleted,
				Output: []relaymodel.OutputItem{
					{
						Type: "reasoning",
						Content: []relaymodel.OutputContent{
							{Type: "output_text", Text: "Let me think about this..."},
						},
					},
					{
						Role: "assistant",
						Content: []relaymodel.OutputContent{
							{Type: "text", Text: "Here's my answer!"},
						},
					},
				},
				Usage: &relaymodel.ResponseUsage{
					InputTokens:  12,
					OutputTokens: 18,
					TotalTokens:  30,
				},
			},
			expectedContent: "Here's my answer!",
			expectedFinish:  "STOP",
			hasReasoning:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock response
			respBody, err := json.Marshal(tt.responsesResp)
			require.NoError(t, err)

			httpResp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
			}
			httpResp.Body = &mockReadCloser{Reader: bytes.NewReader(respBody)}

			// Create gin context
			gin.SetMode(gin.TestMode)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			m := &meta.Meta{
				ActualModel: tt.responsesResp.Model,
			}

			// Convert
			usage, err := openai.ConvertResponsesToGeminiResponse(m, c, httpResp)
			require.Nil(t, err)

			// Parse response
			var geminiResp relaymodel.GeminiChatResponse

			err = json.Unmarshal(w.Body.Bytes(), &geminiResp)
			require.NoError(t, err)

			// Verify
			assert.Equal(t, tt.responsesResp.Model, geminiResp.ModelVersion)
			assert.NotEmpty(t, geminiResp.Candidates)

			if tt.hasReasoning {
				// Should have multiple candidates
				assert.GreaterOrEqual(t, len(geminiResp.Candidates), 2)

				// First candidate should be reasoning (thought)
				reasoningCandidate := geminiResp.Candidates[0]
				assert.NotEmpty(t, reasoningCandidate.Content.Parts)
				assert.True(t, reasoningCandidate.Content.Parts[0].Thought)
				assert.Equal(
					t,
					"Let me think about this...",
					reasoningCandidate.Content.Parts[0].Text,
				)

				// Second candidate should be the actual response
				answerCandidate := geminiResp.Candidates[1]
				assert.Equal(t, tt.expectedFinish, answerCandidate.FinishReason)
				assert.NotEmpty(t, answerCandidate.Content.Parts)
				assert.Equal(t, tt.expectedContent, answerCandidate.Content.Parts[0].Text)
			} else {
				candidate := geminiResp.Candidates[0]
				assert.Equal(t, tt.expectedFinish, candidate.FinishReason)
				assert.NotEmpty(t, candidate.Content.Parts)
				assert.Equal(t, tt.expectedContent, candidate.Content.Parts[0].Text)
			}

			assert.NotNil(t, usage)
			assert.Equal(t, tt.responsesResp.Usage.TotalTokens, int64(usage.TotalTokens))
		})
	}
}

// mockReadCloser is a helper to create a ReadCloser from a Reader
type mockReadCloser struct {
	*bytes.Reader
}

func (m *mockReadCloser) Close() error {
	return nil
}
