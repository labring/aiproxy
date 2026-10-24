package adaptors_test

import (
	"testing"

	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
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
