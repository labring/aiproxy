package amap

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/client/transport"
)

var configTemplates = map[string]mcpservers.ConfigTemplate{
	"key": {
		Name:        "Key",
		Required:    mcpservers.ConfigRequiredTypeInitOnly,
		Example:     "1234567890",
		Description: "The key of the AMap MCP server: https://console.amap.com/dev/key/app",
	},

	"url": {
		Name:        "URL",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "https://mcp.amap.com/sse",
		Description: "The URL of the AMap MCP server",
	},
}

func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	key := config["key"]
	if key == "" {
		return nil, errors.New("key is required")
	}
	u := config["url"]
	if u == "" {
		u = "https://mcp.amap.com/sse"
	}

	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	query := parsedURL.Query()
	query.Set("key", key)
	parsedURL.RawQuery = query.Encode()

	client, err := transport.NewSSE(parsedURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create sse client: %w", err)
	}

	err = client.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to start sse client: %w", err)
	}

	return mcpservers.WrapMCPClient2ServerWithCleanup(client), nil
}
