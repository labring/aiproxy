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
)

func TestConvertRerankRequest(t *testing.T) {
	// Test successful conversion
	t.Run("successful conversion", func(t *testing.T) {
		// Create mock request body
		requestBody := map[string]interface{}{
			"model": "original-model",
			"documents": []string{
				"This is document 1",
				"This is document 2",
			},
			"query": "Find relevant documents",
		}

		jsonBody, err := sonic.Marshal(requestBody)
		assert.NoError(t, err)

		// Create mock HTTP request
		req, err := http.NewRequest(http.MethodPost, "/rerank", bytes.NewReader(jsonBody))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Create mock meta
		testMeta := &meta.Meta{
			ActualModel: "text-embeddings-model",
		}

		// Call the function under test
		method, _, bodyReader, err := textembeddingsinference.ConvertRerankRequest(testMeta, req)

		// Assert no error
		assert.NoError(t, err)

		// Assert method
		assert.Equal(t, http.MethodPost, method)

		// Read the transformed body
		bodyBytes, err := io.ReadAll(bodyReader)
		assert.NoError(t, err)

		// Parse the body back to verify the transformation
		var transformedBody map[string]interface{}
		err = sonic.Unmarshal(bodyBytes, &transformedBody)
		assert.NoError(t, err)

		// Verify the model was replaced
		assert.Equal(t, "text-embeddings-model", transformedBody["model"])

		// Verify documents was renamed to texts
		assert.NotContains(t, transformedBody, "documents")

		// Verify texts contains the documents content
		textsArray, ok := transformedBody["texts"].([]interface{})
		assert.True(t, ok, "texts should be an array")
		assert.Len(t, textsArray, 2)
		assert.Equal(t, "This is document 1", textsArray[0])
		assert.Equal(t, "This is document 2", textsArray[1])

		// Verify query remains unchanged
		assert.Equal(t, "Find relevant documents", transformedBody["query"])
	})

	// Test missing documents field
	t.Run("missing documents field", func(t *testing.T) {
		// Create mock request body without documents
		requestBody := map[string]interface{}{
			"model": "original-model",
			"query": "Find relevant documents",
		}

		jsonBody, err := sonic.Marshal(requestBody)
		assert.NoError(t, err)

		// Create mock HTTP request
		req, err := http.NewRequest(http.MethodPost, "/rerank", bytes.NewReader(jsonBody))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Create mock meta
		testMeta := &meta.Meta{
			ActualModel: "text-embeddings-model",
		}

		// Call the function under test
		_, _, _, err = textembeddingsinference.ConvertRerankRequest(testMeta, req)

		// Assert error for missing documents
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "documents field not found")
	})

	// Test invalid JSON
	t.Run("invalid json", func(t *testing.T) {
		// Create invalid JSON body
		invalidJSON := []byte(`{"model": "test", "documents": [`)

		// Create mock HTTP request
		req, err := http.NewRequest(http.MethodPost, "/rerank", bytes.NewReader(invalidJSON))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Create mock meta
		testMeta := &meta.Meta{
			ActualModel: "text-embeddings-model",
		}

		// Call the function under test
		_, _, _, err = textembeddingsinference.ConvertRerankRequest(testMeta, req)

		// Assert error for invalid JSON
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse request body")
	})
}
