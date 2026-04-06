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

// ChatCompletions should prefer PPIO (native OpenAI passthrough) over
// Anthropic (requires ChatCompletions→Anthropic protocol conversion).
func TestFilterChannels_PrefersNativeOpenAIOverAnthropicConversion(t *testing.T) {
	channels := []*model.Channel{
		{
			ID:     3,
			Type:   model.ChannelTypePPIO,
			Status: model.ChannelStatusEnabled,
		},
		{
			ID:     4,
			Type:   model.ChannelTypeAnthropic,
			Status: model.ChannelStatusEnabled,
		},
	}

	got := filterChannels(channels, mode.ChatCompletions, map[int64]float64{}, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 native channel, got %d", len(got))
	}

	if got[0].ID != 3 {
		t.Fatalf("expected native PPIO channel (id=3), got channel id %d", got[0].ID)
	}
}

// A native channel with a high error rate should still be preferred over a
// healthy non-native channel, because protocol conversion itself is a failure
// source (e.g. max_tokens semantics mismatch).
func TestFilterChannels_PrefersHighErrorNativeOverHealthyNonNative(t *testing.T) {
	channels := []*model.Channel{
		{
			ID:     3,
			Type:   model.ChannelTypePPIO,
			Status: model.ChannelStatusEnabled,
		},
		{
			ID:     4,
			Type:   model.ChannelTypeAnthropic,
			Status: model.ChannelStatusEnabled,
		},
	}

	errorRates := map[int64]float64{
		3: 0.9, // native channel has high error rate
		4: 0.1, // non-native channel is healthy
	}

	got := filterChannels(channels, mode.ChatCompletions, errorRates, 0.75)
	if len(got) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(got))
	}

	if got[0].ID != 3 {
		t.Fatalf("expected high-error native PPIO channel (id=3), got channel id %d", got[0].ID)
	}
}

// When a native channel is healthy (below error threshold), it should be
// returned and the high-error native channels excluded.
func TestFilterChannels_FiltersErrorRateWithinNativeChannels(t *testing.T) {
	channels := []*model.Channel{
		{
			ID:     1,
			Type:   model.ChannelTypePPIO,
			Status: model.ChannelStatusEnabled,
		},
		{
			ID:     2,
			Type:   model.ChannelTypePPIO,
			Status: model.ChannelStatusEnabled,
		},
		{
			ID:     3,
			Type:   model.ChannelTypeAnthropic,
			Status: model.ChannelStatusEnabled,
		},
	}

	errorRates := map[int64]float64{
		1: 0.9, // native, unhealthy
		2: 0.1, // native, healthy
		3: 0.1, // non-native, healthy
	}

	got := filterChannels(channels, mode.ChatCompletions, errorRates, 0.75)
	if len(got) != 1 {
		t.Fatalf("expected 1 healthy native channel, got %d", len(got))
	}

	if got[0].ID != 2 {
		t.Fatalf("expected healthy native PPIO channel (id=2), got channel id %d", got[0].ID)
	}
}

// Banned (ignored) channels must be excluded regardless of native status.
func TestFilterChannels_ExcludesBannedChannelsBeforePartition(t *testing.T) {
	channels := []*model.Channel{
		{
			ID:     3,
			Type:   model.ChannelTypePPIO,
			Status: model.ChannelStatusEnabled,
		},
		{
			ID:     4,
			Type:   model.ChannelTypeAnthropic,
			Status: model.ChannelStatusEnabled,
		},
	}

	banned := map[int64]struct{}{3: {}}

	got := filterChannels(channels, mode.ChatCompletions, map[int64]float64{}, 0, banned)
	if len(got) != 1 {
		t.Fatalf("expected 1 channel after banning, got %d", len(got))
	}

	if got[0].ID != 4 {
		t.Fatalf("expected non-native fallback channel (id=4), got channel id %d", got[0].ID)
	}
}

// When all channels are disabled or banned, filterChannels returns nil/empty.
func TestFilterChannels_ReturnsEmptyWhenAllFiltered(t *testing.T) {
	channels := []*model.Channel{
		{
			ID:     1,
			Type:   model.ChannelTypePPIO,
			Status: model.ChannelStatusDisabled,
		},
		{
			ID:     2,
			Type:   model.ChannelTypeAnthropic,
			Status: model.ChannelStatusDisabled,
		},
	}

	got := filterChannels(channels, mode.ChatCompletions, map[int64]float64{}, 0)
	if len(got) != 0 {
		t.Fatalf("expected 0 channels, got %d", len(got))
	}
}
