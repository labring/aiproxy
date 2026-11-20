package firecrawl

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/mark3labs/mcp-go/mcp"
)

// addSearchTool adds the search tool
func (s *Server) addSearchTool() {
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
func (s *Server) handleSearch(
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
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	if !response.Success {
		return mcp.NewToolResultError("Search failed: " + response.Error), nil
	}

	// Format the results
	results := make([]string, 0, len(response.Data))
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
func (s *Server) addExtractTool() {
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
	})
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

// jobPollInterval is how often the deep research and LLMs.txt tools check a running job
const jobPollInterval = 2 * time.Second

// deepResearchSchema asks the agent for the report that /v1/deep-research returned as finalAnalysis
var deepResearchSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"report": map[string]any{
			"type":        "string",
			"description": "A markdown report that answers the question, citing its sources as markdown links",
		},
	},
	"required": []string{"report"},
}

// addDeepResearchTool adds the deep research tool
func (s *Server) addDeepResearchTool() {
	researchTool := mcp.Tool{
		Name: "firecrawl_deep_research",
		Description: `Conduct deep web research on a query with the Firecrawl agent, which searches, browses and analyzes the web for you.

**Best for:** Complex research questions requiring multiple sources, in-depth analysis.
**Not recommended for:** Simple questions that can be answered with a single search; when you need very specific information from a known page (use scrape); when you need results quickly (deep research can take time).
**Arguments:**
- query (string, required): The research question or topic to explore.
- maxCredits (number, optional): Maximum credits the research may spend (Firecrawl default: 2500).
**Prompt Example:** "Research the environmental impact of electric vehicles versus gasoline vehicles."
**Usage Example:**
` + "```json\n" + `{
  "name": "firecrawl_deep_research",
  "arguments": {
    "query": "What are the environmental impacts of electric vehicles compared to gasoline vehicles?",
    "maxCredits": 500
  }
}
` + "```\n" + `**Returns:** A markdown report that answers the query, citing its sources.`,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The query to research",
				},
				"maxCredits": map[string]any{
					"type":        "number",
					"description": "Maximum credits the research may spend",
				},
			},
			Required: []string{"query"},
		},
	}

	s.AddTool(researchTool, s.handleDeepResearch)
}

// handleDeepResearch handles the deep research tool. Firecrawl shuts down /v1/deep-research on
// 2026-12-10, so it runs on the agent API: https://docs.firecrawl.dev/migrating-from-deep-research
func (s *Server) handleDeepResearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	params := AgentParams{
		Prompt: query,
		Schema: deepResearchSchema,
		Origin: "mcp-server",
	}

	if maxCredits, ok := args["maxCredits"].(float64); ok {
		maxCreditsInt := int(maxCredits)
		params.MaxCredits = &maxCreditsInt
	}

	// Only the start is retried: a retried start would launch, and bill, a second run.
	var started *AgentResponse

	err := s.withRetry(ctx, func() error {
		var err error

		started, err = s.client.StartAgent(ctx, params)
		return err
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Deep research failed: %v", err)), nil
	}

	if !started.Success || started.ID == "" {
		return mcp.NewToolResultError("Deep research failed: " + started.Error), nil
	}

	status, err := s.waitForAgent(ctx, started.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Deep research failed: %v", err)), nil
	}

	if status.Status != "completed" {
		return mcp.NewToolResultError("Deep research failed: " + status.Error), nil
	}

	report, _ := status.Data["report"].(string)

	return mcp.NewToolResultText(trimResponseText(report)), nil
}

// waitForAgent polls an agent run until it is no longer processing
func (s *Server) waitForAgent(ctx context.Context, id string) (*AgentStatusResponse, error) {
	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()

	for {
		var status *AgentStatusResponse

		err := s.withRetry(ctx, func() error {
			var err error

			status, err = s.client.GetAgentStatus(ctx, id)
			return err
		})
		if err != nil {
			return nil, err
		}

		if status.Status != "processing" {
			return status, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// llmsTxtSummaryFormat asks each page for the title and description that its llms.txt line needs
var llmsTxtSummaryFormat = map[string]any{
	"type":   "json",
	"prompt": "Write a 3-4 word title and a 9-10 word description of this page.",
	"schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":       map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
		},
		"required": []string{"title", "description"},
	},
}

// addGenerateLLMsTextTool adds the generate LLMs.txt tool
func (s *Server) addGenerateLLMsTextTool() {
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

// handleGenerateLLMsText handles the generate LLMs.txt tool. Firecrawl shuts down /v1/llmstxt on
// 2026-12-10, so the files are built from map and batch scrape results:
// https://docs.firecrawl.dev/migrating-from-llmstxt
func (s *Server) handleGenerateLLMsText(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, errors.New("url is required")
	}

	// /v1/llmstxt processed 10 URLs unless told otherwise, while map returns up to 5000
	maxURLs := 10
	if v, ok := args["maxUrls"].(float64); ok {
		maxURLs = int(v)
	}

	showFullText, _ := args["showFullText"].(bool)
	includeSubdomains := false

	var mapped *MapLinksResponse

	err := s.withRetry(ctx, func() error {
		var err error

		mapped, err = s.client.MapLinks(ctx, MapLinksParams{
			URL:               url,
			IncludeSubdomains: &includeSubdomains,
			Limit:             &maxURLs,
			Origin:            "mcp-server",
		})

		return err
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("LLMs.txt generation failed: %v", err)), nil
	}

	if !mapped.Success {
		return mcp.NewToolResultError("LLMs.txt generation failed: " + mapped.Error), nil
	}

	urls := make([]string, 0, len(mapped.Links))
	for _, link := range mapped.Links {
		urls = append(urls, link.URL)
	}

	var pages []ScrapedPage

	if len(urls) > 0 {
		pages, err = s.scrapeLLMsTxtPages(ctx, urls, showFullText)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("LLMs.txt generation failed: %v", err)), nil
		}
	}

	llmsTxt, llmsFullTxt := buildLLMsTxt(url, urls, pages)

	resultText := "LLMs.txt content:\n\n" + llmsTxt

	if showFullText && llmsFullTxt != "" {
		resultText += "\n\nLLMs-full.txt content:\n\n" + llmsFullTxt
	}

	return mcp.NewToolResultText(trimResponseText(resultText)), nil
}

