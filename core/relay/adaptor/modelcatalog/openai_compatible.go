package modelcatalog

import (
	"context"
	"encoding/json"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

func OpenAICompatibleModels(
	ctx context.Context,
	channel *model.Channel,
	defaultBaseURL string,
	path ...string,
) ([]adaptor.ModelCatalogItem, error) {
	return openAIModels(
		ctx,
		channel,
		defaultBaseURL,
		func(rawModel json.RawMessage) (adaptor.ModelCatalogItem, error) {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rawModel, &fields); err != nil || fields == nil {
				return nil, ErrInvalidResponse
			}

			return openAICompatibleModelItem(fields)
		},
		path...,
	)
}

func openAICompatibleModelItem(
	fields map[string]json.RawMessage,
) (adaptor.ModelCatalogItem, error) {
	item, err := openRouterModelItem(fields)
	if err != nil {
		return nil, err
	}

	for _, field := range []struct{ source, target string }{
		{"context_window", "context_length"},
		{"max_context_length", "context_length"},
		{"context_size", "context_length"},
		{"title", "name"},
		{"shutdown_date", "expiration_date"},
		{"supported_sampling_parameters", "supported_parameters"},
	} {
		if _, exists := item[field.target]; exists {
			continue
		}

		copyModelField(fields, item, field.source, field.target)
	}

	if _, exists := item["top_provider"]; !exists {
		for _, source := range []string{
			"max_output_tokens", "max_completion_tokens", "max_completions_tokens", "max_output_length",
		} {
			value, ok := decodeModelField("context_length", fields[source])
			if ok {
				item["top_provider"] = map[string]any{"max_completion_tokens": value}
				break
			}
		}
	}

	architecture, _ := item["architecture"].(map[string]any)
	if architecture == nil {
		architecture = make(map[string]any)
	}

	for _, field := range []struct{ source, target string }{
		{"input_modalities", "input_modalities"},
		{"output_modalities", "output_modalities"},
	} {
		if _, exists := architecture[field.target]; exists {
			continue
		}

		copyModelField(fields, architecture, field.source, field.target)
	}

	if len(architecture) > 0 {
		item["architecture"] = architecture
	}

	return item, nil
}
