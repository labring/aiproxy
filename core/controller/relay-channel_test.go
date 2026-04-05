package controller

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestFilterChannels_PrefersNativeAnthropicChannels(t *testing.T) {
	channels := []*model.Channel{
		{
			ID:     1,
			Type:   model.ChannelTypePPIO,
			Status: model.ChannelStatusEnabled,
		},
		{
			ID:     2,
			Type:   model.ChannelTypeAnthropic,
			Status: model.ChannelStatusEnabled,
		},
	}

	got := filterChannels(channels, mode.Anthropic, map[int64]float64{}, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 native channel, got %d", len(got))
	}

	if got[0].ID != 2 {
		t.Fatalf("expected native anthropic channel to be preferred, got channel id %d", got[0].ID)
	}
}

func TestFilterChannels_FallsBackToConvertibleChannelWhenNoNativeExists(t *testing.T) {
	channels := []*model.Channel{
		{
			ID:     1,
			Type:   model.ChannelTypePPIO,
			Status: model.ChannelStatusEnabled,
		},
	}

	got := filterChannels(channels, mode.Anthropic, map[int64]float64{}, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 fallback channel, got %d", len(got))
	}

	if got[0].ID != 1 {
		t.Fatalf("expected fallback PPIO channel, got channel id %d", got[0].ID)
	}
}
