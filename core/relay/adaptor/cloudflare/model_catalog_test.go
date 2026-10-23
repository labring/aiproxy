package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

func TestListModelsParsesOpenRouterResponseAndPaginates(t *testing.T) {
	const (
		accountID = "0123456789abcdef0123456789abcdef"
		apiToken  = "cloudflare-api-token"
	)
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/client/v4/accounts/"+accountID+"/ai/models/search" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+apiToken {
			t.Errorf("unexpected authorization header")
		}
		if r.URL.Query().Get("format") != "openrouter" || r.URL.Query().Get("per_page") != "100" {
			t.Errorf("unexpected model list query: %s", r.URL.RawQuery)
		}

		currentPage := r.URL.Query().Get("page")
		page++
		var data []map[string]any
		switch currentPage {
		case "1":
			data = make([]map[string]any, 0, cloudflareModelPageSize)
			for i := 0; i < cloudflareModelPageSize; i++ {
				data = append(data, map[string]any{
					"id":   fmt.Sprintf("@cf/test/model-%d", i),
					"name": fmt.Sprintf("Test Model %d", i),
				})
			}
			data[0] = map[string]any{
				"id":                  "@cf/openai/gpt-oss-120b",
				"hugging_face_id":     "openai/gpt-oss-120b",
				"name":                "OpenAI: Gpt Oss 120B",
				"created":             1754389649,
				"context_length":      128000,
				"input_modalities":    []string{"text"},
				"description":         "OpenAI's open-weight models for reasoning and agentic tasks.",
				"provider_only_field": "drop",
			}
		case "2":
			data = []map[string]any{
				{
					"id":   "@cf/qwen/qwq-32b",
					"name": "Qwen: Qwq 32B",
				},
			}
		default:
			t.Errorf("unexpected page: %q", currentPage)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": data,
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Key:     apiToken,
		BaseURL: server.URL + "/client/v4/accounts/" + accountID + "/",
	}
	models, err := (&Adaptor{}).ListModels(context.Background(), channel, server.URL)
	require.NoError(t, err)
	require.Len(t, models, cloudflareModelPageSize+1)
	require.Equal(t, 2, page)
	require.Equal(t, "@cf/openai/gpt-oss-120b", models[0]["id"])
	require.Equal(t, "OpenAI: Gpt Oss 120B", models[0]["name"])
	require.Equal(t, "OpenAI's open-weight models for reasoning and agentic tasks.", models[0]["description"])
	require.Equal(t, int64(128000), models[0]["context_length"])
	require.Equal(t, "@cf/qwen/qwq-32b", models[len(models)-1]["id"])
	require.NotContains(t, models[0], "provider_only_field")
}

func TestListModelsAllowsEmptyCatalogWithZeroPages(t *testing.T) {
	const accountID = "0123456789abcdef0123456789abcdef"
	const apiToken = "cloudflare-api-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "1", r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":[]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := (&Adaptor{}).ListModels(context.Background(), &model.Channel{
		Key:     apiToken,
		BaseURL: server.URL + "/client/v4/accounts/" + accountID + "/",
	}, server.URL)
	require.NoError(t, err)
	require.Empty(t, models)
}

func TestListModelsParsesAccountIDFromAIGatewayURL(t *testing.T) {
	const (
		apiToken   = "cloudflare-api-token"
		accountID  = "0123456789abcdef0123456789abcdef"
		gatewayURL = "https://gateway.ai.cloudflare.com/v1/" + accountID + "/test-gateway/workers-ai"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/client/v4/accounts/"+accountID+"/ai/models/search", r.URL.Path)
		require.Equal(t, "Bearer "+apiToken, r.Header.Get("Authorization"))
		_, err := w.Write([]byte(`{"data":[]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := (&Adaptor{}).ListModels(context.Background(), &model.Channel{
		Key:     apiToken,
		BaseURL: gatewayURL,
	}, server.URL)
	require.NoError(t, err)
	require.Empty(t, models)
}

func TestGetRequestURLUsesAccountScopedBaseURL(t *testing.T) {
	const (
		accountID  = "0123456789abcdef0123456789abcdef"
		channelKey = "cloudflare-api-token"
		baseURL    = "https://api.cloudflare.com/client/v4/accounts/" + accountID + "/"
	)
	channel := &model.Channel{
		Key:     channelKey,
		BaseURL: baseURL,
	}
	testCases := []struct {
		name        string
		mode        mode.Mode
		actualModel string
		wantURL     string
	}{
		{
			name:    "chat",
			mode:    mode.ChatCompletions,
			wantURL: "https://api.cloudflare.com/client/v4/accounts/" + accountID + "/ai/v1/chat/completions",
		},
		{
			name:    "embeddings",
			mode:    mode.Embeddings,
			wantURL: "https://api.cloudflare.com/client/v4/accounts/" + accountID + "/ai/v1/embeddings",
		},
		{
			name:        "model run",
			mode:        mode.Unknown,
			actualModel: "@cf/meta/llama",
			wantURL:     "https://api.cloudflare.com/client/v4/accounts/" + accountID + "/ai/run/@cf/meta/llama",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestMeta := &meta.Meta{Mode: testCase.mode}
			requestMeta.SetChannel(channel)
			requestMeta.ActualModel = testCase.actualModel

			request, err := (&Adaptor{}).GetRequestURL(requestMeta, nil, nil)
			require.NoError(t, err)
			require.Equal(t, http.MethodPost, request.Method)
			require.Equal(t, testCase.wantURL, request.URL)
			require.NotContains(t, request.URL, channelKey)
		})
	}
	properties := (&Adaptor{}).Metadata().ConfigSchema["properties"]
	require.NotContains(t, properties, "account_id")
	require.NotContains(t, properties, "model_catalog_base_url")
}

func TestGetRequestURLRequiresAccountScopedBaseURL(t *testing.T) {
	channel := &model.Channel{Key: "cloudflare-api-token", BaseURL: "https://api.cloudflare.com"}
	requestMeta := &meta.Meta{Mode: mode.ChatCompletions}
	requestMeta.SetChannel(channel)

	_, err := (&Adaptor{}).GetRequestURL(requestMeta, nil, nil)
	require.ErrorContains(t, err, "/client/v4/accounts/{account_id}")
}

func TestGetRequestURLAllowsAIGatewayWithoutAccountID(t *testing.T) {
	const gatewayURL = "https://gateway.ai.cloudflare.com/v1/account/gateway/workers-ai"
	channel := &model.Channel{Key: "cloudflare-api-token", BaseURL: gatewayURL}
	requestMeta := &meta.Meta{Mode: mode.ChatCompletions}
	requestMeta.SetChannel(channel)

	request, err := (&Adaptor{}).GetRequestURL(requestMeta, nil, nil)
	require.NoError(t, err)
	require.Equal(t, gatewayURL+"/v1/chat/completions", request.URL)
	require.Nil(t, (&Adaptor{}).Metadata().ConfigSchema["required"])
}
