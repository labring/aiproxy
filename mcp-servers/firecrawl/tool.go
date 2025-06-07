package firecrawl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/mark3labs/mcp-go/mcp"
)

// addSearchTool adds the search tool
func (s *FirecrawlServer) addSearchTool() {
	searchTool := mcp.Tool{
		Name: "firecrawl_search",
		Description: `Search the web and optionally extract content from search results.

**Best for:** Finding specific information across multiple websites, when you don't know which website has the information; when you need the most relevant content for a query.
**Not recommended for:** When you already know which website to scrape (use scrape); when you need comprehensive coverage of a single website (use map or crawl).
**Common mistakes:** Using crawl or map for open-ended questions (use search instead).
**Prompt Example:** "Find the latest research papers on AI published in 2023."
**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_search",
  "arguments": {
    "query": "latest AI research papers 2023",
    "limit": 5,
    "lang": "en",
    "country": "us",
    "scrapeOptions": {
      "formats": ["markdown"],
      "onlyMainContent": true
    }
  }
}
` + "```\n" + `**Returns:** Array of search results (with optional scraped content).`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query string",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of results to return (default: 5)",
				},
				"lang": map[string]any{
					"type":        "string",
					"description": "Language code for search results (default: en)",
				},
				"country": map[string]any{
					"type":        "string",
					"description": "Country code for search results (default: us)",
				},
				"tbs": map[string]any{
					"type":        "string",
					"description": "Time-based search filter",
				},
				"filter": map[string]any{
					"type":        "string",
					"description": "Search filter",
				},
				"scrapeOptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"formats": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
								"enum": []string{"markdown", "html", "rawHtml"},
							},
							"description": "Content formats to extract from search results",
						},
						"onlyMainContent": map[string]any{
							"type":        "boolean",
							"description": "Extract only the main content from results",
						},
						"waitFor": map[string]any{
							"type":        "number",
							"description": "Time in milliseconds to wait for dynamic content",
						},
					},
					"description": "Options for scraping search results",
				},
			},
			Required: []string{"query"},
		},
	}

	s.AddTool(searchTool, s.handleSearch)
}

// handleSearch handles the search tool
func (s *FirecrawlServer) handleSearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	params := SearchParams{
		Query:  query,
		Origin: "mcp-server",
	}

	// Parse optional parameters
	if limit, ok := args["limit"].(float64); ok {
		limitInt := int(limit)
		params.Limit = &limitInt
	}

	if lang, ok := args["lang"].(string); ok {
		params.Lang = lang
	}

	if country, ok := args["country"].(string); ok {
		params.Country = country
	}

	if tbs, ok := args["tbs"].(string); ok {
		params.TBS = tbs
	}

	if filter, ok := args["filter"].(string); ok {
		params.Filter = filter
	}

	// Parse scrape options
	if scrapeOptions, ok := args["scrapeOptions"].(map[string]any); ok {
		scrapeConfig := &ScrapeConfig{}

		if formats, ok := scrapeOptions["formats"].([]any); ok {
			for _, format := range formats {
				if formatStr, ok := format.(string); ok {
					scrapeConfig.Formats = append(scrapeConfig.Formats, formatStr)
				}
			}
		}

		if onlyMainContent, ok := scrapeOptions["onlyMainContent"].(bool); ok {
			scrapeConfig.OnlyMainContent = &onlyMainContent
		}

		if waitFor, ok := scrapeOptions["waitFor"].(float64); ok {
			waitForInt := int(waitFor)
			scrapeConfig.WaitFor = &waitForInt
		}

		params.ScrapeOptions = scrapeConfig
	}

	var response *SearchResponse
	err := s.withRetry(ctx, func() error {
		var err error
		response, err = s.client.Search(ctx, params)
		return err
	}, "search operation")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError("Search failed: " + response.Error), nil
	}

	// Format the results
	var results []string
	for _, result := range response.Data {
		resultText := fmt.Sprintf("URL: %s\nTitle: %s\nDescription: %s",
			getStringOrDefault(result.URL, "No URL"),
			getStringOrDefault(result.Title, "No title"),
			getStringOrDefault(result.Description, "No description"))

		if result.Markdown != "" {
			resultText += "\n\nContent:\n" + result.Markdown
		}

		results = append(results, resultText)
	}

	return mcp.NewToolResultText(trimResponseText(strings.Join(results, "\n\n"))), nil
}

