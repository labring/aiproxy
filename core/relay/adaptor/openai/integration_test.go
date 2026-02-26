package openai_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationChatCompletionToResponsesFlow tests the complete flow:
// 1. ChatCompletion request -> Responses API request conversion
// 2. Mock Responses API response
// 3. Responses API response -> ChatCompletion response conversion
func TestIntegrationChatCompletionToResponsesFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Step 1: Create ChatCompletion request
	chatReq := relaymodel.GeneralOpenAIRequest{
		Model: "gpt-5-codex",
		Messages: []relaymodel.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What is 2+2?"},
		},
		Temperature: floatPtr(0.7),
		MaxTokens:   100,
	}

	reqBody, err := json.Marshal(chatReq)
	require.NoError(t, err)

	httpReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewReader(reqBody),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	m := &meta.Meta{
		ActualModel: "gpt-5-codex",
		Mode:        mode.ChatCompletions,
	}

	// Step 2: Convert to Responses API request
	convertResult, err := openai.ConvertChatCompletionToResponsesRequest(m, httpReq)
	require.NoError(t, err)

	var responsesReq relaymodel.CreateResponseRequest

	err = json.NewDecoder(convertResult.Body).Decode(&responsesReq)
	require.NoError(t, err)

	// Verify conversion
	assert.Equal(t, "gpt-5-codex", responsesReq.Model)

	// Verify input is an array with 2 messages
	if inputArray, ok := responsesReq.Input.([]any); ok {
		assert.Equal(t, 2, len(inputArray))
	} else {
		t.Errorf("Input is not an array")
	}

	assert.NotNil(t, responsesReq.Store)
	assert.False(t, *responsesReq.Store, "Store should be false")
	assert.NotNil(t, responsesReq.Temperature)
	assert.Equal(t, 0.7, *responsesReq.Temperature)

	// Step 3: Mock Responses API response
	mockResponse := relaymodel.Response{
		ID:        "resp_test_123",
		Model:     "gpt-5-codex",
		CreatedAt: 1234567890,
		Status:    relaymodel.ResponseStatusCompleted,
		Output: []relaymodel.OutputItem{
			{
				Role: "assistant",
				Content: []relaymodel.OutputContent{
					{Type: "text", Text: "2 + 2 equals 4."},
				},
			},
		},
		Usage: &relaymodel.ResponseUsage{
			InputTokens:  25,
			OutputTokens: 10,
			TotalTokens:  35,
		},
	}

	respBody, err := json.Marshal(mockResponse)
	require.NoError(t, err)

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockReadCloser{Reader: bytes.NewReader(respBody)},
		Header:     make(http.Header),
	}

	// Step 4: Convert back to ChatCompletion response
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	usage, err := openai.ConvertResponsesToChatCompletionResponse(m, c, httpResp)
	require.Nil(t, err)

	// Step 5: Verify final ChatCompletion response
	var finalResp relaymodel.TextResponse

	err = json.Unmarshal(w.Body.Bytes(), &finalResp)
	require.NoError(t, err)

	assert.Equal(t, "chat.completion", finalResp.Object)
	assert.Equal(t, "resp_test_123", finalResp.ID)
	assert.Equal(t, "gpt-5-codex", finalResp.Model)
	assert.NotEmpty(t, finalResp.Choices)
	assert.Contains(t, finalResp.Choices[0].Message.Content, "2 + 2 equals 4")
	assert.Equal(t, int64(25), int64(usage.Usage.InputTokens))
	assert.Equal(t, int64(10), int64(usage.Usage.OutputTokens))
}

// TestIntegrationModelDetection tests that IsResponsesOnlyModel correctly
// determines when to use conversion
func TestIntegrationModelDetection(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		shouldConvert bool
	}{
		{
			name:          "gpt-5-codex should convert",
			model:         "gpt-5-codex",
			shouldConvert: true,
		},
		{
			name:          "gpt-5-pro should convert",
			model:         "gpt-5-pro",
			shouldConvert: true,
		},
		{
			name:          "gpt-4o should not convert",
			model:         "gpt-4o",
			shouldConvert: false,
		},
		{
			name:          "gpt-3.5-turbo should not convert",
			model:         "gpt-3.5-turbo",
			shouldConvert: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := openai.IsResponsesOnlyModel(nil, tt.model)
			assert.Equal(t, tt.shouldConvert, result)
		})
	}
}

