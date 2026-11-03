package openai

import (
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

var ModelList = []model.ModelConfig{
	{
		Model: "gpt-6-astra",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:         0.020,
			OutputPrice:        0.075,
			CachedPrice:        0.002,
			CacheCreationPrice: 0.025,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.020,
						OutputPrice:        0.100,
						CachedPrice:        0.002,
						CacheCreationPrice: 0.025,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.040,
						OutputPrice:        0.150,
						CachedPrice:        0.004,
						CacheCreationPrice: 0.050,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.005,
						OutputPrice:        0.025,
						CachedPrice:        0.0005,
						CacheCreationPrice: 0.00625,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.010,
						OutputPrice:        0.0375,
						CachedPrice:        0.001,
						CacheCreationPrice: 0.0125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:         0.010,
						OutputPrice:        0.050,
						CachedPrice:        0.001,
						CacheCreationPrice: 0.0125,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(1050000),
			model.WithModelConfigMaxInputTokens(922000),
			model.WithModelConfigMaxOutputTokens(128000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.6",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:         0.010,
			OutputPrice:        0.045,
			CachedPrice:        0.001,
			CacheCreationPrice: 0.0125,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.010,
						OutputPrice:        0.060,
						CachedPrice:        0.001,
						CacheCreationPrice: 0.0125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.020,
						OutputPrice:        0.090,
						CachedPrice:        0.002,
						CacheCreationPrice: 0.025,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.0025,
						OutputPrice:        0.015,
						CachedPrice:        0.00025,
						CacheCreationPrice: 0.003125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.005,
						OutputPrice:        0.0225,
						CachedPrice:        0.0005,
						CacheCreationPrice: 0.00625,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:         0.005,
						OutputPrice:        0.030,
						CachedPrice:        0.0005,
						CacheCreationPrice: 0.00625,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(1050000),
			model.WithModelConfigMaxOutputTokens(128000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.6-sol",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:         0.010,
			OutputPrice:        0.045,
			CachedPrice:        0.001,
			CacheCreationPrice: 0.0125,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.010,
						OutputPrice:        0.060,
						CachedPrice:        0.001,
						CacheCreationPrice: 0.0125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.020,
						OutputPrice:        0.090,
						CachedPrice:        0.002,
						CacheCreationPrice: 0.025,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.0025,
						OutputPrice:        0.015,
						CachedPrice:        0.00025,
						CacheCreationPrice: 0.003125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.005,
						OutputPrice:        0.0225,
						CachedPrice:        0.0005,
						CacheCreationPrice: 0.00625,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:         0.005,
						OutputPrice:        0.030,
						CachedPrice:        0.0005,
						CacheCreationPrice: 0.00625,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(1050000),
			model.WithModelConfigMaxOutputTokens(128000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.6-terra",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:         0.005,
			OutputPrice:        0.0225,
			CachedPrice:        0.0005,
			CacheCreationPrice: 0.00625,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.005,
						OutputPrice:        0.030,
						CachedPrice:        0.0005,
						CacheCreationPrice: 0.00625,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.010,
						OutputPrice:        0.045,
						CachedPrice:        0.001,
						CacheCreationPrice: 0.0125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.00125,
						OutputPrice:        0.0075,
						CachedPrice:        0.000125,
						CacheCreationPrice: 0.0015625,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.0025,
						OutputPrice:        0.01125,
						CachedPrice:        0.00025,
						CacheCreationPrice: 0.003125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:         0.0025,
						OutputPrice:        0.015,
						CachedPrice:        0.00025,
						CacheCreationPrice: 0.003125,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(1050000),
			model.WithModelConfigMaxOutputTokens(128000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.6-luna",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:         0.002,
			OutputPrice:        0.009,
			CachedPrice:        0.0002,
			CacheCreationPrice: 0.0025,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.002,
						OutputPrice:        0.012,
						CachedPrice:        0.0002,
						CacheCreationPrice: 0.0025,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:         0.004,
						OutputPrice:        0.018,
						CachedPrice:        0.0004,
						CacheCreationPrice: 0.005,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.0005,
						OutputPrice:        0.003,
						CachedPrice:        0.00005,
						CacheCreationPrice: 0.000625,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001, ServiceTier: "flex"},
					Price: model.Price{
						InputPrice:         0.001,
						OutputPrice:        0.0045,
						CachedPrice:        0.0001,
						CacheCreationPrice: 0.00125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:         0.001,
						OutputPrice:        0.006,
						CachedPrice:        0.0001,
						CacheCreationPrice: 0.00125,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(1050000),
			model.WithModelConfigMaxOutputTokens(128000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "o3",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.002,
			OutputPrice: 0.008,
			CachedPrice: 0.0005,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0035,
						OutputPrice: 0.014,
						CachedPrice: 0.000875,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "o3-mini",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0011,
			OutputPrice: 0.0044,
			CachedPrice: 0.00055,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "o3-pro",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.020,
			OutputPrice: 0.080,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "o4-mini",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0011,
			OutputPrice: 0.0044,
			CachedPrice: 0.000275,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.002,
						OutputPrice: 0.008,
						CachedPrice: 0.0005,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "o1",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.015,
			OutputPrice: 0.060,
			CachedPrice: 0.0075,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
		),
	},
	{
		Model: "o1-pro",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.150,
			OutputPrice: 0.600,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.5",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.010,
			OutputPrice: 0.045,
			CachedPrice: 0.001,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0125,
						OutputPrice: 0.075,
						CachedPrice: 0.00125,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:  0.005,
						OutputPrice: 0.030,
						CachedPrice: 0.0005,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001},
					Price: model.Price{
						InputPrice:  0.010,
						OutputPrice: 0.045,
						CachedPrice: 0.001,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.5-pro",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.060,
			OutputPrice: 0.270,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:  0.030,
						OutputPrice: 0.180,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001},
					Price: model.Price{
						InputPrice:  0.060,
						OutputPrice: 0.270,
					},
				},
			},
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.4",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.005,
			OutputPrice: 0.0225,
			CachedPrice: 0.0005,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000, ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.005,
						OutputPrice: 0.030,
						CachedPrice: 0.0005,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:  0.0025,
						OutputPrice: 0.015,
						CachedPrice: 0.00025,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001},
					Price: model.Price{
						InputPrice:  0.005,
						OutputPrice: 0.0225,
						CachedPrice: 0.0005,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.4-mini",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00075,
			OutputPrice: 0.0045,
			CachedPrice: 0.000075,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0015,
						OutputPrice: 0.009,
						CachedPrice: 0.00015,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.4-nano",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0002,
			OutputPrice: 0.00125,
			CachedPrice: 0.00002,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.4-pro",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.060,
			OutputPrice: 0.270,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{InputTokenMax: 272000},
					Price: model.Price{
						InputPrice:  0.030,
						OutputPrice: 0.180,
					},
				},
				{
					Condition: model.PriceCondition{InputTokenMin: 272001},
					Price: model.Price{
						InputPrice:  0.060,
						OutputPrice: 0.270,
					},
				},
			},
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.2",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00175,
			OutputPrice: 0.014,
			CachedPrice: 0.000175,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0035,
						OutputPrice: 0.028,
						CachedPrice: 0.00035,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.2-pro",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.021,
			OutputPrice: 0.168,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.1",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00125,
			OutputPrice: 0.010,
			CachedPrice: 0.000125,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0025,
						OutputPrice: 0.020,
						CachedPrice: 0.00025,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00125,
			OutputPrice: 0.010,
			CachedPrice: 0.000125,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0025,
						OutputPrice: 0.020,
						CachedPrice: 0.00025,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5-mini",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00025,
			OutputPrice: 0.002,
			CachedPrice: 0.000025,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.00045,
						OutputPrice: 0.0036,
						CachedPrice: 0.000045,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5-nano",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00005,
			OutputPrice: 0.0004,
			CachedPrice: 0.000005,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5-pro",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.015,
			OutputPrice: 0.120,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4.1",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.002,
			OutputPrice: 0.008,
			CachedPrice: 0.0005,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0035,
						OutputPrice: 0.014,
						CachedPrice: 0.000875,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(1000000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4.1-mini",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0004,
			OutputPrice: 0.0016,
			CachedPrice: 0.0001,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0007,
						OutputPrice: 0.0028,
						CachedPrice: 0.000175,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(1000000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4.1-nano",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0001,
			OutputPrice: 0.0004,
			CachedPrice: 0.000025,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0002,
						OutputPrice: 0.0008,
						CachedPrice: 0.00005,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(1000000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5-search-api",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00125,
			OutputPrice: 0.010,
			CachedPrice: 0.000125,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4o-search-preview",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0025,
			OutputPrice: 0.010,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(128000),
		),
	},
	{
		Model: "gpt-4o-mini-search-preview",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00015,
			OutputPrice: 0.0006,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(128000),
		),
	},
	{
		Model: "o3-deep-research",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.010,
			OutputPrice: 0.040,
			CachedPrice: 0.0025,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "o4-mini-deep-research",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.002,
			OutputPrice: 0.008,
			CachedPrice: 0.0005,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "computer-use-preview",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.012,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(8192),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.3-codex",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00175,
			OutputPrice: 0.014,
			CachedPrice: 0.000175,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.0035,
						OutputPrice: 0.028,
						CachedPrice: 0.00035,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.2-codex",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00175,
			OutputPrice: 0.014,
			CachedPrice: 0.000175,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.1-codex-max",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00125,
			OutputPrice: 0.010,
			CachedPrice: 0.000125,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.1-codex",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00125,
			OutputPrice: 0.010,
			CachedPrice: 0.000125,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.1-codex-mini",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00025,
			OutputPrice: 0.002,
			CachedPrice: 0.000025,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5-codex",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00125,
			OutputPrice: 0.010,
			CachedPrice: 0.000125,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "codex-mini-latest",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0015,
			OutputPrice: 0.006,
			CachedPrice: 0.000375,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.3-chat-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00175,
			OutputPrice: 0.014,
			CachedPrice: 0.000175,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.2-chat-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00175,
			OutputPrice: 0.014,
			CachedPrice: 0.000175,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5.1-chat-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00125,
			OutputPrice: 0.010,
			CachedPrice: 0.000125,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5-chat-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00125,
			OutputPrice: 0.010,
			CachedPrice: 0.000125,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "chat-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.005,
			OutputPrice: 0.030,
			CachedPrice: 0.0005,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(400000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-3.5-turbo",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0005,
			OutputPrice: 0.0015,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(4096),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-3.5-turbo-16k",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.003,
			OutputPrice: 0.004,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(16384),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-3.5-turbo-instruct",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0015,
			OutputPrice: 0.002,
		},
	},
	{
		Model: "gpt-4",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.030,
			OutputPrice: 0.060,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(8192),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4-32k",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.060,
			OutputPrice: 0.120,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(32768),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4-turbo",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.010,
			OutputPrice: 0.030,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(131072),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4o",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0025,
			OutputPrice: 0.010,
			CachedPrice: 0.00125,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.00425,
						OutputPrice: 0.017,
						CachedPrice: 0.002125,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(131072),
			model.WithModelConfigVision(true),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "chatgpt-4o-latest",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.005,
			OutputPrice: 0.015,
		},
	},
	{
		Model: "gpt-4o-mini",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.00015,
			OutputPrice: 0.0006,
			CachedPrice: 0.000075,
			ConditionalPrices: []model.ConditionalPrice{
				{
					Condition: model.PriceCondition{ServiceTier: "priority"},
					Price: model.Price{
						InputPrice:  0.00025,
						OutputPrice: 0.001,
						CachedPrice: 0.000125,
					},
				},
			},
		},
		SummaryServiceTier: true,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(131072),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4-vision-preview",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "o1-mini",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:  0.0011,
			OutputPrice: 0.0044,
			CachedPrice: 0.00055,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(131072),
		),
	},
	{
		Model: "o1-preview",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(131072),
		),
	},

	{
		Model: "text-embedding-ada-002",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice: 0.0001,
		},
	},
	{
		Model: "text-embedding-3-small",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice: 0.00002,
		},
	},
	{
		Model: "text-embedding-3-large",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice: 0.00013,
		},
	},
	{
		Model: "text-curie-001",
		Type:  mode.Completions,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "text-babbage-001",
		Type:  mode.Completions,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "text-ada-001",
		Type:  mode.Completions,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "text-davinci-002",
		Type:  mode.Completions,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "text-davinci-003",
		Type:  mode.Completions,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "text-moderation-latest",
		Type:  mode.Moderations,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "text-moderation-stable",
		Type:  mode.Moderations,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "text-davinci-edit-001",
		Type:  mode.ImagesEdits,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "davinci-002",
		Type:  mode.Completions,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "babbage-002",
		Type:  mode.Completions,
		Owner: model.ModelOwnerOpenAI,
	},

	{
		Model: "dall-e-2",
		Type:  mode.ImagesGenerations,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "dall-e-3",
		Type:  mode.ImagesGenerations,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "gpt-image-2",
		Type:  mode.ImagesGenerations,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:      0.005,
			ImageInputPrice: 0.008,
			OutputPrice:     0.030,
			CachedPrice:     0.00125,
		},
		MaxImageGenerationCount: 5,
		TimeoutConfig: model.TimeoutConfig{
			RequestTimeout: 1800,
		},
	},
	{
		Model: "gpt-image-1.5",
		Type:  mode.ImagesGenerations,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:       0.005,
			ImageInputPrice:  0.008,
			OutputPrice:      0.010,
			ImageOutputPrice: 0.032,
			CachedPrice:      0.00125,
		},
	},
	{
		Model: "gpt-image-1-mini",
		Type:  mode.ImagesGenerations,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:      0.002,
			ImageInputPrice: 0.0025,
			OutputPrice:     0.008,
			CachedPrice:     0.0002,
		},
	},
	{
		Model: "gpt-image-1",
		Type:  mode.ImagesGenerations,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:      0.005,
			ImageInputPrice: 0.01,
			OutputPrice:     0.04,
			CachedPrice:     0.00125,
		},
	},
	{
		Model: "chatgpt-image-latest",
		Type:  mode.ImagesGenerations,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:       0.005,
			ImageInputPrice:  0.008,
			OutputPrice:      0.010,
			ImageOutputPrice: 0.032,
			CachedPrice:      0.00125,
		},
	},

	{
		Model: "whisper-1",
		Type:  mode.AudioTranscription,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "tts-1",
		Type:  mode.AudioSpeech,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "tts-1-1106",
		Type:  mode.AudioSpeech,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "tts-1-hd",
		Type:  mode.AudioSpeech,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "tts-1-hd-1106",
		Type:  mode.AudioSpeech,
		Owner: model.ModelOwnerOpenAI,
	},
}

