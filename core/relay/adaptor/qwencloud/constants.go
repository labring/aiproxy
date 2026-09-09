package qwencloud

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

// https://docs.qwencloud.com/developer-guides/getting-started/introduction
// https://docs.qwencloud.com/api-reference/text-embedding/openai-embedding
var ModelList = []model.ModelConfig{
	{
		Model: "qwen3.8-max",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen3.7-plus",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen3.8-flash",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "text-embedding-v4",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "text-embedding-v3",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerAlibaba,
	},
}
