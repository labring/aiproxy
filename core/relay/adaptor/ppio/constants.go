package ppio

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

// https://ppinfra.com/docs/get-started/quickstart.html

var ModelList = []model.ModelConfig{
	// DeepSeek models
	{
		Model: "deepseek/deepseek-r1",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-r1-turbo",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-v3",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-v3-turbo",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-r1-distill-qwen-32b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-r1-distill-qwen-14b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-r1-distill-llama-70b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-r1-distill-llama-8b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-prover-v2",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},

	// Qwen models
	{
		Model: "qwen/qwq-32b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen2.5-72b-instruct",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen2.5-coder-32b-instruct",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},

	// MiniMax models
	{
		Model: "minimax/minimax-m1",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMiniMax,
	},

	// Llama models
	{
		Model: "meta-llama/llama-3.3-70b-instruct",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMeta,
	},

	// Embedding models
	{
		Model: "baai/bge-m3",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerBAAI,
	},
}
