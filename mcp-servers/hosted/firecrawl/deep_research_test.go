//nolint:testpackage
package firecrawl

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

// fakeAgentAPI accepts agent runs and answers status polls with the given responses in order
type fakeAgentAPI struct {
	*httptest.Server

	mu       sync.Mutex
	starts   []string
	statuses []string
}

func newFakeAgentAPI(t *testing.T, statuses ...string) *fakeAgentAPI {
	t.Helper()

	api := &fakeAgentAPI{statuses: statuses}
	api.Server = httptest.NewServer(http.HandlerFunc(api.serveHTTP))
	t.Cleanup(api.Close)

	return api
}

func (api *fakeAgentAPI) serveHTTP(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v2/agent":
		body, _ := io.ReadAll(r.Body)
		api.starts = append(api.starts, string(body))
		_, _ = io.WriteString(w, `{"success":true,"id":"job-1"}`)
	case r.Method == http.MethodGet && r.URL.Path == "/v2/agent/job-1" && len(api.statuses) > 0:
		status := api.statuses[0]
		api.statuses = api.statuses[1:]

		if status == "" {
			http.Error(w, `{"error":"temporary failure"}`, http.StatusBadGateway)
			return
		}

		_, _ = io.WriteString(w, status)
	default:
		http.NotFound(w, r)
	}
}

func (api *fakeAgentAPI) startedRuns() []string {
	api.mu.Lock()
	defer api.mu.Unlock()

	return append([]string(nil), api.starts...)
}

func callDeepResearch(t *testing.T, apiURL string, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	srv, err := NewServer(
		map[string]string{"firecrawl_api_key": "fc-test", "firecrawl_api_url": apiURL},
		nil,
	)
	require.NoError(t, err)

	s, ok := srv.(*Server)
	require.True(t, ok)

	result, err := s.handleDeepResearch(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "firecrawl_deep_research", Arguments: args},
	})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)

	return result
}

func TestDeepResearchRunsOnTheAgentAPI(t *testing.T) {
	t.Parallel()

	api := newFakeAgentAPI(t,
		`{"success":true,"status":"processing"}`,
		`{"success":true,"status":"completed","data":{"report":"# Findings\n"}}`,
	)

	result := callDeepResearch(t, api.URL, map[string]any{
		"query":      "How do EVs and gas cars compare on lifetime emissions?",
		"maxCredits": float64(500),
		"maxDepth":   float64(3),
	})

	require.False(t, result.IsError, mcp.GetTextFromContent(result.Content[0]))
	require.Equal(t, "# Findings", mcp.GetTextFromContent(result.Content[0]))

	starts := api.startedRuns()
	require.Len(t, starts, 1)
	require.JSONEq(t, `{
		"prompt": "How do EVs and gas cars compare on lifetime emissions?",
		"schema": {
			"type": "object",
			"properties": {
				"report": {
					"type": "string",
					"description": "A markdown report that answers the question, citing its sources as markdown links"
				}
			},
			"required": ["report"]
		},
		"maxCredits": 500,
		"origin": "mcp-server"
	}`, starts[0])
}

func TestDeepResearchReportsAFailedAgentRun(t *testing.T) {
	t.Parallel()

	api := newFakeAgentAPI(
		t,
		`{"success":true,"status":"failed","error":"Agent reached max credits"}`,
	)

	result := callDeepResearch(t, api.URL, map[string]any{"query": "What is Firecrawl?"})

	require.True(t, result.IsError)
	require.Equal(
		t,
		"Deep research failed: Agent reached max credits",
		mcp.GetTextFromContent(result.Content[0]),
	)
}

func TestDeepResearchRetriesStatusPollsWithoutStartingAnotherRun(t *testing.T) {
	t.Parallel()

	api := newFakeAgentAPI(t, "", `{"success":true,"status":"completed","data":{"report":"Done"}}`)

	result := callDeepResearch(t, api.URL, map[string]any{"query": "What is Firecrawl?"})

	require.False(t, result.IsError, mcp.GetTextFromContent(result.Content[0]))
	require.Equal(t, "Done", mcp.GetTextFromContent(result.Content[0]))
	require.Len(t, api.startedRuns(), 1)
}
