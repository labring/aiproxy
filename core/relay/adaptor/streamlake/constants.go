package streamlake

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

var ModelList = []model.ModelConfig{
	{
		Model: "KAT-Coder",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
		Price: model.Price{
			InputPrice:  0.0095,
			OutputPrice: 0.038,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{
						InputTokenMax: 32000,
					},
					Price: model.Price{
						InputPrice:  0.0055,
						OutputPrice: 0.022,
					},
				},
				{
					Condition: model.PriceCondition{
						InputTokenMin: 32001,
						InputTokenMax: 128000,
					},
					Price: model.Price{
						InputPrice:  0.0065,
						OutputPrice: 0.026,
					},
				},
				{
					Condition: model.PriceCondition{
						InputTokenMin: 128001,
					},
					Price: model.Price{
						InputPrice:  0.0095,
						OutputPrice: 0.038,
					},
				},
			},
		},
		RPM: 20,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(256000),
			model.WithModelConfigMaxOutputTokens(32000),
			model.WithModelConfigToolChoice(true),
		),
	},
}
