package openai

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

var ModelList = []model.ModelConfig{
	{
		Model: "gpt-3.5-turbo",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(4096),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-3.5-turbo-16k",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(16384),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-3.5-turbo-instruct",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "gpt-4",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(8192),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4-32k",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(32768),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4-turbo",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(131072),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-4o",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
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
	},
	{
		Model: "gpt-4o-mini",
		Type:  mode.ChatCompletions,
		Owner: model.ModelOwnerOpenAI,
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
	},
	{
		Model: "text-embedding-3-small",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerOpenAI,
	},
	{
		Model: "text-embedding-3-large",
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerOpenAI,
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
		Model: "gpt-image-1",
		Type:  mode.ImagesGenerations,
		Owner: model.ModelOwnerOpenAI,
		Price: model.Price{
			InputPrice:      0.005,
			ImageInputPrice: 0.01,
			OutputPrice:     0.04,
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
	{
		Model: "gpt-5-codex",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
	{
		Model: "gpt-5-pro",
		Type:  mode.Responses,
		Owner: model.ModelOwnerOpenAI,
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(200000),
			model.WithModelConfigToolChoice(true),
		),
	},
}

var responsesOnlyModels = map[string]struct{}{
	"gpt-5-codex": {},
	"gpt-5-pro":   {},
}

// IsResponsesOnlyModel checks if a model only supports the Responses API
// First parameter is the model config, used to check Type field if model name check fails
// Second parameter is the model name, checked first for quick lookup
func IsResponsesOnlyModel(modelConfig *model.ModelConfig, modelName string) bool {
	// First, check model name for quick lookup
	if _, ok := responsesOnlyModels[modelName]; ok {
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
