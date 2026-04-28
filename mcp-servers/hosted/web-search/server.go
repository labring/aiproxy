package websearch

import (
	"context"
	// embed static files
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/labring/aiproxy/mcp-servers/hosted/web-search/engine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Configuration templates for the web search server
var configTemplates = map[string]mcpservers.ConfigTemplate{
	// Google Search Configuration
	"google_api_key": {
		Name:        "Google API Key",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "AIzaSyC...",
		Description: "Google Custom Search API key",
	},
	"google_cx": {
		Name:        "Google Search Engine ID",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "017576662512468239146:omuauf_lfve",
		Description: "Google Custom Search Engine ID",
	},

	// Bing Search Configuration
	"bing_api_key": {
		Name:        "Bing API Key",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "1234567890abcdef",
		Description: "Bing Search API key",
	},

	// Common Configuration
	"default_engine": {
		Name:        "Default Search Engine",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "google",
		Description: "Default search engine to use (google, bing, arxiv)",
		Validator: func(value string) error {
			validEngines := []string{"google", "bing", "arxiv", "searchxng", "bingcn"}
			if slices.Contains(validEngines, value) {
				return nil
			}

			return fmt.Errorf(
				"invalid engine: %s, must be one of: %s",
				value,
				strings.Join(validEngines, ", "),
			)
		},
	},
	"max_results": {
		Name:        "Max Results",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "10",
		Description: "Maximum number of search results to return (default: 10)",
		Validator: func(value string) error {
			// Validate it's a number between 1 and 50
			var num int
			if _, err := fmt.Sscanf(value, "%d", &num); err != nil {
				return errors.New("must be a number")
			}

			if num < 1 || num > 50 {
				return errors.New("must be between 1 and 50")
			}

			return nil
		},
	},
	"searchxng_base_url": {
		Name:        "SearchXNG Base URL",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "https://searchxng.com",
		Description: "Base URL for SearchXNG",
	},
}

// searchQuery represents a search query with parameters
type searchQuery struct {
	Query      string
	MaxResults int
	Type       string // "general", "academic", "news"
}

// NewServer creates a new MCP server for web search
func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	// Create MCP server
	mcpServer := server.NewMCPServer(
		"web-search",
		"1.0.0",
	)

	// Initialize search engines and settings
	engines, defaultEngine, maxResults := initializeEngines(config)

	// Add tools to the server
	addWebSearchTool(mcpServer, engines, defaultEngine, maxResults)
	addMultiSearchTool(mcpServer, engines, maxResults)

	// Add smart search tool if engines are available
	if len(engines) > 0 {
		addSmartSearchTool(mcpServer, engines)
	}

	return mcpServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	mcpServer := server.NewMCPServer("web-search", "1.0.0")
	addWebSearchTool(mcpServer, nil, "", 0)
	addMultiSearchTool(mcpServer, nil, 0)
	addSmartSearchTool(mcpServer, nil)

	return mcpservers.ListServerTools(ctx, mcpServer)
}

// initializeEngines sets up search engines based on configuration
func initializeEngines(config map[string]string) (map[string]engine.Engine, string, int) {
	engines := make(map[string]engine.Engine)

	// Google Search
	if apiKey := config["google_api_key"]; apiKey != "" {
		if cx := config["google_cx"]; cx != "" {
			engines["google"] = engine.NewGoogleEngine(apiKey, cx)
		}
	}

	// Bing Search
	if apiKey := config["bing_api_key"]; apiKey != "" {
		engines["bing"] = engine.NewBingEngine(apiKey)
	}

	// Bing CN Search
	engines["bingcn"] = engine.NewBingCNEngine()

	// Arxiv is always available (no API key required)
	engines["arxiv"] = engine.NewArxivEngine()

	// SearchXNG
	if baseURL := config["searchxng_base_url"]; baseURL != "" {
		engines["searchxng"] = engine.NewSearchXNGEngine(baseURL)
	}

	// Get default settings
	defaultEngine := config["default_engine"]
	if defaultEngine == "" && len(engines) > 0 {
		// Pick first available engine as default
		for name := range engines {
			defaultEngine = name
			break
		}
	}

	maxResults := 10
	if maxStr := config["max_results"]; maxStr != "" {
		maxResults, _ = strconv.Atoi(maxStr)
	}

	return engines, defaultEngine, maxResults
}