// addExtractTool adds the extract tool
func (s *FirecrawlServer) addExtractTool() {
	extractTool := mcp.Tool{
		Name: "firecrawl_extract",
		Description: `Extract structured information from web pages using LLM capabilities. Supports both cloud AI and self-hosted LLM extraction.

**Best for:** Extracting specific structured data like prices, names, details.
**Not recommended for:** When you need the full content of a page (use scrape); when you're not looking for specific structured data.
**Arguments:**
- urls: Array of URLs to extract information from
- prompt: Custom prompt for the LLM extraction
- systemPrompt: System prompt to guide the LLM
- schema: JSON schema for structured data extraction
- allowExternalLinks: Allow extraction from external links
- enableWebSearch: Enable web search for additional context
- includeSubdomains: Include subdomains in extraction
**Prompt Example:** "Extract the product name, price, and description from these product pages."
**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_extract",
  "arguments": {
    "urls": ["https://example.com/page1", "https://example.com/page2"],
    "prompt": "Extract product information including name, price, and description",
    "systemPrompt": "You are a helpful assistant that extracts product information",
    "schema": {
      "type": "object",
      "properties": {
        "name": { "type": "string" },
        "price": { "type": "number" },
        "description": { "type": "string" }
      },
      "required": ["name", "price"]
    },
    "allowExternalLinks": false,
    "enableWebSearch": false,
    "includeSubdomains": false
  }
}
` + "```\n" + `**Returns:** Extracted structured data as defined by your schema.`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"urls": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "List of URLs to extract information from",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "Prompt for the LLM extraction",
				},
				"systemPrompt": map[string]any{
					"type":        "string",
					"description": "System prompt for LLM extraction",
				},
				"schema": map[string]any{
					"type":        "object",
					"description": "JSON schema for structured data extraction",
				},
				"allowExternalLinks": map[string]any{
					"type":        "boolean",
					"description": "Allow extraction from external links",
				},
				"enableWebSearch": map[string]any{
					"type":        "boolean",
					"description": "Enable web search for additional context",
				},
				"includeSubdomains": map[string]any{
					"type":        "boolean",
					"description": "Include subdomains in extraction",
				},
			},
			Required: []string{"urls"},
		},
	}

	s.AddTool(extractTool, s.handleExtract)
}