// scrapeLLMsTxtPages batch scrapes the mapped pages and returns every page of the results
func (s *Server) scrapeLLMsTxtPages(
	ctx context.Context,
	urls []string,
	showFullText bool,
) ([]ScrapedPage, error) {
	formats := []any{llmsTxtSummaryFormat}
	if showFullText {
		formats = []any{"markdown", llmsTxtSummaryFormat}
	}

	// Only the start is retried as a unit: a retried poll must not start, and bill, a second batch.
	var started *BatchScrapeResponse

	err := s.withRetry(ctx, func() error {
		var err error

		started, err = s.client.StartBatchScrape(ctx, BatchScrapeParams{
			URLs:    urls,
			Formats: formats,
			Origin:  "mcp-server",
		})

		return err
	})
	if err != nil {
		return nil, err
	}

	if !started.Success || started.ID == "" {
		return nil, jobError("batch scrape did not start", started.Error)
	}

	status, err := s.waitForBatchScrape(ctx, started.ID)
	if err != nil {
		return nil, err
	}

	if status.Status != "completed" {
		return nil, jobError("batch scrape "+status.Status, status.Error)
	}

	// A finished batch scrape pages its results through next, which ends after the last page
	pages := status.Data
	for next := status.Next; next != ""; {
		var page *BatchScrapeStatusResponse

		err := s.withRetry(ctx, func() error {
			var err error

			page, err = s.client.GetBatchScrapeStatus(ctx, started.ID, next)
			return err
		})
		if err != nil {
			return nil, err
		}

		pages = append(pages, page.Data...)

		if page.Next == next {
			break
		}

		next = page.Next
	}

	return pages, nil
}

// waitForBatchScrape polls a batch scrape until it is no longer scraping
func (s *Server) waitForBatchScrape(
	ctx context.Context,
	id string,
) (*BatchScrapeStatusResponse, error) {
	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()

	for {
		var status *BatchScrapeStatusResponse

		err := s.withRetry(ctx, func() error {
			var err error

			status, err = s.client.GetBatchScrapeStatus(ctx, id, "")
			return err
		})
		if err != nil {
			return nil, err
		}

		if status.Status != "scraping" {
			return status, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// jobError returns Firecrawl's error message, or fallback when it sent none
func jobError(fallback, message string) error {
	if message != "" {
		return errors.New(message)
	}

	return errors.New(fallback)
}

// buildLLMsTxt writes llms.txt and llms-full.txt in the format /v1/llmstxt returned. Pages that came
// back with a status of 400 or more are left out, and the rest go back into map order, since batch
// results come back in the order they finish.
func buildLLMsTxt(
	rootURL string,
	urls []string,
	pages []ScrapedPage,
) (llmsTxt, llmsFullTxt string) {
	mapOrder := make(map[string]int, len(urls))
	for i, pageURL := range urls {
		if _, seen := mapOrder[pageURL]; !seen {
			mapOrder[pageURL] = i
		}
	}

	position := func(page ScrapedPage) int {
		if i, ok := mapOrder[firstString(page.Metadata["sourceURL"])]; ok {
			return i
		}

		return len(urls)
	}

	kept := make([]ScrapedPage, 0, len(pages))
	for _, page := range pages {
		if statusCode, _ := page.Metadata["statusCode"].(float64); statusCode >= 400 {
			continue
		}

		kept = append(kept, page)
	}

	slices.SortStableFunc(kept, func(a, b ScrapedPage) int {
		return cmp.Compare(position(a), position(b))
	})

	var index, full strings.Builder

	index.WriteString("# " + rootURL + " llms.txt\n\n")
	full.WriteString("# " + rootURL + " llms-full.txt\n\n")

	for _, page := range kept {
		pageURL := firstString(page.Metadata["url"], page.Metadata["sourceURL"])
		title := firstString(page.JSON["title"], page.Metadata["title"], pageURL)
		description := firstString(page.JSON["description"], page.Metadata["description"])

		index.WriteString("- [" + title + "](" + pageURL + "): " + description + "\n")
		full.WriteString("## " + title + "\n" + page.Markdown + "\n\n")
	}

	return index.String(), full.String()
}

// firstString returns the first non-empty string among values. Page metadata can hold a string or a
// list of strings, so a list counts as its first item.
func firstString(values ...any) string {
	for _, value := range values {
		if list, ok := value.([]any); ok && len(list) > 0 {
			value = list[0]
		}

		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}

	return ""
}

// addScrapeTools adds the scrape tool
func (s *Server) addScrapeTools() {
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

func (s *Server) loadScrapeParams(args map[string]any) (*ScrapeParams, error) {
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

	return &params, nil
}

// handleScrape handles the scrape tool
func (s *Server) handleScrape(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	params, err := s.loadScrapeParams(args)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Scraping failed: %v", err)), nil
	}

	var response *ScrapeResponse

	err = s.withRetry(ctx, func() error {
		var err error

		response, err = s.client.ScrapeURL(ctx, params)
		return err
	})
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
func (s *Server) addMapTool() {
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
func (s *Server) handleMap(
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
	})
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
func (s *Server) addCrawlTools() {
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
func (s *Server) handleCrawl(
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
	})
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
func (s *Server) handleCrawlStatus(
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