// addWebSearchTool adds the basic web search tool to the server
func addWebSearchTool(
	mcpServer *server.MCPServer,
	engines map[string]engine.Engine,
	defaultEngine string,
	maxResults int,
) {
	mcpServer.AddTool(
		mcp.Tool{
			Name:        "web_search",
			Description: "Search the web using various search engines (Google, Bing, Arxiv)",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query",
					},
					"engine": map[string]any{
						"type":        "string",
						"description": "Search engine to use",
						"enum":        getAvailableEngines(engines),
						"default":     defaultEngine,
					},
					"max_results": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results to return",
						"default":     maxResults,
						"minimum":     1,
						"maximum":     50,
					},
					"language": map[string]any{
						"type":        "string",
						"description": "Language code for search results (e.g., 'en', 'zh')",
					},
					"arxiv_category": map[string]any{
						"type":        "string",
						"description": "Arxiv category for academic paper search (e.g., 'cs.AI', 'math.CO')",
					},
				},
				Required: []string{"query"},
			},
		},
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			query, ok := args["query"].(string)
			if !ok || query == "" {
				return nil, errors.New("query is required")
			}

			engineName := defaultEngine
			if e, ok := args["engine"].(string); ok && e != "" {
				engineName = e
			}

			searchEngine, exists := engines[engineName]
			if !exists {
				return nil, fmt.Errorf("search engine '%s' is not available", engineName)
			}

			maxRes := maxResults
			if m, ok := args["max_results"].(float64); ok {
				maxRes = int(m)
			}

			language := ""
			if l, ok := args["language"].(string); ok {
				language = l
			}

			arxivCategory := ""
			if ac, ok := args["arxiv_category"].(string); ok {
				arxivCategory = ac
			}

			// Perform search
			results, err := searchEngine.Search(ctx, engine.SearchQuery{
				Queries:       []string{query},
				MaxResults:    maxRes,
				Language:      language,
				ArxivCategory: arxivCategory,
			})
			if err != nil {
				return nil, fmt.Errorf("search failed: %w", err)
			}

			// Format results
			var formattedResults []map[string]any
			for i, result := range results {
				formattedResults = append(formattedResults, map[string]any{
					"index":   i + 1,
					"title":   result.Title,
					"link":    result.Link,
					"snippet": result.Content,
				})
			}

			response := map[string]any{
				"engine":  engineName,
				"query":   query,
				"count":   len(results),
				"results": formattedResults,
			}

			responseJSON, err := sonic.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(responseJSON)), nil
		},
	)
}

// addMultiSearchTool adds the multi-engine search tool to the server
func addMultiSearchTool(
	mcpServer *server.MCPServer,
	engines map[string]engine.Engine,
	maxResults int,
) {
	mcpServer.AddTool(
		mcp.Tool{
			Name:        "multi_search",
			Description: "Search across multiple search engines simultaneously",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query",
					},
					"engines": map[string]any{
						"type":        "array",
						"description": "List of search engines to use",
						"items": map[string]any{
							"type": "string",
							"enum": getAvailableEngines(engines),
						},
						"default": getAvailableEngines(engines),
					},
					"max_results_per_engine": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results per engine",
						"default":     5,
						"minimum":     1,
						"maximum":     20,
					},
				},
				Required: []string{"query"},
			},
		},
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			query, ok := args["query"].(string)
			if !ok || query == "" {
				return nil, errors.New("query is required")
			}

			engineNames := getAvailableEngines(engines)
			if e, ok := args["engines"].([]any); ok {
				engineNames = []string{}
				for _, eng := range e {
					if engStr, ok := eng.(string); ok {
						engineNames = append(engineNames, engStr)
					}
				}
			}

			if m, ok := args["max_results_per_engine"].(float64); ok {
				maxResults = int(m)
			}

			allResults := make(map[string][]map[string]any)

			for _, engineName := range engineNames {
				searchEngine, exists := engines[engineName]
				if !exists {
					continue
				}

				results, err := searchEngine.Search(ctx, engine.SearchQuery{
					Queries:    []string{query},
					MaxResults: maxResults,
				})
				if err != nil {
					// Log error but continue with other engines
					allResults[engineName] = []map[string]any{
						{"error": err.Error()},
					}
					continue
				}

				var engineResults []map[string]any
				for i, result := range results {
					engineResults = append(engineResults, map[string]any{
						"index":   i + 1,
						"title":   result.Title,
						"link":    result.Link,
						"snippet": result.Content,
					})
				}

				allResults[engineName] = engineResults
			}

			response := map[string]any{
				"query":   query,
				"engines": engineNames,
				"results": allResults,
			}

			responseJSON, err := sonic.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(responseJSON)), nil
		},
	)
}

