//go:build enterprise

package novita

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
)

func setupNovitaChannelTestDB(t *testing.T) {
	t.Helper()

	prevDB := model.DB
	prevUsingSQLite := common.UsingSQLite

	testDB, err := model.OpenSQLite(filepath.Join(t.TempDir(), "novita-sync.db"))
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

func TestEnsureNovitaChannelsFromModels_UpdatesChannelConfigs(t *testing.T) {
	setupNovitaChannelTestDB(t)

	channels := []model.Channel{
		{
			Name:    "Novita (OpenAI)",
			Type:    model.ChannelTypeNovita,
			BaseURL: DefaultNovitaAPIBase,
			Key:     "novita-key",
			Status:  model.ChannelStatusEnabled,
		},
		{
			Name:    "Novita (Anthropic)",
			Type:    model.ChannelTypeAnthropic,
			BaseURL: DefaultNovitaAnthropicBase,
			Key:     "novita-key",
			Status:  model.ChannelStatusEnabled,
		},
	}

	for i := range channels {
		if err := model.DB.Create(&channels[i]).Error; err != nil {
			t.Fatalf("failed to seed channel %q: %v", channels[i].Name, err)
		}
	}

	purePassthrough := true
	allowUnknown := true

	info, err := ensureNovitaChannelsFromModels(
		[]string{"claude-sonnet-4-20250514"},
		[]string{"deepseek-v3"},
		nil,   // multimodalModels
		false, // skipChatUpdate
		true,  // skipMultimodalUpdate (nil multimodal → skip)
		false, // autoCreate
		&purePassthrough,
		&allowUnknown,
		NovitaConfigResult{},
	)
	if err != nil {
		t.Fatalf("ensureNovitaChannelsFromModels returned error: %v", err)
	}

	if !info.Novita.Exists {
		t.Fatalf("expected Novita channel info to exist")
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
		case model.ChannelTypeNovita:
			pathBaseMap, ok := ch.Configs[model.ChannelConfigPathBaseMapKey].(map[string]any)
			if !ok {
				t.Fatalf(
					"openai channel path_base_map missing or wrong type: %#v",
					ch.Configs[model.ChannelConfigPathBaseMapKey],
				)
			}

			if gotBase := pathBaseMap["/v1/responses"]; gotBase != novitaResponsesBase(
				DefaultNovitaAPIBase,
			) {
				t.Fatalf(
					"responses base = %#v, want %q",
					gotBase,
					novitaResponsesBase(DefaultNovitaAPIBase),
				)
			}

			if gotAllow := ch.Configs.GetBool(
				model.ChannelConfigAllowPassthroughUnknown,
			); !gotAllow {
				t.Fatalf("allow_passthrough_unknown = false, want true")
			}
		case model.ChannelTypeAnthropic:
			if gotPure := ch.Configs.GetBool("pure_passthrough"); !gotPure {
				t.Fatalf("pure_passthrough = false, want true")
			}

			if gotSkip := ch.Configs.GetBool("skip_image_conversion"); !gotSkip {
				t.Fatalf("skip_image_conversion = false, want true")
			}

			if gotDisable := ch.Configs.GetBool("disable_context_management"); !gotDisable {
				t.Fatalf("disable_context_management = false, want true")
			}
		}
	}
}

func seedNovitaChannelsWithModels(t *testing.T) {
	t.Helper()

	channels := []model.Channel{
		{
			Name:    "Novita (OpenAI)",
			Type:    model.ChannelTypeNovita,
			BaseURL: DefaultNovitaAPIBase,
			Key:     "novita-key",
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"stale-openai"},
		},
		{
			Name:    "Novita (Anthropic)",
			Type:    model.ChannelTypeAnthropic,
			BaseURL: DefaultNovitaAnthropicBase,
			Key:     "novita-key",
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"stale-claude"},
		},
		{
			Name:    "Novita (Multimodal)",
			Type:    model.ChannelTypeNovitaMultimodal,
			BaseURL: DefaultNovitaMultimodalBase,
			Key:     "novita-key",
			Status:  model.ChannelStatusEnabled,
			Models:  []string{"stale-flux"},
		},
	}

	for i := range channels {
		if err := model.DB.Create(&channels[i]).Error; err != nil {
			t.Fatalf("failed to seed channel %q: %v", channels[i].Name, err)
		}
	}
}

