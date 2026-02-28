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

func TestConvertClaudeToResponsesRequest_WithToolsRequiredField(t *testing.T) {
	tests := []struct {
		name      string
		request   string
		checkFunc func(*testing.T, relaymodel.CreateResponseRequest)
	}{
		{
			name: "claude request with null required field should be removed",
			request: `{
				"model": "gpt-5-codex",
				"messages": [{"role": "user", "content": "Test"}],
				"max_tokens": 1024,
				"tools": [
					{
						"name": "get_weather",
						"description": "Get weather info",
						"input_schema": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": null
						}
					}
				]
			}`,
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.Len(t, responsesReq.Tools, 1)
				assert.Equal(t, "get_weather", responsesReq.Tools[0].Name)

				// Check that required field is removed
				if params, ok := responsesReq.Tools[0].Parameters.(map[string]any); ok {
					_, hasRequired := params["required"]
					assert.False(t, hasRequired, "required field should be removed when it's null")
				} else {
					t.Errorf("Parameters should be a map, got %T", responsesReq.Tools[0].Parameters)
				}
			},
		},
		{
			name: "claude request with empty required array should be removed",
			request: `{
				"model": "gpt-5-codex",
				"messages": [{"role": "user", "content": "Test"}],
				"max_tokens": 1024,
				"tools": [
					{
						"name": "get_weather",
						"description": "Get weather info",
						"input_schema": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": []
						}
					}
				]
			}`,
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
			name: "claude request with valid required array should be kept",
			request: `{
				"model": "gpt-5-codex",
				"messages": [{"role": "user", "content": "Test"}],
				"max_tokens": 1024,
				"tools": [
					{
						"name": "get_weather",
						"description": "Get weather info",
						"input_schema": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": ["location"]
						}
					}
				]
			}`,
			checkFunc: func(t *testing.T, responsesReq relaymodel.CreateResponseRequest) {
				t.Helper()
				require.Len(t, responsesReq.Tools, 1)

				// Check that required field is kept
				if params, ok := responsesReq.Tools[0].Parameters.(map[string]any); ok {
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
				"/v1/messages",
				bytes.NewReader([]byte(tt.request)),
			)
			httpReq.Header.Set("Content-Type", "application/json")

			m := &meta.Meta{
				ActualModel: "gpt-5-codex",
			}

			result, err := openai.ConvertClaudeToResponsesRequest(m, httpReq)
			require.NoError(t, err)

			var responsesReq relaymodel.CreateResponseRequest

			err = json.NewDecoder(result.Body).Decode(&responsesReq)
			require.NoError(t, err)

			tt.checkFunc(t, responsesReq)
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
			assert.Equal(t, tt.responsesResp.Usage.InputTokens, int64(usage.Usage.InputTokens))
		})
	}
}

func TestConvertClaudeToolsToOpenAI_WithRequiredField(t *testing.T) {
	tests := []struct {
		name      string
		tools     []relaymodel.ClaudeTool
		checkFunc func(*testing.T, []relaymodel.Tool)
	}{
		{
			name: "null required field should be removed",
			tools: []relaymodel.ClaudeTool{
				{
					Name:        "get_weather",
					Description: "Get weather info",
					InputSchema: &relaymodel.ClaudeInputSchema{
						Type: "object",
						Properties: map[string]any{
							"location": map[string]any{"type": "string"},
						},
						Required: nil,
					},
				},
			},
			checkFunc: func(t *testing.T, tools []relaymodel.Tool) {
				t.Helper()
				require.Len(t, tools, 1)
				assert.Equal(t, "get_weather", tools[0].Function.Name)

				// Check that required field is removed
				if params, ok := tools[0].Function.Parameters.(map[string]any); ok {
					_, hasRequired := params["required"]
					assert.False(t, hasRequired, "required field should be removed when it's null")
				} else {
					t.Errorf("Parameters should be a map, got %T", tools[0].Function.Parameters)
				}
			},
		},
		{
			name: "empty required array should be removed",
			tools: []relaymodel.ClaudeTool{
				{
					Name:        "get_weather",
					Description: "Get weather info",
					InputSchema: &relaymodel.ClaudeInputSchema{
						Type: "object",
						Properties: map[string]any{
							"location": map[string]any{"type": "string"},
						},
						Required: []string{},
					},
				},
			},
			checkFunc: func(t *testing.T, tools []relaymodel.Tool) {
				t.Helper()
				require.Len(t, tools, 1)

				// Check that required field is removed
				if params, ok := tools[0].Function.Parameters.(map[string]any); ok {
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
			tools: []relaymodel.ClaudeTool{
				{
					Name:        "get_weather",
					Description: "Get weather info",
					InputSchema: &relaymodel.ClaudeInputSchema{
						Type: "object",
						Properties: map[string]any{
							"location": map[string]any{"type": "string"},
						},
						Required: []string{"location"},
					},
				},
			},
			checkFunc: func(t *testing.T, tools []relaymodel.Tool) {
				t.Helper()
				require.Len(t, tools, 1)

				// Check that required field is kept
				if params, ok := tools[0].Function.Parameters.(map[string]any); ok {
					required, hasRequired := params["required"]
					assert.True(t, hasRequired, "required field should be kept when it has values")

					if reqArray, ok := required.([]string); ok {
						assert.Equal(t, 1, len(reqArray))
						assert.Equal(t, "location", reqArray[0])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := openai.ConvertClaudeToolsToOpenAI(tt.tools)
			tt.checkFunc(t, result)
		})
	}
}