// handleExtract handles the extract tool
func (s *FirecrawlServer) handleExtract(
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

	params := ExtractParams{
		URLs:   urls,
		Origin: "mcp-server",
	}

	// Parse optional parameters
	if prompt, ok := args["prompt"].(string); ok {
		params.Prompt = prompt
	}

	if systemPrompt, ok := args["systemPrompt"].(string); ok {
		params.SystemPrompt = systemPrompt
	}

	if schema, ok := args["schema"].(map[string]any); ok {
		params.Schema = schema
	}

	if allowExternalLinks, ok := args["allowExternalLinks"].(bool); ok {
		params.AllowExternalLinks = &allowExternalLinks
	}

	if enableWebSearch, ok := args["enableWebSearch"].(bool); ok {
		params.EnableWebSearch = &enableWebSearch
	}

	if includeSubdomains, ok := args["includeSubdomains"].(bool); ok {
		params.IncludeSubdomains = &includeSubdomains
	}

	var response *ExtractResponse
	err := s.withRetry(ctx, func() error {
		var err error
		response, err = s.client.Extract(ctx, params)
		return err
	}, "extract operation")
	if err != nil {
		// Special handling for self-hosted instance errors
		if strings.Contains(err.Error(), "not supported") {
			return mcp.NewToolResultError(
				"Extraction is not supported by this self-hosted instance. Please ensure LLM support is configured.",
			), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Extraction failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError("Extraction failed: " + response.Error), nil
	}

	resultJSON, err := sonic.MarshalIndent(response.Data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format result: %v", err)), nil
	}

	return mcp.NewToolResultText(trimResponseText(string(resultJSON))), nil
}

// addDeepResearchTool adds the deep research tool
func (s *FirecrawlServer) addDeepResearchTool() {
	researchTool := mcp.Tool{
		Name: "firecrawl_deep_research",
		Description: `Conduct deep web research on a query using intelligent crawling, search, and LLM analysis.

**Best for:** Complex research questions requiring multiple sources, in-depth analysis.
**Not recommended for:** Simple questions that can be answered with a single search; when you need very specific information from a known page (use scrape); when you need results quickly (deep research can take time).
**Arguments:**
- query (string, required): The research question or topic to explore.
- maxDepth (number, optional): Maximum recursive depth for crawling/search (default: 3).
- timeLimit (number, optional): Time limit in seconds for the research session (default: 120).
- maxUrls (number, optional): Maximum number of URLs to analyze (default: 50).
**Prompt Example:** "Research the environmental impact of electric vehicles versus gasoline vehicles."
**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_deep_research",
  "arguments": {
    "query": "What are the environmental impacts of electric vehicles compared to gasoline vehicles?",
    "maxDepth": 3,
    "timeLimit": 120,
    "maxUrls": 50
  }
}
` + "```\n" + `**Returns:** Final analysis generated by an LLM based on research. (data.finalAnalysis); may also include structured activities and sources used in the research process.`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The query to research",
				},
				"maxDepth": map[string]any{
					"type":        "number",
					"description": "Maximum depth of research iterations (1-10)",
				},
				"timeLimit": map[string]any{
					"type":        "number",
					"description": "Time limit in seconds (30-300)",
				},
				"maxUrls": map[string]any{
					"type":        "number",
					"description": "Maximum number of URLs to analyze (1-1000)",
				},
			},
			Required: []string{"query"},
		},
	}

	s.AddTool(researchTool, s.handleDeepResearch)
}

// handleDeepResearch handles the deep research tool
func (s *FirecrawlServer) handleDeepResearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	params := DeepResearchParams{
		Query:  query,
		Origin: "mcp-server",
	}

	// Parse optional parameters
	if maxDepth, ok := args["maxDepth"].(float64); ok {
		maxDepthInt := int(maxDepth)
		params.MaxDepth = &maxDepthInt
	}

	if timeLimit, ok := args["timeLimit"].(float64); ok {
		timeLimitInt := int(timeLimit)
		params.TimeLimit = &timeLimitInt
	}

	if maxURLs, ok := args["maxUrls"].(float64); ok {
		maxURLsInt := int(maxURLs)
		params.MaxURLs = &maxURLsInt
	}

	var response *DeepResearchResponse
	err := s.withRetry(ctx, func() error {
		var err error
		response, err = s.client.DeepResearch(ctx, params)
		return err
	}, "deep research operation")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Deep research failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError("Deep research failed: " + response.Error), nil
	}

	return mcp.NewToolResultText(trimResponseText(response.Data.FinalAnalysis)), nil
}

