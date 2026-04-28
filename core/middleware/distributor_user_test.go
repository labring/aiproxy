//nolint:testpackage
package middleware

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestUserFromNodeAnthropic(t *testing.T) {
	t.Parallel()

	t.Run("prefers metadata user_id", func(t *testing.T) {
		t.Parallel()

		node, err := sonic.Get([]byte(`{
			"user":"top-level-user",
			"metadata":{"user_id":"anthropic-user"}
		}`))
		require.NoError(t, err)

		user, err := getRequestUserFromNode(&node, mode.Anthropic)
		require.NoError(t, err)
		assert.Equal(t, "anthropic-user", user)
	})

	t.Run("falls back to user when metadata user_id missing", func(t *testing.T) {
		t.Parallel()

		node, err := sonic.Get([]byte(`{"user":"top-level-user","metadata":{"team":"core"}}`))
		require.NoError(t, err)

		user, err := getRequestUserFromNode(&node, mode.Anthropic)
		require.NoError(t, err)
		assert.Equal(t, "top-level-user", user)
	})
}
