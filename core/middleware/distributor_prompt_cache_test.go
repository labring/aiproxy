package middleware_test

import (
	"testing"

	"github.com/labring/aiproxy/core/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPromptCacheKeyFromJSON(t *testing.T) {
	t.Parallel()

	t.Run("returns prompt cache key when present", func(t *testing.T) {
		key, err := middleware.GetPromptCacheKeyFromJSON(
			[]byte(`{"model":"gpt-5","prompt_cache_key":"cache-key-1"}`),
		)
		require.NoError(t, err)
		assert.Equal(t, "cache-key-1", key)
	})

	t.Run("returns empty when missing", func(t *testing.T) {
		key, err := middleware.GetPromptCacheKeyFromJSON([]byte(`{"model":"gpt-5"}`))
		require.NoError(t, err)
		assert.Empty(t, key)
	})
}
