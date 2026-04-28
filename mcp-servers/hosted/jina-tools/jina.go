package jinatools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Configuration templates for the Jina AI server
var configTemplates = mcpservers.ConfigTemplates{
	"jina_api_key": {
		Name:        "Jina API Key",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "jina_xxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		Description: "Jina AI API key for enhanced features (optional)",
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

// JinaServer represents the MCP server for Jina AI tools
type JinaServer struct {
	*server.MCPServer
	apiKey     string
	httpClient *http.Client
}

// JinaReaderRequest represents the request structure for Jina Reader
type JinaReaderRequest struct {
	URL string `json:"url"`
}

// JinaReaderResponse represents the response structure from Jina Reader
type JinaReaderResponse struct {
	Data struct {
		Content string `json:"content"`
	} `json:"data"`
}

// JinaSearchResult represents a single search result
type JinaSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Date        string `json:"date,omitempty"`
}

// JinaSearchResponse represents the response structure from Jina Search
type JinaSearchResponse struct {
	Data []JinaSearchResult `json:"data"`
}

// JinaFactCheckRequest represents the request structure for Jina Fact Check
type JinaFactCheckRequest struct {
	Statement string `json:"statement"`
	Deepdive  bool   `json:"deepdive"`
}

// NewServer creates a new MCP server for Jina AI tools
func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	// Get API key from config or environment
	apiKey := config["jina_api_key"]
	if apiKey == "" {
		return nil, errors.New("jina_api_key is required")
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
		"jina-mcp-tools",
		"1.0.3",
	)

	jinaServer := &JinaServer{
		MCPServer:  mcpServer,
		apiKey:     apiKey,
		httpClient: httpClient,
	}

	// Add tools
	jinaServer.addJinaReaderTool()
	jinaServer.addJinaSearchTool()
	jinaServer.addJinaFactCheckTool()

	return jinaServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	jinaServer := &JinaServer{
		MCPServer: server.NewMCPServer("jina-mcp-tools", "1.0.3"),
	}
	jinaServer.addJinaReaderTool()
	jinaServer.addJinaSearchTool()
	jinaServer.addJinaFactCheckTool()

	return mcpservers.ListServerTools(ctx, jinaServer)
}

// createHeaders creates HTTP headers with optional API key
func (s *JinaServer) createHeaders(baseHeaders map[string]string) map[string]string {
	headers := maps.Clone(baseHeaders)

	// Add authorization if API key is available
	if s.apiKey != "" {
		headers["Authorization"] = "Bearer " + s.apiKey
	}

	return headers
}

// addJinaReaderTool adds the Jina Reader tool
func (s *JinaServer) addJinaReaderTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "jina_reader",
			Description: "Read and extract content from web pages using Jina AI's powerful web reader",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "URL of the webpage to read and extract content from",
						"format":      "uri",
					},
					"format": map[string]any{
						"type":        "string",
						"description": "Output format for the extracted content",
						"enum": []string{
							"Default",
							"Markdown",
							"HTML",
							"Text",
							"Screenshot",
							"Pageshot",
						},
						"default": "Markdown",
					},
					"withLinks": map[string]any{
						"type":        "boolean",
						"description": "Include links in the extracted content",
						"default":     false,
					},
					"withImages": map[string]any{
						"type":        "boolean",
						"description": "Include images in the extracted content",
						"default":     false,
					},
				},
				Required: []string{"url"},
			},
		},
		s.handleJinaReader,
	)
}

// addJinaSearchTool adds the Jina Search tool
func (s *JinaServer) addJinaSearchTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "jina_search",
			Description: "Search the web for information using Jina AI's semantic search engine",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query to find information on the web",
						"minLength":   1,
					},
					"count": map[string]any{
						"type":        "integer",
						"description": "Number of search results to return",
						"default":     5,
						"minimum":     1,
						"maximum":     20,
					},
					"returnFormat": map[string]any{
						"type":        "string",
						"description": "Format of the returned search results",
						"enum":        []string{"markdown", "text", "html"},
						"default":     "markdown",
					},
				},
				Required: []string{"query"},
			},
		},
		s.handleJinaSearch,
	)
}

// addJinaFactCheckTool adds the Jina Fact Check tool
func (s *JinaServer) addJinaFactCheckTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "jina_fact_check",
			Description: "Verify the factuality of statements using Jina AI's fact-checking capability",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"statement": map[string]any{
						"type":        "string",
						"description": "Statement to fact-check for accuracy",
						"minLength":   1,
					},
					"deepdive": map[string]any{
						"type":        "boolean",
						"description": "Enable deep analysis with more comprehensive research",
						"default":     false,
					},
				},
				Required: []string{"statement"},
			},
		},
		s.handleJinaFactCheck,
	)
}

