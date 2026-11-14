package firecrawl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
)

// Client represents the Firecrawl API client
type Client struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
}

// ScrapeParams represents parameters for scraping
type ScrapeParams struct {
	URL                 string          `json:"url"`
	Formats             []string        `json:"formats,omitempty"`
	OnlyMainContent     *bool           `json:"onlyMainContent,omitempty"`
	IncludeTags         []string        `json:"includeTags,omitempty"`
	ExcludeTags         []string        `json:"excludeTags,omitempty"`
	WaitFor             *int            `json:"waitFor,omitempty"`
	Timeout             *int            `json:"timeout,omitempty"`
	Actions             []Action        `json:"actions,omitempty"`
	Extract             *ExtractConfig  `json:"extract,omitempty"`
	Mobile              *bool           `json:"mobile,omitempty"`
	SkipTLSVerification *bool           `json:"skipTlsVerification,omitempty"`
	RemoveBase64Images  *bool           `json:"removeBase64Images,omitempty"`
	Location            *LocationConfig `json:"location,omitempty"`
	Origin              string          `json:"origin,omitempty"`
}

// Action represents an action to perform before scraping
type Action struct {
	Type         string `json:"type"`
	Selector     string `json:"selector,omitempty"`
	Milliseconds *int   `json:"milliseconds,omitempty"`
	Text         string `json:"text,omitempty"`
	Key          string `json:"key,omitempty"`
	Direction    string `json:"direction,omitempty"`
	Script       string `json:"script,omitempty"`
	FullPage     *bool  `json:"fullPage,omitempty"`
}

// ExtractConfig represents extraction configuration
type ExtractConfig struct {
	Schema       map[string]any `json:"schema,omitempty"`
	SystemPrompt string         `json:"systemPrompt,omitempty"`
	Prompt       string         `json:"prompt,omitempty"`
}

// LocationConfig represents location settings
type LocationConfig struct {
	Country   string   `json:"country,omitempty"`
	Languages []string `json:"languages,omitempty"`
}

// MapParams represents parameters for mapping
type MapParams struct {
	URL               string `json:"url"`
	Search            string `json:"search,omitempty"`
	IgnoreSitemap     *bool  `json:"ignoreSitemap,omitempty"`
	SitemapOnly       *bool  `json:"sitemapOnly,omitempty"`
	IncludeSubdomains *bool  `json:"includeSubdomains,omitempty"`
	Limit             *int   `json:"limit,omitempty"`
	Origin            string `json:"origin,omitempty"`
}

// CrawlParams represents parameters for crawling
type CrawlParams struct {
	URL                    string        `json:"url"`
	ExcludePaths           []string      `json:"excludePaths,omitempty"`
	IncludePaths           []string      `json:"includePaths,omitempty"`
	MaxDepth               *int          `json:"maxDepth,omitempty"`
	IgnoreSitemap          *bool         `json:"ignoreSitemap,omitempty"`
	Limit                  *int          `json:"limit,omitempty"`
	AllowBackwardLinks     *bool         `json:"allowBackwardLinks,omitempty"`
	AllowExternalLinks     *bool         `json:"allowExternalLinks,omitempty"`
	Webhook                any           `json:"webhook,omitempty"`
	DeduplicateSimilarURLs *bool         `json:"deduplicateSimilarURLs,omitempty"`
	IgnoreQueryParameters  *bool         `json:"ignoreQueryParameters,omitempty"`
	ScrapeOptions          *ScrapeParams `json:"scrapeOptions,omitempty"`
	Origin                 string        `json:"origin,omitempty"`
}

// SearchParams represents parameters for searching
type SearchParams struct {
	Query         string          `json:"query"`
	Limit         *int            `json:"limit,omitempty"`
	Lang          string          `json:"lang,omitempty"`
	Country       string          `json:"country,omitempty"`
	TBS           string          `json:"tbs,omitempty"`
	Filter        string          `json:"filter,omitempty"`
	Location      *LocationConfig `json:"location,omitempty"`
	ScrapeOptions *ScrapeConfig   `json:"scrapeOptions,omitempty"`
	Origin        string          `json:"origin,omitempty"`
}

// ScrapeConfig represents scrape configuration for search
type ScrapeConfig struct {
	Formats         []string `json:"formats,omitempty"`
	OnlyMainContent *bool    `json:"onlyMainContent,omitempty"`
	WaitFor         *int     `json:"waitFor,omitempty"`
	IncludeTags     []string `json:"includeTags,omitempty"`
	ExcludeTags     []string `json:"excludeTags,omitempty"`
	Timeout         *int     `json:"timeout,omitempty"`
}

// ExtractParams represents parameters for extraction
type ExtractParams struct {
	URLs               []string       `json:"urls"`
	Prompt             string         `json:"prompt,omitempty"`
	SystemPrompt       string         `json:"systemPrompt,omitempty"`
	Schema             map[string]any `json:"schema,omitempty"`
	AllowExternalLinks *bool          `json:"allowExternalLinks,omitempty"`
	EnableWebSearch    *bool          `json:"enableWebSearch,omitempty"`
	IncludeSubdomains  *bool          `json:"includeSubdomains,omitempty"`
	Origin             string         `json:"origin,omitempty"`
}

// AgentParams represents parameters for an agent run
type AgentParams struct {
	Prompt     string         `json:"prompt"`
	Schema     map[string]any `json:"schema,omitempty"`
	MaxCredits *int           `json:"maxCredits,omitempty"`
	Origin     string         `json:"origin,omitempty"`
}

