//go:build enterprise

package ppio

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	ppiorelay "github.com/labring/aiproxy/core/relay/adaptor/ppio"
)

func setupPPIOChannelTestDB(t *testing.T) {
	t.Helper()

	prevDB := model.DB
	prevUsingSQLite := common.UsingSQLite

	testDB, err := model.OpenSQLite(filepath.Join(t.TempDir(), "ppio-sync.db"))
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = testDB
	common.UsingSQLite = true
	t.Cleanup(func() {
		model.DB = prevDB
		common.UsingSQLite = prevUsingSQLite
	})

	if err := testDB.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("failed to migrate channel table: %v", err)
	}
}

func TestEnsurePPIOChannelsFromModels_UpdatesChannelConfigs(t *testing.T) {
	setupPPIOChannelTestDB(t)

	channels := []model.Channel{
		{
			Name:    "PPIO (OpenAI)",
			Type:    model.ChannelTypePPIO,
			BaseURL: DefaultPPIOAPIBase,
			Key:     "ppio-key",
			Status:  model.ChannelStatusEnabled,
		},
		{
			Name:    "PPIO (Anthropic)",
			Type:    model.ChannelTypeAnthropic,
			BaseURL: DefaultPPIOAnthropicBase,
			Key:     "ppio-key",
			Status:  model.ChannelStatusEnabled,
			Configs: model.ChannelConfigs{
				"skip_image_conversion": false,
			},
		},
	}

	for i := range channels {
		if err := model.DB.Create(&channels[i]).Error; err != nil {
			t.Fatalf("failed to seed channel %q: %v", channels[i].Name, err)
		}
	}

	purePassthrough := true
	allowUnknown := true

	info, err := ensurePPIOChannelsFromModels(
		[]string{"claude-sonnet-4-20250514"},
		[]string{"deepseek-v3"},
		[]string{"seedream-5.0-lite"},
		false, // skipChatUpdate
		false, // skipMultimodalUpdate
		false, // autoCreate
		&purePassthrough,
		&allowUnknown,
		PPIOConfigResult{},
	)
	if err != nil {
		t.Fatalf("ensurePPIOChannelsFromModels returned error: %v", err)
	}

	if !info.PPIO.Exists {
		t.Fatalf("expected PPIO channel info to exist")
	}

	var got []model.Channel
	if err := model.DB.Order("id asc").Find(&got).Error; err != nil {
		t.Fatalf("failed to load updated channels: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(got))
	}

	for _, ch := range got {
		switch ch.Type {
		case model.ChannelTypePPIO:
			if len(ch.Models) != 1 || ch.Models[0] != "deepseek-v3" {
				t.Fatalf("openai channel models = %#v, want deepseek-v3", ch.Models)
			}

			pathBaseMap, ok := ch.Configs[model.ChannelConfigPathBaseMapKey].(map[string]any)
			if !ok {
				t.Fatalf(
					"openai channel path_base_map missing or wrong type: %#v",
					ch.Configs[model.ChannelConfigPathBaseMapKey],
				)
			}

			if gotBase := pathBaseMap[ppiorelay.PathPrefixResponses]; gotBase != ppioResponsesBase(
				DefaultPPIOAPIBase,
			) {
				t.Fatalf(
					"responses base = %#v, want %q",
					gotBase,
					ppioResponsesBase(DefaultPPIOAPIBase),
				)
			}

			if gotBase := pathBaseMap[ppiorelay.PathPrefixWebSearch]; gotBase != ppioWebSearchBase(
				DefaultPPIOAPIBase,
			) {
				t.Fatalf(
					"web search base = %#v, want %q",
					gotBase,
					ppioWebSearchBase(DefaultPPIOAPIBase),
				)
			}

			if gotAllow := ch.Configs.GetBool(
				model.ChannelConfigAllowPassthroughUnknown,
			); !gotAllow {
				t.Fatalf("allow_passthrough_unknown = false, want true")
			}
		case model.ChannelTypeAnthropic:
			if len(ch.Models) != 1 || ch.Models[0] != "claude-sonnet-4-20250514" {
				t.Fatalf("anthropic channel models = %#v, want claude-sonnet-4-20250514", ch.Models)
			}

			if gotPure := ch.Configs.GetBool("pure_passthrough"); !gotPure {
				t.Fatalf("pure_passthrough = false, want true")
			}

			if gotSkip := ch.Configs["skip_image_conversion"]; gotSkip != false {
				t.Fatalf("existing skip_image_conversion should be preserved, got %#v", gotSkip)
			}

			if gotDisable := ch.Configs.GetBool("disable_context_management"); !gotDisable {
				t.Fatalf("disable_context_management = false, want true")
			}
		}
	}
}

func TestCreatePPIOChannels_SetsPurePassthroughAndPathBaseMap(t *testing.T) {
	setupPPIOChannelTestDB(t)

	created, err := createPPIOChannels(
		PPIOConfigResult{
			APIKey:  "ppio-key",
			APIBase: DefaultPPIOAPIBase,
		},
		true,
		false,
		[]string{"claude-sonnet-4-20250514"},
		[]string{"deepseek-v3"},
		[]string{"seedream-5.0-lite"},
	)
	if err != nil {
		t.Fatalf("createPPIOChannels returned error: %v", err)
	}

	// OpenAI + Anthropic + Multimodal
	if len(created) != 3 {
		t.Fatalf("expected 3 created channels, got %d", len(created))
	}

	var anthropicFound, multimodalFound bool
	for _, ch := range created {
		switch ch.Type {
		case model.ChannelTypeAnthropic:
			anthropicFound = true

			if gotPure := ch.Configs.GetBool("pure_passthrough"); !gotPure {
				t.Fatalf("anthropic pure_passthrough = false, want true")
			}
		case model.ChannelTypePPIO:
			pathBaseMap, ok := ch.Configs[model.ChannelConfigPathBaseMapKey].(map[string]string)
			if !ok {
				t.Fatalf(
					"openai channel path_base_map missing or wrong type: %#v",
					ch.Configs[model.ChannelConfigPathBaseMapKey],
				)
			}

			if gotBase := pathBaseMap[ppiorelay.PathPrefixResponses]; gotBase != ppioResponsesBase(
				DefaultPPIOAPIBase,
			) {
				t.Fatalf(
					"responses base = %q, want %q",
					gotBase,
					ppioResponsesBase(DefaultPPIOAPIBase),
				)
			}
		case model.ChannelTypePPIOMultimodal:
			multimodalFound = true

			if !ch.Configs.GetBool(model.ChannelConfigAllowPassthroughUnknown) {
				t.Fatalf("multimodal allow_passthrough_unknown = false, want true")
			}

			if ch.BaseURL != DefaultPPIOMultimodalBase {
				t.Fatalf("multimodal base_url = %q, want %q", ch.BaseURL, DefaultPPIOMultimodalBase)
			}
		}
	}

	if !anthropicFound {
		t.Fatalf("expected anthropic channel to be created")
	}

	if !multimodalFound {
		t.Fatalf("expected multimodal channel to be created")
	}
}

// seedPPIOChannelsWithModels creates the three PPIO channel types pre-populated
// with stale model lists so tests can verify which channel types get replaced
// vs preserved under different skip-flag combinations.
func seedPPIOChannelsWithModels(t *testing.T) {
	t.Helper()

	channels := []model.Channel{
		{
			Name:    "PPIO (OpenAI)",
			Type:    model.ChannelTypePPIO,
			BaseURL: DefaultPPIOAPIBase,
			Key:     "ppio-key",
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"stale-openai"},
		},
		{
			Name:    "PPIO (Anthropic)",
			Type:    model.ChannelTypeAnthropic,
			BaseURL: DefaultPPIOAnthropicBase,
			Key:     "ppio-key",
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"stale-claude"},
		},
		{
			Name:    "PPIO (Multimodal)",
			Type:    model.ChannelTypePPIOMultimodal,
			BaseURL: DefaultPPIOMultimodalBase,
			Key:     "ppio-key",
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"stale-seedream"},
		},
	}

	for i := range channels {
		if err := model.DB.Create(&channels[i]).Error; err != nil {
			t.Fatalf("failed to seed channel %q: %v", channels[i].Name, err)
		}
	}
}

