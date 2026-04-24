package vertexai_test

import (
	"net/http"
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	vertexai "github.com/labring/aiproxy/core/relay/adaptor/vertexai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURL_UsesOriginModelNameForRouting(t *testing.T) {
	adaptor := &vertexai.Adaptor{}

	t.Run("origin gemini chooses google publisher endpoint", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.ChatCompletions, "gemini-2.5-pro", coremodel.ModelConfig{})
		m.ActualModel = "mapped-model"
		m.Channel.Key = "us-central1|apikey"
		m.Set("stream", false)

		reqURL, err := adaptor.GetRequestURL(m, nil, nil)
		require.NoError(t, err)
		assert.Equal(
			t,
			http.MethodPost,
			reqURL.Method,
		)
		assert.Contains(
			t,
			reqURL.URL,
			"/publishers/google/models/mapped-model:generateContent",
		)
	})

	t.Run("origin claude chooses anthropic publisher endpoint", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.ChatCompletions, "claude-sonnet-4-5", coremodel.ModelConfig{})
		m.ActualModel = "mapped-model"
		m.Channel.Key = "us-central1|apikey"
		m.Set("stream", true)

		reqURL, err := adaptor.GetRequestURL(m, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, reqURL.Method)
		assert.Contains(
			t,
			reqURL.URL,
			"/publishers/anthropic/models/mapped-model:streamRawPredict?alt=sse",
		)
	})
}
