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

func TestConvertGeminiToResponsesRequest_WithFunctionCalls(t *testing.T) {
	// Create a Gemini request with function call and response
	geminiReq := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": "使用axum实现一个简单的http api"},
				},
			},
			{
				"role": "model",
				"parts": []map[string]any{
					{
						"functionCall": map[string]any{
							"name": "read_file",
							"args": map[string]any{
								"file_path": "Cargo.toml",
								"offset":    0,
								"limit":     400,
							},
						},
					},
				},
			},
			{
				"role": "user",
				"parts": []map[string]any{
					{
						"functionResponse": map[string]any{
							"id":   "read_file-123",
							"name": "read_file",
							"response": map[string]any{
								"output": "[package]\nname = \"test-axum\"\nversion = \"0.1.0\"\n",
							},
						},
					},
				},
			},
		},
		"systemInstruction": map[string]any{
			"parts": []map[string]any{
				{"text": "You are a helpful assistant."},
			},
		},
	}

	reqBody, err := json.Marshal(geminiReq)
	require.NoError(t, err)

	httpReq := httptest.NewRequest(
		http.MethodPost,
		"/v1beta/models/gpt-5-codex:streamGenerateContent",
		bytes.NewReader(reqBody),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	m := &meta.Meta{
		ActualModel: "gpt-5-codex",
	}

	// Convert
	result, err := openai.ConvertGeminiToResponsesRequest(m, httpReq)
	require.NoError(t, err)

	// Parse result
	var responsesReq relaymodel.CreateResponseRequest

	err = json.NewDecoder(result.Body).Decode(&responsesReq)
	require.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, "gpt-5-codex", responsesReq.Model)
	assert.True(t, responsesReq.Stream)

	// Verify input structure
	inputArray, ok := responsesReq.Input.([]any)
	require.True(t, ok, "Input should be an array")
	require.Equal(
		t,
		4,
		len(inputArray),
		"Should have 4 items: system message, user message, function call, function result",
	)

	// Verify system message
	systemMsg, ok := inputArray[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "message", systemMsg["type"])
	assert.Equal(t, "system", systemMsg["role"])

	// Verify user message
	userMsg, ok := inputArray[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "message", userMsg["type"])
	assert.Equal(t, "user", userMsg["role"])
	userContent, ok := userMsg["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, userContent)
	userContentItem, ok := userContent[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_text", userContentItem["type"])
	assert.Contains(t, userContentItem["text"], "axum")

	// Verify function call item (separate item, not content within a message)
	functionCallItem, ok := inputArray[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function_call", functionCallItem["type"], "Item type should be function_call")
	assert.Equal(t, "read_file", functionCallItem["name"], "Function name should be read_file")
	assert.NotEmpty(t, functionCallItem["call_id"], "Function call should have a call_id")
	assert.NotEmpty(t, functionCallItem["arguments"], "Function call should have arguments")

	// Verify arguments contain expected data
	var args map[string]any

	argsStr, ok := functionCallItem["arguments"].(string)
	require.True(t, ok)

	err = json.Unmarshal([]byte(argsStr), &args)
	require.NoError(t, err)
	assert.Equal(t, "Cargo.toml", args["file_path"])
	assert.Equal(t, float64(0), args["offset"])
	assert.Equal(t, float64(400), args["limit"])

	// Verify function call output item (separate item, not content within a message)
	functionResultItem, ok := inputArray[3].(map[string]any)
	require.True(t, ok)
	assert.Equal(
		t,
		"function_call_output",
		functionResultItem["type"],
		"Item type should be function_call_output",
	)
	assert.NotEmpty(t, functionResultItem["call_id"], "Function call output should have call_id")
	assert.Contains(
		t,
		functionResultItem["output"],
		"test-axum",
		"Function call output should contain output",
	)
	// Verify that function_call_output does NOT have a name field
	_, hasName := functionResultItem["name"]
	assert.False(t, hasName, "Function call output should NOT have a name field")

	// Verify that function_call and function_call_output have matching call_id
	assert.Equal(
		t,
		functionCallItem["call_id"],
		functionResultItem["call_id"],
		"Function call and output should have matching call_id",
	)
}

func TestConvertGeminiToResponsesRequest_WithToolsRequiredField(t *testing.T) {
	// Create a Gemini request with tools that have null required field
	geminiReq := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": "Get diagnostics"},
				},
			},
		},
		"tools": []map[string]any{
			{
				"functionDeclarations": []map[string]any{
					{
						"name":        "getDiagnostics",
						"description": "Get diagnostics",
						"parameters": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"uri": map[string]any{
									"type": "string",
								},
							},
							"required": nil, // This should be removed
						},
					},
					{
						"name":        "getFile",
						"description": "Get file content",
						"parameters": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"path": map[string]any{
									"type": "string",
								},
							},
							"required": []any{}, // This should be removed
						},
					},
					{
						"name":        "writeFile",
						"description": "Write to file",
						"parameters": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"path": map[string]any{
									"type": "string",
								},
								"content": map[string]any{
									"type": "string",
								},
							},
							"required": []any{"path", "content"}, // This should be kept
						},
					},
				},
			},
		},
	}

	reqBody, err := json.Marshal(geminiReq)
	require.NoError(t, err)

	httpReq := httptest.NewRequest(
		http.MethodPost,
		"/v1beta/models/gpt-5-codex:streamGenerateContent",
		bytes.NewReader(reqBody),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	m := &meta.Meta{
		ActualModel: "gpt-5-codex",
	}

	// Convert
	result, err := openai.ConvertGeminiToResponsesRequest(m, httpReq)
	require.NoError(t, err)

	// Parse result
	var responsesReq relaymodel.CreateResponseRequest

	err = json.NewDecoder(result.Body).Decode(&responsesReq)
	require.NoError(t, err)

	// Verify we have 3 tools
	require.Len(t, responsesReq.Tools, 3)

	// Verify first tool: required field should be removed (was null)
	tool1 := responsesReq.Tools[0]
	assert.Equal(t, "getDiagnostics", tool1.Name)

	if params, ok := tool1.Parameters.(map[string]any); ok {
		_, hasRequired := params["required"]
		assert.False(t, hasRequired, "Tool 1: required field should be removed when it's null")
	}

	// Verify second tool: required field should be removed (was empty array)
	tool2 := responsesReq.Tools[1]
	assert.Equal(t, "getFile", tool2.Name)

	if params, ok := tool2.Parameters.(map[string]any); ok {
		_, hasRequired := params["required"]
		assert.False(
			t,
			hasRequired,
			"Tool 2: required field should be removed when it's empty array",
		)
	}

	// Verify third tool: required field should be kept (has values)
	tool3 := responsesReq.Tools[2]
	assert.Equal(t, "writeFile", tool3.Name)

	if params, ok := tool3.Parameters.(map[string]any); ok {
		required, hasRequired := params["required"]
		assert.True(t, hasRequired, "Tool 3: required field should be kept when it has values")

		if reqArray, ok := required.([]any); ok {
			assert.Len(t, reqArray, 2)
			assert.Contains(t, reqArray, "path")
			assert.Contains(t, reqArray, "content")
		}
	}
}

