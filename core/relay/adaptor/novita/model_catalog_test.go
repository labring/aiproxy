package novita

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestListModelsUsesDocumentedModelsEndpoint(t *testing.T) {
	const apiKey = "novita-test-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/openai/v1/models", r.URL.Path)
		require.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":[{"id":"deepseek/deepseek-v4.1-flash","title":"DeepSeek V4.1 Flash","created":1792375957,"context_size":1048576,"max_output_tokens":393216,"input_modalities":["text","image"],"output_modalities":["text"],"pricing":{"prompt":{"price_per_m":3000}},"provider_metadata":"drop"}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := (&Adaptor{}).ListModels(context.Background(), &model.Channel{
		Key:     apiKey,
		BaseURL: server.URL + "/v3/openai",
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "deepseek/deepseek-v4.1-flash", models[0]["id"])
	require.Equal(t, "DeepSeek V4.1 Flash", models[0]["name"])
	require.Equal(t, int64(1048576), models[0]["context_length"])
	require.Equal(t, map[string]any{"max_completion_tokens": int64(393216)}, models[0]["top_provider"])
	require.Equal(t, map[string]any{
		"input_modalities":  []string{"text", "image"},
		"output_modalities": []string{"text"},
	}, models[0]["architecture"])
	require.NotContains(t, models[0], "pricing")
	require.NotContains(t, models[0], "provider_metadata")
	_, err = json.Marshal(models)
	require.NoError(t, err)
}
