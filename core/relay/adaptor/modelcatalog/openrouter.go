package modelcatalog

import (
	"encoding/json"

	"github.com/labring/aiproxy/core/relay/adaptor"
)

var openRouterModelFields = []string{
	"canonical_slug",
	"hugging_face_id",
	"name",
	"created",
	"description",
	"context_length",
	"architecture",
	"top_provider",
	"per_request_limits",
	"supported_parameters",
	"default_parameters",
	"expiration_date",
}

var openRouterNestedModelFields = map[string][]string{
	"architecture": {
		"modality", "input_modalities", "output_modalities", "tokenizer", "instruct_type",
	},
	"top_provider":       {"context_length", "max_completion_tokens", "is_moderated"},
	"per_request_limits": {"prompt_tokens", "completion_tokens"},
	"default_parameters": {
		"temperature",
		"top_p",
		"top_k",
		"frequency_penalty",
		"presence_penalty",
		"repetition_penalty",
	},
}

func OpenRouterModelItem(rawModel json.RawMessage) (adaptor.ModelCatalogItem, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawModel, &fields); err != nil || fields == nil {
		return nil, ErrInvalidResponse
	}

	return openRouterModelItem(fields)
}

func openRouterModelItem(fields map[string]json.RawMessage) (adaptor.ModelCatalogItem, error) {
	if fields == nil {
		return nil, ErrInvalidResponse
	}

	var id string
	if err := json.Unmarshal(fields["id"], &id); err != nil || id == "" {
		return nil, ErrInvalidResponse
	}

	item := adaptor.ModelCatalogItem{"id": id}
	for _, field := range openRouterModelFields {
		rawValue, ok := fields[field]
		if !ok || string(rawValue) == "null" {
			continue
		}

		value, ok := decodeModelField(field, rawValue)
		if ok {
			item[field] = value
		}
	}

	return item, nil
}

func decodeModelField(field string, raw json.RawMessage) (any, bool) {
	switch field {
	case "canonical_slug", "hugging_face_id", "name", "description", "expiration_date":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false
		}

		return value, true
	case "modality", "tokenizer", "instruct_type":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false
		}

		if field == "modality" {
			return normalizeModality(value)
		}

		return value, true
	case "created", "context_length", "max_completion_tokens", "top_k":
		var value int64
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false
		}

		return value, true
	case "temperature",
		"top_p",
		"frequency_penalty",
		"presence_penalty",
		"repetition_penalty",
		"prompt_tokens",
		"completion_tokens":
		var value float64
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false
		}

		return value, true
	case "is_moderated":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false
		}

		return value, true
	case "supported_parameters":
		var value []string
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return nil, false
		}

		return value, true
	case "architecture", "top_provider", "per_request_limits", "default_parameters":
		return decodeModelObject(field, raw)
	case "input_modalities", "output_modalities":
		var value []string
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return nil, false
		}

		return value, true
	default:
		return nil, false
	}
}

func normalizeModality(value string) (any, bool) {
	switch value {
	case "text",
		"image",
		"video",
		"audio",
		"speech",
		"transcription",
		"embeddings",
		"rerank",
		"decisions":
		return value, true
	default:
		return nil, false
	}
}

func decodeModelObject(field string, raw json.RawMessage) (map[string]any, bool) {
	allowedFields := openRouterNestedModelFields[field]
	if len(allowedFields) == 0 {
		return nil, false
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, false
	}

	value := make(map[string]any, len(allowedFields))
	for _, nestedField := range allowedFields {
		if rawValue, ok := fields[nestedField]; ok && string(rawValue) != "null" {
			if nestedValue, ok := decodeModelField(nestedField, rawValue); ok {
				value[nestedField] = nestedValue
			}
		}
	}

	if len(value) == 0 {
		return nil, false
	}

	return value, true
}
