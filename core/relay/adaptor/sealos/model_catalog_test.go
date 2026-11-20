package sealos_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/sealos"
	"github.com/stretchr/testify/require"
)

func TestListModelsReadsAIProxyGatewayCatalog(t *testing.T) {
	const apiKey = "aiproxy-gateway-key"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))

		_, err := w.Write(
			[]byte(
				`{"data":[{"id":"gateway-model","object":"model","created":1720000000,"owned_by":"system","shutdown_date":"2027-01-20"}]}`,
			),
		)
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := (&sealos.Adaptor{}).ListModels(context.Background(), &model.Channel{
		Key:     apiKey,
		BaseURL: server.URL + "/v1",
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "gateway-model", models[0]["id"])
	require.Equal(t, int64(1720000000), models[0]["created"])
	require.Equal(t, "2027-01-20", models[0]["expiration_date"])
	require.NotContains(t, models[0], "object")
	require.NotContains(t, models[0], "owned_by")
}
