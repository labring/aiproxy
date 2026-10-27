package gemini

import (
	"context"
	"encoding/json"
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

	baseURL := channel.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	endpoint, err := modelcatalog.JoinPath(baseURL, "v1beta", "models")
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
		query.Set("pageSize", "1000")
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		parsedURL.RawQuery = query.Encode()

		headers := make(http.Header)
		headers.Set("X-Goog-Api-Key", channel.Key)
		body, err := modelcatalog.Fetch(ctx, channel, http.MethodGet, parsedURL.String(), headers, nil)
		if err != nil {
			return nil, err
		}

		var response struct {
			Models *[]struct {
				Name             string `json:"name"`
				DisplayName      string `json:"displayName"`
				Description      string `json:"description"`
				InputTokenLimit  *int64 `json:"inputTokenLimit"`
				OutputTokenLimit *int64 `json:"outputTokenLimit"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &response); err != nil || response.Models == nil {
			return nil, modelcatalog.ErrInvalidResponse
		}
		for _, upstreamModel := range *response.Models {
			id := strings.TrimPrefix(upstreamModel.Name, "models/")
			if id == "" {
				return nil, modelcatalog.ErrInvalidResponse
			}

			item := adaptor.ModelCatalogItem{"id": id}
			if upstreamModel.DisplayName != "" {
				item["name"] = upstreamModel.DisplayName
			}
			if upstreamModel.Description != "" {
				item["description"] = upstreamModel.Description
			}
			if upstreamModel.InputTokenLimit != nil {
				item["context_length"] = *upstreamModel.InputTokenLimit
			}
			if upstreamModel.OutputTokenLimit != nil {
				item["top_provider"] = map[string]any{
					"max_completion_tokens": *upstreamModel.OutputTokenLimit,
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
