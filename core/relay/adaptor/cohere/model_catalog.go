package cohere

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
)

func (a *Adaptor) ListModels(
	ctx context.Context,
	channel *model.Channel,
	defaultBaseURL string,
) ([]adaptor.ModelCatalogItem, error) {
	if channel == nil {
		return nil, modelcatalog.ErrInvalidConfiguration
	}
	baseURL := strings.TrimSpace(channel.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	endpoint, err := modelcatalog.JoinPath(baseURL, "v1", "models")
	if err != nil {
		return nil, err
	}

	models := make([]adaptor.ModelCatalogItem, 0)
	seenTokens := make(map[string]struct{})
	pageToken := ""
	for {
		parsedURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, modelcatalog.ErrInvalidConfiguration
		}
		query := parsedURL.Query()
		query.Set("page_size", "100")
		if pageToken != "" {
			query.Set("page_token", pageToken)
		}
		parsedURL.RawQuery = query.Encode()

		headers := make(http.Header)
		headers.Set("Authorization", "Bearer "+channel.Key)
		body, err := modelcatalog.Fetch(ctx, channel, http.MethodGet, parsedURL.String(), headers, nil)
		if err != nil {
			return nil, err
		}

		var response struct {
			Models *[]struct {
				Name             string   `json:"name"`
				ContextLength    *float64 `json:"context_length"`
				SamplingDefaults *struct {
					Temperature      *float64 `json:"temperature"`
					K                *int64   `json:"k"`
					P                *float64 `json:"p"`
					FrequencyPenalty *float64 `json:"frequency_penalty"`
					PresencePenalty  *float64 `json:"presence_penalty"`
				} `json:"sampling_defaults"`
			} `json:"models"`
			NextPageToken string `json:"next_page_token"`
		}
		if err := json.Unmarshal(body, &response); err != nil || response.Models == nil {
			return nil, modelcatalog.ErrInvalidResponse
		}
		for _, upstreamModel := range *response.Models {
			if upstreamModel.Name == "" {
				return nil, modelcatalog.ErrInvalidResponse
			}
			item := adaptor.ModelCatalogItem{"id": upstreamModel.Name}
			if length := upstreamModel.ContextLength; length != nil &&
				*length >= 0 && math.Trunc(*length) == *length && *length < 1<<63 {
				item["context_length"] = int64(*length)
			}
			if defaults := upstreamModel.SamplingDefaults; defaults != nil {
				parameters := make(map[string]any, 5)
				if value := defaults.Temperature; value != nil {
					parameters["temperature"] = *value
				}
				if value := defaults.K; value != nil {
					parameters["top_k"] = *value
				}
				if value := defaults.P; value != nil {
					parameters["top_p"] = *value
				}
				if value := defaults.FrequencyPenalty; value != nil {
					parameters["frequency_penalty"] = *value
				}
				if value := defaults.PresencePenalty; value != nil {
					parameters["presence_penalty"] = *value
				}
				if len(parameters) > 0 {
					item["default_parameters"] = parameters
				}
			}
			models = append(models, item)
		}

		if response.NextPageToken == "" {
			return models, nil
		}
		if _, ok := seenTokens[response.NextPageToken]; ok {
			return nil, modelcatalog.ErrInvalidResponse
		}
		seenTokens[response.NextPageToken] = struct{}{}
		pageToken = response.NextPageToken
	}
}
