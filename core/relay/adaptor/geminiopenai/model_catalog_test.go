package geminiopenai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestListModelsUsesNativeGeminiEndpoint(t *testing.T) {
	const apiKey = "gemini-test-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1beta/models" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != apiKey || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Gemini authentication headers")
		}
		_, err := w.Write([]byte(`{"models":[{"name":"models/gemini-2.5-flash"}]}`))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	baseURL := server.URL + "/v1beta/openai"
	channel := &model.Channel{Key: apiKey, BaseURL: baseURL}
	models, err := (&Adaptor{}).ListModels(context.Background(), channel, baseURL)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "gemini-2.5-flash", models[0]["id"])
}
