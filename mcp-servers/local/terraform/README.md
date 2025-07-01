# <img src="public/images/Terraform-LogoMark_onDark.svg" width="30" align="left" style="margin-right: 12px;"/> Terraform MCP Server

The Terraform MCP Server is a [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction)
server that provides seamless integration with Terraform Registry APIs, enabling advanced
automation and interaction capabilities for Infrastructure as Code (IaC) development.

## Features

- **Dual Transport Support**: Both Stdio and StreamableHTTP transports
- **Terraform Provider Discovery**: Query and explore Terraform providers and their documentation
- **Module Search & Analysis**: Search and retrieve detailed information about Terraform modules
- **Registry Integration**: Direct integration with Terraform Registry APIs
- **Container Ready**: Docker support for easy deployment

> **Caution:** The outputs and recommendations provided by the MCP server are generated dynamically and may vary based on the query, model, and the connected MCP server. Users should **thoroughly review all outputs/recommendations** to ensure they align with their organization's **security best practices**, **cost-efficiency goals**, and **compliance requirements** before implementation.

## Prerequisites

1. To run the server in a container, you will need to have [Docker](https://www.docker.com/) installed.
2. Once Docker is installed, you will need to ensure Docker is running.

## Transport Support

The Terraform MCP Server supports multiple transport protocols:

### 1. Stdio Transport (Default)

Standard input/output communication using JSON-RPC messages. Ideal for local development and direct integration with MCP clients.

### 2. StreamableHTTP Transport

Modern HTTP-based transport supporting both direct HTTP requests and Server-Sent Events (SSE) streams. This is the recommended transport for remote/distributed setups.

**Features:**

- **Endpoint**: `http://{hostname}:8080/mcp`
- **Health Check**: `http://{hostname}:8080/health`
- **Environment Configuration**: Set `MODE=http` or `PORT=8080` to enable

**Environment Variables:**

| Variable | Description | Default |
|----------|-------------|---------|
| `MODE` | Set to `http` to enable HTTP transport | `stdio` |
| `PORT` | HTTP server port | `8080` |

## Command Line Options

```bash
# Stdio mode
terraform-mcp-server stdio [--log-file /path/to/log]

# HTTP mode
terraform-mcp-server http [--port 8080] [--host 0.0.0.0] [--log-file /path/to/log]
```

## Installation

### Usage with VS Code

Add the following JSON block to your User Settings (JSON) file in VS Code. You can do this by pressing `Ctrl + Shift + P` and typing `Preferences: Open User Settings (JSON)`.

More about using MCP server tools in VS Code's [agent mode documentation](https://code.visualstudio.com/docs/copilot/chat/mcp-servers).

```json
{
  "mcp": {
    "servers": {
      "terraform": {
        "command": "docker",
        "args": [
          "run",
          "-i",
          "--rm",
          "hashicorp/terraform-mcp-server"
        ]
      }
    }
  }
}
```

Optionally, you can add a similar example (i.e. without the mcp key) to a file called `.vscode/mcp.json` in your workspace. This will allow you to share the configuration with others.

```json
{
  "servers": {
    "terraform": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "hashicorp/terraform-mcp-server"
      ]
    }
  }
}
```

### Usage with Claude Desktop / Amazon Q Developer / Amazon Q CLI

More about using MCP server tools in Claude Desktop [user documentation](https://modelcontextprotocol.io/quickstart/user).
Read more about using MCP server in Amazon Q from the [documentation](https://docs.aws.amazon.com/amazonq/latest/qdeveloper-ug/qdev-mcp.html).

```json
{
  "mcpServers": {
    "terraform": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "hashicorp/terraform-mcp-server"
      ]
    }
  }
}
```

## Tool Configuration

### Available Toolsets

The following sets of tools are available:

| Toolset     | Tool                   | Description                                                                                                                                                                                                                                                    |
|-------------|------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `providers` | `resolveProviderDocID` | Queries the Terraform Registry to find and list available documentation for a specific provider using the specified `serviceSlug`. Returns a list of provider document IDs with their titles and categories for resources, data sources, functions, or guides. |
| `providers` | `getProviderDocs`      | Fetches the complete documentation content for a specific provider resource, data source, or function using a document ID obtained from the `resolveProviderDocID` tool. Returns the raw documentation in markdown format.                                     |
| `modules`   | `searchModules`        | Searches the Terraform Registry for modules based on specified `moduleQuery` with pagination. Returns a list of module IDs with their names, descriptions, download counts, verification status, and publish dates                                             |
| `modules`   | `moduleDetails`        | Retrieves detailed documentation for a module using a module ID obtained from the `searchModules` tool including inputs, outputs, configuration, submodules, and examples.                                                                                     |
| `policies`  | `searchPolicies`       | Queries the Terraform Registry to find and list the appropriate Sentinel Policy based on the provided query `policyQuery`. Returns a list of matching policies with terraformPolicyIDs with their name, title and download counts.                             |
| `policies`  | `policyDetails`        | Retrieves detailed documentation for a policy set using a terraformPolicyID obtained from the `searchPolicies` tool including policy readme and implementation details.                                                                                        |

### Install from source

Use the latest release version:

```console
go install github.com/hashicorp/terraform-mcp-server/cmd/terraform-mcp-server@latest
```

Use the main branch:

```console
go install github.com/hashicorp/terraform-mcp-server/cmd/terraform-mcp-server@main
```

```json
{
  "mcp": {
    "servers": {
      "terraform": {
        "command": "/path/to/terraform-mcp-server",
        "args": ["stdio"]
      }
    }
  }
}
```

## Building the Docker Image locally

Before using the server, you need to build the Docker image locally:

1. Clone the repository:

```bash
git clone https://github.com/hashicorp/terraform-mcp-server.git
cd terraform-mcp-server
```

2. Build the Docker image:

```bash
make docker-build
```

3. This will create a local Docker image that you can use in the following configuration.

```bash
# Run in stdio mode
docker run -i --rm terraform-mcp-server:dev

# Run in http mode
docker run -p 8080:8080 --rm -e MODE=http terraform-mcp-server:dev
```

4. (Optional) Test connection in http mode
  
```bash
# Test the connection
curl http://localhost:8080/health
```

5. You can use it on your AI assistant as follow:

```json
{
  "mcpServers": {
    "terraform": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "terraform-mcp-server:dev"
      ]
    }
  }
}
```

## Development

### Available Make Commands

| Command | Description |
|---------|-------------|
| `make build` | Build the binary |
| `make test` | Run all tests |
| `make test-e2e` | Run end-to-end tests |
| `make docker-build` | Build Docker image |
| `make run-http` | Run HTTP server locally |
| `make docker-run-http` | Run HTTP server in Docker |
| `make test-http` | Test HTTP health endpoint |
| `make clean` | Remove build artifacts |
| `make help` | Show all available commands |

## Contributing

1. Fork the repository
2. Create your feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## Security

For security issues, please contact <security@hashicorp.com> or follow our [security policy](https://www.hashicorp.com/en/trust/security/vulnerability-management).

## Support

For bug reports and feature requests, please open an issue on GitHub.

For general questions and discussions, open a GitHub Discussion.
