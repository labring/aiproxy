//go:build enterprise

package novita

import (
	"path/filepath"
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
		false,
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
				t.Fatalf("openai channel path_base_map missing or wrong type: %#v", ch.Configs[model.ChannelConfigPathBaseMapKey])
			}

			if gotBase := pathBaseMap["/v1/responses"]; gotBase != novitaResponsesBase(DefaultNovitaAPIBase) {
				t.Fatalf("responses base = %#v, want %q", gotBase, novitaResponsesBase(DefaultNovitaAPIBase))
			}

			if gotAllow := ch.Configs.GetBool(model.ChannelConfigAllowPassthroughUnknown); !gotAllow {
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
