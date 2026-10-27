package cloudflare

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

const cloudflareModelPageSize = 100

func (a *Adaptor) ListModels(
	ctx context.Context,
	channel *model.Channel,
	defaultBaseURL string,
) ([]adaptor.ModelCatalogItem, error) {
	if channel == nil {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	if channel.Key == "" {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	baseURL := strings.TrimSpace(channel.BaseURL)
	isGateway := isAIGateWay(strings.TrimRight(baseURL, "/"))

	var endpoint string
	if isGateway {
		accountID := cloudflareAIGatewayAccountID(baseURL)
		if accountID == "" {
			return nil, modelcatalog.ErrInvalidConfiguration
		}

		baseURL = defaultBaseURL

		parsed, err := url.Parse(baseURL)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, modelcatalog.ErrInvalidConfiguration
		}

		parsed.Path = ""
		parsed.RawPath = ""
		parsed.RawQuery = ""
		baseURL = parsed.String()

		catalogURL, err := modelcatalog.JoinPath(
			baseURL,
			"client",
			"v4",
			"accounts",
			accountID,
			"ai",
			"models",
			"search",
		)
		if err != nil {
			return nil, err
		}

		endpoint = catalogURL
	} else {
		aiBaseURL, err := cloudflareAIBaseURL(baseURL)
		if err != nil {
			return nil, modelcatalog.ErrInvalidConfiguration
		}

		catalogURL, err := modelcatalog.JoinPath(aiBaseURL, "models", "search")
		if err != nil {
			return nil, err
		}

		endpoint = catalogURL
	}

	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+channel.Key)

	models := make([]adaptor.ModelCatalogItem, 0)

	seenIDs := make(map[string]struct{})
	for page := 1; ; page++ {
		parsedURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, modelcatalog.ErrInvalidConfiguration
		}

		query := parsedURL.Query()
		query.Set("format", "openrouter")
		query.Set("page", strconv.Itoa(page))
		query.Set("per_page", strconv.Itoa(cloudflareModelPageSize))
		parsedURL.RawQuery = query.Encode()

		requestURL := parsedURL.String()

		body, err := modelcatalog.Fetch(ctx, channel, http.MethodGet, requestURL, headers, nil)
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
				return nil, modelcatalog.ErrInvalidResponse
			}

			id, ok := item["id"].(string)
			if !ok {
				return nil, modelcatalog.ErrInvalidResponse
			}

			if _, ok := seenIDs[id]; ok {
				return nil, modelcatalog.ErrInvalidResponse
			}

			seenIDs[id] = struct{}{}

			models = append(models, item)
		}

		if len(*response.Data) < cloudflareModelPageSize {
			return models, nil
		}
	}
}
