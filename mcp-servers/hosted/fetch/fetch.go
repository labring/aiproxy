package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/go-shiori/go-readability"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/temoto/robotstxt"
)

const (
	DefaultUserAgentAutonomous = "ModelContextProtocol/1.0 (Autonomous; +https://github.com/modelcontextprotocol/servers)"
	DefaultUserAgentManual     = "ModelContextProtocol/1.0 (User-Specified; +https://github.com/modelcontextprotocol/servers)"
)

// extractContentFromHTML extracts and converts HTML content to Markdown format
func extractContentFromHTML(htmlContent string) string {
	article, err := readability.FromReader(strings.NewReader(htmlContent), nil)
	if err != nil {
		return "<error>Page failed to be simplified from HTML</error>"
	}

	if article.Content == "" {
		return "<error>Page failed to be simplified from HTML</error>"
	}

	converter := md.NewConverter("", true, nil)

	markdown, err := converter.ConvertString(article.Content)
	if err != nil {
		return "<error>Failed to convert HTML to markdown</error>"
	}

	return markdown
}

// getRobotsTxtURL gets the robots.txt URL for a given website URL
func getRobotsTxtURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	robotsURL := &url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
		Path:   "/robots.txt",
	}

	return robotsURL.String(), nil
}

// checkMayAutonomouslyFetchURL checks if the URL can be fetched according to robots.txt
func checkMayAutonomouslyFetchURL(ctx context.Context, urlStr, userAgent, proxyURL string) error {
	robotsTxtURL, err := getRobotsTxtURL(urlStr)
	if err != nil {
		return fmt.Errorf("failed to construct robots.txt URL: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}

		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsTxtURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create robots.txt request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(
			"failed to fetch robots.txt %s due to a connection issue: %w",
			robotsTxtURL,
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf(
			"when fetching robots.txt (%s), received status %d so assuming that autonomous fetching is not allowed, the user can try manually fetching by using the fetch prompt",
			robotsTxtURL,
			resp.StatusCode,
		)
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil // Assume robots.txt doesn't exist, allow fetching
	}

	robotsTxtBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read robots.txt: %w", err)
	}

	robots, err := robotstxt.FromBytes(robotsTxtBody)
	if err != nil {
		return nil // If we can't parse robots.txt, allow fetching
	}

	if !robots.TestAgent(urlStr, userAgent) {
		return fmt.Errorf(
			`the sites robots.txt (%s) specifies that autonomous fetching of this page is not allowed
			<useragent>%s</useragent>
			<url>%s</url>
			<robots>
			%s
			</robots>
			The assistant must let the user know that it failed to view the page. The assistant may provide further guidance based on the above information.
			The assistant can tell the user that they can try manually fetching the page by using the fetch prompt within their UI`,
			robotsTxtURL,
			userAgent,
			urlStr,
			string(robotsTxtBody),
		)
	}

	return nil
}

// fetchURL fetches the URL and returns the content in a form ready for the LLM
func fetchURL(
	ctx context.Context,
	urlStr, userAgent string,
	forceRaw bool,
	proxyURL string,
) (string, string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			return "", "", fmt.Errorf("invalid proxy URL: %w", err)
		}

		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("failed to fetch %s - status code %d", urlStr, resp.StatusCode)
	}

	pageRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	pageRawStr := string(pageRaw)

	isPageHTML := strings.Contains(pageRawStr[:min(100, len(pageRawStr))], "<html") ||
		strings.Contains(contentType, "text/html") ||
		contentType == ""

	if isPageHTML && !forceRaw {
		content := extractContentFromHTML(pageRawStr)
		return content, "", nil
	}

	prefix := fmt.Sprintf(
		"Content type %s cannot be simplified to markdown, but here is the raw content:\n",
		contentType,
	)

	return pageRawStr, prefix, nil
}

