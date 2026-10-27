package ali_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/ali"
	"github.com/stretchr/testify/require"
)

func TestListModelsPagesThroughWorkspaceCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/models", r.URL.Path)
		require.Equal(t, "Bearer catalog-test-key", r.Header.Get("Authorization"))
		require.Equal(t, "100", r.URL.Query().Get("page_size"))
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("page_no") {
		case "1":
			_, err := w.Write(
				[]byte(
					`{"output":{"models":[{"model":"qwen-plus","name":"Qwen Plus","description":"Qwen Plus model","features":["function-calling","structured-outputs","web-search"],"capabilities":["TG","VU","Reasoning"],"prices":[{"range_name":"more than 128k","prices":[{"type":"input_token","price":"99"}]},{"range_name":"Default","prices":[{"type":"input_token","price":"12"},{"type":"output_token","price":"36"},{"type":"input_token_cache","price":"1.5"},{"type":"input_token_cache_read","price":"1"}]}],"published_time":"2026-08-02 12:14:32","inference_metadata":{"request_modality":["Text","IMAGE","Audio","File","text"],"response_modality":["Video","3D-Generation",""]},"model_info":{"context_window":128000,"max_output_tokens":8192}}],"total":2}}`,
				),
			)
			require.NoError(t, err)
		case "2":
			_, err := w.Write(
				[]byte(`{"output":{"models":[{"model":"qwen-max","name":"Qwen Max"}],"total":2}}`),
			)
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected page number %q", r.URL.Query().Get("page_no"))
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    model.ChannelTypeAli,
		Key:     "catalog-test-key",
		BaseURL: server.URL,
	}
	models, err := (&ali.Adaptor{}).ListModels(context.Background(), channel, "")
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "qwen-plus", models[0]["id"])
	require.Equal(t, "Qwen Plus", models[0]["name"])
	require.Equal(t, "Qwen Plus model", models[0]["description"])
	require.Equal(t, int64(128000), models[0]["context_length"])
	require.Equal(t, int64(1785672872), models[0]["created"])
	require.Equal(t, map[string]any{
		"input_modalities":  []string{"text", "image", "audio"},
		"output_modalities": []string{"video"},
	}, models[0]["architecture"])
	require.NotContains(t, models[0], "pricing")
	require.Equal(t, map[string]any{
		"max_completion_tokens": int64(8192),
	}, models[0]["top_provider"])
	require.Equal(t, []string{"tools", "structured_outputs"}, models[0]["supported_parameters"])
	require.Equal(t, "qwen-max", models[1]["id"])
}

func TestListModelsOnlyFallsBackForEmbeddingsWithoutResponseModality(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(
			[]byte(
				`{"output":{"total":4,"models":[{"model":"text-embedding-v4","capabilities":["TR"],"inference_metadata":{"request_modality":["Text"],"response_modality":[]}},{"model":"text-embedding-v3","capabilities":["ME"],"inference_metadata":{"request_modality":["Text"],"response_modality":[]}},{"model":"qwen-text","capabilities":["TR"],"inference_metadata":{"request_modality":["Text"],"response_modality":["Text"]}},{"model":"wan-video","capabilities":["VG"],"inference_metadata":{"request_modality":["Text"],"response_modality":[]}}]}}`,
			),
		)
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := (&ali.Adaptor{}).ListModels(context.Background(), &model.Channel{
		BaseURL: server.URL,
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 4)

	for _, index := range []int{0, 1} {
		architecture, ok := models[index]["architecture"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "embeddings", architecture["modality"])
	}

	for _, index := range []int{2, 3} {
		architecture, ok := models[index]["architecture"].(map[string]any)
		require.True(t, ok)
		require.NotContains(t, architecture, "modality")
	}
}

func TestListModelsReadsAllPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageNo, err := strconv.Atoi(r.URL.Query().Get("page_no"))
		require.NoError(t, err)

		start := (pageNo - 1) * 100
		end := min(start+100, 357)

		models := make([]map[string]any, 0, end-start)
		for i := start; i < end; i++ {
			models = append(models, map[string]any{"model": fmt.Sprintf("model-%03d", i)})
		}

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{"total": 357, "models": models},
		}))
	}))
	defer server.Close()

	models, err := (&ali.Adaptor{}).ListModels(context.Background(), &model.Channel{
		Key:     "catalog-test-key",
		BaseURL: server.URL,
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 357)
	require.Equal(t, "model-000", models[0]["id"])
	require.Equal(t, "model-356", models[356]["id"])
}
