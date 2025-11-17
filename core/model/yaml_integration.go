package model

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/relay/mode"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ChannelItem wraps Channel for YAML configuration
// Adds TypeName field for human-readable channel type specification
type ChannelItem struct {
	Channel  `       yaml:",inline"` // Embed Channel to inherit all fields
	TypeName string                  `yaml:"type_name,omitempty"` // Alternative to Type (e.g., "openai", "claude")
}

// GetChannelType returns the channel type, converting from TypeName if Type is not set
func (c *ChannelItem) GetChannelType() ChannelType {
	if c.Type != 0 {
		return c.Type
	}
	// Convert TypeName to Type
	return ChannelType(ChannelTypeNameToType(c.TypeName))
}

// ModelConfigItem wraps ModelConfig for YAML configuration
// Adds TypeName field for human-readable model type specification
type ModelConfigItem struct {
	ModelConfig `       yaml:",inline"` // Embed ModelConfig to inherit all fields
	TypeName    string                  `yaml:"type_name,omitempty"` // Alternative to Type (e.g., "chat", "embedding")
}

// GetModelType returns the model type, converting from TypeName if Type is not set
func (m *ModelConfigItem) GetModelType() mode.Mode {
	if m.Type != 0 {
		return m.Type
	}
	// Convert TypeName to Type
	return ModelTypeNameToType(m.TypeName)
}

// ChannelTypeNameToType converts a channel type name to its numeric type
func ChannelTypeNameToType(typeName string) int {
	typeName = strings.ToLower(strings.TrimSpace(typeName))

	typeMap := map[string]int{
		"openai":                                1,
		"azure":                                 3,
		"azure2":                                4,
		"google gemini (openai)":                12,
		"gemini-openai":                         12,
		"baidu v2":                              13,
		"baiduv2":                               13,
		"anthropic":                             14,
		"claude":                                14,
		"baidu":                                 15,
		"zhipu":                                 16,
		"ali":                                   17,
		"aliyun":                                17,
		"xunfei":                                18,
		"ai360":                                 19,
		"360":                                   19,
		"openrouter":                            20,
		"tencent":                               23,
		"google gemini":                         24,
		"gemini":                                24,
		"moonshot":                              25,
		"baichuan":                              26,
		"minimax":                               27,
		"mistral":                               28,
		"groq":                                  29,
		"ollama":                                30,
		"lingyiwanwu":                           31,
		"stepfun":                               32,
		"aws":                                   33,
		"coze":                                  34,
		"cohere":                                35,
		"deepseek":                              36,
		"cloudflare":                            37,
		"doubao":                                40,
		"novita":                                41,
		"vertexai":                              42,
		"vertex":                                42,
		"siliconflow":                           43,
		"doubao audio":                          44,
		"doubaoaudio":                           44,
		"xai":                                   45,
		"doc2x":                                 46,
		"jina":                                  47,
		"huggingface text-embeddings-inference": 48,
		"text-embeddings-inference":             48,
		"tei":                                   48,
		"qianfan":                               49,
		"sangfor aicp":                          50,
		"aicp":                                  50,
		"streamlake":                            51,
		"zhipu coding":                          52,
		"zhipucoding":                           52,
	}

	if typ, ok := typeMap[typeName]; ok {
		return typ
	}

	return 0
}

// ModelTypeNameToType converts a model type name to its numeric type
func ModelTypeNameToType(typeName string) mode.Mode {
	typeName = strings.ToLower(strings.TrimSpace(typeName))

	typeMap := map[string]mode.Mode{
		"chat":                    mode.ChatCompletions,
		"chatcompletion":          mode.ChatCompletions,
		"chatcompletions":         mode.ChatCompletions,
		"completion":              mode.Completions,
		"completions":             mode.Completions,
		"embedding":               mode.Embeddings,
		"embeddings":              mode.Embeddings,
		"moderation":              mode.Moderations,
		"moderations":             mode.Moderations,
		"image":                   mode.ImagesGenerations,
		"imagegeneration":         mode.ImagesGenerations,
		"imagegenerations":        mode.ImagesGenerations,
		"imageedit":               mode.ImagesEdits,
		"imageedits":              mode.ImagesEdits,
		"audio":                   mode.AudioSpeech,
		"audiospeech":             mode.AudioSpeech,
		"speech":                  mode.AudioSpeech,
		"audiotranscription":      mode.AudioTranscription,
		"transcription":           mode.AudioTranscription,
		"audiotranslation":        mode.AudioTranslation,
		"translation":             mode.AudioTranslation,
		"rerank":                  mode.Rerank,
		"parsepdf":                mode.ParsePdf,
		"pdf":                     mode.ParsePdf,
		"anthropic":               mode.Anthropic,
		"videogeneration":         mode.VideoGenerationsJobs,
		"videogenerationsjobs":    mode.VideoGenerationsJobs,
		"videogenerationsgetjobs": mode.VideoGenerationsGetJobs,
		"videogenerationscontent": mode.VideoGenerationsContent,
		"responses":               mode.Responses,
		"responsesget":            mode.ResponsesGet,
		"responsesdelete":         mode.ResponsesDelete,
		"responsescancel":         mode.ResponsesCancel,
		"responsesinputitems":     mode.ResponsesInputItems,
	}

	if typ, ok := typeMap[typeName]; ok {
		return typ
	}

	return mode.Unknown
}

