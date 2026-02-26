package tavily

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Configuration templates for the Tavily server
var configTemplates = mcpservers.ConfigTemplates{
	"tavily_api_key": {
		Name:        "Tavily API Key",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "tvly-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		Description: "Tavily API key for accessing search services",
	},
	"timeout": {
		Name:        "Request Timeout",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "30",
		Description: "Request timeout in seconds (default: 30)",
		Validator: func(value string) error {
			timeout, err := strconv.Atoi(value)
			if err != nil {
				return errors.New("timeout must be a number")
			}

			if timeout < 1 || timeout > 120 {
				return errors.New("timeout must be between 1 and 120 seconds")
			}

			return nil
		},
	},
}

// Server represents the MCP server for Tavily
type Server struct {
	*server.MCPServer
	apiKey     string
	httpClient *http.Client
	baseURLs   map[string]string
}

// Response types
type Response struct {
	Query             string         `json:"query"`
	FollowUpQuestions []string       `json:"follow_up_questions,omitempty"`
	Answer            string         `json:"answer,omitempty"`
	Images            []any          `json:"images,omitempty"`
	Results           []SearchResult `json:"results"`
}

type SearchResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	Score         float64 `json:"score"`
	PublishedDate string  `json:"published_date,omitempty"`
	RawContent    string  `json:"raw_content,omitempty"`
}

type CrawlResponse struct {
	BaseURL      string        `json:"base_url"`
	Results      []CrawlResult `json:"results"`
	ResponseTime float64       `json:"response_time"`
}

type CrawlResult struct {
	URL        string `json:"url"`
	RawContent string `json:"raw_content"`
}

type MapResponse struct {
	BaseURL      string   `json:"base_url"`
	Results      []string `json:"results"`
	ResponseTime float64  `json:"response_time"`
}

// NewServer creates a new MCP server for Tavily
func NewServer(config, reuse map[string]string) (mcpservers.Server, error) {
	// Get API key from config or environment
	apiKey := config["tavily_api_key"]
	if apiKey == "" {
		apiKey = reuse["tavily_api_key"]
	}

	if apiKey == "" {
		return nil, errors.New("tavily_api_key is required")
	}

	// Set up timeout
	timeout := 30 * time.Second
	if timeoutStr := config["timeout"]; timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: timeout,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"tavily-mcp",
		"0.2.2",
	)

	tavilyServer := &Server{
		MCPServer:  mcpServer,
		apiKey:     apiKey,
		httpClient: httpClient,
		baseURLs: map[string]string{
			"search":  "https://api.tavily.com/search",
			"extract": "https://api.tavily.com/extract",
			"crawl":   "https://api.tavily.com/crawl",
			"map":     "https://api.tavily.com/map",
		},
	}

	// Add all tools
	tavilyServer.addTools()

	return tavilyServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	tavilyServer := &Server{
		MCPServer: server.NewMCPServer("tavily-mcp", "0.2.2"),
	}
	tavilyServer.addTools()

	return mcpservers.ListServerTools(ctx, tavilyServer)
}

// addTools adds all Tavily tools to the server
func (s *Server) addTools() {
	s.addSearchTool()
	s.addExtractTool()
	s.addCrawlTool()
	s.addMapTool()
}

