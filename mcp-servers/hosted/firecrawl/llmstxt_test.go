//nolint:testpackage
package firecrawl

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

const (
	homePage = `{"markdown":"# Example\nWelcome.",` +
		`"json":{"title":"Example Home","description":"The landing page that introduces Example and its main features."},` +
		`"metadata":{"sourceURL":"https://example.com","url":"https://example.com/","statusCode":200,"title":"Example"}}`
	docsPage = `{"markdown":"# Getting started\nInstall it.",` +
		`"json":{"title":"Docs Getting Started","description":"How to install Example and make your first API request."},` +
		`"metadata":{"sourceURL":"https://example.com/docs","url":"https://example.com/docs","statusCode":200}}`
	missingPage = `{"markdown":"Not found","json":{"title":"Missing Page","description":"Nothing here."},` +
		`"metadata":{"sourceURL":"https://example.com/blog","url":"https://example.com/blog","statusCode":404}}`
	// JSON extraction came back empty, so the title and description come from the page metadata
	pricingPage = `{"markdown":"# Pricing\nFree and paid plans.",` +
		`"metadata":{"sourceURL":"https://example.com/pricing","url":"https://example.com/pricing","statusCode":200,` +
		`"title":["Pricing","Pricing | Example"],"description":"Plans and prices for Example."}}`
)

const llmsTxtSummaryFormatJSON = `{
	"type": "json",
	"prompt": "Write a 3-4 word title and a 9-10 word description of this page.",
	"schema": {
		"type": "object",
		"properties": {"title": {"type": "string"}, "description": {"type": "string"}},
		"required": ["title", "description"]
	}
}`

// fakeLLMsTxtAPI serves the Firecrawl API as it behaves after 2026-12-10: /v1/llmstxt answers 410 Gone,
// map returns the given links, and batch scrape status polls get the given responses in order
type fakeLLMsTxtAPI struct {
	*httptest.Server

	mu       sync.Mutex
	links    string
	statuses []string
	pages    map[string]string
	maps     []string
	batches  []string
}

func newFakeLLMsTxtAPI(t *testing.T, links string, statuses ...string) *fakeLLMsTxtAPI {
	t.Helper()

	api := &fakeLLMsTxtAPI{links: links, statuses: statuses, pages: map[string]string{}}
	api.Server = httptest.NewServer(http.HandlerFunc(api.serveHTTP))
	t.Cleanup(api.Close)

	return api
}

func (api *fakeLLMsTxtAPI) serveHTTP(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	body, _ := io.ReadAll(r.Body)

	switch {
	case strings.HasPrefix(r.URL.Path, "/v1/llmstxt"):
		http.Error(w, `{"success":false,"error":"/v1/llmstxt was shut down"}`, http.StatusGone)
	case r.Method == http.MethodPost && r.URL.Path == "/v2/map":
		api.maps = append(api.maps, string(body))
		_, _ = io.WriteString(w, `{"success":true,"links":`+api.links+`}`)
	case r.Method == http.MethodPost && r.URL.Path == "/v2/batch/scrape":
		api.batches = append(api.batches, string(body))
		_, _ = io.WriteString(w, `{"success":true,"id":"batch-1"}`)
	case r.Method == http.MethodGet && r.URL.Path == "/v2/batch/scrape/batch-1" && r.URL.RawQuery != "":
		page, ok := api.pages[r.URL.RawQuery]
		if !ok {
			http.NotFound(w, r)
			return
		}

		_, _ = io.WriteString(w, page)
	case r.Method == http.MethodGet && r.URL.Path == "/v2/batch/scrape/batch-1" && len(api.statuses) > 0:
		status := api.statuses[0]
		api.statuses = api.statuses[1:]

		if status == "" {
			http.Error(w, `{"error":"temporary failure"}`, http.StatusBadGateway)
			return
		}

		_, _ = io.WriteString(w, strings.ReplaceAll(status, "{api}", api.URL))
	default:
		http.NotFound(w, r)
	}
}

func (api *fakeLLMsTxtAPI) requests() (maps, batches []string) {
	api.mu.Lock()
	defer api.mu.Unlock()

	return append([]string(nil), api.maps...), append([]string(nil), api.batches...)
}

func callGenerateLLMsText(t *testing.T, apiURL string, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	srv, err := NewServer(
		map[string]string{"firecrawl_api_key": "fc-test", "firecrawl_api_url": apiURL},
		nil,
	)
	require.NoError(t, err)

	s, ok := srv.(*Server)
	require.True(t, ok)

	result, err := s.handleGenerateLLMsText(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "firecrawl_generate_llmstxt", Arguments: args},
	})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)

	return result
}

