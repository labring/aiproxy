package openai

import "github.com/labring/aiproxy/core/relay/meta"

type Config struct {
	MapReasoningToReasoningContent bool `json:"map_reasoning_to_reasoning_content"`
}

func loadConfig(meta *meta.Meta) (Config, error) {
	cfg := Config{}

	if err := meta.ChannelConfigs.LoadConfig(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func configSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
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
	cfg, err := loadConfig(meta)
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
