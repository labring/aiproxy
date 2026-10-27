package textembeddingsinference

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
)

func (a *Adaptor) ListModels(
	ctx context.Context,
	channel *model.Channel,
	_ string,
) ([]adaptor.ModelCatalogItem, error) {
	if channel == nil {
		return nil, modelcatalog.ErrInvalidConfiguration
	}
	endpoint, err := modelcatalog.JoinPath(channel.BaseURL, "info")
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
		ModelID        string `json:"model_id"`
		MaxInputLength *int64 `json:"max_input_length"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.ModelID == "" {
		return nil, modelcatalog.ErrInvalidResponse
	}

	item := adaptor.ModelCatalogItem{"id": response.ModelID}
	if response.MaxInputLength != nil {
		item["context_length"] = *response.MaxInputLength
	}

	return []adaptor.ModelCatalogItem{item}, nil
}
