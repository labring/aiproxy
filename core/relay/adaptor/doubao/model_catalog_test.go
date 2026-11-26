package doubao_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/doubao"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
	"github.com/stretchr/testify/require"
)

func listModelsFrom(t *testing.T, body string) ([]adaptor.ModelCatalogItem, error) {
	t.Helper()

	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(body))
			require.NoError(t, err)
		}),
	)
	t.Cleanup(server.Close)

	return (&doubao.Adaptor{}).ListModels(context.Background(), &model.Channel{
		Type:    model.ChannelTypeDoubao,
		BaseURL: server.URL,
	}, "")
}

func TestListModelsParsesArkCatalog(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/v3/models", r.URL.Path)
		require.Equal(t, "Bearer ark-test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{
			"object": "list",
			"data": [
				{
					"id": "doubao-seed-2-0-pro-260215",
					"name": "doubao-seed-2-0-pro",
					"created": 1792808954,
					"domain": "VLM",
					"context_window": 262144,
					"max_output_tokens": 32768,
					"modalities": {
						"input_modalities": ["text", "image", "video", "text"],
						"output_modalities": ["text"]
					},
					"features": {
						"tools": {"function_calling": true, "web_search": true},
						"structured_outputs": {"json_object": true, "json_schema": true},
						"thinking": {"types": ["enabled", "disabled"]}
					},
					"status": "Published"
				},
				{
					"id": "doubao-token-limits-fallback",
					"token_limits": {
						"context_window": 131072,
						"max_output_token_length": 16384
					},
					"features": {
						"thinking": {"types": ["disabled"]}
					}
				},
				{
					"id": "doubao-embedding-vision-251215",
					"name": "doubao-embedding-vision",
					"domain": "Embedding",
					"context_window": 131072,
					"modalities": {
						"input_modalities": ["text", "image", "video"],
						"output_modalities": ["embedding"]
					},
					"api_supported": {"embeddings/multimodal": true}
				},
				{
					"id": "doubao-embedding-large-text-250515",
					"domain": "",
					"api_supported": {"embeddings": true},
					"status": "Retiring"
				},
				{
					"id": "doubao-seed3d-2-0-260328",
					"domain": "3DGeneration",
					"modalities": {
						"input_modalities": ["image"],
						"output_modalities": ["3D"]
					}
				}
			]
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    model.ChannelTypeDoubao,
		Key:     "ark-test-key",
		BaseURL: server.URL,
	}

	models, err := (&doubao.Adaptor{}).ListModels(context.Background(), channel, "")
	require.NoError(t, err)
	require.Len(t, models, 5)

	require.Equal(t, "doubao-seed-2-0-pro-260215", models[0]["id"])
	require.Equal(t, "doubao-seed-2-0-pro", models[0]["name"])
	require.Equal(t, int64(1792808954), models[0]["created"])
	require.Equal(t, int64(262144), models[0]["context_length"])
	require.Equal(t, map[string]any{
		"max_completion_tokens": int64(32768),
	}, models[0]["top_provider"])
	require.Equal(t, map[string]any{
		"input_modalities":  []string{"text", "image", "video"},
		"output_modalities": []string{"text"},
	}, models[0]["architecture"])
	require.Equal(
		t,
		[]string{"tools", "structured_outputs", "reasoning"},
		models[0]["supported_parameters"],
	)

	require.Equal(t, int64(131072), models[1]["context_length"])
	require.Equal(t, map[string]any{
		"max_completion_tokens": int64(16384),
	}, models[1]["top_provider"])
	require.NotContains(t, models[1], "supported_parameters")

	require.Equal(t, map[string]any{
		"input_modalities": []string{"text", "image", "video"},
		"modality":         "embeddings",
	}, models[2]["architecture"])

	require.Equal(t, map[string]any{
		"modality": "embeddings",
	}, models[3]["architecture"])

	require.Equal(t, map[string]any{
		"input_modalities": []string{"image"},
	}, models[4]["architecture"])
}

func TestListModelsBaseURLNormalization(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		baseURLSuffix  string
		expectedPath   string
		useDefaultBase bool
	}{
		{
			name:           "default base url",
			baseURLSuffix:  "",
			expectedPath:   "/api/v3/models",
			useDefaultBase: true,
		},
		{
			name:          "root base url",
			baseURLSuffix: "",
			expectedPath:  "/api/v3/models",
		},
		{
			name:          "explicit api v3 base url",
			baseURLSuffix: "/api/v3",
			expectedPath:  "/api/v3/models",
		},
		{
			name:          "explicit api v3 trailing slash",
			baseURLSuffix: "/api/v3/",
			expectedPath:  "/api/v3/models",
		},
		{
			name:          "coding api v3 base url",
			baseURLSuffix: "/api/coding/v3",
			expectedPath:  "/api/coding/v3/models",
		},
		{
			name:          "custom proxy prefix",
			baseURLSuffix: "/ark-proxy",
			expectedPath:  "/ark-proxy/api/v3/models",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotPath = r.URL.Path

					w.Header().Set("Content-Type", "application/json")
					_, err := w.Write([]byte(`{"object":"list","data":[{"id":"doubao-pro"}]}`))
					require.NoError(t, err)
				}),
			)
			defer server.Close()

			channel := &model.Channel{
				Type: model.ChannelTypeDoubao,
				Key:  "ark-test-key",
			}
			defaultBaseURL := ""

			if tc.useDefaultBase {
				defaultBaseURL = server.URL
			} else {
				channel.BaseURL = server.URL + tc.baseURLSuffix
			}

			models, err := (&doubao.Adaptor{}).ListModels(
				context.Background(),
				channel,
				defaultBaseURL,
			)
			require.NoError(t, err)
			require.Equal(t, tc.expectedPath, gotPath)
			require.Len(t, models, 1)
			require.Equal(t, "doubao-pro", models[0]["id"])
		})
	}
}

