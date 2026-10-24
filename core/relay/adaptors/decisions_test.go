package adaptors_test

import (
	"net/http"
	"testing"

	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

func TestDecisionsModeAndChannelCompatibility(t *testing.T) {
	t.Parallel()

	require.True(t, middleware.CheckRelayMode(mode.Decisions, mode.Decisions))
	require.True(t, middleware.CheckRelayMode(mode.Decisions, mode.ChatCompletions))
	require.True(t, middleware.CheckRelayMode(mode.Decisions, mode.Responses))
	require.True(t, middleware.CheckRelayMode(mode.Decisions, mode.Unknown))
	require.False(t, middleware.CheckRelayMode(mode.Decisions, mode.Anthropic))
	require.False(t, middleware.CheckRelayMode(mode.ChatCompletions, mode.Decisions))
	require.False(t, middleware.CheckRelayMode(mode.Responses, mode.Decisions))

	for _, tt := range []struct {
		channelType model.ChannelType
		supported   bool
	}{
		{model.ChannelTypeOpenAI, true},
		{model.ChannelTypeAIProxyHZH, true},
		{model.ChannelTypeAIProxyUSW1, true},
		{model.ChannelTypeOpenRouter, false},
		{model.ChannelTypeDeepseek, false},
		{model.ChannelTypeAnthropic, false},
	} {
		t.Run(tt.channelType.String(), func(t *testing.T) {
			t.Parallel()

			a, ok := adaptors.GetAdaptor(tt.channelType)
			require.True(t, ok)
			require.Equal(t, tt.supported, a.SupportMode(&meta.Meta{
				Mode:    mode.Decisions,
				Channel: meta.ChannelMeta{Type: tt.channelType},
			}))
		})
	}
}

func TestDecisionsRequestURL(t *testing.T) {
	t.Parallel()

	for _, baseURL := range []string{"https://api.openai.com/v1", "https://api.openai.com/v1/"} {
		t.Run(baseURL, func(t *testing.T) {
			t.Parallel()

			a := &openai.Adaptor{}
			requestURL, err := a.GetRequestURL(&meta.Meta{
				Mode: mode.Decisions, Channel: meta.ChannelMeta{BaseURL: baseURL},
			}, nil, nil)
			require.NoError(t, err)
			require.Equal(t, http.MethodPost, requestURL.Method)
			require.Equal(t, "https://api.openai.com/v1/decisions", requestURL.URL)
		})
	}

	t.Run("invalid base URL", func(t *testing.T) {
		t.Parallel()

		a := &openai.Adaptor{}
		_, err := a.GetRequestURL(&meta.Meta{
			Mode: mode.Decisions, Channel: meta.ChannelMeta{BaseURL: "http://[::1"},
		}, nil, nil)
		require.Error(t, err)
	})

	t.Run("legacy support stays intact", func(t *testing.T) {
		t.Parallel()

		a := &openai.Adaptor{}
		require.True(t, a.SupportMode(&meta.Meta{Mode: mode.ChatCompletions}))
		require.True(t, a.SupportMode(&meta.Meta{Mode: mode.Responses}))
		require.False(t, a.SupportMode(&meta.Meta{Mode: mode.Unknown}))
		require.False(t, a.SupportMode(nil))
	})
}
