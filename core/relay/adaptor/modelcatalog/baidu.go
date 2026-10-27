package modelcatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

func BaiduSystemModelSets(
	ctx context.Context,
	channel *model.Channel,
	baseURL, accessKey, secretKey string,
) ([]adaptor.ModelCatalogItem, error) {
	if channel == nil {
		return nil, ErrInvalidConfiguration
	}
	endpoint, err := JoinPath(baseURL, "model")
	if err != nil {
		return nil, err
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return nil, ErrInvalidConfiguration
	}
	query := parsedEndpoint.Query()
	query.Set("Action", "DescribeSystemModelSets")
	parsedEndpoint.RawQuery = query.Encode()
	endpoint = parsedEndpoint.String()

	const maxKeys = 100
	models := make([]adaptor.ModelCatalogItem, 0)
	seenIDs := make(map[string]struct{})
	seenMarkers := make(map[string]struct{})
	marker := ""
	for {
		requestBody, err := json.Marshal(struct {
			Marker  string `json:"marker,omitempty"`
			MaxKeys int    `json:"maxKeys"`
		}{Marker: marker, MaxKeys: maxKeys})
		if err != nil {
			return nil, ErrInvalidConfiguration
		}
		headers, err := BCEAuthorizationHeaders(http.MethodPost, endpoint, accessKey, secretKey)
		if err != nil {
			return nil, err
		}
		headers.Set("Content-Type", "application/json")
		body, err := Fetch(ctx, channel, http.MethodPost, endpoint, headers, requestBody)
		if err != nil {
			return nil, err
		}

		var response struct {
			Result *struct {
				SystemModelSetList *[]struct {
					ModelNames []string `json:"modelNames"`
				} `json:"systemModelSetList"`
				PageInfo *struct {
					IsTruncated bool   `json:"isTruncated"`
					NextMarker  string `json:"nextMarker"`
				} `json:"pageInfo"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, ErrInvalidResponse
		}
		if response.Result == nil || response.Result.SystemModelSetList == nil || response.Result.PageInfo == nil {
			return nil, ErrInvalidResponse
		}
		for _, modelSet := range *response.Result.SystemModelSetList {
			for _, modelName := range modelSet.ModelNames {
				if modelName == "" {
					return nil, ErrInvalidResponse
				}
				if _, ok := seenIDs[modelName]; ok {
					continue
				}
				seenIDs[modelName] = struct{}{}
				models = append(models, adaptor.ModelCatalogItem{"id": modelName})
			}
		}

		pageInfo := response.Result.PageInfo
		if !pageInfo.IsTruncated {
			return models, nil
		}
		if pageInfo.NextMarker == "" {
			return nil, ErrInvalidResponse
		}
		if _, ok := seenMarkers[pageInfo.NextMarker]; ok {
			return nil, ErrInvalidResponse
		}
		seenMarkers[pageInfo.NextMarker] = struct{}{}
		marker = pageInfo.NextMarker
	}
}
