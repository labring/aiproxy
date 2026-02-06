package azure

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

// https://azure.microsoft.com/en-us/pricing/details/cognitive-services/openai-service/

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) DefaultBaseURL() string {
	return "https://{resource_name}.openai.azure.com"
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	return GetRequestURL(meta, true)
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	model := meta.ActualModel
	newmodel := strings.ReplaceAll(model, ".", "")

	meta.ActualModel = newmodel
	defer func() {
		meta.ActualModel = model
	}()

	return a.Adaptor.ConvertRequest(meta, store, req)
}

//nolint:gocyclo
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
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/deployments",
			model,
			"/images/generations",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.ImagesEdits:
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/deployments",
			model,
			"/images/edits",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.AudioTranscription:
		// https://learn.microsoft.com/en-us/azure/ai-services/openai/whisper-quickstart?tabs=command-line#rest-api
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/deployments",
			model,
			"/audio/transcriptions",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.AudioSpeech:
		// https://learn.microsoft.com/en-us/azure/ai-services/openai/text-to-speech-quickstart?tabs=command-line#rest-api
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/deployments",
			model,
			"/audio/speech",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		// Check if model requires Responses API
		if openai.IsResponsesOnlyModel(&meta.ModelConfig, meta.ActualModel) {
			// Azure Responses API format
			url, err := url.JoinPath(
				meta.Channel.BaseURL,
				"/openai/v1/responses",
			)
			if err != nil {
				return adaptor.RequestURL{}, err
			}

			return adaptor.RequestURL{
				Method: http.MethodPost,
				URL:    fmt.Sprintf("%s?api-version=%s", url, "preview"),
			}, nil
		}

		// https://learn.microsoft.com/en-us/azure/cognitive-services/openai/chatgpt-quickstart?pivots=rest-api&tabs=command-line#rest-api
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/deployments",
			model,
			"/chat/completions",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.Completions:
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/deployments",
			model,
			"/completions",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.Embeddings:
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/deployments",
			model,
			"/embeddings",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.VideoGenerationsJobs:
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/v1/video/generations/jobs",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.VideoGenerationsGetJobs:
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/v1/video/generations/jobs",
			meta.JobID,
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil
	case mode.VideoGenerationsContent:
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/v1/video/generations",
			meta.GenerationID,
			"/content/video",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, apiVersion),
		}, nil

	// Add support for Responses API endpoints
	case mode.Responses:
		// POST https://YOUR-RESOURCE-NAME.openai.azure.com/openai/v1/responses?api-version=preview
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/v1/responses",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, "preview"),
		}, nil

	case mode.ResponsesGet:
		// GET https://YOUR-RESOURCE-NAME.openai.azure.com/openai/v1/responses/{response_id}?api-version=preview
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/v1/responses",
			meta.ResponseID,
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    fmt.Sprintf("%s?api-version=%s", url, "preview"),
		}, nil

	case mode.ResponsesDelete:
		// DELETE https://YOUR-RESOURCE-NAME.openai.azure.com/openai/v1/responses/{response_id}?api-version=preview
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/v1/responses",
			meta.ResponseID,
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodDelete,
			URL:    fmt.Sprintf("%s?api-version=%s", url, "preview"),
		}, nil

	case mode.ResponsesCancel:
		// POST https://YOUR-RESOURCE-NAME.openai.azure.com/openai/v1/responses/{response_id}/cancel?api-version=preview
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/v1/responses",
			meta.ResponseID,
			"/cancel",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s?api-version=%s", url, "preview"),
		}, nil

	case mode.ResponsesInputItems:
		// GET https://YOUR-RESOURCE-NAME.openai.azure.com/openai/v1/responses/{response_id}/input_items?api-version=preview
		url, err := url.JoinPath(
			meta.Channel.BaseURL,
			"/openai/v1/responses",
			meta.ResponseID,
			"/input_items",
		)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    fmt.Sprintf("%s?api-version=%s", url, "preview"),
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
		Readme: fmt.Sprintf(
			"Model names do not contain '.' character, dots will be removed\nFor example: gpt-3.5-turbo becomes gpt-35-turbo\nAPI version is optional, default is '%s'\nGemini support",
			DefaultAPIVersion,
		),
		KeyHelp: "key or key|api-version",
		Models:  openai.ModelList,
	}
}
