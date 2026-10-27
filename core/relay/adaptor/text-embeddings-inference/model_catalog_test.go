package textembeddingsinference

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestListModelsUsesInstanceInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/info", r.URL.Path)
		require.Equal(t, "Bearer catalog-test-key", r.Header.Get("Authorization"))
		_, err := w.Write([]byte(`{"model_id":"sentence-transformers/example","model_sha":"sha","model_type":"bert","max_input_length":512}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    model.ChannelTypeTextEmbeddingsInference,
		Key:     "catalog-test-key",
		BaseURL: server.URL,
	}
	models, err := (&Adaptor{}).ListModels(context.Background(), channel, "https://unused.invalid")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "sentence-transformers/example", models[0]["id"])
	require.Equal(t, int64(512), models[0]["context_length"])

	channel.BaseURL = ""
	_, err = (&Adaptor{}).ListModels(context.Background(), channel, "https://unused.invalid")
	require.Error(t, err)
}
