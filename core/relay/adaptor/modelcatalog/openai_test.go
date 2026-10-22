package modelcatalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/stretchr/testify/require"
)

func TestOpenAIModelsMapsDocumentedFieldsToOpenRouterStandard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"data":[{"id":"gpt-test","object":"model","created":1762988221,"owned_by":"system","shutdown_date":"2026-07-23","name":"drop","context_length":128000,"architecture":{"modality":"text"},"provider_metadata":{"secret":"drop"}}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := OpenAIDocumentedModels(context.Background(), &model.Channel{BaseURL: server.URL + "/v1"}, "")
	require.NoError(t, err)
	require.Equal(t, adaptor.ModelCatalogItem{
		"id":              "gpt-test",
		"created":         int64(1762988221),
		"expiration_date": "2026-07-23",
	}, models[0])
}
