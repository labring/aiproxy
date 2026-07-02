package utils_test

import (
	"io"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestGeminiVideo(t *testing.T) {
	body, relayMode, err := utils.BuildRequest(model.ModelConfig{
		Model: "veo-3.1-generate-preview",
		Type:  mode.GeminiVideo,
	})
	require.NoError(t, err)
	require.Equal(t, mode.GeminiVideo, relayMode)

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"instances":[{"prompt":"A calm cinematic shot of clouds moving over a mountain."}]
		}`,
		string(data),
	)
}

func TestBuildRequestAliVideo(t *testing.T) {
	body, relayMode, err := utils.BuildRequest(model.ModelConfig{
		Model: "wan2.5-t2v-preview",
		Type:  mode.AliVideo,
	})
	require.NoError(t, err)
	require.Equal(t, mode.AliVideo, relayMode)

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"model":"wan2.5-t2v-preview",
			"input":{"prompt":"A calm cinematic shot of clouds moving over a mountain."},
			"parameters":{"duration":5,"size":"720P"}
		}`,
		string(data),
	)
}

func TestBuildRequestDoubaoVideo(t *testing.T) {
	body, relayMode, err := utils.BuildRequest(model.ModelConfig{
		Model: "doubao-seedance-2-0-260128",
		Type:  mode.DoubaoVideo,
	})
	require.NoError(t, err)
	require.Equal(t, mode.DoubaoVideo, relayMode)

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"model":"doubao-seedance-2-0-260128",
			"content":[
				{
					"type":"text",
					"text":"A calm cinematic shot of clouds moving over a mountain."
				}
			],
			"duration":5,
			"resolution":"720p",
			"ratio":"16:9"
		}`,
		string(data),
	)
}

func TestBuildRequestAnthropic(t *testing.T) {
	body, relayMode, err := utils.BuildRequest(model.ModelConfig{
		Model: "claude-sonnet-4-5",
		Type:  mode.Anthropic,
	})
	require.NoError(t, err)
	require.Equal(t, mode.Anthropic, relayMode)

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{
			"model":"claude-sonnet-4-5",
			"messages":[
				{
					"role":"user",
					"content":"hi"
				}
			],
			"max_tokens":16
		}`,
		string(data),
	)
}

func TestBuildRequestVideoGenerationJob(t *testing.T) {
	body, relayMode, err := utils.BuildRequest(model.ModelConfig{
		Model: "happyhorse-1.0-t2v",
		Type:  mode.VideoGenerationsJobs,
	})
	require.NoError(t, err)
	require.Equal(t, mode.VideoGenerationsJobs, relayMode)

	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var request relaymodel.VideoGenerationJobRequest
	require.NoError(t, sonic.Unmarshal(data, &request))
	require.Equal(t, "happyhorse-1.0-t2v", request.Model)
	require.Contains(t, request.Prompt, "clouds")
	require.Zero(t, request.NVariants)
	require.Zero(t, request.NSeconds)
	require.NotContains(t, string(data), "size")
	require.NotContains(t, string(data), "n_variants")
	require.NotContains(t, string(data), "n_seconds")
}

func TestBuildRequestVideos(t *testing.T) {
	body, relayMode, err := utils.BuildRequest(model.ModelConfig{
		Model: "sora-2",
		Type:  mode.Videos,
	})
	require.NoError(t, err)
	require.Equal(t, mode.Videos, relayMode)

	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var request relaymodel.VideosRequest
	require.NoError(t, sonic.Unmarshal(data, &request))
	require.Equal(t, "sora-2", request.Model)
	require.Contains(t, request.Prompt, "clouds")
	require.Empty(t, request.Size)
	require.Empty(t, request.Seconds)
	require.NotContains(t, string(data), "size")
	require.NotContains(t, string(data), "seconds")
}

func TestBuildImagesGenerationsRequestUsesImagePrompt(t *testing.T) {
	body, relayMode, err := utils.BuildRequest(model.ModelConfig{
		Model: "gemini-3.1-flash-image-preview",
		Type:  mode.GeminiImage,
	})
	require.NoError(t, err)
	require.Equal(t, mode.ImagesGenerations, relayMode)

	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var request relaymodel.GeneralOpenAIRequest
	require.NoError(t, sonic.Unmarshal(data, &request))
	require.Equal(t, "gemini-3.1-flash-image-preview", request.Model)
	require.NotEqual(t, "hi", request.Prompt)
	require.Contains(t, request.Prompt, "red square")
	require.Equal(t, "1024x1024", request.Size)
}
