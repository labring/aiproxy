package mistral_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/mistral"
	"github.com/stretchr/testify/require"
)

func TestListModelsParsesModelCards(t *testing.T) {
	const apiKey = "mistral-test-key"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Errorf("unexpected authorization header")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"id": "mistral-large-latest",
					"capabilities": map[string]bool{
						"completion_chat":  true,
						"function_calling": true,
					},
					"job":                "job-1",
					"root":               "mistral-large",
					"object":             "model",
					"created":            1720000000,
					"owned_by":           "mistral",
					"max_context_length": 128000,
					"aliases":            []string{"mistral-large"},
					"TYPE":               "fine-tuned",
					"archived":           false,
				},
			},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	channel := &model.Channel{Key: apiKey, BaseURL: server.URL + "/v1"}
	models, err := (&mistral.Adaptor{}).ListModels(context.Background(), channel, "")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "mistral-large-latest", models[0]["id"])
	require.Equal(t, "model", models[0]["object"])
	require.Equal(t, json.Number("1720000000"), models[0]["created"])
	require.Equal(t, "mistral", models[0]["owned_by"])
	require.Equal(t, "mistral-large", models[0]["root"])
	require.Equal(t, int64(128000), models[0]["context_length"])
	require.Equal(
		t,
		map[string]bool{"completion_chat": true, "function_calling": true},
		models[0]["capabilities"],
	)
	require.Equal(t, "job-1", models[0]["job"])
	require.Equal(t, []string{"mistral-large"}, models[0]["aliases"])
	require.Equal(t, "fine-tuned", models[0]["type"])
	require.Equal(t, false, models[0]["archived"])
}

func TestListModelsDistinguishesEmptyAndNullCatalogs(t *testing.T) {
	for _, test := range []struct {
		name      string
		body      string
		wantError bool
	}{
		{name: "empty catalog", body: `{"object":"list","data":[]}`},
		{name: "null catalog", body: `{"object":"list","data":null}`, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write([]byte(test.body))
					require.NoError(t, err)
				}),
			)
			defer server.Close()

			models, err := (&mistral.Adaptor{}).ListModels(context.Background(), &model.Channel{
				Key:     "mistral-test-key",
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