func novitaModelsByType(t *testing.T) map[model.ChannelType][]string {
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
func TestEnsureNovitaChannelsFromModels_SkipMultimodalPreservesChannel(t *testing.T) {
	setupNovitaChannelTestDB(t)
	seedNovitaChannelsWithModels(t)

	_, err := ensureNovitaChannelsFromModels(
		[]string{"claude-sonnet-4-20250514"},
		[]string{"deepseek-v3"},
		nil,
		false, // skipChatUpdate
		true,  // skipMultimodalUpdate
		false,
		nil, nil, NovitaConfigResult{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := novitaModelsByType(t)

	if want := []string{
		"claude-sonnet-4-20250514",
	}; !slices.Equal(
		got[model.ChannelTypeAnthropic],
		want,
	) {
		t.Errorf("anthropic Models = %v, want %v", got[model.ChannelTypeAnthropic], want)
	}

	if want := []string{"deepseek-v3"}; !slices.Equal(got[model.ChannelTypeNovita], want) {
		t.Errorf("openai Models = %v, want %v", got[model.ChannelTypeNovita], want)
	}

	if want := []string{"stale-flux"}; !slices.Equal(got[model.ChannelTypeNovitaMultimodal], want) {
		t.Errorf(
			"multimodal Models = %v, want preserved %v",
			got[model.ChannelTypeNovitaMultimodal],
			want,
		)
	}
}

// Regression: chat API fetch failure must not wipe OpenAI/Anthropic channels.
func TestEnsureNovitaChannelsFromModels_SkipChatPreservesChannels(t *testing.T) {
	setupNovitaChannelTestDB(t)
	seedNovitaChannelsWithModels(t)

	_, err := ensureNovitaChannelsFromModels(
		nil, nil,
		[]string{"flux-schnell"},
		true,  // skipChatUpdate
		false, // skipMultimodalUpdate
		false,
		nil, nil, NovitaConfigResult{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := novitaModelsByType(t)

	if want := []string{"stale-claude"}; !slices.Equal(got[model.ChannelTypeAnthropic], want) {
		t.Errorf("anthropic Models = %v, want preserved %v", got[model.ChannelTypeAnthropic], want)
	}

	if want := []string{"stale-openai"}; !slices.Equal(got[model.ChannelTypeNovita], want) {
		t.Errorf("openai Models = %v, want preserved %v", got[model.ChannelTypeNovita], want)
	}

	if want := []string{
		"flux-schnell",
	}; !slices.Equal(
		got[model.ChannelTypeNovitaMultimodal],
		want,
	) {
		t.Errorf("multimodal Models = %v, want %v", got[model.ChannelTypeNovitaMultimodal], want)
	}
}

// Startup refresh: both sources empty means preserve all channel Models.
func TestEnsureNovitaChannelsFromModels_SkipBothPreservesAll(t *testing.T) {
	setupNovitaChannelTestDB(t)
	seedNovitaChannelsWithModels(t)

	_, err := ensureNovitaChannelsFromModels(
		nil, nil, nil,
		true, true,
		false,
		nil, nil, NovitaConfigResult{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := novitaModelsByType(t)

	if want := []string{"stale-claude"}; !slices.Equal(got[model.ChannelTypeAnthropic], want) {
		t.Errorf("anthropic Models = %v, want preserved %v", got[model.ChannelTypeAnthropic], want)
	}

	if want := []string{"stale-openai"}; !slices.Equal(got[model.ChannelTypeNovita], want) {
		t.Errorf("openai Models = %v, want preserved %v", got[model.ChannelTypeNovita], want)
	}

	if want := []string{"stale-flux"}; !slices.Equal(got[model.ChannelTypeNovitaMultimodal], want) {
		t.Errorf(
			"multimodal Models = %v, want preserved %v",
			got[model.ChannelTypeNovitaMultimodal],
			want,
		)
	}
}