// addSearchTool adds the Tavily search tool
func (s *Server) addSearchTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "tavily-search",
			Description: "A powerful web search tool that provides comprehensive, real-time results using Tavily's AI search engine. Returns relevant web content with customizable parameters for result count, content type, and domain filtering. Ideal for gathering current information, news, and detailed web content analysis.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query",
					},
					"search_depth": map[string]any{
						"type":        "string",
						"enum":        []string{"basic", "advanced"},
						"description": "The depth of the search. It can be 'basic' or 'advanced'",
						"default":     "basic",
					},
					"topic": map[string]any{
						"type":        "string",
						"enum":        []string{"general", "news"},
						"description": "The category of the search. This will determine which of our agents will be used for the search",
						"default":     "general",
					},
					"days": map[string]any{
						"type":        "number",
						"description": "The number of days back from the current date to include in the search results. This specifies the time frame of data to be retrieved. Please note that this feature is only available when using the 'news' search topic",
						"default":     3,
					},
					"time_range": map[string]any{
						"type":        "string",
						"description": "The time range back from the current date to include in the search results. This feature is available for both 'general' and 'news' search topics",
						"enum":        []string{"day", "week", "month", "year", "d", "w", "m", "y"},
					},
					"max_results": map[string]any{
						"type":        "number",
						"description": "The maximum number of search results to return",
						"default":     10,
						"minimum":     5,
						"maximum":     20,
					},
					"include_images": map[string]any{
						"type":        "boolean",
						"description": "Include a list of query-related images in the response",
						"default":     false,
					},
					"include_image_descriptions": map[string]any{
						"type":        "boolean",
						"description": "Include a list of query-related images and their descriptions in the response",
						"default":     false,
					},
					"include_raw_content": map[string]any{
						"type":        "boolean",
						"description": "Include the cleaned and parsed HTML content of each search result",
						"default":     false,
					},
					"include_domains": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "A list of domains to specifically include in the search results, if the user asks to search on specific sites set this to the domain of the site",
						"default":     []any{},
					},
					"exclude_domains": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of domains to specifically exclude, if the user asks to exclude a domain set this to the domain of the site",
						"default":     []any{},
					},
					"country": map[string]any{
						"type":        "string",
						"enum":        getCountryList(),
						"description": "Boost search results from a specific country. This will prioritize content from the selected country in the search results. Available only if topic is general.",
					},
				},
				Required: []string{"query"},
			},
		},
		s.handleSearch,
	)
}

// addExtractTool adds the Tavily extract tool
func (s *Server) addExtractTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "tavily-extract",
			Description: "A powerful web content extraction tool that retrieves and processes raw content from specified URLs, ideal for data collection, content analysis, and research tasks.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"urls": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of URLs to extract content from",
					},
					"extract_depth": map[string]any{
						"type":        "string",
						"enum":        []string{"basic", "advanced"},
						"description": "Depth of extraction - 'basic' or 'advanced', if urls are linkedin use 'advanced' or if explicitly told to use advanced",
						"default":     "basic",
					},
					"include_images": map[string]any{
						"type":        "boolean",
						"description": "Include a list of images extracted from the urls in the response",
						"default":     false,
					},
					"format": map[string]any{
						"type":        "string",
						"enum":        []string{"markdown", "text"},
						"description": "The format of the extracted web page content. markdown returns content in markdown format. text returns plain text and may increase latency.",
						"default":     "markdown",
					},
				},
				Required: []string{"urls"},
			},
		},
		s.handleExtract,
	)
}

// addCrawlTool adds the Tavily crawl tool
func (s *Server) addCrawlTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "tavily-crawl",
			Description: "A powerful web crawler that initiates a structured web crawl starting from a specified base URL. The crawler expands from that point like a tree, following internal links across pages. You can control how deep and wide it goes, and guide it to focus on specific sections of the site.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "The root URL to begin the crawl",
					},
					"max_depth": map[string]any{
						"type":        "integer",
						"description": "Max depth of the crawl. Defines how far from the base URL the crawler can explore.",
						"default":     1,
						"minimum":     1,
					},
					"max_breadth": map[string]any{
						"type":        "integer",
						"description": "Max number of links to follow per level of the tree (i.e., per page)",
						"default":     20,
						"minimum":     1,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Total number of links the crawler will process before stopping",
						"default":     50,
						"minimum":     1,
					},
					"instructions": map[string]any{
						"type":        "string",
						"description": "Natural language instructions for the crawler",
					},
					"select_paths": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Regex patterns to select only URLs with specific path patterns (e.g., /docs/.*, /api/v1.*)",
						"default":     []any{},
					},
					"select_domains": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Regex patterns to select crawling to specific domains or subdomains (e.g., ^docs\\.example\\.com$)",
						"default":     []any{},
					},
					"allow_external": map[string]any{
						"type":        "boolean",
						"description": "Whether to allow following links that go to external domains",
						"default":     false,
					},
					"categories": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string", "enum": getCategoryList()},
						"description": "Filter URLs using predefined categories like documentation, blog, api, etc",
						"default":     []any{},
					},
					"extract_depth": map[string]any{
						"type":        "string",
						"enum":        []string{"basic", "advanced"},
						"description": "Advanced extraction retrieves more data, including tables and embedded content, with higher success but may increase latency",
						"default":     "basic",
					},
					"format": map[string]any{
						"type":        "string",
						"enum":        []string{"markdown", "text"},
						"description": "The format of the extracted web page content. markdown returns content in markdown format. text returns plain text and may increase latency.",
						"default":     "markdown",
					},
				},
				Required: []string{"url"},
			},
		},
		s.handleCrawl,
	)
}

