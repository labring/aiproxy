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
	case mode.ImagesGenerations, mode.GeminiImage:
		return buildModelConfigRequest(
			modelConfig,
			mode.ImagesGenerations,
			BuildImagesGenerationsRequest,
		)
	case mode.ImagesEdits:
		return nil, mode.Unknown, NewErrUnsupportedModelType("edits")
	case mode.AudioSpeech, mode.GeminiTTS:
		return buildModelRequest(modelConfig.Model, mode.AudioSpeech, BuildAudioSpeechRequest)
	case mode.AudioTranscription:
		return nil, mode.Unknown, NewErrUnsupportedModelType("audio transcription")
	case mode.AudioTranslation:
		return nil, mode.Unknown, NewErrUnsupportedModelType("audio translation")
	case mode.Rerank:
		return buildModelRequest(modelConfig.Model, mode.Rerank, BuildRerankRequest)
	case mode.Anthropic:
		return buildModelRequest(modelConfig.Model, mode.Anthropic, BuildAnthropicRequest)
	case mode.VideoGenerationsJobs:
		return buildModelRequest(
			modelConfig.Model,
			mode.VideoGenerationsJobs,
			BuildVideoGenerationJobRequest,
		)
	case mode.Videos:
		return buildModelRequest(modelConfig.Model, mode.Videos, BuildVideosRequest)
	case mode.ParsePdf:
		return nil, mode.Unknown, NewErrUnsupportedModelType("parse pdf")
	case mode.GeminiVideo:
		return buildModelRequest(modelConfig.Model, mode.GeminiVideo, BuildGeminiVideoRequest)
	case mode.AliVideo:
		return buildModelRequest(modelConfig.Model, mode.AliVideo, BuildAliVideoRequest)
	case mode.DoubaoVideo:
		return buildModelRequest(modelConfig.Model, mode.DoubaoVideo, BuildDoubaoVideoRequest)
	default:
		return nil, mode.Unknown, NewErrUnsupportedModelType(modelConfig.Type.String())
	}
}

func buildModelRequest(
	modelName string,
	relayMode mode.Mode,
	build func(string) (io.Reader, error),
) (io.Reader, mode.Mode, error) {
	body, err := build(modelName)
	if err != nil {
		return nil, mode.Unknown, err
	}

	return body, relayMode, nil
}

func buildModelConfigRequest(
	modelConfig model.ModelConfig,
	relayMode mode.Mode,
	build func(model.ModelConfig) (io.Reader, error),
) (io.Reader, mode.Mode, error) {
	body, err := build(modelConfig)
	if err != nil {
		return nil, mode.Unknown, err
	}

	return body, relayMode, nil
}

func marshalRequestReader(request any) (io.Reader, error) {
	jsonBytes, err := sonic.Marshal(request)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(jsonBytes), nil
}

func BuildChatCompletionRequest(model string) (io.Reader, error) {
	testRequest := &relaymodel.GeneralOpenAIRequest{
		Model: model,
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	}

	return marshalRequestReader(testRequest)
}

func BuildCompletionsRequest(model string) (io.Reader, error) {
	completionsRequest := &relaymodel.GeneralOpenAIRequest{
		Model:  model,
		Prompt: "hi",
	}

	return marshalRequestReader(completionsRequest)
}

func BuildEmbeddingsRequest(model string) (io.Reader, error) {
	embeddingsRequest := &relaymodel.GeneralOpenAIRequest{
		Model: model,
		Input: "hi",
	}

	return marshalRequestReader(embeddingsRequest)
}

func BuildModerationsRequest(model string) (io.Reader, error) {
	moderationsRequest := &relaymodel.GeneralOpenAIRequest{
		Model: model,
		Input: "hi",
	}

	return marshalRequestReader(moderationsRequest)
}

func BuildImagesGenerationsRequest(modelConfig model.ModelConfig) (io.Reader, error) {
	imagesGenerationsRequest := &relaymodel.GeneralOpenAIRequest{
		Model:  modelConfig.Model,
		Prompt: "A simple red square icon on a white background.",
		Size:   "1024x1024",
	}

	return marshalRequestReader(imagesGenerationsRequest)
}

func BuildAudioSpeechRequest(model string) (io.Reader, error) {
	audioSpeechRequest := &relaymodel.GeneralOpenAIRequest{
		Model: model,
		Input: "hi",
	}

	return marshalRequestReader(audioSpeechRequest)
}

func BuildRerankRequest(model string) (io.Reader, error) {
	rerankRequest := &relaymodel.RerankRequest{
		Model:     model,
		Query:     "hi",
		Documents: []string{"hi"},
	}

	return marshalRequestReader(rerankRequest)
}

func BuildAnthropicRequest(model string) (io.Reader, error) {
	anthropicRequest := map[string]any{
		"model":      model,
		"max_tokens": 16,
		"messages": []relaymodel.Message{
			{
				Role:    relaymodel.RoleUser,
				Content: "hi",
			},
		},
	}

	return marshalRequestReader(anthropicRequest)
}

func BuildVideoGenerationJobRequest(model string) (io.Reader, error) {
	testRequest := map[string]any{
		"model":  model,
		"prompt": "A calm cinematic shot of clouds moving over a mountain.",
	}

	return marshalRequestReader(testRequest)
}

func BuildVideosRequest(model string) (io.Reader, error) {
	testRequest := &relaymodel.VideosRequest{
		Model:  model,
		Prompt: "A calm cinematic shot of clouds moving over a mountain.",
	}

	return marshalRequestReader(testRequest)
}

func BuildGeminiVideoRequest(_ string) (io.Reader, error) {
	testRequest := map[string]any{
		"instances": []map[string]any{
			{
				"prompt": "A calm cinematic shot of clouds moving over a mountain.",
			},
		},
	}

	return marshalRequestReader(testRequest)
}

func BuildAliVideoRequest(model string) (io.Reader, error) {
	testRequest := map[string]any{
		"model": model,
		"input": map[string]any{
			"prompt": "A calm cinematic shot of clouds moving over a mountain.",
		},
		"parameters": map[string]any{
			"duration": 5,
			"size":     "720P",
		},
	}

	return marshalRequestReader(testRequest)
}

func BuildDoubaoVideoRequest(model string) (io.Reader, error) {
	testRequest := map[string]any{
		"model": model,
		"content": []map[string]any{
			{
				"type": "text",
				"text": "A calm cinematic shot of clouds moving over a mountain.",
			},
		},
		"duration":   5,
		"resolution": "720p",
		"ratio":      "16:9",
	}

	return marshalRequestReader(testRequest)
}
