package doubao

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
)

type doubaoModel struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Created         *int64             `json:"created"`
	Domain          string             `json:"domain"`
	ContextWindow   *int64             `json:"context_window"`
	MaxOutputTokens *int64             `json:"max_output_tokens"`
	TokenLimits     *doubaoTokenLimits `json:"token_limits"`
	Modalities      *doubaoModalities  `json:"modalities"`
	APISupported    map[string]bool    `json:"api_supported"`
	Features        *doubaoFeatures    `json:"features"`
}

type doubaoTokenLimits struct {
	ContextWindow        *int64 `json:"context_window"`
	MaxOutputTokenLength *int64 `json:"max_output_token_length"`
}

type doubaoModalities struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

type doubaoFeatures struct {
	Tools             *doubaoToolFeatures       `json:"tools"`
	StructuredOutputs *doubaoStructuredOutputs  `json:"structured_outputs"`
	Thinking          *doubaoThinkingCapability `json:"thinking"`
}

type doubaoToolFeatures struct {
	FunctionCalling bool `json:"function_calling"`
}

type doubaoStructuredOutputs struct {
	JSONObject bool `json:"json_object"`
	JSONSchema bool `json:"json_schema"`
}

type doubaoThinkingCapability struct {
	Types []string `json:"types"`
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

	endpoint, err := doubaoCatalogEndpoint(baseURL)
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
		Data *[]doubaoModel `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.Data == nil {
		return nil, modelcatalog.ErrInvalidResponse
	}

	upstreamModels := *response.Data
	models := make([]adaptor.ModelCatalogItem, 0, len(upstreamModels))

	seenIDs := make(map[string]struct{}, len(upstreamModels))
	for _, upstreamModel := range upstreamModels {
		if upstreamModel.ID == "" {
			return nil, modelcatalog.ErrInvalidResponse
		}

		if _, ok := seenIDs[upstreamModel.ID]; ok {
			return nil, modelcatalog.ErrInvalidResponse
		}

		seenIDs[upstreamModel.ID] = struct{}{}
		models = append(models, doubaoModelItem(upstreamModel))
	}

	return models, nil
}

func doubaoCatalogEndpoint(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", modelcatalog.ErrInvalidConfiguration
	}

	parsed.RawQuery = ""

	trimmedPath := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(trimmedPath, "/api/v3") ||
		strings.HasSuffix(trimmedPath, "/api/coding/v3") {
		return modelcatalog.JoinPath(parsed.String(), "models")
	}

	return modelcatalog.JoinPath(parsed.String(), "api", "v3", "models")
}

func doubaoModelItem(upstreamModel doubaoModel) adaptor.ModelCatalogItem {
	item := adaptor.ModelCatalogItem{"id": upstreamModel.ID}
	if upstreamModel.Name != "" {
		item["name"] = upstreamModel.Name
	}

	if upstreamModel.Created != nil {
		item["created"] = *upstreamModel.Created
	}

	if contextLength := doubaoContextLength(upstreamModel); contextLength != nil {
		item["context_length"] = *contextLength
	}

	if maxOutput := doubaoMaxOutputTokens(upstreamModel); maxOutput != nil {
		item["top_provider"] = map[string]any{
			"max_completion_tokens": *maxOutput,
		}
	}

	if architecture := doubaoArchitecture(upstreamModel); architecture != nil {
		item["architecture"] = architecture
	}

	if supported := doubaoSupportedParameters(upstreamModel.Features); len(supported) > 0 {
		item["supported_parameters"] = supported
	}

	return item
}

func doubaoContextLength(upstreamModel doubaoModel) *int64 {
	if upstreamModel.ContextWindow != nil {
		return upstreamModel.ContextWindow
	}

	if upstreamModel.TokenLimits != nil {
		return upstreamModel.TokenLimits.ContextWindow
	}

	return nil
}

func doubaoMaxOutputTokens(upstreamModel doubaoModel) *int64 {
	if upstreamModel.MaxOutputTokens != nil {
		return upstreamModel.MaxOutputTokens
	}

	if upstreamModel.TokenLimits != nil {
		return upstreamModel.TokenLimits.MaxOutputTokenLength
	}

	return nil
}

func doubaoArchitecture(upstreamModel doubaoModel) map[string]any {
	architecture := make(map[string]any, 3)

	if upstreamModel.Modalities != nil {
		if modalities := normalizeDoubaoModalities(
			upstreamModel.Modalities.InputModalities,
		); len(modalities) > 0 {
			architecture["input_modalities"] = modalities
		}

		if modalities := normalizeDoubaoModalities(
			upstreamModel.Modalities.OutputModalities,
		); len(modalities) > 0 {
			architecture["output_modalities"] = modalities
		}
	}

	if _, hasOutput := architecture["output_modalities"]; !hasOutput &&
		isDoubaoEmbeddingModel(upstreamModel) {
		architecture["modality"] = "embeddings"
	}

	if len(architecture) == 0 {
		return nil
	}

	return architecture
}

func normalizeDoubaoModalities(values []string) []string {
	modalities := make([]string, 0, len(values))

	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		var modality string
		switch strings.ToLower(strings.TrimSpace(value)) {
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

func isDoubaoEmbeddingModel(upstreamModel doubaoModel) bool {
	if strings.EqualFold(upstreamModel.Domain, "embedding") {
		return true
	}

	if upstreamModel.APISupported["embeddings"] ||
		upstreamModel.APISupported["embeddings/multimodal"] {
		return true
	}

	if upstreamModel.Modalities == nil {
		return false
	}

	return slices.ContainsFunc(upstreamModel.Modalities.OutputModalities, func(value string) bool {
		return strings.EqualFold(strings.TrimSpace(value), "embedding")
	})
}

func doubaoSupportedParameters(features *doubaoFeatures) []string {
	if features == nil {
		return nil
	}

	parameters := make([]string, 0, 3)
	if features.Tools != nil && features.Tools.FunctionCalling {
		parameters = append(parameters, "tools")
	}

	if features.StructuredOutputs != nil &&
		(features.StructuredOutputs.JSONObject || features.StructuredOutputs.JSONSchema) {
		parameters = append(parameters, "structured_outputs")
	}

	if features.Thinking != nil && slices.ContainsFunc(
		features.Thinking.Types,
		func(value string) bool {
			normalized := strings.ToLower(strings.TrimSpace(value))
			return normalized == "enabled" || normalized == "auto"
		},
	) {
		parameters = append(parameters, "reasoning")
	}

	return parameters
}