func TestListModelsRejectsInvalidConfigAndResponses(t *testing.T) {
	t.Parallel()

	_, err := (&doubao.Adaptor{}).ListModels(context.Background(), nil, "")
	require.ErrorIs(t, err, modelcatalog.ErrInvalidConfiguration)

	_, err = (&doubao.Adaptor{}).ListModels(context.Background(), &model.Channel{
		BaseURL: "://bad-url",
	}, "")
	require.ErrorIs(t, err, modelcatalog.ErrInvalidConfiguration)

	badPayloads := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{`},
		{name: "null data", body: `{"data":null}`},
		{name: "empty id", body: `{"data":[{"id":""}]}`},
		{name: "duplicate id", body: `{"data":[{"id":"dup"},{"id":"dup"}]}`},
	}

	for _, tc := range badPayloads {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, err := w.Write([]byte(tc.body))
					require.NoError(t, err)
				}),
			)
			defer server.Close()

			_, err := (&doubao.Adaptor{}).ListModels(context.Background(), &model.Channel{
				BaseURL: server.URL,
			}, "")
			require.ErrorIs(t, err, modelcatalog.ErrInvalidResponse)
		})
	}
}

func TestListModelsParsesDocumentedResponse(t *testing.T) {
	t.Parallel()

	// Response example of the Ark Models API documentation.
	models, err := listModelsFrom(t, `{
		"object": "list",
		"data": [
			{
				"id": "doubao-seed-2-1-pro-260915",
				"object": "model",
				"name": "doubao-seed-2-1-pro",
				"version": "260915",
				"created": 1789472816,
				"domain": "VLM",
				"task_type": ["VisualQuestionAnswering", "TextGeneration"],
				"token_limits": {
					"context_window": 1048576,
					"max_output_token_length": 262144,
					"max_input_token_length": 1048576,
					"max_reasoning_token_length": 262144
				},
				"modalities": {
					"input_modalities": ["text", "image", "video"],
					"output_modalities": ["text"]
				},
				"features": {
					"tools": {"function_calling": true},
					"structured_outputs": {"json_object": true, "json_schema": true},
					"cache": {"prefix_cache": false, "session_cache": false},
					"batch": {"batch_job": false, "batch_chat": false}
				},
				"max_output_tokens": 262144
			}
		]
	}`)
	require.NoError(t, err)
	require.Equal(t, []adaptor.ModelCatalogItem{{
		"id":             "doubao-seed-2-1-pro-260915",
		"name":           "doubao-seed-2-1-pro",
		"created":        int64(1789472816),
		"context_length": int64(1048576),
		"top_provider":   map[string]any{"max_completion_tokens": int64(262144)},
		"architecture": map[string]any{
			"input_modalities":  []string{"text", "image", "video"},
			"output_modalities": []string{"text"},
		},
		"supported_parameters": []string{"tools", "structured_outputs"},
	}}, models)
}

func TestListModelsReadsThinkingCapability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		thinking      string
		wantSupported []string
	}{
		{
			name:          "documented type enabled",
			thinking:      `{"type":"enabled"}`,
			wantSupported: []string{"reasoning"},
		},
		{
			name:     "documented type disabled",
			thinking: `{"type":"disabled"}`,
		},
		{
			name:          "types lists enabled",
			thinking:      `{"types":["enabled","disabled"]}`,
			wantSupported: []string{"reasoning"},
		},
		{
			name:          "types lists auto",
			thinking:      `{"types":["auto"]}`,
			wantSupported: []string{"reasoning"},
		},
		{
			name:     "no thinking capability",
			thinking: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			models, err := listModelsFrom(t, fmt.Sprintf(
				`{"object":"list","data":[{"id":"doubao-pro","features":{"thinking":%s}}]}`,
				tt.thinking,
			))
			require.NoError(t, err)
			require.Len(t, models, 1)

			if len(tt.wantSupported) == 0 {
				require.NotContains(t, models[0], "supported_parameters")

				return
			}

			require.Equal(t, tt.wantSupported, models[0]["supported_parameters"])
		})
	}
}