// GenerateLLMsTextParams represents parameters for LLMs.txt generation
type GenerateLLMsTextParams struct {
	URL          string `json:"url"`
	MaxURLs      *int   `json:"maxUrls,omitempty"`
	ShowFullText *bool  `json:"showFullText,omitempty"`
	Origin       string `json:"origin,omitempty"`
}

// Response types
type ScrapeResponse struct {
	Success    bool           `json:"success"`
	Data       *Document      `json:"data,omitempty"`
	Markdown   string         `json:"markdown,omitempty"`
	HTML       string         `json:"html,omitempty"`
	RawHTML    string         `json:"rawHtml,omitempty"`
	Links      []string       `json:"links,omitempty"`
	Screenshot string         `json:"screenshot,omitempty"`
	Extract    map[string]any `json:"extract,omitempty"`
	Warning    string         `json:"warning,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type MapResponse struct {
	Success bool     `json:"success"`
	Links   []string `json:"links,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type CrawlResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id,omitempty"`
	Error   string `json:"error,omitempty"`
}

type CrawlStatusResponse struct {
	Success     bool       `json:"success"`
	Status      string     `json:"status"`
	Completed   int        `json:"completed"`
	Total       int        `json:"total"`
	CreditsUsed int        `json:"creditsUsed"`
	ExpiresAt   string     `json:"expiresAt"`
	Data        []Document `json:"data"`
	Error       string     `json:"error,omitempty"`
}

type SearchResponse struct {
	Success bool       `json:"success"`
	Data    []Document `json:"data,omitempty"`
	Error   string     `json:"error,omitempty"`
}

type ExtractResponse struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data,omitempty"`
	Warning string         `json:"warning,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type AgentResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id,omitempty"`
	Error   string `json:"error,omitempty"`
}

type AgentStatusResponse struct {
	Success bool           `json:"success"`
	Status  string         `json:"status"`
	Data    map[string]any `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type GenerateLLMsTextResponse struct {
	Success bool          `json:"success"`
	Data    *LLMsTextData `json:"data,omitempty"`
	Error   string        `json:"error,omitempty"`
}

type LLMsTextData struct {
	LLMsText     string `json:"llmstxt"`
	LLMsFullText string `json:"llmsfulltxt,omitempty"`
}

type Document struct {
	URL         string         `json:"url,omitempty"`
	Markdown    string         `json:"markdown,omitempty"`
	HTML        string         `json:"html,omitempty"`
	RawHTML     string         `json:"rawHtml,omitempty"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// NewFirecrawlClient creates a new Firecrawl client
func NewFirecrawlClient(apiKey, apiURL string) *Client {
	return &Client{
		apiKey: apiKey,
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// makeRequest makes an HTTP request to the Firecrawl API
func (c *Client) makeRequest(
	ctx context.Context,
	method, endpoint string,
	body any,
	v any,
) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := sonic.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}

		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+endpoint, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return sonic.ConfigDefault.NewDecoder(resp.Body).Decode(v)
}

// ScrapeURL scrapes a single URL
func (c *Client) ScrapeURL(
	ctx context.Context,
	params *ScrapeParams,
) (*ScrapeResponse, error) {
	var response ScrapeResponse

	err := c.makeRequest(ctx, http.MethodPost, "/v1/scrape", params, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// MapURL maps a website to discover URLs
func (c *Client) MapURL(ctx context.Context, params MapParams) (*MapResponse, error) {
	var response MapResponse

	err := c.makeRequest(ctx, http.MethodPost, "/v1/map", params, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// AsyncCrawlURL starts an asynchronous crawl
func (c *Client) AsyncCrawlURL(
	ctx context.Context,
	params CrawlParams,
) (*CrawlResponse, error) {
	var response CrawlResponse

	err := c.makeRequest(ctx, http.MethodPost, "/v1/crawl", params, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// CheckCrawlStatus checks the status of a crawl job
func (c *Client) CheckCrawlStatus(
	ctx context.Context,
	id string,
) (*CrawlStatusResponse, error) {
	var response CrawlStatusResponse

	err := c.makeRequest(ctx, http.MethodGet, "/v1/crawl/"+id, nil, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// Search searches the web
func (c *Client) Search(
	ctx context.Context,
	params SearchParams,
) (*SearchResponse, error) {
	var response SearchResponse

	err := c.makeRequest(ctx, http.MethodPost, "/v1/search", params, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// Extract extracts structured data
func (c *Client) Extract(
	ctx context.Context,
	params ExtractParams,
) (*ExtractResponse, error) {
	var response ExtractResponse

	err := c.makeRequest(ctx, http.MethodPost, "/v1/extract", params, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// StartAgent starts an agent run
func (c *Client) StartAgent(ctx context.Context, params AgentParams) (*AgentResponse, error) {
	var response AgentResponse

	err := c.makeRequest(ctx, http.MethodPost, "/v2/agent", params, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetAgentStatus gets the status and, once completed, the data of an agent run
func (c *Client) GetAgentStatus(ctx context.Context, id string) (*AgentStatusResponse, error) {
	var response AgentStatusResponse

	err := c.makeRequest(ctx, http.MethodGet, "/v2/agent/"+id, nil, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GenerateLLMsText generates LLMs.txt file
func (c *Client) GenerateLLMsText(
	ctx context.Context,
	params GenerateLLMsTextParams,
) (*GenerateLLMsTextResponse, error) {
	var response GenerateLLMsTextResponse

	err := c.makeRequest(ctx, http.MethodPost, "/v1/llmstxt", params, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
