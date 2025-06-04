package textembeddingsinference_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	textembeddingsinference "github.com/labring/aiproxy/core/relay/adaptor/text-embeddings-inference"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertRerankRequestSuccess tests the successful conversion of a rerank request
func TestConvertRerankRequestSuccess(t *testing.T) {
	t.Parallel()
	// Create mock request body
	requestBody := map[string]any{
		"model": "original-model",
		"documents": []string{
			"This is document 1",
			"This is document 2",
		},
		"query": "Find relevant documents",
	}

	jsonBody, err := sonic.Marshal(requestBody)
	require.NoError(t, err)

	// Create mock HTTP request with context
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/rerank",
		bytes.NewReader(jsonBody),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create mock meta
	testMeta := &meta.Meta{
		ActualModel: "text-embeddings-model",
	}

	// Call the function under test
	result, err := textembeddingsinference.ConvertRerankRequest(testMeta, req)

	// Assert no error
	require.NoError(t, err)

	// Read the transformed body
	bodyBytes, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	// Parse the body back to verify the transformation
	var transformedBody map[string]any
	err = sonic.Unmarshal(bodyBytes, &transformedBody)
	require.NoError(t, err)

	// Verify the model was replaced
	assert.Equal(t, "text-embeddings-model", transformedBody["model"])

	// Verify documents was renamed to texts
	assert.NotContains(t, transformedBody, "documents")

	// Verify texts contains the documents content
	textsArray, ok := transformedBody["texts"].([]any)
	assert.True(t, ok, "texts should be an array")
	assert.Len(t, textsArray, 2)
	assert.Equal(t, "This is document 1", textsArray[0])
	assert.Equal(t, "This is document 2", textsArray[1])

	// Verify query remains unchanged
	assert.Equal(t, "Find relevant documents", transformedBody["query"])
}

// TestConvertRerankRequestMissingDocuments tests the error case when documents field is missing
func TestConvertRerankRequestMissingDocuments(t *testing.T) {
	t.Parallel()
	// Create mock request body without documents
	requestBody := map[string]any{
		"model": "original-model",
		"query": "Find relevant documents",
	}

	jsonBody, err := sonic.Marshal(requestBody)
	require.NoError(t, err)

	// Create mock HTTP request with context
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/rerank",
		bytes.NewReader(jsonBody),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create mock meta
	testMeta := &meta.Meta{
		ActualModel: "text-embeddings-model",
	}

	// Call the function under test
	_, err = textembeddingsinference.ConvertRerankRequest(testMeta, req)

	// Assert error for missing documents
	require.Error(t, err)
	assert.Contains(t, err.Error(), "documents field not found")
}

// TestConvertRerankRequestInvalidJSON tests the error case when the request body contains invalid
// JSON
func TestConvertRerankRequestInvalidJSON(t *testing.T) {
	t.Parallel()
	// Create invalid JSON body
	invalidJSON := []byte(`{"model": "test", "documents": [`)

	// Create mock HTTP request with context
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/rerank",
		bytes.NewReader(invalidJSON),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create mock meta
	testMeta := &meta.Meta{
		ActualModel: "text-embeddings-model",
	}

	// Call the function under test
	_, err = textembeddingsinference.ConvertRerankRequest(testMeta, req)

	// Assert error for invalid JSON
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse request body")
}
