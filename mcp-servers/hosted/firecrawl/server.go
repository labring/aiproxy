package firecrawl

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Configuration templates for the Firecrawl server
var configTemplates = mcpservers.ConfigTemplates{
	"firecrawl_api_key": {
		Name:        "Firecrawl API Key",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "fc-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		Description: "Firecrawl API key for cloud service",
	},
	"firecrawl_api_url": {
		Name:        "Firecrawl API URL",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "https://api.firecrawl.dev",
		Description: "Firecrawl API URL (for self-hosted instances)",
		Validator: func(value string) error {
			_, err := url.Parse(value)
			return err
		},
	},
	"retry_max_attempts": {
		Name:        "Retry Max Attempts",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "3",
		Description: "Maximum number of retry attempts",
		Validator: func(value string) error {
			if _, err := strconv.Atoi(value); err != nil {
				return errors.New("must be a number")
			}
			return nil
		},
	},
}

// Server represents the MCP server for Firecrawl integration
type Server struct {
	*server.MCPServer
	client *Client
	config RetryConfig
}

// RetryConfig represents retry configuration
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// NewServer creates a new MCP server for Firecrawl functionality
func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	apiKey := config["firecrawl_api_key"]
	apiURL := config["firecrawl_api_url"]

	if apiURL == "" && apiKey == "" {
		return nil, errors.New(
			"FIRECRAWL_API_KEY and FIRECRAWL_API_URL are required",
		)
	}

	// Parse retry configuration
	retryConfig := RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1000 * time.Millisecond,
		MaxDelay:      10000 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	if maxAttempts := config["retry_max_attempts"]; maxAttempts != "" {
		if attempts, err := strconv.Atoi(maxAttempts); err == nil {
			retryConfig.MaxAttempts = attempts
		}
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"firecrawl-mcp",
		"1.7.0",
	)

	// Create Firecrawl client
	client := NewFirecrawlClient(apiKey, apiURL)

	firecrawlServer := &Server{
		MCPServer: mcpServer,
		client:    client,
		config:    retryConfig,
	}

	// Add all tools
	firecrawlServer.addAllTools()

	return firecrawlServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	firecrawlServer := &Server{
		MCPServer: server.NewMCPServer("firecrawl-mcp", "1.7.0"),
	}
	firecrawlServer.addAllTools()

	return mcpservers.ListServerTools(ctx, firecrawlServer)
}

// withRetry implements retry logic with exponential backoff
func (s *Server) withRetry(
	ctx context.Context,
	operation func() error,
) error {
	var lastErr error

	for attempt := 1; attempt <= s.config.MaxAttempts; attempt++ {
		if err := operation(); err != nil {
			lastErr = err

			// Check if it's a rate limit error
			isRateLimit := strings.Contains(err.Error(), "rate limit") ||
				strings.Contains(err.Error(), "429")

			if isRateLimit && attempt < s.config.MaxAttempts {
				delay := min(time.Duration(float64(s.config.InitialDelay)*
					float64(int(1)<<(attempt-1))*s.config.BackoffFactor), s.config.MaxDelay)

				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if attempt == s.config.MaxAttempts {
				return lastErr
			}
		} else {
			return nil
		}
	}

	return lastErr
}

// trimResponseText trims trailing whitespace from text responses
func trimResponseText(text string) string {
	return strings.TrimSpace(text)
}

// formatResults formats crawl results
func formatResults(data []Document) string {
	results := make([]string, 0, len(data))
	for _, doc := range data {
		content := doc.Markdown
		if content == "" {
			content = doc.HTML
		}

		if content == "" {
			content = doc.RawHTML
		}

		if content == "" {
			content = "No content"
		}

		if len(content) > 100 {
			content = content[:100] + "..."
		}

		result := fmt.Sprintf("URL: %s\nContent: %s",
			getStringOrDefault(doc.URL, "Unknown URL"), content)

		if doc.Title != "" {
			result += "\nTitle: " + doc.Title
		}

		results = append(results, result)
	}

	return strings.Join(results, "\n\n")
}

// getStringOrDefault returns the string value or a default if empty
func getStringOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// addAllTools adds all Firecrawl tools to the server
func (s *Server) addAllTools() {
	s.addScrapeTools()
	s.addMapTool()
	s.addCrawlTools()
	s.addSearchTool()
	s.addExtractTool()
	s.addDeepResearchTool()
	s.addGenerateLLMsTextTool()
}
