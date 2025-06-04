package model

import "github.com/labring/aiproxy/core/model"

// https://platform.openai.com/docs/api-reference/images/create

type ImageRequest struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	Background        string `json:"background,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	OutputCompression int    `json:"output_compression,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`   // png, jpeg, webp
	Size              string `json:"size,omitempty"`            // 1024x1024, 1536x1024, 1024x1536, auto, 256x256, 512x512, 1792x1024, 1024x1792
	Quality           string `json:"quality,omitempty"`         // auto, high, medium, low, hd, standard
	ResponseFormat    string `json:"response_format,omitempty"` // url, b64_json
	Style             string `json:"style,omitempty"`           // vivid, natural
	User              string `json:"user,omitempty"`
	N                 int    `json:"n,omitempty"`
}

type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64Json       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type ImageResponse struct {
	Created int64        `json:"created"`
	Data    []*ImageData `json:"data"`
	// For gpt-image-1 only, the token usage information for the image generation.
	Usage *ImageUsage `json:"usage"`
}

type ImageUsage struct {
	// The number of tokens (images and text) in the input prompt.
	InputTokens int64 `json:"input_tokens"`
	// The number of image tokens in the output image.
	OutputTokens int64 `json:"output_tokens"`
	// The total number of tokens (images and text) used for the image generation.
	TotalTokens int64 `json:"total_tokens"`
	// The input tokens detailed information for the image generation.
	InputTokensDetails ImageInputTokensDetails `json:"input_tokens_details"`
}

func (i *ImageUsage) ToModelUsage() model.Usage {
	return model.Usage{
		InputTokens:      model.ZeroNullInt64(i.InputTokens),
		ImageInputTokens: model.ZeroNullInt64(i.InputTokensDetails.ImageTokens),
		OutputTokens:     model.ZeroNullInt64(i.OutputTokens),
		TotalTokens:      model.ZeroNullInt64(i.TotalTokens),
	}
}

type ImageInputTokensDetails struct {
	// The number of text tokens in the input prompt.
	TextTokens int64 `json:"text_tokens"`
	// The number of image tokens in the input prompt.
	ImageTokens int64 `json:"image_tokens"`
}
