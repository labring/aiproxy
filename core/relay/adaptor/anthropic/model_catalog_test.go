package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestListModelsDoesNotReturnPartialCatalog(t *testing.T) {
	const key = "catalog-test-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		require.Equal(t, key, r.Header.Get("x-api-key"))
		require.NotEmpty(t, r.Header.Get("anthropic-version"))

		if r.URL.Query().Get("after_id") == "" {
			_, err := w.Write([]byte(`{"data":[{"id":"model-1","display_name":"Model 1","created_at":"2024-01-02T03:04:05Z","type":"model"}],"has_more":true,"last_id":"model-1"}`))
			require.NoError(t, err)
			return
		}

		require.Equal(t, "model-1", r.URL.Query().Get("after_id"))
		http.Error(w, "upstream failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    model.ChannelTypeAnthropic,
		Key:     key,
		BaseURL: server.URL + "/v1",
	}

	models, err := (&Adaptor{}).ListModels(context.Background(), channel, "")
	require.Error(t, err)
	require.Nil(t, models)
}

func TestListModelsValidatesDataArray(t *testing.T) {
	for _, test := range []struct {
		name      string
		body      string
		wantError bool
	}{
		{name: "empty catalog", body: `{"data":[]}`},
		{name: "missing data", body: `{}`, wantError: true},
		{name: "null data", body: `{"data":null}`, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, err := w.Write([]byte(test.body))
				require.NoError(t, err)
			}))
			defer server.Close()

			models, err := (&Adaptor{}).ListModels(context.Background(), &model.Channel{
				Key:     "catalog-test-key",
				BaseURL: server.URL + "/v1",
			}, "")
			if test.wantError {
				require.Error(t, err)
				require.Nil(t, models)
				return
			}
			require.NoError(t, err)
			require.Empty(t, models)
		})
	}
}
