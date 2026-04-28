//nolint:testpackage
package streamfake

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamingDataWithContentFilterFields(t *testing.T) {
	tests := []struct {
		name                      string
		chunks                    []string
		expectedContent           string
		hasPromptFilterResults    bool
		hasContentFilterResults   bool
		hasContentFilterResult    bool
		expectedFinishReason      string
		checkObfuscationPreserved bool
	}{
		{
			name: "Azure OpenAI stream with content filter fields",
			chunks: []string{
				`{"choices":[],"created":0,"id":"","model":"gpt-4.1-mini","object":"","prompt_filter_results":[{"prompt_index":0,"content_filter_results":{}}]}`,
				`{"choices":[{"content_filter_result":{"error":{"code":"content_filter_error","message":"The contents are not filtered"}},"content_filter_results":{},"delta":{"content":"","refusal":null,"role":"assistant"},"finish_reason":null,"index":0,"logprobs":null}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4.1-mini","obfuscation":"x","object":"chat.completion.chunk","system_fingerprint":"fp_test","usage":null}`,
				`{"choices":[{"content_filter_result":{"error":{"code":"content_filter_error","message":"The contents are not filtered"}},"delta":{"content":"Hello"},"finish_reason":null,"index":0,"logprobs":null}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4.1-mini","obfuscation":"abc123","object":"chat.completion.chunk","system_fingerprint":"fp_test","usage":null}`,
				`{"choices":[{"content_filter_result":{"error":{"code":"content_filter_error","message":"The contents are not filtered"}},"delta":{"content":" World"},"finish_reason":null,"index":0,"logprobs":null}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4.1-mini","obfuscation":"def456","object":"chat.completion.chunk","system_fingerprint":"fp_test","usage":null}`,
				`{"choices":[{"content_filter_result":{"error":{"code":"content_filter_error","message":"The contents are not filtered"}},"delta":{},"finish_reason":"stop","index":0,"logprobs":null}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4.1-mini","obfuscation":"ghi789","object":"chat.completion.chunk","system_fingerprint":"fp_test","usage":null}`,
				`{"choices":[],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4.1-mini","obfuscation":"G","object":"chat.completion.chunk","system_fingerprint":"fp_test","usage":{"completion_tokens":2,"completion_tokens_details":{"accepted_prediction_tokens":0,"audio_tokens":0,"reasoning_tokens":0,"rejected_prediction_tokens":0},"prompt_tokens":886,"prompt_tokens_details":{"audio_tokens":0,"cached_tokens":0},"total_tokens":888}}`,
			},
			expectedContent:           "Hello World",
			hasPromptFilterResults:    true,
			hasContentFilterResults:   true,
			hasContentFilterResult:    true,
			expectedFinishReason:      "stop",
			checkObfuscationPreserved: true,
		},
		{
			name: "simple stream without filter fields",
			chunks: []string{
				`{"choices":[{"delta":{"content":"Hi"},"finish_reason":null,"index":0}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4","object":"chat.completion.chunk"}`,
				`{"choices":[{"delta":{},"finish_reason":"stop","index":0}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4","object":"chat.completion.chunk","usage":{"completion_tokens":1,"prompt_tokens":10,"total_tokens":11}}`,
			},
			expectedContent:         "Hi",
			hasPromptFilterResults:  false,
			hasContentFilterResults: false,
			hasContentFilterResult:  false,
			expectedFinishReason:    "stop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := &fakeStreamResponseWriter{}

			// Parse all chunks
			for _, chunk := range tt.chunks {
				err := rw.parseStreamingData([]byte(chunk))
				require.NoError(t, err)
			}

			// Check content
			assert.Equal(t, tt.expectedContent, rw.contentBuilder.String())

			// Check finish reason
			assert.Equal(t, tt.expectedFinishReason, rw.finishReason)

			// Check prompt_filter_results
			if tt.hasPromptFilterResults {
				assert.NotNil(t, rw.promptFilterResults)
			} else {
				assert.Nil(t, rw.promptFilterResults)
			}

			// Check content_filter_results
			if tt.hasContentFilterResults {
				assert.NotNil(t, rw.contentFilterResults)
			} else {
				assert.Nil(t, rw.contentFilterResults)
			}

			// Check content_filter_result
			if tt.hasContentFilterResult {
				assert.NotNil(t, rw.contentFilterResult)
			} else {
				assert.Nil(t, rw.contentFilterResult)
			}

			// Convert to non-stream and verify
			result, err := rw.convertToNonStream()
			require.NoError(t, err)

			var response map[string]any

			err = sonic.Unmarshal(result, &response)
			require.NoError(t, err)

			// Check object type is changed to non-stream
			assert.Equal(t, "chat.completion", response["object"])

			// Check prompt_filter_results is preserved
			if tt.hasPromptFilterResults {
				assert.NotNil(t, response["prompt_filter_results"])
				promptFilters, ok := response["prompt_filter_results"].([]any)
				require.True(t, ok)
				assert.Len(t, promptFilters, 1)
			}

			// Check choices structure
			choices, ok := response["choices"].([]any)
			require.True(t, ok)
			require.Len(t, choices, 1)
			choice, ok := choices[0].(map[string]any)
			require.True(t, ok)

			// Check message content
			message, ok := choice["message"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "assistant", message["role"])
			assert.Equal(t, tt.expectedContent, message["content"])

			// Check content_filter_results in choice
			if tt.hasContentFilterResults {
				assert.NotNil(t, choice["content_filter_results"])
			}

			// Check content_filter_result in choice
			if tt.hasContentFilterResult {
				assert.NotNil(t, choice["content_filter_result"])
				cfr, ok := choice["content_filter_result"].(map[string]any)
				require.True(t, ok)
				assert.NotNil(t, cfr["error"])
			}

			// Check obfuscation field is preserved (in lastChunk)
			if tt.checkObfuscationPreserved {
				assert.NotNil(t, response["obfuscation"])
			}

			// Check usage is preserved
			assert.NotNil(t, response["usage"])
		})
	}
}

