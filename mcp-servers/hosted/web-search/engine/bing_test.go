//nolint:testpackage
package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBingEngineDefaultBaseURL(t *testing.T) {
	t.Parallel()

	b := NewBingEngine("key", "")
	require.Equal(t, "https://api.bing.microsoft.com/v7.0/search", b.baseURL)
}

func TestBingEngineSearchUsesBaseURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v7.0/search" {
			http.NotFound(w, r)
			return
		}

		if r.Header.Get("Ocp-Apim-Subscription-Key") != "test-key" ||
			r.URL.Query().Get("q") != "aiproxy" || r.URL.Query().Get("count") != "3" {
			http.Error(w, "unexpected request: "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(
				`{"webPages":{"value":[{"name":"t","url":"https://example.com","snippet":"s"}]}}`,
			),
		)
	}))
	defer server.Close()

	b := NewBingEngine("test-key", server.URL+"/v7.0/search")

	results, err := b.Search(t.Context(), SearchQuery{Queries: []string{"aiproxy"}, MaxResults: 3})
	require.NoError(t, err)
	require.Equal(
		t,
		[]SearchResult{{Title: "t", Link: "https://example.com", Content: "s"}},
		results,
	)
}