// addGenerateLLMsTextTool adds the generate LLMs.txt tool
func (s *FirecrawlServer) addGenerateLLMsTextTool() {
	generateTool := mcp.Tool{
		Name: "firecrawl_generate_llmstxt",
		Description: `Generate a standardized llms.txt (and optionally llms-full.txt) file for a given domain. This file defines how large language models should interact with the site.

**Best for:** Creating machine-readable permission guidelines for AI models.
**Not recommended for:** General content extraction or research.
**Arguments:**
- url (string, required): The base URL of the website to analyze.
- maxUrls (number, optional): Max number of URLs to include (default: 10).
- showFullText (boolean, optional): Whether to include llms-full.txt contents in the response.
**Prompt Example:** "Generate an LLMs.txt file for example.com."
**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_generate_llmstxt",
  "arguments": {
    "url": "https://example.com",
    "maxUrls": 20,
    "showFullText": true
  }
}
` + "```\n" + `**Returns:** LLMs.txt file contents (and optionally llms-full.txt).`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to generate LLMs.txt from",
				},
				"maxUrls": map[string]any{
					"type":        "number",
					"description": "Maximum number of URLs to process (1-100, default: 10)",
				},
				"showFullText": map[string]any{
					"type":        "boolean",
					"description": "Whether to show the full LLMs-full.txt in the response",
				},
			},
			Required: []string{"url"},
		},
	}

	s.AddTool(generateTool, s.handleGenerateLLMsText)
}

// handleGenerateLLMsText handles the generate LLMs.txt tool
func (s *FirecrawlServer) handleGenerateLLMsText(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, errors.New("url is required")
	}

	params := GenerateLLMsTextParams{
		URL:    url,
		Origin: "mcp-server",
	}

	// Parse optional parameters
	if maxURLs, ok := args["maxUrls"].(float64); ok {
		maxURLsInt := int(maxURLs)
		params.MaxURLs = &maxURLsInt
	}

	if showFullText, ok := args["showFullText"].(bool); ok {
		params.ShowFullText = &showFullText
	}

	var response *GenerateLLMsTextResponse
	err := s.withRetry(ctx, func() error {
		var err error
		response, err = s.client.GenerateLLMsText(ctx, params)
		return err
	}, "LLMs.txt generation")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("LLMs.txt generation failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError(
			"LLMs.txt generation failed: " + response.Error,
		), nil
	}

	resultText := "LLMs.txt content:\n\n" + response.Data.LLMsText

	if params.ShowFullText != nil && *params.ShowFullText && response.Data.LLMsFullText != "" {
		resultText += "\n\nLLMs-full.txt content:\n\n" + response.Data.LLMsFullText
	}

	return mcp.NewToolResultText(trimResponseText(resultText)), nil
}

// addScrapeTools adds the scrape tool
func (s *FirecrawlServer) addScrapeTools() {
	scrapeTool := mcp.Tool{
		Name: "firecrawl_scrape",
		Description: `Scrape content from a single URL with advanced options.

**Best for:** Single page content extraction, when you know exactly which page contains the information.
**Not recommended for:** Multiple pages (use batch_scrape), unknown page (use search), structured data (use extract).
**Common mistakes:** Using scrape for a list of URLs (use batch_scrape instead).
**Prompt Example:** "Get the content of the page at https://example.com."
**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_scrape",
  "arguments": {
    "url": "https://example.com",
    "formats": ["markdown"]
  }
}
` + "```\n" + `**Returns:** Markdown, HTML, or other formats as specified.`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to scrape",
				},
				"formats": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
						"enum": []string{
							"markdown", "html", "rawHtml", "screenshot",
							"links", "screenshot@fullPage", "extract",
						},
					},
					"default":     []string{"markdown"},
					"description": "Content formats to extract (default: ['markdown'])",
				},
				"onlyMainContent": map[string]any{
					"type":        "boolean",
					"description": "Extract only the main content, filtering out navigation, footers, etc.",
				},
				"includeTags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "HTML tags to specifically include in extraction",
				},
				"excludeTags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "HTML tags to exclude from extraction",
				},
				"waitFor": map[string]any{
					"type":        "number",
					"description": "Time in milliseconds to wait for dynamic content to load",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Maximum time in milliseconds to wait for the page to load",
				},
				"mobile": map[string]any{
					"type":        "boolean",
					"description": "Use mobile viewport",
				},
				"skipTlsVerification": map[string]any{
					"type":        "boolean",
					"description": "Skip TLS certificate verification",
				},
				"removeBase64Images": map[string]any{
					"type":        "boolean",
					"description": "Remove base64 encoded images from output",
				},
			},
			Required: []string{"url"},
		},
	}

	s.AddTool(scrapeTool, s.handleScrape)
}