// TestIntegrationGetRequestURL tests that GetRequestURL returns correct URL
func TestIntegrationGetRequestURL(t *testing.T) {
	adaptor := &openai.Adaptor{}

	tests := []struct {
		name        string
		model       string
		mode        mode.Mode
		expectedURL string
	}{
		{
			name:        "gpt-5-codex with ChatCompletions should use /responses",
			model:       "gpt-5-codex",
			mode:        mode.ChatCompletions,
			expectedURL: "/responses",
		},
		{
			name:        "gpt-5-pro with ChatCompletions should use /responses",
			model:       "gpt-5-pro",
			mode:        mode.ChatCompletions,
			expectedURL: "/responses",
		},
		{
			name:        "gpt-4o with ChatCompletions should use /chat/completions",
			model:       "gpt-4o",
			mode:        mode.ChatCompletions,
			expectedURL: "/chat/completions",
		},
		{
			name:        "gpt-5-codex with Anthropic mode should use /responses",
			model:       "gpt-5-codex",
			mode:        mode.Anthropic,
			expectedURL: "/responses",
		},
		{
			name:        "gpt-5-codex with Gemini mode should use /responses",
			model:       "gpt-5-codex",
			mode:        mode.Gemini,
			expectedURL: "/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &meta.Meta{
				ActualModel: tt.model,
				Mode:        tt.mode,
			}
			m.Channel.BaseURL = "https://api.openai.com"

			result, err := adaptor.GetRequestURL(m, nil, nil)
			require.NoError(t, err)
			assert.Contains(t, result.URL, tt.expectedURL)
			assert.Equal(t, http.MethodPost, result.Method)
		})
	}
}

// TestIntegrationConvertRequestWithDifferentModes tests conversion across different modes
func TestIntegrationConvertRequestWithDifferentModes(t *testing.T) {
	tests := []struct {
		name          string
		mode          mode.Mode
		model         string
		requestBody   string
		shouldConvert bool
	}{
		{
			name:  "ChatCompletions mode with gpt-5-codex",
			mode:  mode.ChatCompletions,
			model: "gpt-5-codex",
			requestBody: `{
				"model": "gpt-5-codex",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			shouldConvert: true,
		},
		{
			name:  "ChatCompletions mode with gpt-4o",
			mode:  mode.ChatCompletions,
			model: "gpt-4o",
			requestBody: `{
				"model": "gpt-4o",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			shouldConvert: false,
		},
		{
			name:  "Anthropic mode with gpt-5-codex",
			mode:  mode.Anthropic,
			model: "gpt-5-codex",
			requestBody: `{
				"model": "gpt-5-codex",
				"messages": [{"role": "user", "content": "Hello"}],
				"max_tokens": 1024
			}`,
			shouldConvert: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq := httptest.NewRequest(
				http.MethodPost,
				"/test",
				bytes.NewReader([]byte(tt.requestBody)),
			)
			httpReq.Header.Set("Content-Type", "application/json")

			m := &meta.Meta{
				ActualModel: tt.model,
				Mode:        tt.mode,
			}

			var (
				result adaptor.ConvertResult
				err    error
			)

			// Call the appropriate conversion based on mode

			switch tt.mode {
			case mode.ChatCompletions:
				if openai.IsResponsesOnlyModel(&m.ModelConfig, tt.model) {
					result, err = openai.ConvertChatCompletionToResponsesRequest(m, httpReq)
				} else {
					result, err = openai.ConvertChatCompletionsRequest(m, httpReq, false)
				}
			case mode.Anthropic:
				if openai.IsResponsesOnlyModel(&m.ModelConfig, tt.model) {
					result, err = openai.ConvertClaudeToResponsesRequest(m, httpReq)
				} else {
					result, err = openai.ConvertClaudeRequest(m, httpReq)
				}
			}

			require.NoError(t, err)

			// If should convert to Responses API, verify the result contains expected fields
			if tt.shouldConvert {
				var responsesReq relaymodel.CreateResponseRequest

				err = json.NewDecoder(result.Body).Decode(&responsesReq)
				require.NoError(t, err)

				assert.Equal(t, tt.model, responsesReq.Model)
				assert.NotNil(t, responsesReq.Store)
				assert.False(t, *responsesReq.Store)
			}
		})
	}
}
