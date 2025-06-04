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

func BuildRequest(modelConfig model.ModelConfig) (io.Reader, mode.Mode, error) {
	switch modelConfig.Type {
	case mode.ChatCompletions:
		body, err := BuildChatCompletionRequest(modelConfig.Model)
		if err != nil {
			return nil, mode.Unknown, err
		}
		return body, mode.ChatCompletions, nil
	case mode.Completions:
		body, err := BuildCompletionsRequest(modelConfig.Model)
		if err != nil {
			return nil, mode.Unknown, err
		}
		return body, mode.Completions, nil
	case mode.Embeddings:
		body, err := BuildEmbeddingsRequest(modelConfig.Model)
		if err != nil {
			return nil, mode.Unknown, err
		}
		return body, mode.Embeddings, nil
	case mode.Moderations:
		body, err := BuildModerationsRequest(modelConfig.Model)
		if err != nil {
			return nil, mode.Unknown, err
		}
		return body, mode.Moderations, nil
	case mode.ImagesGenerations:
		body, err := BuildImagesGenerationsRequest(modelConfig)
		if err != nil {
			return nil, mode.Unknown, err
		}
		return body, mode.ImagesGenerations, nil
	case mode.ImagesEdits:
		return nil, mode.Unknown, NewErrUnsupportedModelType("edits")
	case mode.AudioSpeech:
		body, err := BuildAudioSpeechRequest(modelConfig.Model)
		if err != nil {
			return nil, mode.Unknown, err
		}
		return body, mode.AudioSpeech, nil
	case mode.AudioTranscription:
		return nil, mode.Unknown, NewErrUnsupportedModelType("audio transcription")
	case mode.AudioTranslation:
		return nil, mode.Unknown, NewErrUnsupportedModelType("audio translation")
	case mode.Rerank:
		body, err := BuildRerankRequest(modelConfig.Model)
		if err != nil {
			return nil, mode.Unknown, err
		}
		return body, mode.Rerank, nil
	case mode.ParsePdf:
		return nil, mode.Unknown, NewErrUnsupportedModelType("parse pdf")
	default:
		return nil, mode.Unknown, NewErrUnsupportedModelType(modelConfig.Type.String())
	}
}

func BuildChatCompletionRequest(model string) (io.Reader, error) {
	testRequest := &relaymodel.GeneralOpenAIRequest{
		Model: model,
		Messages: []*relaymodel.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	}
	jsonBytes, err := sonic.Marshal(testRequest)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonBytes), nil
}

func BuildCompletionsRequest(model string) (io.Reader, error) {
	completionsRequest := &relaymodel.GeneralOpenAIRequest{
		Model:  model,
		Prompt: "hi",
	}
	jsonBytes, err := sonic.Marshal(completionsRequest)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonBytes), nil
}

func BuildEmbeddingsRequest(model string) (io.Reader, error) {
	embeddingsRequest := &relaymodel.GeneralOpenAIRequest{
		Model: model,
		Input: "hi",
	}
	jsonBytes, err := sonic.Marshal(embeddingsRequest)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonBytes), nil
}

func BuildModerationsRequest(model string) (io.Reader, error) {
	moderationsRequest := &relaymodel.GeneralOpenAIRequest{
		Model: model,
		Input: "hi",
	}
	jsonBytes, err := sonic.Marshal(moderationsRequest)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonBytes), nil
}

func BuildImagesGenerationsRequest(modelConfig model.ModelConfig) (io.Reader, error) {
	imagesGenerationsRequest := &relaymodel.GeneralOpenAIRequest{
		Model:  modelConfig.Model,
		Prompt: "hi",
		Size:   "1024x1024",
	}
	for size := range modelConfig.ImagePrices {
		imagesGenerationsRequest.Size = size
		break
	}
	jsonBytes, err := sonic.Marshal(imagesGenerationsRequest)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonBytes), nil
}

func BuildAudioSpeechRequest(model string) (io.Reader, error) {
	audioSpeechRequest := &relaymodel.GeneralOpenAIRequest{
		Model: model,
		Input: "hi",
	}
	jsonBytes, err := sonic.Marshal(audioSpeechRequest)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonBytes), nil
}

func BuildRerankRequest(model string) (io.Reader, error) {
	rerankRequest := &relaymodel.RerankRequest{
		Model:     model,
		Query:     "hi",
		Documents: []string{"hi"},
	}
	jsonBytes, err := sonic.Marshal(rerankRequest)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonBytes), nil
}
