package ali

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
)

type aliModel struct {
	ID                string               `json:"model"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	Features          []string             `json:"features"`
	Capabilities      []string             `json:"capabilities"`
	PublishedTime     string               `json:"published_time"`
	InferenceMetadata aliInferenceMetadata `json:"inference_metadata"`
	ModelInfo         aliModelInfo         `json:"model_info"`
}

type aliInferenceMetadata struct {
	RequestModality  []string `json:"request_modality"`
	ResponseModality []string `json:"response_modality"`
}

type aliModelInfo struct {
	ContextWindow *int64 `json:"context_window"`
	MaxOutput     *int64 `json:"max_output_tokens"`
}

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

	parsedBase, err := url.Parse(baseURL)
	if err != nil || parsedBase.Host == "" || parsedBase.User != nil || parsedBase.Fragment != "" ||
		(parsedBase.Scheme != "http" && parsedBase.Scheme != "https") {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	basePath := strings.TrimSuffix(parsedBase.Path, "/compatible-mode/v1")
	parsedBase.Path = path.Join("/", basePath, "api", "v1", "models")
	parsedBase.RawPath = ""
	endpoint := parsedBase.String()

	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+channel.Key)

	const pageSize = 100

	models := make([]adaptor.ModelCatalogItem, 0)

	seenIDs := make(map[string]struct{})
	for pageNo := 1; ; pageNo++ {
		parsedURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, modelcatalog.ErrInvalidConfiguration
		}

		query := parsedURL.Query()
		query.Set("page_no", strconv.Itoa(pageNo))
		query.Set("page_size", "100")
		parsedURL.RawQuery = query.Encode()

		body, err := modelcatalog.Fetch(
			ctx,
			channel,
			http.MethodGet,
			parsedURL.String(),
			headers,
			nil,
		)
		if err != nil {
			return nil, err
		}

		var response struct {
			Output struct {
				Models *[]aliModel `json:"models"`
				Total  *int        `json:"total"`
			} `json:"output"`
		}
		if err := json.Unmarshal(body, &response); err != nil || response.Output.Models == nil {
			return nil, modelcatalog.ErrInvalidResponse
		}

		pageModels := *response.Output.Models
		for _, upstreamModel := range pageModels {
			if upstreamModel.ID == "" {
				return nil, modelcatalog.ErrInvalidResponse
			}

			if _, ok := seenIDs[upstreamModel.ID]; ok {
				return nil, modelcatalog.ErrInvalidResponse
			}

			seenIDs[upstreamModel.ID] = struct{}{}

			models = append(models, aliModelItem(upstreamModel))
		}

		if response.Output.Total != nil {
			if len(models) >= *response.Output.Total {
				return models, nil
			}

			if len(pageModels) == 0 {
				return nil, modelcatalog.ErrInvalidResponse
			}
		} else if len(pageModels) < pageSize {
			return models, nil
		}
	}
}

func aliModelItem(upstreamModel aliModel) adaptor.ModelCatalogItem {
	item := adaptor.ModelCatalogItem{"id": upstreamModel.ID}
	if upstreamModel.Name != "" {
		item["name"] = upstreamModel.Name
	}

	if upstreamModel.Description != "" {
		item["description"] = upstreamModel.Description
	}

	if created := aliPublishedUnix(upstreamModel.PublishedTime); created != nil {
		item["created"] = *created
	}

	if upstreamModel.ModelInfo.ContextWindow != nil {
		item["context_length"] = *upstreamModel.ModelInfo.ContextWindow
	}

	architecture := aliArchitecture(upstreamModel.InferenceMetadata)
	if architecture != nil && len(upstreamModel.InferenceMetadata.ResponseModality) == 0 {
		if modality := aliEmbeddingModality(upstreamModel.Capabilities); modality != "" {
			architecture["modality"] = modality
		}
	}

	if architecture != nil {
		item["architecture"] = architecture
	}

	if maxOutput := upstreamModel.ModelInfo.MaxOutput; maxOutput != nil {
		item["top_provider"] = map[string]any{"max_completion_tokens": *maxOutput}
	}

	if supported := aliSupportedParameters(upstreamModel.Features); len(supported) > 0 {
		item["supported_parameters"] = supported
	}

	return item
}

func aliArchitecture(metadata aliInferenceMetadata) map[string]any {
	architecture := make(map[string]any, 2)
	if modalities := aliModalities(metadata.RequestModality); len(modalities) > 0 {
		architecture["input_modalities"] = modalities
	}

	if modalities := aliModalities(metadata.ResponseModality); len(modalities) > 0 {
		architecture["output_modalities"] = modalities
	}

	if len(architecture) == 0 {
		return nil
	}

	return architecture
}

func aliModalities(values []string) []string {
	modalities := make([]string, 0, len(values))

	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		var modality string
		switch strings.ToLower(value) {
		case "text":
			modality = "text"
		case "image":
			modality = "image"
		case "video":
			modality = "video"
		case "audio":
			modality = "audio"
		default:
			continue
		}

		if _, ok := seen[modality]; ok {
			continue
		}

		seen[modality] = struct{}{}
		modalities = append(modalities, modality)
	}

	return modalities
}

func aliEmbeddingModality(capabilities []string) string {
	for _, capability := range capabilities {
		switch capability {
		case "ME", "TR":
			return "embeddings"
		}
	}

	return ""
}

func aliSupportedParameters(features []string) []string {
	parameters := make([]string, 0, len(features))
	for _, feature := range features {
		switch feature {
		case "function-calling":
			parameters = append(parameters, "tools")
		case "structured-outputs":
			parameters = append(parameters, "structured_outputs")
		}
	}

	return parameters
}

func aliPublishedUnix(published string) *int64 {
	value, err := time.ParseInLocation(time.DateTime, published, time.UTC)
	if err != nil {
		return nil
	}

	unix := value.Unix()

	return &unix
}
