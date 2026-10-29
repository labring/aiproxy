package zhipucoding_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/zhipucoding"
	"github.com/stretchr/testify/require"
)

func TestListModelsUsesCodingEndpoint(t *testing.T) {
	apiKey := t.Name()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/coding/paas/v4/models" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Errorf("unexpected authorization header")
		}

		_, err := w.Write(
			[]byte(
				`{"data":[{"id":"glm-5.3","created":123,"shutdown_date":"2030-01-01","title":"ignored","context_size":32768}]}`,
			),
		)
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	channel := &model.Channel{Key: apiKey, BaseURL: server.URL}
	models, err := (&zhipucoding.Adaptor{}).ListModels(context.Background(), channel, server.URL)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "glm-5.3", models[0]["id"])
	require.Equal(t, int64(123), models[0]["created"])
	require.Equal(t, "2030-01-01", models[0]["expiration_date"])
	require.NotContains(t, models[0], "name")
	require.NotContains(t, models[0], "context_length")
}