// addSmartSearchTool adds the smart search tool to the server
func addSmartSearchTool(mcpServer *server.MCPServer, engines map[string]engine.Engine) {
	mcpServer.AddTool(
		mcp.Tool{
			Name:        "smart_search",
			Description: "Intelligently search the web with query optimization and result summarization",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "The user's question or search intent",
					},
					"search_depth": map[string]any{
						"type":        "string",
						"description": "Search depth: 'quick' (1-2 queries), 'normal' (3-5 queries), 'deep' (5-10 queries)",
						"enum":        []string{"quick", "normal", "deep"},
						"default":     "normal",
					},
					"include_academic": map[string]any{
						"type":        "boolean",
						"description": "Whether to include academic papers from Arxiv",
						"default":     false,
					},
				},
				Required: []string{"question"},
			},
		},
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			question, ok := args["question"].(string)
			if !ok || question == "" {
				return nil, errors.New("question is required")
			}

			searchDepth := "normal"
			if d, ok := args["search_depth"].(string); ok {
				searchDepth = d
			}

			includeAcademic := false
			if ia, ok := args["include_academic"].(bool); ok {
				includeAcademic = ia
			}

			// Generate optimized search queries based on the question
			queries := generateSearchQueries(question, searchDepth)

			allResults := []engine.SearchResult{}
			searchSummary := map[string]any{
				"original_question": question,
				"search_queries":    queries,
				"engines_used":      []string{},
			}

			// Execute searches
			for _, q := range queries {
				// Determine which engine to use based on query type
				engineName := determineEngine(q, engines, includeAcademic)
				if engineName == "" {
					continue
				}

				searchEngine := engines[engineName]

				results, err := searchEngine.Search(ctx, engine.SearchQuery{
					Queries:    []string{q.Query},
					MaxResults: q.MaxResults,
				})
				if err == nil {
					allResults = append(allResults, results...)

					enginesUsed, ok := searchSummary["engines_used"].([]string)
					if !ok {
						continue
					}

					if !slices.Contains(enginesUsed, engineName) {
						searchSummary["engines_used"] = append(
							enginesUsed,
							engineName,
						)
					}
				}
			}

			// Remove duplicates
			uniqueResults := removeDuplicates(allResults)

			// Format final response
			var formattedResults []map[string]any
			for i, result := range uniqueResults {
				formattedResults = append(formattedResults, map[string]any{
					"index":   i + 1,
					"title":   result.Title,
					"link":    result.Link,
					"snippet": result.Content,
				})
			}

			response := map[string]any{
				"summary":       searchSummary,
				"total_results": len(uniqueResults),
				"results":       formattedResults,
				"search_time":   time.Now().Format(time.RFC3339),
			}

			responseJSON, err := sonic.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(responseJSON)), nil
		},
	)
}

// getAvailableEngines returns a list of available search engine names
func getAvailableEngines(engines map[string]engine.Engine) []string {
	names := make([]string, 0, len(engines))
	for name := range engines {
		names = append(names, name)
	}

	return names
}

// generateSearchQueries creates search queries based on the user's question and depth
func generateSearchQueries(question, depth string) []searchQuery {
	// Simple query generation logic - in production, this could use AI
	queries := []searchQuery{}

	baseQueries := 1
	switch depth {
	case "quick":
		baseQueries = 1
	case "normal":
		baseQueries = 2
	case "deep":
		baseQueries = 3
	}

	// Generate variations of the question
	queries = append(queries, searchQuery{
		Query:      question,
		MaxResults: 10,
		Type:       "general",
	})

	if baseQueries >= 2 {
		// Add a more specific query
		queries = append(queries, searchQuery{
			Query:      question + " latest news",
			MaxResults: 5,
			Type:       "news",
		})
	}

	if baseQueries >= 3 {
		// Add an academic query
		queries = append(queries, searchQuery{
			Query:      question + " research papers",
			MaxResults: 5,
			Type:       "academic",
		})
	}

	return queries
}

// determineEngine selects the appropriate search engine for a query
func determineEngine(q searchQuery, engines map[string]engine.Engine, includeAcademic bool) string {
	// Simple engine selection logic
	if q.Type == "academic" && includeAcademic {
		if _, ok := engines["arxiv"]; ok {
			return "arxiv"
		}
	}

	// Prefer Google if available
	if _, ok := engines["google"]; ok {
		return "google"
	}

	// Then Bing
	if _, ok := engines["bing"]; ok {
		return "bing"
	}

	// Then Bing CN
	if _, ok := engines["bingcn"]; ok {
		return "bingcn"
	}

	// Then SearchXNG
	if _, ok := engines["searchxng"]; ok {
		return "searchxng"
	}

	// Return first available engine
	for name := range engines {
		return name
	}

	return ""
}

// removeDuplicates removes duplicate search results based on URL
func removeDuplicates(results []engine.SearchResult) []engine.SearchResult {
	seen := make(map[string]bool, len(results))
	unique := []engine.SearchResult{}

	for _, result := range results {
		if !seen[result.Link] {
			seen[result.Link] = true
			unique = append(unique, result)
		}
	}

	return unique
}
