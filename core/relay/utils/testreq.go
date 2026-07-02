package utils

import (
	"bytes"
	"fmt"
	"io"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type UnsupportedModelTypeError struct {
	ModelType string
}

func (e *UnsupportedModelTypeError) Error() string {
	return fmt.Sprintf("model type '%s' not supported", e.ModelType)
}

func NewErrUnsupportedModelType(modelType string) *UnsupportedModelTypeError {
	return &UnsupportedModelTypeError{ModelType: modelType}
}

//nolint:gocyclo // Each supported relay mode has one explicit test-request builder branch.
func BuildRequest(modelConfig model.ModelConfig) (io.Reader, mode.Mode, error) {
	switch modelConfig.Type {
	case mode.ChatCompletions:
		return buildModelRequest(
			modelConfig.Model,
			mode.ChatCompletions,
			BuildChatCompletionRequest,
		)
	case mode.Completions:
		return buildModelRequest(modelConfig.Model, mode.Completions, BuildCompletionsRequest)
	case mode.Embeddings:
		return buildModelRequest(modelConfig.Model, mode.Embeddings, BuildEmbeddingsRequest)
	case mode.Moderations:
		return buildModelRequest(modelConfig.Model, mode.Moderations, BuildModerationsRequest)