// handleScrape handles the scrape tool
func (s *FirecrawlServer) handleScrape(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, errors.New("url is required")
	}

	params := ScrapeParams{
		URL:    url,
		Origin: "mcp-server",
	}

	// Parse optional parameters
	if formats, ok := args["formats"].([]any); ok {
		for _, format := range formats {
			if formatStr, ok := format.(string); ok {
				params.Formats = append(params.Formats, formatStr)
			}
		}
	}

	if onlyMainContent, ok := args["onlyMainContent"].(bool); ok {
		params.OnlyMainContent = &onlyMainContent
	}

	if includeTags, ok := args["includeTags"].([]any); ok {
		for _, tag := range includeTags {
			if tagStr, ok := tag.(string); ok {
				params.IncludeTags = append(params.IncludeTags, tagStr)
			}
		}
	}

	if excludeTags, ok := args["excludeTags"].([]any); ok {
		for _, tag := range excludeTags {
			if tagStr, ok := tag.(string); ok {
				params.ExcludeTags = append(params.ExcludeTags, tagStr)
			}
		}
	}

	if waitFor, ok := args["waitFor"].(float64); ok {
		waitForInt := int(waitFor)
		params.WaitFor = &waitForInt
	}

	if timeout, ok := args["timeout"].(float64); ok {
		timeoutInt := int(timeout)
		params.Timeout = &timeoutInt
	}

	if mobile, ok := args["mobile"].(bool); ok {
		params.Mobile = &mobile
	}

	if skipTLS, ok := args["skipTlsVerification"].(bool); ok {
		params.SkipTLSVerification = &skipTLS
	}

	if removeBase64, ok := args["removeBase64Images"].(bool); ok {
		params.RemoveBase64Images = &removeBase64
	}

	var response *ScrapeResponse
	err := s.withRetry(ctx, func() error {
		var err error
		response, err = s.client.ScrapeURL(ctx, params)
		return err
	}, "scrape operation")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Scraping failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError("Scraping failed: " + response.Error), nil
	}

	// Format content based on requested formats
	var contentParts []string

	if len(params.Formats) == 0 {
		params.Formats = []string{"markdown"}
	}

	for _, format := range params.Formats {
		switch format {
		case "markdown":
			if response.Markdown != "" {
				contentParts = append(contentParts, response.Markdown)
			}
		case "html":
			if response.HTML != "" {
				contentParts = append(contentParts, response.HTML)
			}
		case "rawHtml":
			if response.RawHTML != "" {
				contentParts = append(contentParts, response.RawHTML)
			}
		case "links":
			if len(response.Links) > 0 {
				contentParts = append(contentParts, strings.Join(response.Links, "\n"))
			}
		case "screenshot":
			if response.Screenshot != "" {
				contentParts = append(contentParts, response.Screenshot)
			}
		case "extract":
			if response.Extract != nil {
				extractJSON, _ := sonic.MarshalIndent(response.Extract, "", "  ")
				contentParts = append(contentParts, string(extractJSON))
			}
		}
	}

	content := strings.Join(contentParts, "\n\n")
	if content == "" {
		content = "No content available"
	}

	return mcp.NewToolResultText(trimResponseText(content)), nil
}