// handleJinaReader handles the Jina Reader tool
func (s *JinaServer) handleJinaReader(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		return nil, errors.New("url is required")
	}

	// Validate URL
	if _, err := url.Parse(urlStr); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	format := "Markdown"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	withLinks := false
	if wl, ok := args["withLinks"].(bool); ok {
		withLinks = wl
	}

	withImages := false
	if wi, ok := args["withImages"].(bool); ok {
		withImages = wi
	}

	// Create headers
	headers := s.createHeaders(map[string]string{
		"Content-Type":          "application/json",
		"Accept":                "application/json",
		"X-With-Links-Summary":  strconv.FormatBool(withLinks),
		"X-With-Images-Summary": strconv.FormatBool(withImages),
		"X-Return-Format":       strings.ToLower(format),
	})

	// Create request body
	reqBody := JinaReaderRequest{
		URL: urlStr,
	}

	jsonBody, err := sonic.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://r.jina.ai/",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	// Make request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	// Parse response
	var jinaResp JinaReaderResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&jinaResp); err != nil {
		// If parsing fails, return raw response
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return mcp.NewToolResultText(jinaResp.Data.Content), nil
}

// handleJinaSearch handles the Jina Search tool
func (s *JinaServer) handleJinaSearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	count := 5
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}

	returnFormat := "markdown"
	if rf, ok := args["returnFormat"].(string); ok && rf != "" {
		returnFormat = rf
	}

	// Create headers
	headers := s.createHeaders(map[string]string{
		"Accept":         "application/json",
		"X-Respond-With": "no-content",
	})

	// Encode query
	encodedQuery := url.QueryEscape(query)
	requestURL := "https://s.jina.ai/?q=" + encodedQuery

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	// Make request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	// Parse response
	var searchResp JinaSearchResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Limit results to requested count
	results := searchResp.Data
	if count > 0 && len(results) > count {
		results = results[:count]
	}

	// Format output based on returnFormat
	var formattedOutput string
	switch returnFormat {
	case "markdown":
		var parts []string
		for i, result := range results {
			part := fmt.Sprintf("%d. **%s**\n   %s\n   %s",
				i+1,
				getStringOrDefault(result.Title, "Untitled"),
				result.URL,
				result.Description)
			if result.Date != "" {
				part += "\n   Date: " + result.Date
			}

			parts = append(parts, part)
		}

		formattedOutput = strings.Join(parts, "\n\n")

	case "html":
		var parts []string
		for _, result := range results {
			part := fmt.Sprintf(`<li><strong>%s</strong><br>
           <a href="%s">%s</a><br>
           %s<br>
           %s</li>`,
				getStringOrDefault(result.Title, "Untitled"),
				result.URL,
				result.URL,
				result.Description,
				func() string {
					if result.Date != "" {
						return "Date: " + result.Date
					}
					return ""
				}())
			parts = append(parts, part)
		}

		formattedOutput = fmt.Sprintf("<ol>%s</ol>", strings.Join(parts, ""))

	default: // text
		var parts []string
		for i, result := range results {
			part := fmt.Sprintf("%d. %s\n   %s\n   %s",
				i+1,
				getStringOrDefault(result.Title, "Untitled"),
				result.URL,
				result.Description)
			if result.Date != "" {
				part += "\n   Date: " + result.Date
			}

			parts = append(parts, part)
		}

		formattedOutput = strings.Join(parts, "\n\n")
	}

	return mcp.NewToolResultText(formattedOutput), nil
}

// handleJinaFactCheck handles the Jina Fact Check tool
func (s *JinaServer) handleJinaFactCheck(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	statement, ok := args["statement"].(string)
	if !ok || statement == "" {
		return nil, errors.New("statement is required")
	}

	deepdive := false
	if dd, ok := args["deepdive"].(bool); ok {
		deepdive = dd
	}

	// Create headers
	headers := s.createHeaders(map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	})

	// Create request body
	reqBody := JinaFactCheckRequest{
		Statement: statement,
		Deepdive:  deepdive,
	}

	jsonBody, err := sonic.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://g.jina.ai/",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	// Make request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	// Parse and format response
	var jsonData any
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&jsonData); err != nil {
		// If JSON parsing fails, return raw response
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	formattedJSON, err := sonic.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(formattedJSON)), nil
}

// getStringOrDefault returns the string value or a default if empty
func getStringOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
