package modelcatalog_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
	"github.com/stretchr/testify/require"
)

func TestOpenAIModelsMapsProviderFactsToStandardFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer catalog-test-key" {
			t.Errorf("unexpected authorization header")
		}

		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write(
			[]byte(
				`{"data":[{"id":"deepseek-chat","name":"DeepSeek Chat","created":1720000000,"context_window":64000,"max_output_length":8192,"supported_sampling_parameters":["temperature","top_p"],"input_modalities":["text","image"],"output_modalities":["text"],"shutdown_date":"2026-01-01","provider_metadata":{"discard":true}}]}`,
			),
		)
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	models, err := modelcatalog.OpenAICompatibleModels(context.Background(), &model.Channel{
		Key:     "catalog-test-key",
		BaseURL: server.URL + "/v1",
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 1)
	encoded, err := json.Marshal(models)
	require.NoError(t, err)

	var response []map[string]any
	require.NoError(t, json.Unmarshal(encoded, &response))
	require.Equal(t, "deepseek-chat", response[0]["id"])
	require.Equal(t, "DeepSeek Chat", response[0]["name"])
	require.Equal(t, float64(64000), response[0]["context_length"])
	require.Equal(
		t,
		map[string]any{"max_completion_tokens": float64(8192)},
		response[0]["top_provider"],
	)
	require.Equal(t, []any{"temperature", "top_p"}, response[0]["supported_parameters"])
	require.Equal(t, map[string]any{
		"input_modalities":  []any{"text", "image"},
		"output_modalities": []any{"text"},
	}, response[0]["architecture"])
	require.Equal(t, "2026-01-01", response[0]["expiration_date"])
	require.NotContains(t, response[0], "provider_metadata")
}

func TestOpenAIModelsOmitsEmptyAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("empty provider key must not create an Authorization header")
		}

		_, err := w.Write(
			[]byte(
				`{"data":[{"id":"jina/embeddings-v3","architecture":{"input_modalities":["text"],"output_modalities":["text"]}}]}`,
			),
		)
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	models, err := modelcatalog.OpenAICompatibleModels(
		context.Background(),
		&model.Channel{BaseURL: server.URL + "/v1"},
		"",
	)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "jina/embeddings-v3", models[0]["id"])
	require.Contains(t, models[0], "architecture")
}

func TestOpenAIModelsOmitsNonstandardOptionalFieldTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write(
			[]byte(
				`{"data":[{"id":"provider/model","name":12,"created":"today","context_length":"large","architecture":[],"supported_parameters":"tools","provider_metadata":{"secret":"drop"}}]}`,
			),
		)
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	models, err := modelcatalog.OpenAICompatibleModels(
		context.Background(),
		&model.Channel{BaseURL: server.URL + "/v1"},
		"",
	)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "provider/model", models[0]["id"])

	for _, field := range []string{
		"name", "created", "context_length", "architecture", "supported_parameters", "provider_metadata",
	} {
		require.NotContains(t, models[0], field)
	}
}

func TestOpenAIModelsFiltersUnrecognizedNestedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(
			[]byte(
				`{"data":[{"id":"provider/model","architecture":{"modality":"text->text","input_modalities":["text"],"tokenizer":"GPT","provider_hint":"drop"},"pricing":{"prompt":"0.1","completion":"0.2","request":"0.01","discount":"0.25","overrides":[{"min_prompt_tokens":200000,"prompt":"0.05"}],"provider_discount":"drop"},"top_provider":{"context_length":32000,"max_completion_tokens":4000,"is_moderated":true,"provider_region":"drop"},"per_request_limits":{"prompt_tokens":128000,"completion_tokens":4000,"provider_limit":100},"default_parameters":{"temperature":0.2,"top_p":0.9,"provider_mode":"drop"}}]}`,
			),
		)
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := modelcatalog.OpenAICompatibleModels(
		context.Background(),
		&model.Channel{BaseURL: server.URL + "/v1"},
		"",
	)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, map[string]any{
		"input_modalities": []string{"text"},
		"tokenizer":        "GPT",
	}, models[0]["architecture"])
	require.NotContains(t, models[0], "pricing")
	require.Equal(t, map[string]any{
		"context_length":        int64(32000),
		"max_completion_tokens": int64(4000),
		"is_moderated":          true,
	}, models[0]["top_provider"])
	require.Equal(t, map[string]any{
		"temperature": float64(0.2),
		"top_p":       float64(0.9),
	}, models[0]["default_parameters"])
	require.Equal(t, map[string]any{
		"prompt_tokens":     float64(128000),
		"completion_tokens": float64(4000),
	}, models[0]["per_request_limits"])
}

func TestOpenAIModelsRestrictsArchitectureModality(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(
			[]byte(
				`{"data":[{"id":"provider/model","architecture":{"modality":"speech","input_modalities":["text"],"output_modalities":["audio"]}}]}`,
			),
		)
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := modelcatalog.OpenAICompatibleModels(
		context.Background(),
		&model.Channel{BaseURL: server.URL + "/v1"},
		"",
	)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"modality":          "speech",
		"input_modalities":  []string{"text"},
		"output_modalities": []string{"audio"},
	}, models[0]["architecture"])
}

func TestOpenRouterModelItemOmitsPricingAndNullFields(t *testing.T) {
	item, err := modelcatalog.OpenRouterModelItem([]byte(`{
		"id": "provider/model",
		"name": null,
		"context_length": null,
		"architecture": null,
		"pricing": {"prompt": null, "completion": "0.2"},
		"top_provider": {"context_length": null},
		"per_request_limits": null,
		"supported_parameters": null
	}`))
	require.NoError(t, err)
	require.Equal(t, adaptor.ModelCatalogItem{"id": "provider/model"}, item)

	_, err = modelcatalog.OpenRouterModelItem([]byte(`{"id": null}`))
	require.ErrorIs(t, err, modelcatalog.ErrInvalidResponse)
}
