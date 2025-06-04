# Embed MCP Servers

This directory contains the implementation of embedded Model Context Protocol (MCP) servers for AI Proxy. Embed MCP allows native Go implementations to be registered directly within the application, providing high-performance, type-safe MCP servers without external dependencies.

## Overview

Embed MCP servers are native Go implementations that run within the AI Proxy process. They offer several advantages over external MCP servers:

- **Performance**: No network overhead, direct function calls
- **Type Safety**: Compile-time validation of server implementations
- **Resource Efficiency**: Shared memory and resources with the main application
- **Hot Reloading**: Dynamic configuration without server restart
- **Integrated Monitoring**: Built-in metrics and logging

## Architecture

### Registration System

Embed MCP servers use a simple registration pattern during application startup:

```go
func init() {
    mcpservers.Register(mcpservers.EmbedMcp{
        ID:              "my-mcp-server",
        Name:            "My MCP Server", 
        NewServer:       NewServer,
        ConfigTemplates: configTemplates,
        Tags:            []string{"example", "demo"},
        Readme:          "Description of the server functionality",
    })
}
```

### Configuration Templates

Configuration templates define the parameters required by your MCP server:

```go
var configTemplates = map[string]mcpservers.ConfigTemplate{
    "api_key": {
        Name:        "API Key",
        Required:    mcpservers.ConfigRequiredTypeInitOnly,
        Example:     "sk-example-key",
        Description: "The API key for authentication",
        Validator:   validateAPIKey,
    },
    "endpoint": {
        Name:        "Endpoint URL", 
        Required:    mcpservers.ConfigRequiredTypeReusingOptional,
        Example:     "https://api.example.com",
        Description: "The base URL for the API",
        Validator:   validateURL,
    },
}
```

### Configuration Types

- **`ConfigRequiredTypeInitOnly`**: Required during server initialization, set once globally
- **`ConfigRequiredTypeReusingOnly`**: Required as reusing parameter, can vary by group
- **`ConfigRequiredTypeInitOrReusingOnly`**: Required in either init or reusing config (mutually exclusive)
- **`ConfigRequiredTypeInitOptional`**: Optional during initialization
- **`ConfigRequiredTypeReusingOptional`**: Optional as reusing parameter

### Server Implementation

```go
func NewServer(config map[string]string, reusingConfig map[string]string) (*server.MCPServer, error) {
    // Access configuration
    apiKey := config["api_key"]           // From init config
    endpoint := reusingConfig["endpoint"] // From reusing config
    
    // Create and configure your MCP server
    mcpServer := server.NewMCPServer("my-server", server.WithMCPCapabilities(
        server.MCPServerCapabilities{
            Tools:     &server.MCPCapabilitiesTools{},
            Resources: &server.MCPCapabilitiesResources{},
        },
    ))
    
    // Register tools, resources, etc.
    mcpServer.AddTool(server.Tool{
        Name:        "example_tool",
        Description: "An example tool",
        // ... tool implementation
    })
    
    return mcpServer, nil
}
```

## API Reference

### Management Endpoints

#### List Available Embed MCP Servers

```http
GET /api/embedmcp/
```

Returns all registered embed MCP servers with their configuration templates and enabled status.

**Response**:

```json
[
  {
    "id": "aiproxy-openapi",
    "enabled": true,
    "name": "AI Proxy OpenAPI",
    "readme": "Exposes AI Proxy API as MCP tools",
    "tags": ["openapi", "admin"],
    "config_templates": {
      "host": {
        "name": "Host",
        "required": true,
        "example": "http://localhost:3000",
        "description": "The host of the OpenAPI server"
      },
      "authorization": {
        "name": "Authorization", 
        "required": false,
        "example": "admin-key",
        "description": "The admin key for authentication"
      }
    }
  }
]
```

#### Save/Configure Embed MCP Server

```http
POST /api/embedmcp/
```

Configure and enable/disable an embed MCP server.

**Request**:

```json
{
  "id": "aiproxy-openapi",
  "enabled": true,
  "init_config": {
    "host": "http://localhost:3000"
  }
}
```

### Testing Endpoints

#### Test SSE Connection

