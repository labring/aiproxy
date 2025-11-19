package gemini

import (
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type EmbeddingRequest struct {
	Model                string                       `json:"model"`
	TaskType             string                       `json:"taskType,omitempty"`
	Title                string                       `json:"title,omitempty"`
	Content              relaymodel.GeminiChatContent `json:"content"`
	OutputDimensionality int                          `json:"outputDimensionality,omitempty"`
}

type BatchEmbeddingRequest struct {
	Requests []EmbeddingRequest `json:"requests"`
}

type EmbeddingData struct {
	Values []float64 `json:"values"`
}

type EmbeddingResponse struct {
	Embeddings []EmbeddingData `json:"embeddings"`
}
