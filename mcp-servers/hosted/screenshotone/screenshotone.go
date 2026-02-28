package screenshotone

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ScreenshotOneBaseURL = "https://api.screenshotone.com"
	DefaultImageQuality  = 80
	DefaultTimeout       = 30 * time.Second
)

// Configuration templates
var configTemplates = mcpservers.ConfigTemplates{
	"screenshotone-api-key": {
		Name:        "ScreenshotOne API Key",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "your-api-key-here",
		Description: "Your ScreenshotOne API access key",
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

			if timeout < 5 || timeout > 300 {
				return errors.New("timeout must be between 5 and 300 seconds")
			}

			return nil
		},
	},
}

// Server represents the ScreenshotOne MCP server
type Server struct {
	*server.MCPServer
	apiKey string
	client *http.Client
}

// ScreenshotArgs represents the arguments for the screenshot tool
type ScreenshotArgs struct {
	URL          string `json:"url"`
	BlockBanners bool   `json:"block_banners"`
	BlockAds     bool   `json:"block_ads"`
	ImageQuality int    `json:"image_quality"`
	FullPage     bool   `json:"full_page"`
	ResponseType string `json:"response_type"`
	Cache        bool   `json:"cache"`
	CacheKey     string `json:"cache_key"`
}

// validateURL validates if the provided string is a valid URL
func validateURL(urlStr string) error {
	if urlStr == "" {
		return errors.New("URL is required")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("URL must include scheme and host")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("URL scheme must be http or https")
	}

	return nil
}

// validateCacheKey validates cache key format
func validateCacheKey(cacheKey string) error {
	if cacheKey == "" {
		return nil // Optional field
	}

	matched, err := regexp.MatchString("^[a-zA-Z0-9]+$", cacheKey)
	if err != nil {
		return fmt.Errorf("failed to validate cache key: %w", err)
	}

	if !matched {
		return errors.New("cache key must contain only alphanumeric characters")
	}

	return nil
}

// validateArgs validates screenshot arguments
func validateArgs(args ScreenshotArgs) error {
	if err := validateURL(args.URL); err != nil {
		return err
	}

	if args.ImageQuality < 1 || args.ImageQuality > 100 {
		return errors.New("image quality must be between 1 and 100")
	}

	if args.ResponseType != "" && args.ResponseType != "json" && args.ResponseType != "by_format" {
		return errors.New("response type must be 'json' or 'by_format'")
	}

	if err := validateCacheKey(args.CacheKey); err != nil {
		return err
	}

	return nil
}

// parseArgs parses and validates arguments from the request
func parseArgs(requestArgs map[string]any) (ScreenshotArgs, error) {
	args := ScreenshotArgs{
		BlockBanners: true,                // Default value
		BlockAds:     true,                // Default value
		ImageQuality: DefaultImageQuality, // Default value
		ResponseType: "by_format",         // Default value
	}

	// Parse URL (required)
	if urlVal, ok := requestArgs["url"].(string); ok {
		args.URL = urlVal
	} else {
		return args, errors.New("url is required and must be a string")
	}

	// Parse optional boolean fields
	if val, ok := requestArgs["block_banners"].(bool); ok {
		args.BlockBanners = val
	}

	if val, ok := requestArgs["block_ads"].(bool); ok {
		args.BlockAds = val
	}

	if val, ok := requestArgs["full_page"].(bool); ok {
		args.FullPage = val
	}

	if val, ok := requestArgs["cache"].(bool); ok {
		args.Cache = val
	}

	// Parse image quality
	if val, ok := requestArgs["image_quality"].(float64); ok {
		args.ImageQuality = int(val)
	}

	// Parse string fields
	if val, ok := requestArgs["response_type"].(string); ok {
		args.ResponseType = val
	}

	if val, ok := requestArgs["cache_key"].(string); ok {
		args.CacheKey = val
	}

	return args, validateArgs(args)
}