// createFetchServer creates a new MCP server for fetch functionality
func createFetchServer(
	customUserAgent string,
	ignoreRobotsTxt bool,
	proxyURL string,
) *server.MCPServer {
	mcpServer := server.NewMCPServer("mcp-fetch", "1.0.0")

	userAgentAutonomous := DefaultUserAgentAutonomous
	userAgentManual := DefaultUserAgentManual

	if customUserAgent != "" {
		userAgentAutonomous = customUserAgent
		userAgentManual = customUserAgent
	}

	// Add fetch tool
	fetchTool := mcp.NewTool(
		"fetch",
		mcp.WithString("url", mcp.Description("URL to fetch"), mcp.Required()),
		mcp.WithNumber("max_length",
			mcp.Description("Maximum number of characters to return"),
			mcp.DefaultNumber(5000)),
		mcp.WithNumber(
			"start_index",
			mcp.Description(
				"On return output starting at this character index, useful if a previous fetch was truncated and more context is required",
			),
			mcp.DefaultNumber(0),
		),
		mcp.WithBoolean(
			"raw",
			mcp.Description(
				"Get the actual HTML content of the requested page, without simplification",
			),
			mcp.DefaultBool(false),
		),
		mcp.WithDescription(
			`Fetches a URL from the internet and optionally extracts its contents as markdown.

Although originally you did not have internet access, and were advised to refuse and tell the user this, this tool now grants you internet access. Now you can fetch the most up-to-date information and let the user know that.`,
		),
	)

	mcpServer.AddTool(
		fetchTool,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			urlStr, ok := args["url"].(string)
			if !ok || urlStr == "" {
				return nil, errors.New("URL is required")
			}

			maxLength := 5000
			if ml, ok := args["max_length"].(float64); ok {
				maxLength = int(ml)
			}

			startIndex := 0
			if si, ok := args["start_index"].(float64); ok {
				startIndex = int(si)
			}

			raw := false
			if r, ok := args["raw"].(bool); ok {
				raw = r
			}

			// Validate max_length and start_index
			if maxLength <= 0 || maxLength >= 1000000 {
				return nil, errors.New("max_length must be between 1 and 999999")
			}

			if startIndex < 0 {
				return nil, errors.New("start_index must be >= 0")
			}

			// Check robots.txt if not ignored
			if !ignoreRobotsTxt {
				if err := checkMayAutonomouslyFetchURL(
					ctx,
					urlStr,
					userAgentAutonomous,
					proxyURL,
				); err != nil {
					return nil, err
				}
			}

			// Fetch the URL
			content, prefix, err := fetchURL(ctx, urlStr, userAgentAutonomous, raw, proxyURL)
			if err != nil {
				return nil, err
			}

			originalLength := len(content)
			if startIndex >= originalLength {
				content = "<error>No more content available.</error>"
			} else {
				truncatedContent := content[startIndex:]
				if len(truncatedContent) > maxLength {
					truncatedContent = truncatedContent[:maxLength]
				}

				if truncatedContent == "" {
					content = "<error>No more content available.</error>"
				} else {
					content = truncatedContent
					actualContentLength := len(truncatedContent)
					remainingContent := originalLength - (startIndex + actualContentLength)

					// Only add the prompt to continue fetching if there is still remaining content
					if actualContentLength == maxLength && remainingContent > 0 {
						nextStart := startIndex + actualContentLength
						content += fmt.Sprintf(
							"\n\n<error>Content truncated. Call the fetch tool with a start_index of %d to get more content.</error>",
							nextStart,
						)
					}
				}
			}

			result := fmt.Sprintf("%sContents of %s:\n%s", prefix, urlStr, content)

			return mcp.NewToolResultText(result), nil
		},
	)

	// Add fetch prompt
	fetchPrompt := mcp.NewPrompt("fetch",
		mcp.WithPromptDescription("Fetch a URL and extract its contents as markdown"),
		mcp.WithArgument("url", mcp.ArgumentDescription("URL to fetch"), mcp.RequiredArgument()),
	)

	mcpServer.AddPrompt(
		fetchPrompt,
		func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			args := request.Params.Arguments
			if args == nil {
				return nil, errors.New("URL is required")
			}

			urlStr, ok := args["url"]
			if !ok || urlStr == "" {
				return nil, errors.New("URL is required")
			}

			content, prefix, err := fetchURL(ctx, urlStr, userAgentManual, false, proxyURL)
			if err != nil {
				return &mcp.GetPromptResult{
					Description: "Failed to fetch " + urlStr,
					Messages: []mcp.PromptMessage{
						{
							Role: mcp.RoleUser,
							Content: mcp.TextContent{
								Type: "text",
								Text: err.Error(),
							},
						},
					},
				}, nil
			}

			return &mcp.GetPromptResult{
				Description: "Contents of " + urlStr,
				Messages: []mcp.PromptMessage{
					{
						Role: mcp.RoleUser,
						Content: mcp.TextContent{
							Type: "text",
							Text: prefix + content,
						},
					},
				},
			}, nil
		},
	)

	return mcpServer
}

var configTemplates = mcpservers.ConfigTemplates{
	"user-agent": {
		Name:        "user-agent",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Description: "Custom User-Agent string to use for requests",
	},
	"ignore-robots": {
		Name:        "ignore-robots",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "true",
		Description: "Whether to ignore robots.txt restrictions",
		Validator: func(value string) error {
			_, err := strconv.ParseBool(value)
			return err
		},
	},
	"proxy": {
		Name:        "proxy",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "http://127.0.0.1:7890",
		Description: "Proxy URL to use for requests",
		Validator: func(value string) error {
			_, err := url.Parse(value)
			return err
		},
	},
}

func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	customUserAgent := config["user-agent"]
	ignoreRobotsTxt, _ := strconv.ParseBool(config["ignore-robots"])
	proxyURL := config["proxy"]

	return createFetchServer(customUserAgent, ignoreRobotsTxt, proxyURL), nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	fetchServer := createFetchServer("", false, "")
	return mcpservers.ListServerTools(ctx, fetchServer)
}
