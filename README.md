<div align="center">
  <h1>AI Proxy</h1>
  <p>Next-generation AI gateway with OpenAI-compatible protocol</p>
  
  [![Release](https://img.shields.io/github/release/labring/aiproxy)](https://github.com/labring/aiproxy/releases)
  [![License](https://img.shields.io/github/license/labring/aiproxy)](https://github.com/labring/aiproxy/blob/main/LICENSE)
  [![Go Version](https://img.shields.io/github/go-mod/go-version/labring/aiproxy?filename=core%2Fgo.mod)](https://github.com/labring/aiproxy/blob/main/core/go.mod)
  [![Build Status](https://img.shields.io/github/actions/workflow/status/labring/aiproxy/release.yml?branch=main)](https://github.com/labring/aiproxy/actions)
  
  [English](./README.md) | [简体中文](./README.zh.md)
</div>

---

## 🚀 Overview

AI Proxy is a powerful, production-ready AI gateway that provides intelligent request routing, comprehensive monitoring, and seamless multi-tenant management. Built with OpenAI-compatible, Anthropic and Gemini protocols, it serves as the perfect middleware for AI applications requiring reliability, scalability, and advanced features.

## ✨ Key Features

### 🔄 **Intelligent Request Management**

- **Smart Retry Logic**: Intelligent retry strategies with automatic error recovery
- **Priority-based Channel Selection**: Route requests based on channel priority and error rates
- **Load Balancing**: Efficiently distribute traffic across multiple AI providers
- **Protocol Conversion**: Seamless protocol conversion between OpenAI Chat Completions, Claude Messages, Gemini, and OpenAI Responses API
  - Chat/Claude/Gemini → Responses API: Use responses-only models with any protocol

### 📊 **Comprehensive Monitoring & Analytics**

- **Real-time Alerts**: Proactive notifications for balance warnings, error rates, and anomalies
- **Detailed Logging**: Complete request/response tracking with audit trails
- **Advanced Analytics**: Request volume, error statistics, RPM/TPM metrics, and cost analysis
- **Channel Performance**: Error rate analysis and performance monitoring

### 🏢 **Multi-tenant Architecture**

- **Organization Isolation**: Complete separation between different organizations
- **Flexible Access Control**: Token-based authentication with subnet restrictions
- **Resource Quotas**: RPM/TPM limits and usage quotas per group
- **Custom Pricing**: Per-group model pricing and billing configuration

### 🤖 **MCP (Model Context Protocol) Support**

- **Public MCP Servers**: Ready-to-use MCP integrations
- **Organization MCP Servers**: Private MCP servers for organizations
- **Embedded MCP**: Built-in MCP servers with configuration templates
- **OpenAPI to MCP**: Automatic conversion of OpenAPI specs to MCP tools

### 🔌 **Plugin System**

- **Cache Plugin**: High-performance caching for identical requests with Redis/memory storage
- **Web Search Plugin**: Real-time web search capabilities with support for Google, Bing, and Arxiv
- **Think Split Plugin**: Support for reasoning models with content splitting, automatically handling `<think>` tags
- **Stream Fake Plugin**: Avoid non-streaming request timeouts through internal streaming transmission
- **Extensible Architecture**: Easy to add custom plugins for additional functionality

### 🔧 **Advanced Capabilities**

- **Multi-format Support**: Text, image, audio, and document processing
- **Model Mapping**: Flexible model aliasing and routing
- **Prompt Caching**: Intelligent caching with billing support
- **Think Mode**: Support for reasoning models with content splitting
- **Built-in Tokenizer**: No external tiktoken dependencies

## 📊 Management Panel

AI Proxy provides a management panel for managing AI Proxy's configuration and monitoring.

![Dashboard](./docs/images/dashboard.png)
![Logs](./docs/images/logs.png)

## 🏗️ Architecture

```mermaid
graph TB
    Client[Client Applications] --> Gateway[AI Proxy Gateway]
    Gateway --> Auth[Authentication & Authorization]
    Gateway --> Router[Intelligent Router]
    Gateway --> Monitor[Monitoring & Analytics]
    Gateway --> Plugins[Plugin System]
    
    Plugins --> CachePlugin[Cache Plugin]
    Plugins --> SearchPlugin[Web Search Plugin]
    Plugins --> ThinkSplitPlugin[Think Split Plugin]
    Plugins --> StreamFakePlugin[Stream Fake Plugin]

    Router --> Provider1[OpenAI]
    Router --> Provider2[Anthropic]
    Router --> Provider3[Azure OpenAI]
    Router --> ProviderN[Other Providers]
    
    Gateway --> MCP[MCP Servers]
    MCP --> PublicMCP[Public MCP]
    MCP --> GroupMCP[Organization MCP]
    MCP --> EmbedMCP[Embedded MCP]
    
    Monitor --> Alerts[Alert System]
    Monitor --> Analytics[Analytics Dashboard]
    Monitor --> Logs[Audit Logs]
```

## 🚀 Quick Start

### Docker (Recommended)

```bash
# Quick start with default configuration
docker run -d \
  --name aiproxy \
  -p 3000:3000 \
  -v $(pwd)/aiproxy:/aiproxy \
  -e ADMIN_KEY=your-admin-key \
  ghcr.io/labring/aiproxy:latest

# Nightly build
docker run -d \
  --name aiproxy \
  -p 3000:3000 \
  -v $(pwd)/aiproxy:/aiproxy \
  -e ADMIN_KEY=your-admin-key \
  ghcr.io/labring/aiproxy:main
```

### Docker Compose

```bash
# Download docker-compose.yaml
curl -O https://raw.githubusercontent.com/labring/aiproxy/main/docker-compose.yaml

# Start services
docker-compose up -d
```

## 🔧 Configuration

### Environment Variables

#### **Core Settings**

```bash
LISTEN=:3000                    # Server listen address
ADMIN_KEY=your-admin-key        # Admin API key
```

#### **Database Configuration**

```bash
SQL_DSN=postgres://user:pass@host:5432/db    # Primary database
LOG_SQL_DSN=postgres://user:pass@host:5432/log_db  # Log database (optional)
REDIS=redis://localhost:6379     # Redis for caching
```

#### **Feature Toggles**

```bash
BILLING_ENABLED=true           # Enable billing features
SAVE_ALL_LOG_DETAIL=true     # Log all request details
```

### Advanced Configuration

<details>
<summary>Click to expand advanced configuration options</summary>

#### **Quotas**

```bash
GROUP_MAX_TOKEN_NUM=100        # Max tokens per group
```

#### **Logging & Retention**

```bash
LOG_STORAGE_HOURS=168          # Log retention (0 = unlimited)
LOG_DETAIL_STORAGE_HOURS=72    # Detail log retention
CLEAN_LOG_BATCH_SIZE=5000      # Log cleanup batch size
```

#### **Security & Access Control**

```bash
IP_GROUPS_THRESHOLD=5          # IP sharing alert threshold
IP_GROUPS_BAN_THRESHOLD=10     # IP sharing ban threshold
```

</details>

## 🔌 Plugins

AI Proxy supports a plugin system that extends its functionality. Currently available plugins:

### Cache Plugin

The Cache Plugin provides high-performance caching for AI API requests:

- **Dual Storage**: Supports both Redis and in-memory caching
- **Content-based Keys**: Uses SHA256 hash of request body
- **Configurable TTL**: Custom time-to-live for cached items
- **Size Limits**: Prevents memory issues with configurable limits

[View Cache Plugin Documentation](./core/relay/plugin/cache/README.md)

### Web Search Plugin

The Web Search Plugin adds real-time web search capabilities:

- **Multiple Search Engines**: Supports Google, Bing, and Arxiv
- **Smart Query Rewriting**: AI-powered query optimization
- **Reference Management**: Automatic citation formatting
- **Dynamic Control**: User-controllable search depth

[View Web Search Plugin Documentation](./core/relay/plugin/web-search/README.md)

### Think Split Plugin

The Think Split Plugin supports content splitting for reasoning models:

- **Automatic Recognition**: Automatically detects `<think>...</think>` tags in responses
- **Content Separation**: Extracts thinking content to `reasoning_content` field
- **Streaming Support**: Supports both streaming and non-streaming responses

[View Think Split Plugin Documentation](./core/relay/plugin/thinksplit/README.md)

### Stream Fake Plugin

The Stream Fake Plugin solves timeout issues with non-streaming requests:

- **Timeout Avoidance**: Prevents request timeouts through internal streaming transmission
- **Transparent Conversion**: Automatically converts non-streaming requests to streaming format, transparent to clients
- **Response Reconstruction**: Collects all streaming data chunks and reconstructs them into complete non-streaming responses
- **Connection Keep-Alive**: Maintains active connections through streaming transmission to avoid network timeouts

[View Stream Fake Plugin Documentation](./core/relay/plugin/streamfake/README.md)

## 📚 API Documentation

### Interactive API Explorer

Visit `http://localhost:3000/swagger/index.html` for the complete API documentation with interactive examples.

### Quick API Examples

#### **List Available Models**

```bash
curl -H "Authorization: Bearer your-token" \
  http://localhost:3000/v1/models
```

#### **Chat Completion**

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

#### **Claude API**

```bash
# Use Claude models through OpenAI API format
curl -X POST http://localhost:3000/v1/messages \
  -H "X-Api-Key: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5",
    "messages": [{"role": "user", "content": "Hello Claude!"}]
  }'
```

## 🔌 Integrations

### Sealos Platform

Deploy instantly on Sealos with built-in model capabilities:
[Deploy to Sealos](https://hzh.sealos.run/?openapp=system-aiproxy)

### FastGPT Integration

Seamlessly integrate with FastGPT for enhanced AI workflows:
[FastGPT Documentation](https://doc.tryfastgpt.ai/docs/development/modelconfig/ai-proxy/)

### Claude Code Integration

Use AI Proxy with Claude Code by configuring these environment variables:

```bash
export ANTHROPIC_BASE_URL=http://127.0.0.1:3000
export ANTHROPIC_AUTH_TOKEN=sk-xxx
export ANTHROPIC_MODEL=gpt-5
export ANTHROPIC_SMALL_FAST_MODEL=gpt-5-nano
```

### Gemini CLI Integration

Use AI Proxy with Gemini CLI by configuring these environment variables:

```bash
export GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:3000
export GEMINI_API_KEY=sk-xxx
```

Alternatively, you can use the `/auth` command in the Gemini CLI to output the `GEMINI_API_KEY`.

### Codex Integration

Use AI Proxy with Codex by configuring `~/.codex/config.toml`:

```toml
# Recall that in TOML, root keys must be listed before tables.
model = "gpt-4o"
model_provider = "aiproxy"

[model_providers.aiproxy]
# Name of the provider that will be displayed in the Codex UI.
name = "AIProxy"
# The path `/chat/completions` will be amended to this URL to make the POST
# request for the chat completions.
base_url = "http://127.0.0.1:3000/v1"
# If `env_key` is set, identifies an environment variable that must be set when
# using Codex with this provider. The value of the environment variable must be
# non-empty and will be used in the `Bearer TOKEN` HTTP header for the POST request.
env_key = "AIPROXY_API_KEY"
# Valid values for wire_api are "chat" and "responses". Defaults to "chat" if omitted.
wire_api = "chat"
```

**Protocol Conversion Support**:
- **Responses-only models**: AI Proxy automatically converts Chat/Claude/Gemini requests to Responses API format for models that only support the Responses API
- **Multi-protocol access**: Use any protocol (Chat Completions, Claude Messages, or Gemini) to access responses-only models
- **Transparent conversion**: No client-side changes needed - AI Proxy handles protocol translation automatically

### MCP (Model Context Protocol)

AI Proxy provides comprehensive MCP support for extending AI capabilities:

- **Public MCP Servers**: Community-maintained integrations
- **Organization MCP Servers**: Private organizational tools
- **Embedded MCP**: Easy-to-configure built-in functionality
- **OpenAPI to MCP**: Automatic tool generation from API specifications

## 🛠️ Development

### Prerequisites

- Go 1.24+
- Node.js 22+ (for frontend development)
- PostgreSQL (optional, SQLite by default)
- Redis (optional, for caching)

### Building from Source

```bash
# Clone repository
git clone https://github.com/labring/aiproxy.git
cd aiproxy

# Build frontend (optional)
cd web && npm install -g pnpm && pnpm install && pnpm run build && cp -r dist ../core/public/dist/ && cd ..

# Build backend
cd core && go build -o aiproxy .

# Run
./aiproxy
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Ways to Contribute

- 🐛 Report bugs and issues
- 💡 Suggest new features
- 📝 Improve documentation
- 🔧 Submit pull requests
- ⭐ Star the repository

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- OpenAI for the API specification
- The open-source community for various integrations
- All contributors and users of AI Proxy
