package mistral

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

	endpoint, err := modelcatalog.JoinPath(baseURL, "models")
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
		Object string `json:"object"`
		Data   *[]struct {
			ID               string          `json:"id"`
			Capabilities     map[string]bool `json:"capabilities"`
			Job              string          `json:"job"`
			Root             string          `json:"root"`
			Object           string          `json:"object"`
			Created          *json.Number    `json:"created"`
			OwnedBy          string          `json:"owned_by"`
			MaxContextLength *int64          `json:"max_context_length"`
			Aliases          []string        `json:"aliases"`
			Type             string          `json:"TYPE"`
			Archived         *bool           `json:"archived"`
		} `json:"data"`
	}
	if err := json.Unmarshal(
		body,
		&response,
	); err != nil || response.Object != "list" ||
		response.Data == nil {
		return nil, modelcatalog.ErrInvalidResponse
	}

	models := make([]adaptor.ModelCatalogItem, 0, len(*response.Data))

	seenIDs := make(map[string]struct{}, len(*response.Data))
	for _, upstreamModel := range *response.Data {
		if upstreamModel.ID == "" {
			return nil, modelcatalog.ErrInvalidResponse
		}

		if _, ok := seenIDs[upstreamModel.ID]; ok {
			return nil, modelcatalog.ErrInvalidResponse
		}

		seenIDs[upstreamModel.ID] = struct{}{}

		item := adaptor.ModelCatalogItem{"id": upstreamModel.ID}
		if upstreamModel.Object != "" {
			item["object"] = upstreamModel.Object
		}

		if upstreamModel.Created != nil {
			item["created"] = *upstreamModel.Created
		}

		if upstreamModel.OwnedBy != "" {
			item["owned_by"] = upstreamModel.OwnedBy
		}

		if upstreamModel.Root != "" {
			item["root"] = upstreamModel.Root
		}

		if upstreamModel.MaxContextLength != nil {
			item["context_length"] = *upstreamModel.MaxContextLength
		}

		if upstreamModel.Capabilities != nil {
			item["capabilities"] = upstreamModel.Capabilities
		}

		if upstreamModel.Job != "" {
			item["job"] = upstreamModel.Job
		}

		if upstreamModel.Aliases != nil {
			item["aliases"] = upstreamModel.Aliases
		}

		if upstreamModel.Type != "" {
			item["type"] = upstreamModel.Type
		}

		if upstreamModel.Archived != nil {
			item["archived"] = *upstreamModel.Archived
		}

		models = append(models, item)
	}

	return models, nil
}