```http
GET /api/test-embedmcp/{id}/sse?key=adminkey&config[key]=value&reusing[key]=value
```

Establishes a Server-Sent Events connection for testing the embed MCP server.

**Query Parameters**:

- `config[key]=value`: Initial configuration parameters
- `reusing[key]=value`: Reusing configuration parameters

#### Test Streamable Connection  

```http
GET|POST|DELETE /api/test-embedmcp/{id}?key=adminkey&config[key]=value&reusing[key]=value
```

HTTP-based request/response interface for testing.

#### Send Test Message

```http
POST /api/test-embedmcp/message?key=adminkey&sessionId={session}
```

Send messages to active SSE test sessions.

**Request Body**: JSON-RPC message

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}
```

## Usage Examples

### Testing an Embed MCP Server

#### 1. Start SSE Connection

```http
http://localhost:3000/api/test-embedmcp/aiproxy-openapi/sse?key=adminkey&config[host]=http://localhost:3000&reusing[authorization]=admin-key
```

This will establish an SSE connection and return:

```sse
event: endpoint
data: /api/test-embedmcp/message?sessionId=abc123&key=your-token

event: message  
data: {"jsonrpc":"2.0","id":1,"method":"ping"}
```

#### 2. Send Messages (in another terminal)

```http
http://localhost:3000/api/test-embedmcp/message?key=adminkey&sessionId=abc123
```

#### 3. Test Streamable Interface

```http
http://localhost:3000/api/test-embedmcp/aiproxy-openapi?key=adminkey&config[host]=http://localhost:3000&reusing[authorization]=admin-key
```

### Configuration Examples

#### Complex Configuration

```bash
# Multiple configuration parameters
curl "http://localhost:3000/api/test-embedmcp/my-server/sse?key=adminkey&config[api_key]=sk-123&config[timeout]=30&reusing[endpoint]=https://api.example.com&reusing[region]=us-east-1"
```

#### Optional Parameters

```bash
# Only required parameters
curl "http://localhost:3000/api/test-embedmcp/my-server/sse?key=adminkey&config[api_key]=sk-123"
```

## Development Guide

### Creating a New Embed MCP Server

#### 1. Create Server Directory

```bash
core/mcpservers/my-server/
├── server.go
└── README.md
```

#### 2. Implement the Server

**server.go**:

```go
package myserver

import (
    "fmt"
    "net/url"
    
    "github.com/labring/aiproxy/core/mcpservers"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

// Configuration templates define required parameters
var configTemplates = map[string]mcpservers.ConfigTemplate{
    "api_endpoint": {
        Name:        "API Endpoint",
        Required:    mcpservers.ConfigRequiredTypeInitOnly,
        Example:     "https://api.example.com",
        Description: "The API endpoint URL",
        Validator:   validateURL,
    },
    "api_key": {
        Name:        "API Key",
        Required:    mcpservers.ConfigRequiredTypeReusingOnly,
        Example:     "sk-example-key",
        Description: "Authentication key for the API",
        Validator:   validateAPIKey,
    },
    "timeout": {
        Name:        "Timeout",
        Required:    mcpservers.ConfigRequiredTypeInitOptional,
        Example:     "30",
        Description: "Request timeout in seconds",
        Validator:   validateTimeout,
    },
}

// Validation functions
func validateURL(value string) error {
    u, err := url.Parse(value)
    if err != nil {
        return err
    }
    if u.Scheme != "http" && u.Scheme != "https" {
        return fmt.Errorf("invalid scheme: %s", u.Scheme)
    }
    return nil
}

func validateAPIKey(value string) error {
    if len(value) < 10 {
        return fmt.Errorf("API key too short")
    }
    return nil
}

func validateTimeout(value string) error {
    // Validation logic here
    return nil
}

// NewServer creates a new instance of the MCP server
func NewServer(config map[string]string, reusingConfig map[string]string) (*server.MCPServer, error) {
    endpoint := config["api_endpoint"]
    apiKey := reusingConfig["api_key"]
    
    // Create MCP server with capabilities
    mcpServer := server.NewMCPServer("my-server", server.WithMCPCapabilities(
        server.MCPServerCapabilities{
            Tools:     &server.MCPCapabilitiesTools{},
            Resources: &server.MCPCapabilitiesResources{},
        },
    ))
    
    // Add tools
    mcpServer.AddTool(server.Tool{
        Name:        "get_data",
        Description: "Retrieve data from the API",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]any{
                "query": {
                    "type":        "string",
                    "description": "Search query",
                },
            },
            Required: []string{"query"},
        },
    }, func(arguments map[string]any) (*server.ToolResult, error) {
        query := arguments["query"].(string)
        
        // Implement your tool logic here
        // Use endpoint and apiKey for API calls
        
        return &server.ToolResult{
            Content: []any{
                server.TextContent{
                    Type: "text",
                    Text: fmt.Sprintf("Retrieved data for query: %s", query),
                },
            },
        }, nil
    })
    
    // Add resources if needed
    mcpServer.AddResource(server.Resource{
        URI:         "file:///data.json",
        Name:        "Data File",
        Description: "Sample data resource",
        MimeType:    "application/json",
    }, func() ([]byte, error) {
        // Return resource content
        return []byte(`{"example": "data"}`), nil
    })
    
    return mcpServer, nil
}

