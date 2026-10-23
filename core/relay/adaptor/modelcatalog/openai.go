package modelcatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

func OpenAIDocumentedModels(
	ctx context.Context,
	channel *model.Channel,
	defaultBaseURL string,
	path ...string,
) ([]adaptor.ModelCatalogItem, error) {
	return openAIModels(ctx, channel, defaultBaseURL, OpenAIModelItem, path...)
}

func openAIModels(
	ctx context.Context,
	channel *model.Channel,
	defaultBaseURL string,
	project func(json.RawMessage) (adaptor.ModelCatalogItem, error),
	path ...string,
) ([]adaptor.ModelCatalogItem, error) {
	if channel == nil {
		return nil, ErrInvalidConfiguration
	}

	baseURL := strings.TrimSpace(channel.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if len(path) == 0 {
		path = []string{"models"}
	}
	endpoint, err := JoinPath(baseURL, path...)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)
	if channel.Key != "" {
		headers.Set("Authorization", "Bearer "+channel.Key)
	}
	body, err := Fetch(ctx, channel, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return nil, err
	}

	var response map[string]json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, ErrInvalidResponse
	}
	rawData, ok := response["data"]
	if !ok || string(rawData) == "null" {
		return nil, ErrInvalidResponse
	}
	var data []json.RawMessage
	if err := json.Unmarshal(rawData, &data); err != nil {
		return nil, ErrInvalidResponse
	}

	models := make([]adaptor.ModelCatalogItem, 0, len(data))
	for _, rawModel := range data {
		item, err := project(rawModel)
		if err != nil {
			return nil, err
		}
		models = append(models, item)
	}

	return models, nil
}

func OpenAIModelItem(rawModel json.RawMessage) (adaptor.ModelCatalogItem, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawModel, &fields); err != nil || fields == nil {
		return nil, ErrInvalidResponse
	}
	var id string
	if err := json.Unmarshal(fields["id"], &id); err != nil || id == "" {
		return nil, ErrInvalidResponse
	}

	item := adaptor.ModelCatalogItem{"id": id}
	copyModelField(fields, item, "created", "created")
	copyModelField(fields, item, "shutdown_date", "expiration_date")
	return item, nil
}

func copyModelField(
	fields map[string]json.RawMessage,
	item map[string]any,
	source, target string,
) {
	if rawValue := fields[source]; rawValue != nil && string(rawValue) != "null" {
		value, ok := decodeModelField(target, rawValue)
		if ok {
			item[target] = value
		}
	}
}
