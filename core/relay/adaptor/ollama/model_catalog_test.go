package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestListModelsUsesInstanceTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/tags", r.URL.Path)
		_, err := w.Write([]byte(`{"models":[{"name":"llama3:latest","model":"llama3:latest","size":123}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := (&Adaptor{}).ListModels(context.Background(), &model.Channel{
		Type:    model.ChannelTypeOllama,
		BaseURL: server.URL,
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "llama3:latest", models[0]["id"])
}