// buildScreenshotURL builds the ScreenshotOne API URL with parameters
func (s *Server) buildScreenshotURL(args ScreenshotArgs) string {
	params := url.Values{}
	params.Set("url", args.URL)
	params.Set("response_type", args.ResponseType)
	params.Set("cache", strconv.FormatBool(args.Cache))
	params.Set("format", "jpeg")
	params.Set("image_quality", strconv.Itoa(args.ImageQuality))
	params.Set("access_key", s.apiKey)
	params.Set("block_cookie_banners", strconv.FormatBool(args.BlockBanners))
	params.Set("block_banners_by_heuristics", strconv.FormatBool(args.BlockBanners))
	params.Set("block_ads", strconv.FormatBool(args.BlockAds))
	params.Set("full_page", strconv.FormatBool(args.FullPage))

	if args.Cache && args.CacheKey != "" {
		params.Set("cache_key", args.CacheKey)
	}

	return ScreenshotOneBaseURL + "/take?" + params.Encode()
}

// makeScreenshotRequest makes a request to ScreenshotOne API
func (s *Server) makeScreenshotRequest(ctx context.Context, screenshotURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, screenshotURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to render screenshot, status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// handleScreenshot handles the screenshot tool request
func (s *Server) handleScreenshot(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	requestArgs := request.GetArguments()

	args, err := parseArgs(requestArgs)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	screenshotURL := s.buildScreenshotURL(args)

	screenshot, err := s.makeScreenshotRequest(ctx, screenshotURL)
	if err != nil {
		return nil, err
	}

	// Encode image data as base64
	encodedData := base64.StdEncoding.EncodeToString(screenshot)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewImageContent(encodedData, "image/jpeg"),
		},
	}, nil
}

// NewServer creates a new ScreenshotOne MCP server
func NewServer(config, reusingConfig map[string]string) (mcpservers.Server, error) {
	apiKey := config["screenshotone-api-key"]
	if apiKey == "" {
		apiKey = reusingConfig["screenshotone-api-key"]
	}

	if apiKey == "" {
		return nil, errors.New("screenshotone-api-key is required")
	}

	timeout := DefaultTimeout
	if timeoutStr := config["timeout"]; timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer("screenshotone", "1.0.0")

	screenshotServer := &Server{
		MCPServer: mcpServer,
		apiKey:    apiKey,
		client:    client,
	}

	// Add the screenshot tool
	screenshotServer.addTools()

	return screenshotServer, nil
}

// addTools adds all tools to the server
func (s *Server) addTools() {
	tool := mcp.Tool{
		Name:        "render-website-screenshot",
		Description: "Renders a screenshot of a website and returns it as an image or a JSON with the cache URL (preferred for full-page screenshots).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url": map[string]any{
					"type":        "string",
					"format":      "uri",
					"description": "URL of the website to screenshot",
				},
				"block_banners": map[string]any{
					"type":        "boolean",
					"default":     true,
					"description": "Block cookie, GDPR, and other banners and popups",
				},
				"block_ads": map[string]any{
					"type":        "boolean",
					"default":     true,
					"description": "Block ads",
				},
				"image_quality": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     100,
					"default":     DefaultImageQuality,
					"description": "Image quality",
				},
				"full_page": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Render the full page screenshot of the website",
				},
				"response_type": map[string]any{
					"type":        "string",
					"enum":        []string{"json", "by_format"},
					"default":     "by_format",
					"description": "Response type: JSON (when the cache URL is needed) or the image itself",
				},
				"cache": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Cache the screenshot to get the cache URL",
				},
				"cache_key": map[string]any{
					"type":        "string",
					"pattern":     "^[a-zA-Z0-9]+$",
					"description": "Cache key to generate a new cache URL for each screenshot, e.g. timestamp",
				},
			},
			Required: []string{"url"},
		},
	}

	s.AddTool(tool, s.handleScreenshot)
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	mcpServer := server.NewMCPServer("screenshotone", "1.0.0")
	screenshotServer := &Server{
		MCPServer: mcpServer,
	}
	screenshotServer.addTools()

	return mcpservers.ListServerTools(ctx, screenshotServer.MCPServer)
}
