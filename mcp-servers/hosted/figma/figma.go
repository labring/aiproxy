package figma

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server represents the Figma MCP server
type Server struct {
	*server.MCPServer
	client *Client
}

// Configuration templates
var configTemplates = mcpservers.ConfigTemplates{
	"figma-auth": {
		Name:        "Figma Authentication",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "figd_xxxxxxxxx",
		Description: "Your Figma Personal Access Token or OAuth Bearer token",
	},
	"figma-auth-type": {
		Name:        "Figma Auth Type",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "pat",
		Description: "Your Figma Authentication Type (pat or oauth)",
		Validator: func(value string) error {
			if value != "pat" && value != "oauth" {
				return errors.New("figma-auth-type must be 'pat' or 'oauth'")
			}
			return nil
		},
	},
}

// NewServer creates a new Figma MCP server
func NewServer(config, reusingConfig map[string]string) (mcpservers.Server, error) {
	// Get authentication options
	auth := AuthOptions{}

	// Try OAuth token first
	if token := config["figma-auth"]; token != "" {
		auth.AuthToken = token
	} else if token := reusingConfig["figma-auth"]; token != "" {
		auth.AuthToken = token
	}

	if authType := config["figma-auth-type"]; authType != "" {
		auth.AuthType = authType
	} else if authType := reusingConfig["figma-auth-type"]; authType != "" {
		auth.AuthType = authType
	}

	// Create MCP server
	mcpServer := server.NewMCPServer("figma-mcp", "1.0.0")

	// Create Figma client
	figmaClient := NewClient(auth)

	figmaServer := &Server{
		MCPServer: mcpServer,
		client:    figmaClient,
	}

	// Add tools
	figmaServer.addTools()

	return figmaServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	figmaServer := &Server{
		MCPServer: server.NewMCPServer("figma-mcp", "1.0.0"),
	}
	figmaServer.addTools()

	return mcpservers.ListServerTools(ctx, figmaServer)
}

// addTools adds all tools to the server
func (s *Server) addTools() {
	s.AddTool(s.getFigmaDataTool(), s.handleGetFigmaData)
	s.AddTool(s.downloadFigmaImagesTool(), s.handleDownloadFigmaImages)
}

// getFigmaDataTool returns the get_figma_data tool definition
func (s *Server) getFigmaDataTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_figma_data",
		Description: "When the nodeId cannot be obtained, obtain the layout information about the entire Figma file",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"fileKey": map[string]any{
					"type":        "string",
					"description": "The key of the Figma file to fetch, often found in a provided URL like figma.com/(file|design)/<fileKey>/...",
				},
				"nodeId": map[string]any{
					"type":        "string",
					"description": "The ID of the node to fetch, often found as URL parameter node-id=<nodeId>, always use if provided",
				},
				"depth": map[string]any{
					"type":        "number",
					"description": "OPTIONAL. Do NOT use unless explicitly requested by the user. Controls how many levels deep to traverse the node tree",
				},
			},
			Required: []string{"fileKey"},
		},
	}
}

// downloadFigmaImagesTool returns the download_figma_images tool definition
func (s *Server) downloadFigmaImagesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "download_figma_images",
		Description: "Download SVG and PNG images used in a Figma file based on the IDs of image or icon nodes",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"fileKey": map[string]any{
					"type":        "string",
					"description": "The key of the Figma file containing the node",
				},
				"nodes": map[string]any{
					"type":        "array",
					"description": "The nodes to fetch as images",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"nodeId": map[string]any{
								"type":        "string",
								"description": "The ID of the Figma image node to fetch, formatted as 1234:5678",
							},
							"imageRef": map[string]any{
								"type":        "string",
								"description": "If a node has an imageRef fill, you must include this variable. Leave blank when downloading Vector SVG images.",
							},
							"fileName": map[string]any{
								"type":        "string",
								"description": "The local name for saving the fetched file",
							},
						},
						"required": []string{"nodeId", "fileName"},
					},
				},
				"pngScale": map[string]any{
					"type":        "number",
					"description": "Export scale for PNG images. Optional, defaults to 2 if not specified. Affects PNG images only.",
					"default":     2,
				},
				"svgOptions": map[string]any{
					"type":        "object",
					"description": "Options for SVG export",
					"properties": map[string]any{
						"outlineText": map[string]any{
							"type":        "boolean",
							"description": "Whether to outline text in SVG exports. Default is true.",
							"default":     true,
						},
						"includeId": map[string]any{
							"type":        "boolean",
							"description": "Whether to include IDs in SVG exports. Default is false.",
							"default":     false,
						},
						"simplifyStroke": map[string]any{
							"type":        "boolean",
							"description": "Whether to simplify strokes in SVG exports. Default is true.",
							"default":     true,
						},
					},
				},
			},
			Required: []string{"fileKey", "nodes"},
		},
	}
}

// handleGetFigmaData handles the get_figma_data tool
func (s *Server) handleGetFigmaData(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	fileKey, ok := args["fileKey"].(string)
	if !ok || fileKey == "" {
		return nil, errors.New("fileKey is required")
	}

	nodeID := ""
	if nid, ok := args["nodeId"].(string); ok {
		nodeID = nid
	}

	var depth *int
	if d, ok := args["depth"].(float64); ok {
		depthInt := int(d)
		depth = &depthInt
	}

	var (
		design *SimplifiedDesign
		err    error
	)

	if nodeID != "" {
		design, err = s.client.GetNode(ctx, fileKey, nodeID, depth)
	} else {
		design, err = s.client.GetFile(ctx, fileKey, depth)
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching file %s: %w", fileKey, err)
	}

	jsonData, err := sonic.Marshal(design)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleDownloadFigmaImages handles the download_figma_images tool
func (s *Server) handleDownloadFigmaImages(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	fileKey, ok := args["fileKey"].(string)
	if !ok || fileKey == "" {
		return nil, errors.New("fileKey is required")
	}

	nodesData, ok := args["nodes"]
	if !ok {
		return nil, errors.New("nodes is required")
	}

	// Parse nodes
	nodesJSON, err := sonic.Marshal(nodesData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %w", err)
	}

	var nodes []ImageNode
	if err := sonic.Unmarshal(nodesJSON, &nodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %w", err)
	}

	// Get optional parameters
	pngScale := 2.0
	if ps, ok := args["pngScale"].(float64); ok {
		pngScale = ps
	}

	svgOptions := SVGOptions{
		OutlineText:    true,
		IncludeID:      false,
		SimplifyStroke: true,
	}
	if so, ok := args["svgOptions"].(map[string]any); ok {
		if ot, ok := so["outlineText"].(bool); ok {
			svgOptions.OutlineText = ot
		}

		if ii, ok := so["includeId"].(bool); ok {
			svgOptions.IncludeID = ii
		}

		if ss, ok := so["simplifyStroke"].(bool); ok {
			svgOptions.SimplifyStroke = ss
		}
	}

	// Download images
	downloads, err := s.client.DownloadImages(ctx, fileKey, nodes, pngScale, svgOptions)
	if err != nil {
		return nil, fmt.Errorf("error downloading images: %w", err)
	}

	results := make([]mcp.Content, len(downloads))
	for i, download := range downloads {
		base64Data := base64.StdEncoding.EncodeToString(download)
		results[i] = mcp.NewImageContent(base64Data, "image/png")
	}

	return &mcp.CallToolResult{
		Content: results,
	}, nil
}
