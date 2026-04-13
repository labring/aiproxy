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

// --- getChannelWithFallback set-ordering tests ---

// buildTestModelCaches creates a ModelCaches with channels organized by set.
// channelsBySet maps set → model → []channel.
func buildTestModelCaches(channelsBySet map[string]map[string][]*model.Channel) *model.ModelCaches {
	return &model.ModelCaches{
		EnabledModel2ChannelsBySet: channelsBySet,
	}
}

// With multiple sets, the first set is preferred. Even though both sets have
// channels for the model, only the first set's channel should be returned.
func TestGetChannelWithFallback_PrefersFirstSet(t *testing.T) {
	overseasCh := &model.Channel{ID: 1, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"overseas": {"gpt-4o": {overseasCh}},
		"default":  {"gpt-4o": {ppioCh}},
	})

	ch, migrated, err := getChannelWithFallback(mc, []string{"overseas", "default"}, "gpt-4o", mode.ChatCompletions, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ch.ID != 1 {
		t.Fatalf("expected overseas channel (id=1), got id=%d", ch.ID)
	}

	// migratedChannels should only contain channels from the selected set.
	if len(migrated) != 1 || migrated[0].ID != 1 {
		t.Fatalf("expected migratedChannels to contain only overseas channel, got %v", migrated)
	}
}

// When the first set has no channels for the model, fall back to the second set.
func TestGetChannelWithFallback_FallsBackWhenFirstSetEmpty(t *testing.T) {
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"overseas": {}, // overseas has no channels for this model
		"default":  {"gpt-4o": {ppioCh}},
	})

	ch, _, err := getChannelWithFallback(mc, []string{"overseas", "default"}, "gpt-4o", mode.ChatCompletions, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ch.ID != 2 {
		t.Fatalf("expected fallback to default channel (id=2), got id=%d", ch.ID)
	}
}

// When all channels in the first set are banned, fall back to the second set.
func TestGetChannelWithFallback_FallsBackWhenFirstSetAllBanned(t *testing.T) {
	overseasCh := &model.Channel{ID: 1, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"overseas": {"gpt-4o": {overseasCh}},
		"default":  {"gpt-4o": {ppioCh}},
	})

	// Ban the overseas channel
	banned := map[int64]struct{}{1: {}}

	ch, _, err := getChannelWithFallback(mc, []string{"overseas", "default"}, "gpt-4o", mode.ChatCompletions, nil, banned)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ch.ID != 2 {
		t.Fatalf("expected fallback to default channel (id=2), got id=%d", ch.ID)
	}
}

// Single set behaves the same as before (fast path).
func TestGetChannelWithFallback_SingleSetUnchanged(t *testing.T) {
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"default": {"gpt-4o": {ppioCh}},
	})

	ch, _, err := getChannelWithFallback(mc, []string{"default"}, "gpt-4o", mode.ChatCompletions, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ch.ID != 2 {
		t.Fatalf("expected default channel (id=2), got id=%d", ch.ID)
	}
}

