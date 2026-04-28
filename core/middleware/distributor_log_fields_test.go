//nolint:testpackage
package middleware

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDistributorLogFieldSetters(t *testing.T) {
	t.Parallel()

	t.Run("sets request user prompt cache key and service tier", func(t *testing.T) {
		t.Parallel()

		fields := logrus.Fields{}

		SetLogRequestUser(fields, "user-1")
		SetLogPromptCacheKey(fields, "cache-key-1")
		SetLogServiceTier(fields, "priority")

		assert.Equal(t, "user-1", fields["user"])
		assert.Equal(t, "cache-key-1", fields["prompt_cache_key"])
		assert.Equal(t, "priority", fields["service_tier"])
	})

	t.Run("skips empty values", func(t *testing.T) {
		t.Parallel()

		fields := logrus.Fields{}

		SetLogRequestUser(fields, "")
		SetLogPromptCacheKey(fields, "")
		SetLogServiceTier(fields, "")

		assert.Empty(t, fields)
	})
}

func TestSetLogFieldsFromMeta(t *testing.T) {
	t.Parallel()

	fields := logrus.Fields{}

	SetLogFieldsFromMeta(&meta.Meta{
		Mode:               mode.ChatCompletions,
		RequestID:          "req_123",
		RequestServiceTier: "priority",
		PromptCacheKey:     "cache-key-1",
		User:               "user-1",
		OriginModel:        "gpt-5",
		ActualModel:        "gpt-5",
	}, fields)

	assert.Equal(t, "priority", fields["service_tier"])
	assert.Equal(t, "cache-key-1", fields["prompt_cache_key"])
	assert.Equal(t, "user-1", fields["user"])
	assert.Equal(t, "req_123", fields["reqid"])
	assert.Equal(t, "ChatCompletions", fields["mode"])
	assert.Equal(t, "gpt-5", fields["model"])
	assert.Equal(t, "gpt-5", fields["actmodel"])
}
