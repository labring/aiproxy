package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestListModelsFetchesAllPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/models", r.URL.Path)
		require.Equal(t, "1000", r.URL.Query().Get("pageSize"))
		require.Equal(t, "catalog-test-key", r.Header.Get("x-goog-api-key"))
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("pageToken") {
		case "":
			_, err := w.Write([]byte(`{"models":[{"name":"models/gemini-one","displayName":"Gemini One","description":"First model","inputTokenLimit":8192,"outputTokenLimit":1024}],"nextPageToken":"next"}`))
			require.NoError(t, err)
		case "next":
			_, err := w.Write([]byte(`{"models":[{"name":"models/gemini-two"}]}`))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected page token %q", r.URL.Query().Get("pageToken"))
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    model.ChannelTypeGoogleGemini,
		Key:     "catalog-test-key",
		BaseURL: server.URL,
	}

	models, err := (&Adaptor{}).ListModels(context.Background(), channel, "")
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "gemini-one", models[0]["id"])
	require.Equal(t, "Gemini One", models[0]["name"])
	require.Equal(t, "First model", models[0]["description"])
	require.Equal(t, int64(8192), models[0]["context_length"])
	require.Equal(t, map[string]any{"max_completion_tokens": int64(1024)}, models[0]["top_provider"])
	require.Equal(t, "gemini-two", models[1]["id"])

	data, err := json.Marshal(models)
	require.NoError(t, err)
	require.NotContains(t, string(data), "supportedGenerationMethods")
}