// addMapTool adds the Tavily map tool
func (s *Server) addMapTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "tavily-map",
			Description: "A powerful web mapping tool that creates a structured map of website URLs, allowing you to discover and analyze site structure, content organization, and navigation paths. Perfect for site audits, content discovery, and understanding website architecture.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "The root URL to begin the mapping",
					},
					"max_depth": map[string]any{
						"type":        "integer",
						"description": "Max depth of the mapping. Defines how far from the base URL the crawler can explore",
						"default":     1,
						"minimum":     1,
					},
					"max_breadth": map[string]any{
						"type":        "integer",
						"description": "Max number of links to follow per level of the tree (i.e., per page)",
						"default":     20,
						"minimum":     1,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Total number of links the crawler will process before stopping",
						"default":     50,
						"minimum":     1,
					},
					"instructions": map[string]any{
						"type":        "string",
						"description": "Natural language instructions for the crawler",
					},
					"select_paths": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Regex patterns to select only URLs with specific path patterns (e.g., /docs/.*, /api/v1.*)",
						"default":     []any{},
					},
					"select_domains": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Regex patterns to select crawling to specific domains or subdomains (e.g., ^docs\\.example\\.com$)",
						"default":     []any{},
					},
					"allow_external": map[string]any{
						"type":        "boolean",
						"description": "Whether to allow following links that go to external domains",
						"default":     false,
					},
					"categories": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string", "enum": getCategoryList()},
						"description": "Filter URLs using predefined categories like documentation, blog, api, etc",
						"default":     []any{},
					},
				},
				Required: []string{"url"},
			},
		},
		s.handleMap,
	)
}

// Tool handlers

// handleSearch handles the Tavily search tool
func (s *Server) handleSearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	// Build request parameters
	params := map[string]any{
		"query":   query,
		"api_key": s.apiKey,
	}

	// Add optional parameters
	if searchDepth, ok := args["search_depth"].(string); ok {
		params["search_depth"] = searchDepth
	}

	if topic, ok := args["topic"].(string); ok {
		params["topic"] = topic
		// If country is set, ensure topic is general
		if country, hasCountry := args["country"].(string); hasCountry {
			params["topic"] = "general"
			params["country"] = country
		}
	}

	if days, ok := args["days"].(float64); ok {
		params["days"] = int(days)
	}

	if timeRange, ok := args["time_range"].(string); ok {
		params["time_range"] = timeRange
	}

	if maxResults, ok := args["max_results"].(float64); ok {
		params["max_results"] = int(maxResults)
	}

	if includeImages, ok := args["include_images"].(bool); ok {
		params["include_images"] = includeImages
	}

	if includeImageDescriptions, ok := args["include_image_descriptions"].(bool); ok {
		params["include_image_descriptions"] = includeImageDescriptions
	}

	if includeRawContent, ok := args["include_raw_content"].(bool); ok {
		params["include_raw_content"] = includeRawContent
	}

	if includeDomains, ok := args["include_domains"].([]any); ok {
		domains := make([]string, 0, len(includeDomains))
		for _, domain := range includeDomains {
			if domainStr, ok := domain.(string); ok {
				domains = append(domains, domainStr)
			}
		}

		params["include_domains"] = domains
	}

	if excludeDomains, ok := args["exclude_domains"].([]any); ok {
		domains := make([]string, 0, len(excludeDomains))
		for _, domain := range excludeDomains {
			if domainStr, ok := domain.(string); ok {
				domains = append(domains, domainStr)
			}
		}

		params["exclude_domains"] = domains
	}

	var searchResponse Response

	err := s.makeRequest(ctx, "search", params, &searchResponse)
	if err != nil {
		return nil, fmt.Errorf("tavily search failed: %w", err)
	}

	return mcp.NewToolResultText(s.formatSearchResults(searchResponse)), nil
}