// addMapTool adds the map tool
func (s *FirecrawlServer) addMapTool() {
	mapTool := mcp.Tool{
		Name: "firecrawl_map",
		Description: `Map a website to discover all indexed URLs on the site.

**Best for:** Discovering URLs on a website before deciding what to scrape; finding specific sections of a website.
**Not recommended for:** When you already know which specific URL you need (use scrape or batch_scrape); when you need the content of the pages (use scrape after mapping).
**Common mistakes:** Using crawl to discover URLs instead of map.
**Prompt Example:** "List all URLs on example.com."
**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_map",
  "arguments": {
    "url": "https://example.com"
  }
}
` + "```\n" + `**Returns:** Array of URLs found on the site.`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Starting URL for URL discovery",
				},
				"search": map[string]any{
					"type":        "string",
					"description": "Optional search term to filter URLs",
				},
				"ignoreSitemap": map[string]any{
					"type":        "boolean",
					"description": "Skip sitemap.xml discovery and only use HTML links",
				},
				"sitemapOnly": map[string]any{
					"type":        "boolean",
					"description": "Only use sitemap.xml for discovery, ignore HTML links",
				},
				"includeSubdomains": map[string]any{
					"type":        "boolean",
					"description": "Include URLs from subdomains in results",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of URLs to return",
				},
			},
			Required: []string{"url"},
		},
	}

	s.AddTool(mapTool, s.handleMap)
}

// handleMap handles the map tool
func (s *FirecrawlServer) handleMap(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, errors.New("url is required")
	}

	params := MapParams{
		URL:    url,
		Origin: "mcp-server",
	}

	// Parse optional parameters
	if search, ok := args["search"].(string); ok {
		params.Search = search
	}

	if ignoreSitemap, ok := args["ignoreSitemap"].(bool); ok {
		params.IgnoreSitemap = &ignoreSitemap
	}

	if sitemapOnly, ok := args["sitemapOnly"].(bool); ok {
		params.SitemapOnly = &sitemapOnly
	}

	if includeSubdomains, ok := args["includeSubdomains"].(bool); ok {
		params.IncludeSubdomains = &includeSubdomains
	}

	if limit, ok := args["limit"].(float64); ok {
		limitInt := int(limit)
		params.Limit = &limitInt
	}

	var response *MapResponse
	err := s.withRetry(ctx, func() error {
		var err error
		response, err = s.client.MapURL(ctx, params)
		return err
	}, "map operation")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Mapping failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError("Mapping failed: " + response.Error), nil
	}

	if len(response.Links) == 0 {
		return mcp.NewToolResultText("No links found"), nil
	}

	return mcp.NewToolResultText(trimResponseText(strings.Join(response.Links, "\n"))), nil
}

// addCrawlTools adds crawl and status check tools
func (s *FirecrawlServer) addCrawlTools() {
	// Add crawl tool
	crawlTool := mcp.Tool{
		Name: "firecrawl_crawl",
		Description: `Starts an asynchronous crawl job on a website and extracts content from all pages.

**Best for:** Extracting content from multiple related pages, when you need comprehensive coverage.
**Not recommended for:** Extracting content from a single page (use scrape); when token limits are a concern (use map + batch_scrape); when you need fast results (crawling can be slow).
**Warning:** Crawl responses can be very large and may exceed token limits. Limit the crawl depth and number of pages, or use map + batch_scrape for better control.
**Common mistakes:** Setting limit or maxDepth too high (causes token overflow); using crawl for a single page (use scrape instead).
**Prompt Example:** "Get all blog posts from the first two levels of example.com/blog."
**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_crawl",
  "arguments": {
    "url": "https://example.com/blog/*",
    "maxDepth": 2,
    "limit": 100,
    "allowExternalLinks": false,
    "deduplicateSimilarURLs": true
  }
}
` + "```\n" + `**Returns:** Operation ID for status checking; use firecrawl_check_crawl_status to check progress.`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Starting URL for the crawl",
				},
				"excludePaths": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "URL paths to exclude from crawling",
				},
				"includePaths": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Only crawl these URL paths",
				},
				"maxDepth": map[string]any{
					"type":        "number",
					"description": "Maximum link depth to crawl",
				},
				"ignoreSitemap": map[string]any{
					"type":        "boolean",
					"description": "Skip sitemap.xml discovery",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of pages to crawl",
				},
				"allowBackwardLinks": map[string]any{
					"type":        "boolean",
					"description": "Allow crawling links that point to parent directories",
				},
				"allowExternalLinks": map[string]any{
					"type":        "boolean",
					"description": "Allow crawling links to external domains",
				},
				"deduplicateSimilarURLs": map[string]any{
					"type":        "boolean",
					"description": "Remove similar URLs during crawl",
				},
				"ignoreQueryParameters": map[string]any{
					"type":        "boolean",
					"description": "Ignore query parameters when comparing URLs",
				},
			},
			Required: []string{"url"},
		},
	}

	s.AddTool(crawlTool, s.handleCrawl)

	// Add status check tool
	statusTool := mcp.Tool{
		Name: "firecrawl_check_crawl_status",
		Description: `Check the status of a crawl job.

**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_check_crawl_status",
  "arguments": {
    "id": "550e8400-e29b-41d4-a716-446655440000"
  }
}
` + "```\n" + `**Returns:** Status and progress of the crawl job, including results if available.`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Crawl job ID to check",
				},
			},
			Required: []string{"id"},
		},
	}

	s.AddTool(statusTool, s.handleCrawlStatus)
}

