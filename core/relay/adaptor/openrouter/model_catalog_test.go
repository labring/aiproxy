package openrouter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openrouter"
	"github.com/stretchr/testify/require"
)

func TestListModelsPaginatesAllModalities(t *testing.T) {
	const apiKey = "openrouter-test-key"

	page := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/models" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Errorf("unexpected authorization header")
		}

		if r.URL.Query().Get("limit") != "500" || r.URL.Query().Get("output_modalities") != "all" {
			t.Errorf("unexpected model list query: %s", r.URL.RawQuery)
		}

		var data []map[string]any
		switch r.URL.Query().Get("offset") {
		case "0":
			page++

			data = make([]map[string]any, 500)
			for i := range data {
				data[i] = map[string]any{"id": fmt.Sprintf("model-%d", i)}
			}
		case "500":
			page++
			data = []map[string]any{{
				"id":             "provider/model",
				"name":           "Provider model",
				"context_length": 32768,
				"architecture": map[string]any{
					"input_modalities":  []string{"text", "image"},
					"output_modalities": []string{"text"},
				},
				"pricing":           map[string]string{"prompt": "0.000001"},
				"provider_metadata": map[string]any{"secret": "drop"},
			}}
		default:
			t.Errorf("unexpected offset: %q", r.URL.Query().Get("offset"))
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	channel := &model.Channel{Key: apiKey, BaseURL: server.URL + "/api/v1"}
	models, err := (&openrouter.Adaptor{}).ListModels(context.Background(), channel, "")
	require.NoError(t, err)
	require.Len(t, models, 501)
	require.Equal(t, 2, page)
	require.Equal(t, "provider/model", models[500]["id"])
	require.Equal(t, "Provider model", models[500]["name"])
	require.NotContains(t, models[500], "pricing")
	require.NotContains(t, models[500], "provider_metadata")

	encoded, err := json.Marshal(models[500])
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, float64(32768), decoded["context_length"])
	architecture, ok := decoded["architecture"].(map[string]any)
	require.True(t, ok)
	require.Equal(
		t,
		[]any{"text", "image"},
		architecture["input_modalities"],
	)
}
