package utils

import (
	"bytes"
	"fmt"
	"io"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/model"
	relaymodel "github.com/labring/aiproxy/relay/model"
	"github.com/labring/aiproxy/relay/relaymode"
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

func BuildRequest(modelConfig *model.ModelConfig) (io.Reader, relaymode.Mode, error) {
	switch modelConfig.Type {
	case relaymode.ChatCompletions:
		body, err := BuildChatCompletionRequest(modelConfig.Model)
		if err != nil {
			return nil, relaymode.Unknown, err
		}
		return body, relaymode.ChatCompletions, nil
	case relaymode.Completions:
		return nil, relaymode.Unknown, NewErrUnsupportedModelType("completions")
	case relaymode.Embeddings:
		body, err := BuildEmbeddingsRequest(modelConfig.Model)
		if err != nil {
			return nil, relaymode.Unknown, err
		}
		return body, relaymode.Embeddings, nil
	case relaymode.Moderations:
		body, err := BuildModerationsRequest(modelConfig.Model)
		if err != nil {
			return nil, relaymode.Unknown, err
		}
		return body, relaymode.Moderations, nil
	case relaymode.ImagesGenerations:
		body, err := BuildImagesGenerationsRequest(modelConfig)
		if err != nil {
			return nil, relaymode.Unknown, err
		}
		return body, relaymode.ImagesGenerations, nil
	case relaymode.Edits:
		return nil, relaymode.Unknown, NewErrUnsupportedModelType("edits")
	case relaymode.AudioSpeech:
		body, err := BuildAudioSpeechRequest(modelConfig.Model)
		if err != nil {
			return nil, relaymode.Unknown, err
		}
		return body, relaymode.AudioSpeech, nil
	case relaymode.AudioTranscription:
		return nil, relaymode.Unknown, NewErrUnsupportedModelType("audio transcription")
	case relaymode.AudioTranslation:
		return nil, relaymode.Unknown, NewErrUnsupportedModelType("audio translation")
	case relaymode.Rerank:
		body, err := BuildRerankRequest(modelConfig.Model)
		if err != nil {
			return nil, relaymode.Unknown, err
		}
		return body, relaymode.Rerank, nil
	case relaymode.ParsePdf:
		return nil, relaymode.Unknown, NewErrUnsupportedModelType("parse pdf")
	default:
		return nil, relaymode.Unknown, NewErrUnsupportedModelType(modelConfig.Type.String())
	}
}

func BuildChatCompletionRequest(model string) (io.Reader, error) {
	testRequest := &relaymodel.GeneralOpenAIRequest{
		MaxTokens: 2,
		Model:     model,
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

func BuildImagesGenerationsRequest(modelConfig *model.ModelConfig) (io.Reader, error) {
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