// YAMLConfig represents the complete configuration with proper types
type YAMLConfig struct {
	Channels     []ChannelItem     `yaml:"channels,omitempty"`
	ModelConfigs []ModelConfigItem `yaml:"modelconfigs,omitempty"`
	Options      map[string]string `yaml:"options,omitempty"`
}

var (
	yamlConfigCache      *YAMLConfig
	yamlConfigCacheTime  time.Time
	yamlConfigCacheMutex sync.RWMutex
	yamlConfigCacheTTL   = 60 * time.Second
)

// LoadYAMLConfig loads and parses YAML configuration with proper types
// Uses a 60-second cache with double-check locking for performance
func LoadYAMLConfig() *YAMLConfig {
	yamlConfigCacheMutex.RLock()

	if yamlConfigCache != nil && time.Since(yamlConfigCacheTime) < yamlConfigCacheTTL {
		cache := yamlConfigCache

		yamlConfigCacheMutex.RUnlock()
		return cache
	}

	yamlConfigCacheMutex.RUnlock()

	// Acquire write lock to update cache
	yamlConfigCacheMutex.Lock()
	defer yamlConfigCacheMutex.Unlock()

	// Double check: another goroutine might have updated the cache
	if yamlConfigCache != nil && time.Since(yamlConfigCacheTime) < yamlConfigCacheTTL {
		return yamlConfigCache
	}

	// Load raw YAML data from file
	data, err := config.LoadYAMLConfigData()
	if err != nil {
		if os.IsNotExist(err) {
			yamlConfigCache = nil
			yamlConfigCacheTime = time.Now()
			return nil
		}

		log.Errorf("load config: %v", err)

		yamlConfigCache = nil
		yamlConfigCacheTime = time.Now()

		return nil
	}

	// Parse YAML directly into our types
	var yamlConfig YAMLConfig
	//nolint:musttag
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		log.Errorf("unmarshal config: %v", err)

		yamlConfigCache = nil
		yamlConfigCacheTime = time.Now()

		return nil
	}

	// Update cache
	yamlConfigCache = &yamlConfig
	yamlConfigCacheTime = time.Now()

	return yamlConfigCache
}

// applyYAMLConfigToModelConfigCache applies YAML model configs to the model config cache
// Creates a wrapper cache that checks YAML first, then falls back to database cache
func applyYAMLConfigToModelConfigCache(
	cache ModelConfigCache,
) ModelConfigCache {
	yamlConfig := LoadYAMLConfig()
	if yamlConfig == nil || len(yamlConfig.ModelConfigs) == 0 {
		// No YAML model configs, use existing cache from database
		return cache
	}

	// Build YAML model config map
	yamlModelConfigMap := make(map[string]ModelConfig)
	for i := range yamlConfig.ModelConfigs {
		modelConfigItem := &yamlConfig.ModelConfigs[i]

		// Convert ModelConfigItem to ModelConfig
		modelConfig := modelConfigItem.ModelConfig

		// Convert TypeName to Type if Type is not set
		if modelConfigItem.TypeName != "" && modelConfig.Type == 0 {
			modelConfig.Type = modelConfigItem.GetModelType()
		}

		if modelConfig.Model != "" {
			yamlModelConfigMap[modelConfig.Model] = modelConfig
		}
	}

	log.Infof("loaded %d model configs from config", len(yamlModelConfigMap))

	// Create wrapper cache: YAML configs override database configs
	wrappedCache := &yamlModelConfigCache{
		yamlConfigs: yamlModelConfigMap,
		dbCache:     cache,
	}

	return wrappedCache
}

// yamlModelConfigCache wraps database cache with YAML overrides
// When looking up a model config:
// 1. First check YAML configs (high priority)
// 2. If not found, fall back to database cache (low priority)
var _ ModelConfigCache = (*yamlModelConfigCache)(nil)

type yamlModelConfigCache struct {
	yamlConfigs map[string]ModelConfig
	dbCache     ModelConfigCache
}

func (y *yamlModelConfigCache) GetModelConfig(model string) (ModelConfig, bool) {
	// First check YAML configs
	if config, ok := y.yamlConfigs[model]; ok {
		return config, true
	}

	// Fall back to database cache
	return y.dbCache.GetModelConfig(model)
}

// NewConfigChannels merges YAML channels with database channels
// YAML channels are assigned negative IDs to distinguish them from database channels
// Note: YAML channels are NOT persisted to the database
func NewConfigChannels(yamlConfig *YAMLConfig, status int) []*Channel {
	if yamlConfig == nil || len(yamlConfig.Channels) == 0 {
		return nil
	}

	newChannels := make([]*Channel, 0, len(yamlConfig.Channels))

	// Start negative IDs from -1000 to avoid conflicts
	nextNegativeID := -1

	// Add all YAML channels with negative IDs (they don't override database channels)
	for _, yamlChannelItem := range yamlConfig.Channels {
		// Convert ChannelItem to Channel
		channel := &yamlChannelItem.Channel

		if status != 0 && channel.Status != status {
			continue
		}

		// Convert TypeName to Type if Type is not set
		if yamlChannelItem.TypeName != "" && channel.Type == 0 {
			channel.Type = yamlChannelItem.GetChannelType()
		}

		// Assign negative ID to distinguish from database channels
		channel.ID = nextNegativeID
		nextNegativeID--

		initializeChannelModels(channel)
		initializeChannelModelMapping(channel)

		newChannels = append(newChannels, channel)
	}

	return newChannels
}
