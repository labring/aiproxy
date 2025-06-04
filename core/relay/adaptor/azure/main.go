package azure

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) DefaultBaseURL() string {
	return "https://{resource_name}.openai.azure.com"
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	return GetRequestURL(meta, true)
}

func GetRequestURL(meta *meta.Meta, replaceDot bool) (adaptor.RequestURL, error) {
	_, apiVersion, err := GetTokenAndAPIVersion(meta.Channel.Key)
	if err != nil {
		return adaptor.RequestURL{}, err
	}
	model := meta.ActualModel
	if replaceDot {
		model = strings.ReplaceAll(model, ".", "")
	}
	switch meta.Mode {
	case mode.ImagesGenerations:
		// https://learn.microsoft.com/en-us/azure/ai-services/openai/dall-e-quickstart?tabs=dalle3%2Ccommand-line&pivots=rest-api
		// https://{resource_name}.openai.azure.com/openai/deployments/dall-e-3/images/generations?api-version=2024-03-01-preview
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/deployments/%s/images/generations?api-version=%s",
				meta.Channel.BaseURL,
				model,
				apiVersion,
			),
		}, nil
	case mode.AudioTranscription:
		// https://learn.microsoft.com/en-us/azure/ai-services/openai/whisper-quickstart?tabs=command-line#rest-api
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/deployments/%s/audio/transcriptions?api-version=%s",
				meta.Channel.BaseURL,
				model,
				apiVersion,
			),
		}, nil
	case mode.AudioSpeech:
		// https://learn.microsoft.com/en-us/azure/ai-services/openai/text-to-speech-quickstart?tabs=command-line#rest-api
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/deployments/%s/audio/speech?api-version=%s",
				meta.Channel.BaseURL,
				model,
				apiVersion,
			),
		}, nil
	case mode.ChatCompletions:
		// https://learn.microsoft.com/en-us/azure/cognitive-services/openai/chatgpt-quickstart?pivots=rest-api&tabs=command-line#rest-api
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/deployments/%s/chat/completions?api-version=%s",
				meta.Channel.BaseURL,
				model,
				apiVersion,
			),
		}, nil
	case mode.Completions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/deployments/%s/completions?api-version=%s",
				meta.Channel.BaseURL,
				model,
				apiVersion,
			),
		}, nil
	case mode.Embeddings:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/deployments/%s/embeddings?api-version=%s",
				meta.Channel.BaseURL,
				model,
				apiVersion,
			),
		}, nil
	case mode.VideoGenerationsJobs:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/v1/video/generations/jobs?api-version=%s",
				meta.Channel.BaseURL,
				apiVersion,
			),
		}, nil
	case mode.VideoGenerationsGetJobs:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/v1/video/generations/jobs/%s?api-version=%s",
				meta.Channel.BaseURL,
				meta.JobID,
				apiVersion,
			),
		}, nil
	case mode.VideoGenerationsContent:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/openai/v1/video/generations/%s/content/video?api-version=%s",
				meta.Channel.BaseURL,
				meta.GenerationID,
				apiVersion,
			),
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	token, _, err := GetTokenAndAPIVersion(meta.Channel.Key)
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", token)
	return nil
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"Model names do not contain '.' character, dots will be removed",
			"For example: gpt-3.5-turbo becomes gpt-35-turbo",
			fmt.Sprintf("API version is optional, default is '%s'", DefaultAPIVersion),
		},
		KeyHelp: "key or key|api-version",
		Models:  openai.ModelList,
	}
}
