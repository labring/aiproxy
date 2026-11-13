package ollama

import (
	"context"
	"encoding/json"
	"net/http"
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

	endpoint, err := modelcatalog.JoinPath(baseURL, "api", "tags")
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)
	if channel.Key != "" {
		headers.Set("Authorization", "Bearer "+channel.Key)
	}

	body, err := modelcatalog.Fetch(ctx, channel, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Models *[]struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.Models == nil {
		return nil, modelcatalog.ErrInvalidResponse
	}

	models := make([]adaptor.ModelCatalogItem, 0, len(*response.Models))
	for _, upstreamModel := range *response.Models {
		if upstreamModel.Name == "" {
			return nil, modelcatalog.ErrInvalidResponse
		}

		models = append(models, adaptor.ModelCatalogItem{"id": upstreamModel.Name})
	}

	return models, nil
}