// When no set has channels for the model, return ErrChannelsNotFound.
func TestGetChannelWithFallback_NoChannelsInAnySets(t *testing.T) {
	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"overseas": {},
		"default":  {},
	})

	_, _, err := getChannelWithFallback(mc, []string{"overseas", "default"}, "gpt-4o", mode.ChatCompletions, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// When all overseas channels have high error rates, the retry within the same
// set (maxErrorRate=0) should still pick an overseas channel — NOT fall back
// to default. Fallback only happens when channels are completely unavailable.
func TestGetChannelWithFallback_HighErrorRateStaysInSameSet(t *testing.T) {
	overseasCh := &model.Channel{ID: 1, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"overseas": {"gpt-4o": {overseasCh}},
		"default":  {"gpt-4o": {ppioCh}},
	})

	// Overseas channel has very high error rate, but is NOT banned.
	errorRates := map[int64]float64{1: 0.95}

	ch, migrated, err := getChannelWithFallback(mc, []string{"overseas", "default"}, "gpt-4o", mode.ChatCompletions, errorRates, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still pick the overseas channel (retry without error-rate cap).
	if ch.ID != 1 {
		t.Fatalf("expected overseas channel (id=1) despite high error rate, got id=%d", ch.ID)
	}

	// migratedChannels should only contain overseas channels.
	if len(migrated) != 1 || migrated[0].ID != 1 {
		t.Fatalf("expected migratedChannels to contain only overseas channel, got %v", migrated)
	}
}

// migratedChannels returned by getChannelWithFallback must be scoped to the
// selected set. The retry mechanism (getRetryChannel) uses migratedChannels
// to pick alternative channels — if it leaked default-set channels, retries
// could bypass the overseas preference.
func TestGetChannelWithFallback_MigratedChannelsScopedToSelectedSet(t *testing.T) {
	overseasCh1 := &model.Channel{ID: 1, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	overseasCh2 := &model.Channel{ID: 3, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"overseas": {"gpt-4o": {overseasCh1, overseasCh2}},
		"default":  {"gpt-4o": {ppioCh}},
	})

	_, migrated, err := getChannelWithFallback(mc, []string{"overseas", "default"}, "gpt-4o", mode.ChatCompletions, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// migrated must contain exactly the overseas channels, never default.
	if len(migrated) != 2 {
		t.Fatalf("expected 2 overseas channels in migratedChannels, got %d", len(migrated))
	}

	for _, ch := range migrated {
		if ch.ID == 2 {
			t.Fatal("migratedChannels leaked default-set channel (id=2) — retries would bypass overseas preference")
		}
	}
}

// Empty availableSet should still work (traverses all sets via getRandomChannel).
func TestGetChannelWithFallback_EmptyAvailableSet(t *testing.T) {
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"default": {"gpt-4o": {ppioCh}},
	})

	// Empty slice → fast path → getRandomChannel with empty availableSet
	// → iterates all sets in EnabledModel2ChannelsBySet.
	ch, _, err := getChannelWithFallback(mc, []string{}, "gpt-4o", mode.ChatCompletions, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ch.ID != 2 {
		t.Fatalf("expected default channel (id=2), got id=%d", ch.ID)
	}
}

// When some overseas channels are banned AND the remaining ones have high
// error rates, the set should be fully exhausted, triggering fallback to default.
func TestGetChannelWithFallback_BannedPlusHighErrorRateFallsBack(t *testing.T) {
	overseasCh1 := &model.Channel{ID: 1, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	overseasCh2 := &model.Channel{ID: 3, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"overseas": {"gpt-4o": {overseasCh1, overseasCh2}},
		"default":  {"gpt-4o": {ppioCh}},
	})

	// Channel 1 is banned, channel 3 has high error rate but is not banned.
	banned := map[int64]struct{}{1: {}}
	errorRates := map[int64]float64{3: 0.95}

	ch, _, err := getChannelWithFallback(mc, []string{"overseas", "default"}, "gpt-4o", mode.ChatCompletions, errorRates, banned)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Phase 1: channel 1 banned + channel 3 high error → ErrChannelsExhausted.
	// Phase 2 (retry without error cap): channel 1 still banned, channel 3
	// now eligible (error cap removed) → should pick channel 3, staying in overseas.
	if ch.ID != 3 {
		t.Fatalf("expected overseas channel 3 (high error but not banned, retry without cap), got id=%d", ch.ID)
	}
}

// When ALL overseas channels are banned (some also have high error rates),
// the set is completely exhausted and must fall back to default.
func TestGetChannelWithFallback_AllBannedWithMixedErrorRatesFallsBack(t *testing.T) {
	overseasCh1 := &model.Channel{ID: 1, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	overseasCh2 := &model.Channel{ID: 3, Type: model.ChannelTypeNovita, Status: model.ChannelStatusEnabled, Priority: 10}
	ppioCh := &model.Channel{ID: 2, Type: model.ChannelTypePPIO, Status: model.ChannelStatusEnabled, Priority: 10}

	mc := buildTestModelCaches(map[string]map[string][]*model.Channel{
		"overseas": {"gpt-4o": {overseasCh1, overseasCh2}},
		"default":  {"gpt-4o": {ppioCh}},
	})

	// Both overseas channels are banned.
	banned := map[int64]struct{}{1: {}, 3: {}}
	errorRates := map[int64]float64{1: 0.95} // error rate doesn't matter — both banned

	ch, _, err := getChannelWithFallback(mc, []string{"overseas", "default"}, "gpt-4o", mode.ChatCompletions, errorRates, banned)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ch.ID != 2 {
		t.Fatalf("expected fallback to default channel (id=2), got id=%d", ch.ID)
	}
}
