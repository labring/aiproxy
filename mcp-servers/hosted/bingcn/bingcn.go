package bingcn

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// BingSearchServer represents the MCP server for Bing search
type BingSearchServer struct {
	*server.MCPServer
	searchEngine   *SearchEngine
	webpageFetcher *WebpageFetcher
}

// Configuration templates
var configTemplates = mcpservers.ConfigTemplates{
	"user-agent": {
		Name:        "User Agent",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     DefaultUserAgent,
		Description: "Custom User-Agent string to use for requests",
	},
	"timeout": {
		Name:        "Request Timeout",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "15",
		Description: "Request timeout in seconds (default: 15)",
		Validator: func(value string) error {
			timeout, err := strconv.Atoi(value)
			if err != nil {
				return errors.New("timeout must be a number")
			}

			if timeout < 1 || timeout > 60 {
				return errors.New("timeout must be between 1 and 60 seconds")
			}

			return nil
		},
	},
}

// NewServer creates a new Bing search MCP server
func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	userAgent := config["user-agent"]
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	timeout := 15 * time.Second
	if timeoutStr := config["timeout"]; timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	// Create MCP server
	mcpServer := server.NewMCPServer("bing-cn-search", "1.0.0")

	// Create search engine and webpage fetcher
	searchEngine := NewSearchEngine(userAgent, timeout)
	webpageFetcher := NewWebpageFetcher(userAgent, searchEngine.client)

	bingServer := &BingSearchServer{
		MCPServer:      mcpServer,
		searchEngine:   searchEngine,
		webpageFetcher: webpageFetcher,
	}

	bingServer.addTools()

	return bingServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	bingServer := &BingSearchServer{
		MCPServer: server.NewMCPServer("bing-cn-search", "1.0.0"),
	}
	bingServer.addTools()

	return mcpservers.ListServerTools(ctx, bingServer)
}

// addTools adds the search and fetch tools to the server
func (s *BingSearchServer) addTools() {
	s.addBingSearchTool()
	s.addFetchWebpageTool()
}

// addBingSearchTool adds the Bing search tool
func (s *BingSearchServer) addBingSearchTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "bing_search",
			Description: "使用必应搜索指定的关键词，并返回搜索结果列表，包括标题、链接、摘要和ID",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
					"num_results": map[string]any{
						"type":        "integer",
						"description": "返回的结果数量，默认为5",
						"default":     5,
						"minimum":     1,
						"maximum":     20,
					},
					"language": map[string]any{
						"type":        "string",
						"description": "搜索语言设置，如 'zh-CN', 'en-US'",
						"default":     "zh-CN",
					},
				},
				Required: []string{"query"},
			},
		},
		s.handleBingSearch,
	)
}

// addFetchWebpageTool adds the webpage fetch tool
func (s *BingSearchServer) addFetchWebpageTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "fetch_webpage",
			Description: "根据提供的ID获取对应网页的内容",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"result_id": map[string]any{
						"type":        "string",
						"description": "从bing_search返回的结果ID",
					},
					"max_length": map[string]any{
						"type":        "integer",
						"description": "返回内容的最大长度",
						"default":     8000,
						"minimum":     100,
						"maximum":     50000,
					},
				},
				Required: []string{"result_id"},
			},
		},
		s.handleFetchWebpage,
	)
}

// handleBingSearch handles the Bing search tool
func (s *BingSearchServer) handleBingSearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	numResults := 5
	if nr, ok := args["num_results"].(float64); ok {
		numResults = int(nr)
	}

	language := "zh-CN"
	if lang, ok := args["language"].(string); ok && lang != "" {
		language = lang
	}

	// Perform search using search engine
	results, err := s.searchEngine.Search(ctx, SearchOptions{
		Query:      query,
		NumResults: numResults,
		Language:   language,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Format response
	response := map[string]any{
		"query":    query,
		"language": language,
		"count":    len(results),
		"results":  results,
	}

	responseJSON, err := sonic.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(responseJSON)), nil
}

// handleFetchWebpage handles the webpage fetch tool
func (s *BingSearchServer) handleFetchWebpage(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	resultID, ok := args["result_id"].(string)
	if !ok || resultID == "" {
		return nil, errors.New("result_id is required")
	}

	maxLength := 8000
	if ml, ok := args["max_length"].(float64); ok {
		maxLength = int(ml)
	}

	// Get search result from engine
	result, exists := s.searchEngine.GetSearchResult(resultID)
	if !exists {
		return nil, fmt.Errorf("找不到ID为 %s 的搜索结果", resultID)
	}

	// Fetch webpage content using webpage fetcher
	content, err := s.webpageFetcher.FetchWebpageByResult(ctx, result, maxLength)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch webpage: %w", err)
	}

	// Format response
	response := map[string]any{
		"result_id": resultID,
		"url":       content.URL,
		"title":     content.Title,
		"content":   content.Content,
		"length":    content.Length,
	}

	responseJSON, err := sonic.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(responseJSON)), nil
}
