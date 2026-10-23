package azure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestListModelsUsesAzureDataPlaneAPI(t *testing.T) {
	const apiKey = "azure-test-key"
	var versions []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/openai/models" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("api-key") != apiKey || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Azure authentication headers")
		}
		versions = append(versions, r.URL.Query().Get("api-version"))
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{{
				"id":            "gpt-4.1",
				"object":        "model",
				"created":       1720000000,
				"owned_by":      "system",
				"shutdown_date": "2027-01-20",
				"created_at":    1800000000,
				"deprecation":   map[string]any{"inference": 1900000000},
			}},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	for _, test := range []struct {
		key         string
		wantVersion string
	}{
		{key: apiKey, wantVersion: "2024-10-21"},
		{key: apiKey + "|2024-10-21-preview", wantVersion: "2024-10-21-preview"},
	} {
		channel := &model.Channel{Key: test.key, BaseURL: server.URL}
		models, err := (&Adaptor{}).ListModels(context.Background(), channel, "")
		require.NoError(t, err)
		require.Len(t, models, 1)
		require.Equal(t, "gpt-4.1", models[0]["id"])
		require.Equal(t, int64(1720000000), models[0]["created"])
		require.Equal(t, "2027-01-20", models[0]["expiration_date"])
		require.NotContains(t, models[0], "object")
		require.NotContains(t, models[0], "owned_by")
		require.NotContains(t, models[0], "created_at")
		require.NotContains(t, models[0], "deprecation")
		require.Equal(t, test.wantVersion, versions[len(versions)-1])
	}
}
