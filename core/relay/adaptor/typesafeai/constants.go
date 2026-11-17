package typesafeai

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

const baseURL = "https://api.typesafe.ai/v1"

// All SystemOne models share the same limits: 64k tokens per request (32k per
// question) and $42 per 1B input tokens with free output tokens. Prices here
// are USD per 1K tokens, the relay default unit.
func systemOneModel(name string) model.ModelConfig {
	return model.ModelConfig{
		Model: name,
		Type:  mode.SystemOne,
		Owner: model.ModelOwnerTypeSafeAI,
		Price: model.Price{InputPrice: model.ZeroNullFloat64(0.000042)},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(64000),
			model.WithModelConfigMaxInputTokens(64000),
		),
	}
}

var ModelList = []model.ModelConfig{
	systemOneModel("jev-latest"),
	systemOneModel("jev-1.14.0"),
	systemOneModel("jev-1.13.0"),
}
