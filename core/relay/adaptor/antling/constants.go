package antling

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

var ModelList = []model.ModelConfig{
	{
		Model: "Ling-1T",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAntGroup,
		Config: model.NewModelConfig(
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "Ling-2.5-1T",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAntGroup,
		Config: model.NewModelConfig(
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "Ling-flash-2.0",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAntGroup,
		Config: model.NewModelConfig(
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "Ling-mini-2.0",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAntGroup,
		Config: model.NewModelConfig(
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "Ring-2.5-1T",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAntGroup,
		Config: model.NewModelConfig(
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "Ring-flash-2.0",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAntGroup,
		Config: model.NewModelConfig(
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "Ring-mini-2.0",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAntGroup,
		Config: model.NewModelConfig(
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "Ming-flash-omni-2.0",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAntGroup,
		Config: model.NewModelConfig(
			model.WithModelConfigVision(true),
			model.WithModelConfigToolChoice(true),
		),
	},
}