// handleCrawl handles the crawl tool
func (s *FirecrawlServer) handleCrawl(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, errors.New("url is required")
	}

	params := CrawlParams{
		URL:    url,
		Origin: "mcp-server",
	}

	// Parse optional parameters
	if excludePaths, ok := args["excludePaths"].([]any); ok {
		for _, path := range excludePaths {
			if pathStr, ok := path.(string); ok {
				params.ExcludePaths = append(params.ExcludePaths, pathStr)
			}
		}
	}

	if includePaths, ok := args["includePaths"].([]any); ok {
		for _, path := range includePaths {
			if pathStr, ok := path.(string); ok {
				params.IncludePaths = append(params.IncludePaths, pathStr)
			}
		}
	}

	if maxDepth, ok := args["maxDepth"].(float64); ok {
		maxDepthInt := int(maxDepth)
		params.MaxDepth = &maxDepthInt
	}

	if ignoreSitemap, ok := args["ignoreSitemap"].(bool); ok {
		params.IgnoreSitemap = &ignoreSitemap
	}

	if limit, ok := args["limit"].(float64); ok {
		limitInt := int(limit)
		params.Limit = &limitInt
	}

	if allowBackwardLinks, ok := args["allowBackwardLinks"].(bool); ok {
		params.AllowBackwardLinks = &allowBackwardLinks
	}

	if allowExternalLinks, ok := args["allowExternalLinks"].(bool); ok {
		params.AllowExternalLinks = &allowExternalLinks
	}

	if deduplicateSimilarURLs, ok := args["deduplicateSimilarURLs"].(bool); ok {
		params.DeduplicateSimilarURLs = &deduplicateSimilarURLs
	}

	if ignoreQueryParameters, ok := args["ignoreQueryParameters"].(bool); ok {
		params.IgnoreQueryParameters = &ignoreQueryParameters
	}

	var response *CrawlResponse
	err := s.withRetry(ctx, func() error {
		var err error
		response, err = s.client.AsyncCrawlURL(ctx, params)
		return err
	}, "crawl operation")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Crawl failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError("Crawl failed: " + response.Error), nil
	}

	message := fmt.Sprintf(
		"Started crawl for %s with job ID: %s. Use firecrawl_check_crawl_status to check progress.",
		url,
		response.ID,
	)
	return mcp.NewToolResultText(trimResponseText(message)), nil
}

// handleCrawlStatus handles the crawl status check tool
func (s *FirecrawlServer) handleCrawlStatus(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, errors.New("id is required")
	}

	response, err := s.client.CheckCrawlStatus(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Status check failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError("Status check failed: " + response.Error), nil
	}

	status := fmt.Sprintf(`Crawl Status:
Status: %s
Progress: %d/%d
Credits Used: %d
Expires At: %s`,
		response.Status,
		response.Completed,
		response.Total,
		response.CreditsUsed,
		response.ExpiresAt)

	if len(response.Data) > 0 {
		status += "\n\nResults:\n" + formatResults(response.Data)
	}

	return mcp.NewToolResultText(trimResponseText(status)), nil
}
