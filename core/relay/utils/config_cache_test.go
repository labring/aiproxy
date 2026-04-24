package utils_test

import (
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

type testChannelConfig struct {
	Enabled bool `json:"enabled"`
}

type testPluginConfig struct {
	Enabled bool `json:"enabled"`
}

func TestChannelConfigCacheUsesChannelID(t *testing.T) {
	cache := &utils.ChannelConfigCache[testChannelConfig]{}
	channel := &coremodel.Channel{
		ID:      7,
		Configs: coremodel.ChannelConfigs{"enabled": true},
	}

	m := meta.NewMeta(channel, mode.ChatCompletions, "test-model", coremodel.ModelConfig{})

	cfg, err := cache.Load(m, testChannelConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("expected enabled config from initial load")
	}

	m.ChannelConfigs = coremodel.ChannelConfigs{"enabled": false}

	cfg, err = cache.Load(m, testChannelConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("expected cached config for same channel id")
	}
}

func TestPluginConfigCacheUsesModelName(t *testing.T) {
	cache := &utils.PluginConfigCache[testPluginConfig]{}
	modelConfig := coremodel.ModelConfig{
		Model: "test-model",
		Plugin: map[string]map[string]any{
			"test-plugin": {"enabled": true},
		},
	}

	m := meta.NewMeta(nil, mode.ChatCompletions, "test-model", modelConfig)

	cfg, err := cache.Load(m, "test-plugin", testPluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("expected enabled plugin config from initial load")
	}

	m.ModelConfig.Plugin["test-plugin"]["enabled"] = false

	cfg, err = cache.Load(m, "test-plugin", testPluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("expected cached plugin config for same model name")
	}
}

func TestChannelConfigCacheBypassesZeroChannelID(t *testing.T) {
	cache := &utils.ChannelConfigCache[testChannelConfig]{}
	channel := &coremodel.Channel{
		ID:      0,
		Configs: coremodel.ChannelConfigs{"enabled": true},
	}

	m := meta.NewMeta(channel, mode.ChatCompletions, "test-model", coremodel.ModelConfig{})

	cfg, err := cache.Load(m, testChannelConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("expected enabled config from initial load")
	}

	m.ChannelConfigs = coremodel.ChannelConfigs{"enabled": false}

	cfg, err = cache.Load(m, testChannelConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Enabled {
		t.Fatal("expected uncached config reload for zero channel id")
	}
}
