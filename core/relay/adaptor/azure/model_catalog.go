package azure

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

const modelCatalogAPIVersion = "2024-10-21"

func (a *Adaptor) ListModels(
	ctx context.Context,
	channel *model.Channel,
	defaultBaseURL string,
) ([]adaptor.ModelCatalogItem, error) {
	if channel == nil {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	token, apiVersion, err := GetTokenAndAPIVersion(channel.Key)
	if err != nil || token == "" {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	if !strings.Contains(channel.Key, "|") {
		apiVersion = modelCatalogAPIVersion
	}

	if apiVersion == "" {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	baseURL := strings.TrimSpace(channel.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	endpoint, err := modelcatalog.JoinPath(baseURL, "openai", "models")
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	query := parsedURL.Query()
	query.Set("api-version", apiVersion)
	parsedURL.RawQuery = query.Encode()

	headers := make(http.Header)
	headers.Set("Api-Key", token)

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

	models := make([]adaptor.ModelCatalogItem, 0, len(*response.Data))

	seenIDs := make(map[string]struct{}, len(*response.Data))
	for _, rawModel := range *response.Data {
		item, err := modelcatalog.OpenAIModelItem(rawModel)
		if err != nil {
			return nil, err
		}

		id, _ := item["id"].(string)
		if _, ok := seenIDs[id]; ok {
			return nil, modelcatalog.ErrInvalidResponse
		}

		seenIDs[id] = struct{}{}

		models = append(models, item)
	}

	return models, nil
}