func TestGenerateLLMsTextBuildsTheFilesFromMapAndBatchScrape(t *testing.T) {
	t.Parallel()

	api := newFakeLLMsTxtAPI(
		t,
		`[{"url":"https://example.com"},{"url":"https://example.com/docs"},`+
			`{"url":"https://example.com/blog"},{"url":"https://example.com/pricing"}]`,
		"",
		`{"success":true,"status":"scraping","completed":1,"total":4,"data":[`+docsPage+`],`+
			`"next":"{api}/v2/batch/scrape/batch-1?skip=1"}`,
		`{"success":true,"status":"completed","completed":4,"total":4,"data":[`+pricingPage+`,`+docsPage+`],`+
			`"next":"{api}/v2/batch/scrape/batch-1?skip=2"}`,
	)
	api.pages["skip=2"] = `{"success":true,"status":"completed","completed":4,"total":4,` +
		`"data":[` + missingPage + `,` + homePage + `],"next":null}`

	result := callGenerateLLMsText(t, api.URL, map[string]any{
		"url":          "https://example.com",
		"maxUrls":      float64(5),
		"showFullText": true,
	})

	require.False(t, result.IsError, mcp.GetTextFromContent(result.Content[0]))
	require.Equal(t, strings.Join([]string{
		"LLMs.txt content:",
		"",
		"# https://example.com llms.txt",
		"",
		"- [Example Home](https://example.com/): The landing page that introduces Example and its main features.",
		"- [Docs Getting Started](https://example.com/docs): How to install Example and make your first API request.",
		"- [Pricing](https://example.com/pricing): Plans and prices for Example.",
		"",
		"",
		"LLMs-full.txt content:",
		"",
		"# https://example.com llms-full.txt",
		"",
		"## Example Home",
		"# Example",
		"Welcome.",
		"",
		"## Docs Getting Started",
		"# Getting started",
		"Install it.",
		"",
		"## Pricing",
		"# Pricing",
		"Free and paid plans.",
	}, "\n"), mcp.GetTextFromContent(result.Content[0]))

	maps, batches := api.requests()
	require.Len(t, maps, 1)
	require.JSONEq(t, `{
		"url": "https://example.com",
		"includeSubdomains": false,
		"limit": 5,
		"origin": "mcp-server"
	}`, maps[0])
	require.Len(t, batches, 1, "a failed status poll must not start another batch")
	require.JSONEq(t, `{
		"urls": [
			"https://example.com",
			"https://example.com/docs",
			"https://example.com/blog",
			"https://example.com/pricing"
		],
		"formats": ["markdown", `+llmsTxtSummaryFormatJSON+`],
		"origin": "mcp-server"
	}`, batches[0])
}

func TestGenerateLLMsTextDefaultsToTenURLsAndLeavesOutTheFullText(t *testing.T) {
	t.Parallel()

	api := newFakeLLMsTxtAPI(t,
		`[{"url":"https://example.com/docs"}]`,
		`{"success":true,"status":"completed","completed":1,"total":1,"data":[`+docsPage+`]}`,
	)

	result := callGenerateLLMsText(t, api.URL, map[string]any{"url": "https://example.com"})

	require.False(t, result.IsError, mcp.GetTextFromContent(result.Content[0]))
	require.Equal(
		t,
		"LLMs.txt content:\n\n# https://example.com llms.txt\n\n"+
			"- [Docs Getting Started](https://example.com/docs): How to install Example and make your first API request.",
		mcp.GetTextFromContent(result.Content[0]),
	)

	maps, batches := api.requests()
	require.Len(t, maps, 1)
	require.JSONEq(t, `{
		"url": "https://example.com",
		"includeSubdomains": false,
		"limit": 10,
		"origin": "mcp-server"
	}`, maps[0])
	require.Len(t, batches, 1)
	require.JSONEq(t, `{
		"urls": ["https://example.com/docs"],
		"formats": [`+llmsTxtSummaryFormatJSON+`],
		"origin": "mcp-server"
	}`, batches[0])
}

func TestGenerateLLMsTextReportsAFailedBatchScrape(t *testing.T) {
	t.Parallel()

	api := newFakeLLMsTxtAPI(t,
		`[{"url":"https://example.com"}]`,
		`{"success":false,"status":"failed","error":"Insufficient credits"}`,
	)

	result := callGenerateLLMsText(t, api.URL, map[string]any{"url": "https://example.com"})

	require.True(t, result.IsError)
	require.Equal(
		t,
		"LLMs.txt generation failed: Insufficient credits",
		mcp.GetTextFromContent(result.Content[0]),
	)
}