// no dot
var responsesOnlyModels = map[string]struct{}{
	"codex-mini-latest":     {},
	"computer-use-preview":  {},
	"gpt-5":                 {},
	"gpt-5-mini":            {},
	"gpt-5-nano":            {},
	"gpt-5-pro":             {},
	"gpt-5-codex":           {},
	"gpt-5-search-api":      {},
	"o3":                    {},
	"o3-deep-research":      {},
	"o3-mini":               {},
	"o3-pro":                {},
	"o4-mini":               {},
	"o4-mini-deep-research": {},
	"gpt-51":                {},
	"gpt-51-codex":          {},
	"gpt-51-codex-max":      {},
	"gpt-51-codex-mini":     {},
	"gpt-52":                {},
	"gpt-52-pro":            {},
	"gpt-52-codex":          {},
	"gpt-53-codex":          {},
	"gpt-54":                {},
	"gpt-54-mini":           {},
	"gpt-54-nano":           {},
	"gpt-54-pro":            {},
	"gpt-55":                {},
	"gpt-55-pro":            {},
	"gpt-6-luna":            {},
	"gpt-6-sol":             {},
	"gpt-6-astra":           {},
	"gpt-61-sol":            {},
}

// IsResponsesOnlyModel checks if a model only supports the Responses API
// First parameter is the model config, used to check Type field if model name check fails
// Second parameter is the model name, checked first for quick lookup
func IsResponsesOnlyModel(modelConfig *model.ModelConfig, modelName string) bool {
	// First, check model name for quick lookup
	if _, ok := responsesOnlyModels[modelName]; ok {
		return true
	}

	noDotModelName := strings.ReplaceAll(modelName, ".", "")
	if _, ok := responsesOnlyModels[noDotModelName]; ok {
		return true
	}

	// If model config is provided, check if Type is any Responses-related mode
	if modelConfig != nil {
		switch modelConfig.Type {
		case mode.Responses,
			mode.ResponsesGet,
			mode.ResponsesDelete,
			mode.ResponsesCancel,
			mode.ResponsesInputItems:
			return true
		}
	}

	return false
}

func IsResponsesOnlyModelAny(
	modelConfig *model.ModelConfig,
	originModel string,
	actualModel string,
) bool {
	if IsResponsesOnlyModel(modelConfig, originModel) {
		return true
	}

	if actualModel != "" && actualModel != originModel {
		return IsResponsesOnlyModel(modelConfig, actualModel)
	}

	return false
}