func modelsByType(t *testing.T) map[model.ChannelType][]string {
	t.Helper()

	var got []model.Channel
	if err := model.DB.Order("id asc").Find(&got).Error; err != nil {
		t.Fatalf("failed to load channels: %v", err)
	}

	out := make(map[model.ChannelType][]string, len(got))
	for _, ch := range got {
		out[ch.Type] = append([]string(nil), ch.Models...)
	}

	return out
}

// Regression: multimodal API fetch failure must not wipe the multimodal channel.
// skipMultimodalUpdate=true should preserve existing Models while chat channels
// still get their fresh lists.
func TestEnsurePPIOChannelsFromModels_SkipMultimodalPreservesChannel(t *testing.T) {
	setupPPIOChannelTestDB(t)
	seedPPIOChannelsWithModels(t)

	_, err := ensurePPIOChannelsFromModels(
		[]string{"claude-sonnet-4-20250514"},
		[]string{"deepseek-v3"},
		nil,   // multimodal fetch skipped
		false, // skipChatUpdate
		true,  // skipMultimodalUpdate
		false, // autoCreate
		nil, nil, PPIOConfigResult{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := modelsByType(t)

	if want := []string{
		"claude-sonnet-4-20250514",
	}; !slices.Equal(
		got[model.ChannelTypeAnthropic],
		want,
	) {
		t.Errorf("anthropic Models = %v, want %v", got[model.ChannelTypeAnthropic], want)
	}

	if want := []string{"deepseek-v3"}; !slices.Equal(got[model.ChannelTypePPIO], want) {
		t.Errorf("openai Models = %v, want %v", got[model.ChannelTypePPIO], want)
	}

	// Critical: multimodal must be preserved, not wiped.
	if want := []string{
		"stale-seedream",
	}; !slices.Equal(
		got[model.ChannelTypePPIOMultimodal],
		want,
	) {
		t.Errorf(
			"multimodal Models = %v, want preserved %v",
			got[model.ChannelTypePPIOMultimodal],
			want,
		)
	}
}

// Regression: chat API fetch failure must not wipe OpenAI/Anthropic channels.
// skipChatUpdate=true should preserve existing Models while multimodal channel
// still gets its fresh list.
func TestEnsurePPIOChannelsFromModels_SkipChatPreservesChannels(t *testing.T) {
	setupPPIOChannelTestDB(t)
	seedPPIOChannelsWithModels(t)

	_, err := ensurePPIOChannelsFromModels(
		nil, nil,
		[]string{"seedream-5.0-lite"},
		true,  // skipChatUpdate
		false, // skipMultimodalUpdate
		false, // autoCreate
		nil, nil, PPIOConfigResult{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := modelsByType(t)

	if want := []string{"stale-claude"}; !slices.Equal(got[model.ChannelTypeAnthropic], want) {
		t.Errorf("anthropic Models = %v, want preserved %v", got[model.ChannelTypeAnthropic], want)
	}

	if want := []string{"stale-openai"}; !slices.Equal(got[model.ChannelTypePPIO], want) {
		t.Errorf("openai Models = %v, want preserved %v", got[model.ChannelTypePPIO], want)
	}

	if want := []string{
		"seedream-5.0-lite",
	}; !slices.Equal(
		got[model.ChannelTypePPIOMultimodal],
		want,
	) {
		t.Errorf("multimodal Models = %v, want %v", got[model.ChannelTypePPIOMultimodal], want)
	}
}

// Startup refresh path: both sources empty means preserve all channel Models,
// only channel configs get updated.
func TestEnsurePPIOChannelsFromModels_SkipBothPreservesAll(t *testing.T) {
	setupPPIOChannelTestDB(t)
	seedPPIOChannelsWithModels(t)

	_, err := ensurePPIOChannelsFromModels(
		nil, nil, nil,
		true, true, // both skipped
		false,
		nil, nil, PPIOConfigResult{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := modelsByType(t)

	if want := []string{"stale-claude"}; !slices.Equal(got[model.ChannelTypeAnthropic], want) {
		t.Errorf("anthropic Models = %v, want preserved %v", got[model.ChannelTypeAnthropic], want)
	}

	if want := []string{"stale-openai"}; !slices.Equal(got[model.ChannelTypePPIO], want) {
		t.Errorf("openai Models = %v, want preserved %v", got[model.ChannelTypePPIO], want)
	}

	if want := []string{
		"stale-seedream",
	}; !slices.Equal(
		got[model.ChannelTypePPIOMultimodal],
		want,
	) {
		t.Errorf(
			"multimodal Models = %v, want preserved %v",
			got[model.ChannelTypePPIOMultimodal],
			want,
		)
	}
}
