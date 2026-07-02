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

func TestChannelConfigCacheScopesGroupChannelID(t *testing.T) {
	cache := &utils.ChannelConfigCache[testChannelConfig]{}
	globalMeta := meta.NewMeta(
		&coremodel.Channel{
			ID:      7,
			Configs: coremodel.ChannelConfigs{"enabled": true},
		},
		mode.ChatCompletions,
		"test-model",
		coremodel.ModelConfig{},
	)
	groupMeta := meta.NewMeta(
		&coremodel.Channel{
			ID:      7,
			Configs: coremodel.ChannelConfigs{"enabled": false},
		},
		mode.ChatCompletions,
		"test-model",
		coremodel.ModelConfig{},
	)
	groupMeta.Channel.Scope = coremodel.ChannelScopeGroup
	groupMeta.Channel.GroupID = "group-1"

	globalConfig, err := cache.Load(globalMeta, testChannelConfig{})
	if err != nil {
		t.Fatalf("load global config: %v", err)
	}

	if !globalConfig.Enabled {
		t.Fatal("expected global config to be enabled")
	}

	groupConfig, err := cache.Load(groupMeta, testChannelConfig{Enabled: true})
	if err != nil {
		t.Fatalf("load group config: %v", err)
	}

	if groupConfig.Enabled {
		t.Fatal("expected group config to use scoped channel configs")
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

func TestPluginConfigCacheScopesGroupModelConfig(t *testing.T) {
	cache := &utils.PluginConfigCache[testPluginConfig]{}
	groupAConfig := coremodel.ModelConfig{
		Model: "test-model",
		Plugin: map[string]map[string]any{
			"test-plugin": {"enabled": true},
		},
	}
	groupBConfig := coremodel.ModelConfig{
		Model: "test-model",
		Plugin: map[string]map[string]any{
			"test-plugin": {"enabled": false},
		},
	}

	groupAMeta := meta.NewMeta(nil, mode.ChatCompletions, "test-model", groupAConfig)
	groupAMeta.Channel.Scope = coremodel.ChannelScopeGroup
	groupAMeta.Channel.GroupID = "group-a"

	groupBMeta := meta.NewMeta(nil, mode.ChatCompletions, "test-model", groupBConfig)
	groupBMeta.Channel.Scope = coremodel.ChannelScopeGroup
	groupBMeta.Channel.GroupID = "group-b"

	groupAPluginConfig, err := cache.Load(groupAMeta, "test-plugin", testPluginConfig{})
	if err != nil {
		t.Fatalf("load group A plugin config: %v", err)
	}

	if !groupAPluginConfig.Enabled {
		t.Fatal("expected group A plugin config to be enabled")
	}

	groupBPluginConfig, err := cache.Load(groupBMeta, "test-plugin", testPluginConfig{})
	if err != nil {
		t.Fatalf("load group B plugin config: %v", err)
	}

	if groupBPluginConfig.Enabled {
		t.Fatal("expected group B plugin config to use its scoped model config")
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