// handleExtract handles the Tavily extract tool
func (s *Server) handleExtract(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	urlsInterface, ok := args["urls"].([]any)
	if !ok {
		return nil, errors.New("urls is required and must be an array")
	}

	var urls []string
	for _, urlInterface := range urlsInterface {
		if urlStr, ok := urlInterface.(string); ok {
			urls = append(urls, urlStr)
		}
	}

	if len(urls) == 0 {
		return nil, errors.New("at least one URL is required")
	}

	// Build request parameters
	params := map[string]any{
		"urls":    urls,
		"api_key": s.apiKey,
	}

	// Add optional parameters
	if extractDepth, ok := args["extract_depth"].(string); ok {
		params["extract_depth"] = extractDepth
	}

	if includeImages, ok := args["include_images"].(bool); ok {
		params["include_images"] = includeImages
	}

	if format, ok := args["format"].(string); ok {
		params["format"] = format
	}

	var extractResponse Response

	err := s.makeRequest(ctx, "extract", params, &extractResponse)
	if err != nil {
		return nil, fmt.Errorf("tavily extract failed: %w", err)
	}

	return mcp.NewToolResultText(s.formatSearchResults(extractResponse)), nil
}

// handleCrawl handles the Tavily crawl tool
func (s *Server) handleCrawl(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, errors.New("url is required")
	}

	// Build request parameters
	params := map[string]any{
		"url":     url,
		"api_key": s.apiKey,
	}

	// Add optional parameters
	if maxDepth, ok := args["max_depth"].(float64); ok {
		params["max_depth"] = int(maxDepth)
	}

	if maxBreadth, ok := args["max_breadth"].(float64); ok {
		params["max_breadth"] = int(maxBreadth)
	}

	if limit, ok := args["limit"].(float64); ok {
		params["limit"] = int(limit)
	}

	if instructions, ok := args["instructions"].(string); ok {
		params["instructions"] = instructions
	}

	if selectPaths, ok := args["select_paths"].([]any); ok {
		paths := make([]string, 0, len(selectPaths))
		for _, path := range selectPaths {
			if pathStr, ok := path.(string); ok {
				paths = append(paths, pathStr)
			}
		}

		params["select_paths"] = paths
	}

	if selectDomains, ok := args["select_domains"].([]any); ok {
		domains := make([]string, 0, len(selectDomains))
		for _, domain := range selectDomains {
			if domainStr, ok := domain.(string); ok {
				domains = append(domains, domainStr)
			}
		}

		params["select_domains"] = domains
	}

	if allowExternal, ok := args["allow_external"].(bool); ok {
		params["allow_external"] = allowExternal
	}

	if categories, ok := args["categories"].([]any); ok {
		cats := make([]string, 0, len(categories))
		for _, category := range categories {
			if catStr, ok := category.(string); ok {
				cats = append(cats, catStr)
			}
		}

		params["categories"] = cats
	}

	if extractDepth, ok := args["extract_depth"].(string); ok {
		params["extract_depth"] = extractDepth
	}

	if format, ok := args["format"].(string); ok {
		params["format"] = format
	}

	var crawlResponse CrawlResponse

	err := s.makeRequest(ctx, "crawl", params, &crawlResponse)
	if err != nil {
		return nil, fmt.Errorf("tavily crawl failed: %w", err)
	}

	return mcp.NewToolResultText(s.formatCrawlResults(crawlResponse)), nil
}

