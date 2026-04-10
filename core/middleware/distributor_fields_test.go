package middleware_test

import (
	"testing"

	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestFieldExtractorsFromJSON(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"gpt-5",
		"previous_response_id":"resp_123",
		"prompt_cache_key":"cache-key-1",
		"user":"user-1",
		"metadata":{"env":"test","team":"core"}
	}`)

	modelName, err := middleware.GetModelFromJSON(body)
	require.NoError(t, err)
	assert.Equal(t, "gpt-5", modelName)

	responseID, err := middleware.GetPreviousResponseIDFromJSON(body)
	require.NoError(t, err)
	assert.Equal(t, "resp_123", responseID)

	promptCacheKey, err := middleware.GetPromptCacheKeyFromJSON(body)
	require.NoError(t, err)
	assert.Equal(t, "cache-key-1", promptCacheKey)

	user, err := middleware.GetRequestUserFromJSON(body, mode.ChatCompletions)
	require.NoError(t, err)
	assert.Equal(t, "user-1", user)

	metadata, err := middleware.GetRequestMetadataFromJSON(body)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"env":  "test",
		"team": "core",
	}, metadata)
}

func TestRequestFieldExtractorsFromJSONMissingFields(t *testing.T) {
	t.Parallel()

	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)

	modelName, err := middleware.GetModelFromJSON(body)
	require.NoError(t, err)
	assert.Empty(t, modelName)

	responseID, err := middleware.GetPreviousResponseIDFromJSON(body)
	require.NoError(t, err)
	assert.Empty(t, responseID)

	promptCacheKey, err := middleware.GetPromptCacheKeyFromJSON(body)
	require.NoError(t, err)
	assert.Empty(t, promptCacheKey)

	user, err := middleware.GetRequestUserFromJSON(body, mode.ChatCompletions)
	require.NoError(t, err)
	assert.Empty(t, user)

	metadata, err := middleware.GetRequestMetadataFromJSON(body)
	require.NoError(t, err)
	assert.Nil(t, metadata)
}