func TestConvertGeminiToResponsesRequest_WithoutFunctionCalls(t *testing.T) {
	// Create a simple Gemini request without function calls
	geminiReq := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": "Hello, how are you?"},
				},
			},
		},
	}

	reqBody, err := json.Marshal(geminiReq)
	require.NoError(t, err)

	httpReq := httptest.NewRequest(
		http.MethodPost,
		"/v1beta/models/gpt-5-codex:streamGenerateContent",
		bytes.NewReader(reqBody),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	m := &meta.Meta{
		ActualModel: "gpt-5-codex",
	}

	// Convert
	result, err := openai.ConvertGeminiToResponsesRequest(m, httpReq)
	require.NoError(t, err)

	// Parse result
	var responsesReq relaymodel.CreateResponseRequest

	err = json.NewDecoder(result.Body).Decode(&responsesReq)
	require.NoError(t, err)

	// Verify input structure
	inputArray, ok := responsesReq.Input.([]any)
	require.True(t, ok)
	require.Len(t, inputArray, 1)

	// Verify user message
	userMsg, ok := inputArray[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", userMsg["role"])
	userContent, ok := userMsg["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, userContent)

	contentItem, ok := userContent[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_text", contentItem["type"])
	assert.Equal(t, "Hello, how are you?", contentItem["text"])
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