// handleMap handles the Tavily map tool
func (s *Server) handleMap(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, errors.New("url is required")
	}

	// Build request parameters
	params := map[string]any{
		"url":     url,
		"api_key": s.apiKey,
	}

	// Add optional parameters
	if maxDepth, ok := args["max_depth"].(float64); ok {
		params["max_depth"] = int(maxDepth)
	}

	if maxBreadth, ok := args["max_breadth"].(float64); ok {
		params["max_breadth"] = int(maxBreadth)
	}

	if limit, ok := args["limit"].(float64); ok {
		params["limit"] = int(limit)
	}

	if instructions, ok := args["instructions"].(string); ok {
		params["instructions"] = instructions
	}

	if selectPaths, ok := args["select_paths"].([]any); ok {
		paths := make([]string, 0, len(selectPaths))
		for _, path := range selectPaths {
			if pathStr, ok := path.(string); ok {
				paths = append(paths, pathStr)
			}
		}

		params["select_paths"] = paths
	}

	if selectDomains, ok := args["select_domains"].([]any); ok {
		domains := make([]string, 0, len(selectDomains))
		for _, domain := range selectDomains {
			if domainStr, ok := domain.(string); ok {
				domains = append(domains, domainStr)
			}
		}

		params["select_domains"] = domains
	}

	if allowExternal, ok := args["allow_external"].(bool); ok {
		params["allow_external"] = allowExternal
	}

	if categories, ok := args["categories"].([]any); ok {
		cats := make([]string, 0, len(categories))
		for _, category := range categories {
			if catStr, ok := category.(string); ok {
				cats = append(cats, catStr)
			}
		}

		params["categories"] = cats
	}

	var mapResponse MapResponse

	err := s.makeRequest(ctx, "map", params, &mapResponse)
	if err != nil {
		return nil, fmt.Errorf("tavily map failed: %w", err)
	}

	return mcp.NewToolResultText(s.formatMapResults(mapResponse)), nil
}

// makeRequest makes a request to the Tavily API
func (s *Server) makeRequest(
	ctx context.Context,
	endpoint string,
	params map[string]any,
	v any,
) error {
	url, exists := s.baseURLs[endpoint]
	if !exists {
		return fmt.Errorf("unknown endpoint: %s", endpoint)
	}

	jsonData, err := sonic.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errors.New("invalid API key")
	case resp.StatusCode == http.StatusTooManyRequests:
		return errors.New("usage limit exceeded")
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, resp.Status)
	}

	return sonic.ConfigDefault.NewDecoder(resp.Body).Decode(v)
}

// Formatting functions
func (s *Server) formatSearchResults(response Response) string {
	var output []string

	// Include answer if available
	if response.Answer != "" {
		output = append(output, "Answer: "+response.Answer)
	}

	// Format detailed search results
	output = append(output, "Detailed Results:")
	for _, result := range response.Results {
		output = append(output, "\nTitle: "+result.Title)
		output = append(output, "URL: "+result.URL)

		output = append(output, "Content: "+result.Content)
		if result.RawContent != "" {
			output = append(output, "Raw Content: "+result.RawContent)
		}
	}

	// Add images section if available
	if len(response.Images) > 0 {
		output = append(output, "\nImages:")
		for i, image := range response.Images {
			if imageStr, ok := image.(string); ok {
				output = append(output, fmt.Sprintf("\n[%d] URL: %s", i+1, imageStr))
			} else if imageMap, ok := image.(map[string]any); ok {
				if url, ok := imageMap["url"].(string); ok {
					output = append(output, fmt.Sprintf("\n[%d] URL: %s", i+1, url))
					if description, ok := imageMap["description"].(string); ok {
						output = append(output, "   Description: "+description)
					}
				}
			}
		}
	}

	return strings.Join(output, "\n")
}

func (s *Server) formatCrawlResults(response CrawlResponse) string {
	var output []string

	output = append(output, "Crawl Results:")
	output = append(output, "Base URL: "+response.BaseURL)

	output = append(output, "\nCrawled Pages:")
	for i, page := range response.Results {
		output = append(output, fmt.Sprintf("\n[%d] URL: %s", i+1, page.URL))
		if page.RawContent != "" {
			// Truncate content if it's too long
			contentPreview := page.RawContent
			if len(contentPreview) > 200 {
				contentPreview = contentPreview[:200] + "..."
			}

			output = append(output, "Content: "+contentPreview)
		}
	}

	return strings.Join(output, "\n")
}

