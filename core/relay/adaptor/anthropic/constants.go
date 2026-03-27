package anthropic

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

var ModelList = []model.ModelConfig{
	{
		Model: "claude-3-haiku-20240307",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.0025,
			OutputPrice: 0.0125,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(4096),
		),
	},
	{
		Model: "claude-3-opus-20240229",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.015,
			OutputPrice: 0.075,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(4096),
		),
	},
	{
		Model: "claude-3-5-haiku-20241022",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.0008,
			OutputPrice: 0.004,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(4096),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "claude-3-5-sonnet-20240620",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(8192),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "claude-3-5-sonnet-20241022",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(8192),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "claude-3-5-sonnet-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(8192),
			model.WithModelConfigToolChoice(true),
		),
	},
	// Claude 3.7 Sonnet
	{
		Model: "claude-3-7-sonnet-20250219",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(131072),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "claude-3-7-sonnet-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(131072),
			model.WithModelConfigToolChoice(true),
		),
	},
	// Claude Sonnet 4
	{
		Model: "claude-sonnet-4-20250514",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(65536),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "claude-sonnet-4-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(65536),
			model.WithModelConfigToolChoice(true),
		),
	},
	// Claude Opus 4
	{
		Model: "claude-opus-4-20250514",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.015,
			OutputPrice: 0.075,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(32768),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "claude-opus-4-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.015,
			OutputPrice: 0.075,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(32768),
			model.WithModelConfigToolChoice(true),
		),
	},
	// Claude Sonnet 4.5
	{
		Model: "claude-sonnet-4-5-20250514",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(64000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "claude-sonnet-4-5-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(64000),
			model.WithModelConfigToolChoice(true),
		),
	},
	// Claude Opus 4.5
	{
		Model: "claude-opus-4-5-20250514",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.015,
			OutputPrice: 0.075,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(64000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "claude-opus-4-5-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAnthropic,
		Price: model.Price{
			InputPrice:  0.015,
			OutputPrice: 0.075,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigMaxOutputTokens(64000),
			model.WithModelConfigToolChoice(true),
		),
	},
}
