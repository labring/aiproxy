package openai

import (
	"fmt"
	"time"

	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

const defaultResponsesFirstEventTimeoutSeconds uint32 = 2

type Config struct {
	ChatUpstreamProtocol           string `json:"chat_upstream_protocol"`
	MapReasoningToReasoningContent bool   `json:"map_reasoning_to_reasoning_content"`
	ResponsesFirstEventTimeout     uint32 `json:"responses_first_event_timeout"`
}

// DoResponseOptions contains the response handling options needed by the OpenAI Responses path.
type DoResponseOptions struct {
	ResponsesFirstEventTimeout time.Duration
}

func defaultConfig() Config {
	return Config{
		ResponsesFirstEventTimeout: defaultResponsesFirstEventTimeoutSeconds,
	}
}

// ConfigSchema returns the channel configuration schema shared by OpenAI-compatible adaptors.
func ConfigSchema() map[string]any {
	return configSchema()
}

// ChatUpstreamProtocolConfigSchema exposes protocol selection only on adaptors
// that use the complete OpenAI request and response conversion path.
func ChatUpstreamProtocolConfigSchema() map[string]any {
	schema := configSchema()
	properties, _ := schema["properties"].(map[string]any)
	properties["chat_upstream_protocol"] = map[string]any{
		"type":        "string",
		"title":       "Chat upstream protocol",
		"description": "Upstream protocol for incoming Chat Completions requests. Explicit choices override model type and model-name detection for every model on this channel. Other incoming endpoints are unchanged. The upstream must support the selected protocol.",
		"enum":        []string{"auto", "responses", "chat_completions"},
		"default":     "auto",
	}

	return schema
}

func (c Config) responsesFirstEventTimeout() time.Duration {
	return time.Duration(c.ResponsesFirstEventTimeout) * time.Second
}

func (c Config) doResponseOptions() DoResponseOptions {
	return DoResponseOptions{
		ResponsesFirstEventTimeout: c.responsesFirstEventTimeout(),
	}
}

// LoadDoResponseOptions loads OpenAI response handling options from channel config.
func LoadDoResponseOptions(meta *meta.Meta) (DoResponseOptions, error) {
	cfg := defaultConfig()
	if meta == nil {
		return cfg.doResponseOptions(), nil
	}

	if err := meta.ChannelConfigs.LoadConfig(&cfg); err != nil {
		return DoResponseOptions{}, err
	}

	return cfg.doResponseOptions(), nil
}

func (a *Adaptor) loadConfig(meta *meta.Meta) (Config, error) {
	cfg := defaultConfig()
	return a.configCache.Load(meta, cfg)
}

func configSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"responses_first_event_timeout": map[string]any{
				"type":        "integer",
				"title":       "Responses first event timeout",
				"description": "Maximum seconds to buffer initial Responses API lifecycle events while waiting for the first output or error event. Increase this value to allow late upstream errors to trigger channel retries.",
				"default":     defaultResponsesFirstEventTimeoutSeconds,
				"minimum":     0,
			},
			"map_reasoning_to_reasoning_content": map[string]any{
				"type":        "boolean",
				"title":       "Map reasoning To reasoning_content",
				"description": "Rewrite upstream chat completion `reasoning` fields to `reasoning_content` in both streaming and non-streaming responses.",
			},
		},
	}
}

func getChatCompletionResponsePreHandlers(
	meta *meta.Meta,
) (streamPreHandler, handlerPreHandler PreHandler, err error) {
	return (&Adaptor{}).getChatCompletionResponsePreHandlers(meta)
}

func (a *Adaptor) getChatCompletionResponsePreHandlers(
	meta *meta.Meta,
) (streamPreHandler, handlerPreHandler PreHandler, err error) {
	cfg, err := a.loadConfig(meta)
	if err != nil {
		return nil, nil, err
	}

	if !cfg.MapReasoningToReasoningContent {
		return nil, nil, nil
	}

	return StreamReasoningToReasoningContentPreHandler,
		ReasoningToReasoningContentPreHandler,
		nil
}

// ShouldUseResponsesAPI selects the upstream protocol for the current channel.
// Explicit channel settings only override Chat Completions requests; other modes
// retain their existing model-based behavior. Read the selected channel snapshot
// directly so retries cannot reuse a protocol decision from a previous channel.
func ShouldUseResponsesAPI(mt *meta.Meta) (bool, error) {
	if mt.Mode == mode.ChatCompletions {
		cfg := defaultConfig()
		if err := mt.ChannelConfigs.LoadConfig(&cfg); err != nil {
			return false, fmt.Errorf("load channel config: %w", err)
		}

		switch cfg.ChatUpstreamProtocol {
		case "", "auto":
		case "responses":
			return true, nil
		case "chat_completions":
			return false, nil
		default:
			return false, fmt.Errorf(
				"invalid chat_upstream_protocol %q: expected auto, responses, or chat_completions",
				cfg.ChatUpstreamProtocol,
			)
		}
	}

	return IsResponsesOnlyModelAny(&mt.ModelConfig, mt.OriginModel, mt.ActualModel), nil
}