func (s *Server) formatMapResults(response MapResponse) string {
	output := make([]string, 3+len(response.Results))

	output = append(output, "Site Map Results:")
	output = append(output, "Base URL: "+response.BaseURL)

	output = append(output, "\nMapped Pages:")
	for i, page := range response.Results {
		output = append(output, fmt.Sprintf("\n[%d] URL: %s", i+1, page))
	}

	return strings.Join(output, "\n")
}

// Helper functions for enum values

func getCountryList() []string {
	return []string{
		"afghanistan",
		"albania",
		"algeria",
		"andorra",
		"angola",
		"argentina",
		"armenia",
		"australia",
		"austria",
		"azerbaijan",
		"bahamas",
		"bahrain",
		"bangladesh",
		"barbados",
		"belarus",
		"belgium",
		"belize",
		"benin",
		"bhutan",
		"bolivia",
		"bosnia and herzegovina",
		"botswana",
		"brazil",
		"brunei",
		"bulgaria",
		"burkina faso",
		"burundi",
		"cambodia",
		"cameroon",
		"canada",
		"cape verde",
		"central african republic",
		"chad",
		"chile",
		"china",
		"colombia",
		"comoros",
		"congo",
		"costa rica",
		"croatia",
		"cuba",
		"cyprus",
		"czech republic",
		"denmark",
		"djibouti",
		"dominican republic",
		"ecuador",
		"egypt",
		"el salvador",
		"equatorial guinea",
		"eritrea",
		"estonia",
		"ethiopia",
		"fiji",
		"finland",
		"france",
		"gabon",
		"gambia",
		"georgia",
		"germany",
		"ghana",
		"greece",
		"guatemala",
		"guinea",
		"haiti",
		"honduras",
		"hungary",
		"iceland",
		"india",
		"indonesia",
		"iran",
		"iraq",
		"ireland",
		"israel",
		"italy",
		"jamaica",
		"japan",
		"jordan",
		"kazakhstan",
		"kenya",
		"kuwait",
		"kyrgyzstan",
		"latvia",
		"lebanon",
		"lesotho",
		"liberia",
		"libya",
		"liechtenstein",
		"lithuania",
		"luxembourg",
		"madagascar",
		"malawi",
		"malaysia",
		"maldives",
		"mali",
		"malta",
		"mauritania",
		"mauritius",
		"mexico",
		"moldova",
		"monaco",
		"mongolia",
		"montenegro",
		"morocco",
		"mozambique",
		"myanmar",
		"namibia",
		"nepal",
		"netherlands",
		"new zealand",
		"nicaragua",
		"niger",
		"nigeria",
		"north korea",
		"north macedonia",
		"norway",
		"oman",
		"pakistan",
		"panama",
		"papua new guinea",
		"paraguay",
		"peru",
		"philippines",
		"poland",
		"portugal",
		"qatar",
		"romania",
		"russia",
		"rwanda",
		"saudi arabia",
		"senegal",
		"serbia",
		"singapore",
		"slovakia",
		"slovenia",
		"somalia",
		"south africa",
		"south korea",
		"south sudan",
		"spain",
		"sri lanka",
		"sudan",
		"sweden",
		"switzerland",
		"syria",
		"taiwan",
		"tajikistan",
		"tanzania",
		"thailand",
		"togo",
		"trinidad and tobago",
		"tunisia",
		"turkey",
		"turkmenistan",
		"uganda",
		"ukraine",
		"united arab emirates",
		"united kingdom",
		"united states",
		"uruguay",
		"uzbekistan",
		"venezuela",
		"vietnam",
		"yemen",
		"zambia",
		"zimbabwe",
	}
}

func getCategoryList() []string {
	return []string{
		"Careers",
		"Blog",
		"Documentation",
		"About",
		"Pricing",
		"Community",
		"Developers",
		"Contact",
		"Media",
	}
}
