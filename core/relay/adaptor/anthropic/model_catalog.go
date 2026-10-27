package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

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
	endpoint, err := modelcatalog.JoinPath(baseURL, "models")
	if err != nil {
		return nil, err
	}

	models := make([]adaptor.ModelCatalogItem, 0)
	seenCursors := make(map[string]struct{})
	afterID := ""
	for {
		parsedURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, modelcatalog.ErrInvalidConfiguration
		}
		query := parsedURL.Query()
		query.Set("limit", "100")
		if afterID != "" {
			query.Set("after_id", afterID)
		}
		parsedURL.RawQuery = query.Encode()

		headers := make(http.Header)
		headers.Set(AnthropicTokenHeader, channel.Key)
		headers.Set("Anthropic-Version", AnthropicVersion)
		body, err := modelcatalog.Fetch(ctx, channel, http.MethodGet, parsedURL.String(), headers, nil)
		if err != nil {
			return nil, err
		}

		var response struct {
			Data *[]struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				CreatedAt   string `json:"created_at"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, modelcatalog.ErrInvalidResponse
		}
		if response.Data == nil {
			return nil, modelcatalog.ErrInvalidResponse
		}
		for _, upstreamModel := range *response.Data {
			if upstreamModel.ID == "" {
				return nil, modelcatalog.ErrInvalidResponse
			}

			item := adaptor.ModelCatalogItem{"id": upstreamModel.ID}
			if upstreamModel.DisplayName != "" {
				item["name"] = upstreamModel.DisplayName
			}
			if upstreamModel.CreatedAt != "" {
				createdAt, err := time.Parse(time.RFC3339Nano, upstreamModel.CreatedAt)
				if err != nil {
					return nil, modelcatalog.ErrInvalidResponse
				}
				item["created"] = createdAt.Unix()
			}
			models = append(models, item)
		}

		if !response.HasMore {
			return models, nil
		}
		if response.LastID == "" {
			return nil, modelcatalog.ErrInvalidResponse
		}
		if _, ok := seenCursors[response.LastID]; ok {
			return nil, modelcatalog.ErrInvalidResponse
		}
		seenCursors[response.LastID] = struct{}{}
		afterID = response.LastID
	}
}
