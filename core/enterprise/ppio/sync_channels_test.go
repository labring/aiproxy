//go:build enterprise

package ppio

import (
	"path/filepath"
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
		false,
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
				t.Fatalf("openai channel path_base_map missing or wrong type: %#v", ch.Configs[model.ChannelConfigPathBaseMapKey])
			}

			if gotBase := pathBaseMap[ppiorelay.PathPrefixResponses]; gotBase != ppioResponsesBase(DefaultPPIOAPIBase) {
				t.Fatalf("responses base = %#v, want %q", gotBase, ppioResponsesBase(DefaultPPIOAPIBase))
			}

			if gotBase := pathBaseMap[ppiorelay.PathPrefixWebSearch]; gotBase != ppioWebSearchBase(DefaultPPIOAPIBase) {
				t.Fatalf("web search base = %#v, want %q", gotBase, ppioWebSearchBase(DefaultPPIOAPIBase))
			}

			if gotAllow := ch.Configs.GetBool(model.ChannelConfigAllowPassthroughUnknown); !gotAllow {
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
	)
	if err != nil {
		t.Fatalf("createPPIOChannels returned error: %v", err)
	}

	if len(created) != 2 {
		t.Fatalf("expected 2 created channels, got %d", len(created))
	}

	var anthropicFound bool
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
				t.Fatalf("openai channel path_base_map missing or wrong type: %#v", ch.Configs[model.ChannelConfigPathBaseMapKey])
			}

			if gotBase := pathBaseMap[ppiorelay.PathPrefixResponses]; gotBase != ppioResponsesBase(DefaultPPIOAPIBase) {
				t.Fatalf("responses base = %q, want %q", gotBase, ppioResponsesBase(DefaultPPIOAPIBase))
			}
		}
	}

	if !anthropicFound {
		t.Fatalf("expected anthropic channel to be created")
	}
}
