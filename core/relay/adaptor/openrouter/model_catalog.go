package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
)

const openRouterModelPageSize = 500

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
	endpoint, err := modelcatalog.JoinPath(baseURL, "models")
	if err != nil {
		return nil, err
	}

	models := make([]adaptor.ModelCatalogItem, 0)
	seenIDs := make(map[string]struct{})
	for offset := 0; ; {
		parsedURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, modelcatalog.ErrInvalidConfiguration
		}
		query := parsedURL.Query()
		query.Set("limit", strconv.Itoa(openRouterModelPageSize))
		query.Set("offset", strconv.Itoa(offset))
		query.Set("output_modalities", "all")
		parsedURL.RawQuery = query.Encode()

		headers := make(http.Header)
		if channel.Key != "" {
			headers.Set("Authorization", "Bearer "+channel.Key)
		}
		body, err := modelcatalog.Fetch(ctx, channel, http.MethodGet, parsedURL.String(), headers, nil)
		if err != nil {
			return nil, err
		}

		var response struct {
			Data *[]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil || response.Data == nil {
			return nil, modelcatalog.ErrInvalidResponse
		}
		for _, rawModel := range *response.Data {
			item, err := modelcatalog.OpenRouterModelItem(rawModel)
			if err != nil {
				return nil, err
			}
			id := item["id"].(string)
			if _, ok := seenIDs[id]; ok {
				return nil, modelcatalog.ErrInvalidResponse
			}
			seenIDs[id] = struct{}{}
			models = append(models, item)
		}

		if len(*response.Data) < openRouterModelPageSize {
			return models, nil
		}
		offset += len(*response.Data)
	}
}
