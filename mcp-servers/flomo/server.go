package flomo

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Configuration templates for the Flomo server
var configTemplates = mcpservers.ConfigTemplates{
	"flomo_api_url": {
		Name:        "Flomo API URL",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "https://flomoapp.com/iwh/xxx",
		Description: "Flomo API webhook URL for writing notes",
		Validator: func(value string) error {
			_, err := url.Parse(value)
			return err
		},
	},
}

// Server represents the MCP server for Flomo integration
type Server struct {
	*server.MCPServer
	flomoClient *Client
}

// NewServer creates a new MCP server for Flomo functionality
func NewServer(config, reuse map[string]string) (mcpservers.Server, error) {
	// Get API URL from config or environment
	apiURL := config["flomo_api_url"]
	if apiURL == "" {
		apiURL = reuse["flomo_api_url"]
	}
	if apiURL == "" {
		return nil, errors.New(
			"flomo API URL not set. Please provide flomo_api_url in config or FLOMO_API_URL environment variable",
		)
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"mcp-server-flomo",
		"0.0.3",
	)

	// Create Flomo client
	flomoClient := NewClient(apiURL)

	flomoServer := &Server{
		MCPServer:   mcpServer,
		flomoClient: flomoClient,
	}

	// Add tools
	flomoServer.addWriteNoteTool()

	return flomoServer, nil
}

// addWriteNoteTool adds the write_note tool to the server
func (s *Server) addWriteNoteTool() {
	writeNoteTool := mcp.Tool{
		Name:        "write_note",
		Description: "Write note to flomo",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "Text content of the note with markdown format",
				},
			},
			Required: []string{"content"},
		},
	}

	s.AddTool(
		writeNoteTool,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			content, ok := args["content"].(string)
			if !ok || content == "" {
				return nil, errors.New("content is required")
			}

			// Write note to Flomo
			result, err := s.flomoClient.WriteNote(ctx, content)
			if err != nil {
				return nil, fmt.Errorf("failed to write note to flomo: %w", err)
			}

			// Check if the note was successfully created
			if result.Memo == nil || result.Memo.Slug == "" {
				message := "unknown error"
				if result.Message != "" {
					message = result.Message
				}
				return nil, fmt.Errorf("failed to write note to flomo: %s", message)
			}

			// Format the response
			resultJSON, err := sonic.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			responseText := "Write note to flomo success, result: " + string(resultJSON)

			return mcp.NewToolResultText(responseText), nil
		},
	)
}
