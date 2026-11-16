package voyage

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

// Prices are USD per 1K tokens (the relay default unit), converted from the
// published Voyage price per 1M tokens.

func textEmbeddingModel(name string, pricePerK float64, contextTokens int) model.ModelConfig {
	return model.ModelConfig{
		Model: name,
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerVoyage,
		Price: model.Price{InputPrice: model.ZeroNullFloat64(pricePerK)},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(contextTokens),
			model.WithModelConfigMaxInputTokens(contextTokens),
		),
	}
}

// contextualEmbeddingModel is a voyage-context-* model, served by
// /contextualizedembeddings with a 32k context.
func contextualEmbeddingModel(name string, pricePerK float64) model.ModelConfig {
	return textEmbeddingModel(name, pricePerK, 32000)
}

// multimodalEmbeddingModel is priced at $0.12/1M text tokens and $0.60/1B image
// pixels; at 560 pixels per image token the image rate is $0.336/1M image tokens.
func multimodalEmbeddingModel(name string) model.ModelConfig {
	return model.ModelConfig{
		Model: name,
		Type:  mode.Embeddings,
		Owner: model.ModelOwnerVoyage,
		Price: model.Price{
			InputPrice:      0.00012,
			ImageInputPrice: 0.000336,
		},
		Config: model.NewModelConfig(
			model.WithModelConfigMaxContextTokens(32000),
			model.WithModelConfigMaxInputTokens(32000),
			model.WithModelConfigVision(true),
		),
	}
}

func rerankModel(name string, pricePerK float64) model.ModelConfig {
	return model.ModelConfig{
		Model: name,
		Type:  mode.Rerank,
		Owner: model.ModelOwnerVoyage,
		Price: model.Price{InputPrice: model.ZeroNullFloat64(pricePerK)},
	}
}

var multimodalModels = map[string]struct{}{
	"voyage-multimodal-3":   {},
	"voyage-multimodal-3.5": {},
}

var contextualModels = map[string]struct{}{
	"voyage-context-4": {},
	"voyage-context-3": {},
}

// isContextualModel reports whether the model is served by /contextualizedembeddings.
func isContextualModel(name string) bool {
	_, ok := contextualModels[name]
	return ok
}

// isMultimodalModel reports whether the model is served by /multimodalembeddings.
func isMultimodalModel(name string) bool {
	_, ok := multimodalModels[name]
	return ok
}

var ModelList = []model.ModelConfig{
	// Text embeddings, 32k context.
	textEmbeddingModel("voyage-4-large", 0.00012, 32000),
	textEmbeddingModel("voyage-4", 0.00006, 32000),
	textEmbeddingModel("voyage-4-lite", 0.00002, 32000),
	textEmbeddingModel("voyage-code-4", 0.00012, 32000),
	textEmbeddingModel("voyage-3-large", 0.00018, 32000),
	textEmbeddingModel("voyage-3.5", 0.00006, 32000),
	textEmbeddingModel("voyage-3.5-lite", 0.00002, 32000),
	textEmbeddingModel("voyage-code-3", 0.00018, 32000),
	textEmbeddingModel("voyage-finance-2", 0.00012, 32000),
	textEmbeddingModel("voyage-law-2", 0.00012, 16000),

	// Contextualized embeddings ($0.12 and $0.18 per 1M tokens).
	contextualEmbeddingModel("voyage-context-4", 0.00012),
	contextualEmbeddingModel("voyage-context-3", 0.00018),

	// Multimodal embeddings.
	multimodalEmbeddingModel("voyage-multimodal-3"),
	multimodalEmbeddingModel("voyage-multimodal-3.5"),

	// Rerankers.
	rerankModel("rerank-3", 0.00005),
	rerankModel("rerank-3-lite", 0.00002),
	rerankModel("rerank-2.5", 0.00005),
	rerankModel("rerank-2.5-lite", 0.00002),
}