// Register the server during package initialization
func init() {
    mcpservers.Register(mcpservers.EmbedMcp{
        ID:              "my-server",
        Name:            "My Custom Server",
        NewServer:       NewServer,
        ConfigTemplates: configTemplates,
        Tags:            []string{"example", "custom"},
        Readme:          "A custom MCP server implementation for demonstration",
    })
}
```

#### 3. Register in Init System

Add import to `core/mcpservers/mcpregister/init.go`:

```go
package mcpregister

import (
    // register embed mcp
    _ "github.com/labring/aiproxy/core/mcpservers/aiproxy-openapi"
    _ "github.com/labring/aiproxy/core/mcpservers/my-server"  // Add this line
)
```

#### 4. Test Your Server

```http
http://localhost:3000/api/test-embedmcp/my-server/sse?key=adminkey&config[api_endpoint]=https://api.example.com&reusing[api_key]=sk-test-key
```

### Advanced Features

#### Server Caching

For servers without configuration requirements, the system automatically caches instances:

```go
// No configuration required - server will be cached
var configTemplates = map[string]mcpservers.ConfigTemplate{}

func NewServer(config map[string]string, reusingConfig map[string]string) (*server.MCPServer, error) {
    // This server instance will be cached and reused
    return server.NewMCPServer("static-server"), nil
}
```

#### Dynamic Configuration

Servers can access both init and reusing configuration:

```go
func NewServer(config map[string]string, reusingConfig map[string]string) (*server.MCPServer, error) {
    // Init config: Set once per server deployment  
    baseURL := config["base_url"]
    
    // Reusing config: Can vary per group/user
    apiKey := reusingConfig["api_key"]
    region := reusingConfig["region"]
    
    // Create server with group-specific configuration
    return createServerWithConfig(baseURL, apiKey, region)
}
```

#### Error Handling

Implement robust error handling:

```go
func NewServer(config map[string]string, reusingConfig map[string]string) (*server.MCPServer, error) {
    endpoint := config["endpoint"]
    if endpoint == "" {
        return nil, fmt.Errorf("endpoint is required")
    }
    
    // Validate endpoint accessibility
    if err := validateEndpointConnectivity(endpoint); err != nil {
        return nil, fmt.Errorf("endpoint validation failed: %w", err)
    }
    
    return createServer(endpoint)
}
```

## Built-in Servers

### AI Proxy OpenAPI Server

The `aiproxy-openapi` server demonstrates a complete embed MCP implementation that exposes AI Proxy's REST API as MCP tools.

**Features**:

- Automatic OpenAPI to MCP conversion
- Admin authentication support
- Full CRUD operations for channels, tokens, groups
- Resource management capabilities

**Configuration**:

- `host` (required): AI Proxy server URL
- `authorization` (optional): Admin authentication key

**Example Tools**:

- `get_channels`: List all channels
- `create_channel`: Create a new channel
- `get_tokens`: List tokens
- `create_token`: Create authentication tokens
- `get_groups`: List user groups

This server serves as both a practical tool and a reference implementation for creating embed MCP servers.
