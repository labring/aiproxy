package cohere_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/cohere"
	"github.com/stretchr/testify/require"
)

func TestListModelsFollowsNextPageToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer catalog-test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("page_token") {
		case "":
			_, err := w.Write(
				[]byte(
					`{"models":[{"name":"command","context_length":4096.0,"endpoints":["chat"],"features":["tools"],"sampling_defaults":{"temperature":0.2,"k":40,"p":0.8,"frequency_penalty":0.1,"presence_penalty":0.2,"max_tokens_per_doc":100}}],"next_page_token":"next"}`,
				),
			)
			require.NoError(t, err)
		case "next":
			_, err := w.Write(
				[]byte(
					`{"models":[{"name":"command-r","context_length":1.1,"sampling_defaults":{"max_tokens_per_doc":100}}]}`,
				),
			)
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected page token %q", r.URL.Query().Get("page_token"))
		}
	}))
	defer server.Close()

	models, err := (&cohere.Adaptor{}).ListModels(context.Background(), &model.Channel{
		Type:    model.ChannelTypeCohere,
		Key:     "catalog-test-key",
		BaseURL: server.URL,
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "command", models[0]["id"])
	require.Equal(t, int64(4096), models[0]["context_length"])
	require.Equal(t, map[string]any{
		"temperature":       0.2,
		"top_k":             int64(40),
		"top_p":             0.8,
		"frequency_penalty": 0.1,
		"presence_penalty":  0.2,
	}, models[0]["default_parameters"])
	require.NotContains(t, models[0], "endpoints")
	require.NotContains(t, models[0], "features")
	require.NotContains(t, models[0], "sampling_defaults")
	require.Equal(t, "command-r", models[1]["id"])
	require.NotContains(t, models[1], "context_length")
	require.NotContains(t, models[1], "default_parameters")
}
