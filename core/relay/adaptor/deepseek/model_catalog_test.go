package deepseek_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/deepseek"
	"github.com/stretchr/testify/require"
)

func TestListModelsUsesProviderModelsEndpoint(t *testing.T) {
	apiKey := t.Name()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/models", r.URL.Path)
		require.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(
			[]byte(
				`{"data":[{"id":"deepseek-chat","name":"DeepSeek Chat","context_window":64000}]}`,
			),
		)
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := (&deepseek.Adaptor{}).ListModels(context.Background(), &model.Channel{
		Key:     apiKey,
		BaseURL: server.URL + "/v1",
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "deepseek-chat", models[0]["id"])
	require.Equal(t, "DeepSeek Chat", models[0]["name"])
	require.Equal(t, int64(64000), models[0]["context_length"])
	_, err = json.Marshal(models)
	require.NoError(t, err)
}
