package ppio

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

// https://ppinfra.com/docs/model/llm.md

var ModelList = []model.ModelConfig{
	// DeepSeek models
	{
		Model: "deepseek/deepseek-v3.2",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-v3.2-exp",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
	{
		Model: "deepseek/deepseek-v3.1-terminus",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerDeepSeek,
	},
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
		Model: "deepseek/deepseek-ocr-2",
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
		Model: "qwen/qwen3.5-397b-a17b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen3.5-plus",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen3.5-122b-a10b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen3.5-27b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen3.5-35b-a3b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen3-coder-next",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen3-vl-235b-a22b-thinking",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwen3-vl-235b-a22b-instruct",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},
	{
		Model: "qwen/qwq-32b",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerAlibaba,
	},

	// MiniMax models
	{
		Model: "minimax/minimax-m2.7",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMiniMax,
	},
	{
		Model: "minimax/minimax-m2.5-highspeed",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMiniMax,
	},
	{
		Model: "minimax/minimax-m2.5",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMiniMax,
	},
	{
		Model: "minimax/minimax-m2.1",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMiniMax,
	},
	{
		Model: "minimax/minimax-m2",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMiniMax,
	},

	// GLM models (Zhipu AI)
	{
		Model: "zai-org/glm-5",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerChatGLM,
	},
	{
		Model: "zai-org/glm-4.7",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerChatGLM,
	},
	{
		Model: "zai-org/glm-4.7-flash",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerChatGLM,
	},
	{
		Model: "zai-org/glm-4.6",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerChatGLM,
	},
	{
		Model: "zai-org/glm-4.6v",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerChatGLM,
	},

	// Moonshot/Kimi models
	{
		Model: "moonshotai/kimi-k2.5",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMoonshot,
	},
	{
		Model: "moonshotai/kimi-k2-thinking",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerMoonshot,
	},

	// Embedding models
	{
		Model: "baai/bge-m3",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerBAAI,
	},
	{
		Model: "qwen/qwen3-embedding-8b",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerAlibaba,
		Price: model.Price{
			InputPrice: 0.0005, // ¥0.5/M tokens (Qwen3-Embedding official)
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(8192),
		),
	},
	{
		Model: "pa/text-embedding-3-large",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice: 0.00091, // $0.13/M tokens × 7 = ¥0.91/M (OpenAI official)
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(8191),
		),
	},

	// Web Search (virtual model for PPIO's standalone web-search API)
	{
		Model: "ppio-web-search",
		Type:  mode.WebSearch,
		Owner: model.ModelOwnerPPIO,
	},
}
