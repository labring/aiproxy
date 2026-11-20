//nolint:testpackage
package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewGoogleEngineDefaultBaseURL(t *testing.T) {
	t.Parallel()

	g := NewGoogleEngine("key", "cx", "")
	require.Equal(t, "https://www.googleapis.com/customsearch/v1", g.baseURL)
}

func TestGoogleEngineSearchUsesBaseURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/customsearch/v1" {
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()
		if q.Get("key") != "test-key" || q.Get("cx") != "test-cx" || q.Get("q") != "aiproxy" ||
			q.Get("num") != "3" {
			http.Error(w, "unexpected query: "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(`{"items":[{"title":"t","link":"https://example.com","snippet":"s"}]}`),
		)
	}))
	defer server.Close()

	g := NewGoogleEngine("test-key", "test-cx", server.URL+"/customsearch/v1/")

	results, err := g.Search(t.Context(), SearchQuery{Queries: []string{"aiproxy"}, MaxResults: 3})
	require.NoError(t, err)
	require.Equal(
		t,
		[]SearchResult{{Title: "t", Link: "https://example.com", Content: "s"}},
		results,
	)
}
