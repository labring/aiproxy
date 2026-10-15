package typesafeai

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

const baseURL = "https://api.typesafe.ai/v1"

var ModelList = []model.ModelConfig{
	{
		Model: "gpt-4o",
		Type:  mode.SystemOne,
		Owner: model.ModelOwnerTypeSafeAI,
		Price: model.Price{
			InputPrice:  0.005,
			OutputPrice: 0.015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(16385),
			model.WithModelConfigMaxOutputTokens(16385),
		),
	},
	{
		Model: "gpt-4o-mini",
		Type:  mode.SystemOne,
		Owner: model.ModelOwnerTypeSafeAI,
		Price: model.Price{
			InputPrice:  0.00015,
			OutputPrice: 0.0006,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(16385),
			model.WithModelConfigMaxOutputTokens(16385),
		),
	},
	{
		Model: "gpt-4-turbo",
		Type:  mode.SystemOne,
		Owner: model.ModelOwnerTypeSafeAI,
		Price: model.Price{
			InputPrice:  0.01,
			OutputPrice: 0.03,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(128000),
			model.WithModelConfigMaxOutputTokens(4096),
		),
	},
	{
		Model: "gpt-3.5-turbo",
		Type:  mode.SystemOne,
		Owner: model.ModelOwnerTypeSafeAI,
		Price: model.Price{
			InputPrice:  0.0005,
			OutputPrice: 0.0015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(16385),
			model.WithModelConfigMaxOutputTokens(4096),
		),
	},
}