func TestConvertToNonStreamWithAllFields(t *testing.T) {
	rw := &fakeStreamResponseWriter{}

	// Simulate parsing Azure OpenAI stream with all fields
	chunks := []string{
		`{"choices":[],"created":0,"id":"chatcmpl-test","model":"gpt-4.1-mini","object":"chat.completion.chunk","prompt_filter_results":[{"prompt_index":0,"content_filter_results":{"hate":{"filtered":false,"severity":"safe"},"self_harm":{"filtered":false,"severity":"safe"}}}]}`,
		`{"choices":[{"content_filter_result":{"error":{"code":"content_filter_error","message":"The contents are not filtered"}},"content_filter_results":{"hate":{"filtered":false,"severity":"safe"}},"delta":{"content":"Test","role":"assistant"},"finish_reason":null,"index":0}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4.1-mini","obfuscation":"xyz","object":"chat.completion.chunk","system_fingerprint":"fp_test"}`,
		`{"choices":[{"content_filter_result":{"error":{"code":"content_filter_error","message":"The contents are not filtered"}},"content_filter_results":{"hate":{"filtered":false,"severity":"safe"}},"delta":{},"finish_reason":"stop","index":0}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4.1-mini","obfuscation":"final","object":"chat.completion.chunk","system_fingerprint":"fp_test","usage":{"completion_tokens":1,"prompt_tokens":100,"total_tokens":101}}`,
	}

	for _, chunk := range chunks {
		err := rw.parseStreamingData([]byte(chunk))
		require.NoError(t, err)
	}

	result, err := rw.convertToNonStream()
	require.NoError(t, err)

	var response map[string]any

	err = sonic.Unmarshal(result, &response)
	require.NoError(t, err)

	// Verify response structure
	assert.Equal(t, "chat.completion", response["object"])
	assert.Equal(t, "chatcmpl-test", response["id"])
	assert.Equal(t, "gpt-4.1-mini", response["model"])
	assert.Equal(t, "final", response["obfuscation"]) // last obfuscation value preserved
	assert.Equal(t, "fp_test", response["system_fingerprint"])

	// Verify prompt_filter_results
	promptFilters, ok := response["prompt_filter_results"].([]any)
	require.True(t, ok)
	require.Len(t, promptFilters, 1)
	promptFilter, ok := promptFilters[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(0), promptFilter["prompt_index"])
	assert.NotNil(t, promptFilter["content_filter_results"])

	// Verify choices
	choices, ok := response["choices"].([]any)
	require.True(t, ok)
	require.Len(t, choices, 1)
	choice, ok := choices[0].(map[string]any)
	require.True(t, ok)

	// Verify content_filter_results in choice
	cfResults, ok := choice["content_filter_results"].(map[string]any)
	require.True(t, ok)
	assert.NotNil(t, cfResults["hate"])

	// Verify content_filter_result in choice
	cfResult, ok := choice["content_filter_result"].(map[string]any)
	require.True(t, ok)
	assert.NotNil(t, cfResult["error"])

	// Verify message
	message, ok := choice["message"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "assistant", message["role"])
	assert.Equal(t, "Test", message["content"])

	// Verify finish_reason
	assert.Equal(t, "stop", choice["finish_reason"])

	// Verify usage
	usage, ok := response["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), usage["completion_tokens"])
	assert.Equal(t, float64(100), usage["prompt_tokens"])
	assert.Equal(t, float64(101), usage["total_tokens"])
}

func TestParseStreamingDataIgnoresSSEData(t *testing.T) {
	rw := &fakeStreamResponseWriter{}

	// SSE data prefix should be ignored (IsValidSSEData checks for "data: " prefix)
	err := rw.parseStreamingData([]byte("data: "))
	assert.NoError(t, err)
	assert.Nil(t, rw.lastChunk)

	// SSE data with actual content should also be ignored
	err = rw.parseStreamingData([]byte("data: {\"test\":1}"))
	assert.NoError(t, err)
	assert.Nil(t, rw.lastChunk)
}

func TestParseStreamingDataWithEmptyChoices(t *testing.T) {
	rw := &fakeStreamResponseWriter{}

	// First chunk with empty choices (prompt_filter_results only)
	chunk := `{"choices":[],"created":0,"id":"","model":"gpt-4.1-mini","object":"","prompt_filter_results":[{"prompt_index":0,"content_filter_results":{}}]}`
	err := rw.parseStreamingData([]byte(chunk))
	require.NoError(t, err)

	assert.NotNil(t, rw.promptFilterResults)
	assert.Equal(t, "", rw.contentBuilder.String()) // No content yet
}

func TestContentFilterResultPreservesErrorDetails(t *testing.T) {
	rw := &fakeStreamResponseWriter{}

	chunk := `{"choices":[{"content_filter_result":{"error":{"code":"content_filter_error","message":"The contents are not filtered"}},"delta":{"content":"test"},"finish_reason":"stop","index":0}],"created":1767597874,"id":"chatcmpl-test","model":"gpt-4","object":"chat.completion.chunk","usage":{"completion_tokens":1,"prompt_tokens":10,"total_tokens":11}}`
	err := rw.parseStreamingData([]byte(chunk))
	require.NoError(t, err)

	result, err := rw.convertToNonStream()
	require.NoError(t, err)

	var response map[string]any

	err = sonic.Unmarshal(result, &response)
	require.NoError(t, err)

	choices, ok := response["choices"].([]any)
	require.True(t, ok)
	choice, ok := choices[0].(map[string]any)
	require.True(t, ok)
	cfResult, ok := choice["content_filter_result"].(map[string]any)
	require.True(t, ok)
	cfError, ok := cfResult["error"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "content_filter_error", cfError["code"])
	assert.Equal(t, "The contents are not filtered", cfError["message"])
}
