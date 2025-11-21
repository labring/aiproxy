<div align="center">
  <h1>AI Proxy</h1>
  <p>新一代 AI 网关，采用 OpenAI 兼容协议</p>
  
  [![Release](https://img.shields.io/github/release/labring/aiproxy)](https://github.com/labring/aiproxy/releases)
  [![License](https://img.shields.io/github/license/labring/aiproxy)](https://github.com/labring/aiproxy/blob/main/LICENSE)
  [![Go Version](https://img.shields.io/github/go-mod/go-version/labring/aiproxy?filename=core%2Fgo.mod)](https://github.com/labring/aiproxy/blob/main/core/go.mod)
  [![Build Status](https://img.shields.io/github/actions/workflow/status/labring/aiproxy/release.yml?branch=main)](https://github.com/labring/aiproxy/actions)

  [English](./README.md) | [简体中文](./README.zh.md)
</div>

---

## 🚀 项目概述

AI Proxy 是一个强大的、生产就绪的 AI 网关，提供智能请求路由、全面监控和无缝多租户管理。基于 OpenAI、Anthropic 和 Gemini 兼容协议构建，是需要可靠性、可扩展性和高级功能的 AI 应用的完美中间件。

## ✨ 核心特性

### 🔄 **智能请求管理**

- **智能重试机制**：智能重试策略与自动错误恢复
- **基于优先级的渠道选择**：根据渠道优先级和错误率路由请求
- **负载均衡**：高效地在多个 AI 提供商之间分配流量
- **协议转换**：在 OpenAI Chat Completions、Claude Messages、Gemini 和 OpenAI Responses API 之间无缝转换
  - Chat/Claude/Gemini → Responses API：使用任意协议访问仅支持 Responses 的模型

### 📊 **全面监控与分析**

- **实时告警**：余额预警、错误率和异常等主动通知
- **详细日志**：完整的请求/响应跟踪和审计轨迹
- **高级分析**：请求量、错误统计、RPM/TPM 指标和成本分析
- **渠道性能**：错误率分析和性能监控

### 🏢 **多租户架构**

- **组织隔离**：不同组织间的完全分离
- **灵活访问控制**：基于令牌的身份验证和子网限制
- **资源配额**：每组的 RPM/TPM 限制和使用配额
- **自定义定价**：每组模型定价和计费配置

### 🤖 **MCP (模型上下文协议) 支持**

- **公共 MCP 服务器**：开箱即用的 MCP 集成
- **组织 MCP 服务器**：组织专用的私有 MCP 服务器
- **嵌入式 MCP**：带配置模板的内置 MCP 服务器
- **OpenAPI 转 MCP**：自动将 OpenAPI 规范转换为 MCP 工具

### 🔌 **插件系统**

- **缓存插件**：高性能缓存，支持 Redis/内存存储，用于相同请求
- **网络搜索插件**：实时网络搜索功能，支持 Google、Bing 和 Arxiv
- **思考模式插件**：支持推理模型的内容分割，自动处理 `<think>` 标签
- **流式伪装插件**：通过内部流式传输避免非流式请求超时问题
- **可扩展架构**：易于添加自定义插件以实现额外功能

### 🔧 **高级功能**

- **多格式支持**：文本、图像、音频和文档处理
- **模型映射**：灵活的模型别名和路由
- **提示词缓存**：智能缓存和计费支持
- **思考模式**：支持推理模型的内容分割
- **内置分词器**：无需外部 tiktoken 依赖

## 📊 管理面板

AI Proxy 提供了一个管理面板，用于管理 AI Proxy 的配置和监控。

![面板](./docs/images/dashboard.png)
![日志](./docs/images/logs.png)

## 🏗️ 架构图

```mermaid
graph TB
    Client[客户端应用] --> Gateway[AI Proxy 网关]
    Gateway --> Auth[身份验证与授权]
    Gateway --> Router[智能路由器]
    Gateway --> Monitor[监控与分析]
    Gateway --> Plugins[插件系统]
    
    Plugins --> CachePlugin[缓存插件]
    Plugins --> SearchPlugin[网络搜索插件]
    Plugins --> ThinkSplitPlugin[思考模式插件]
    Plugins --> StreamFakePlugin[流式伪装插件]
    
    Router --> Provider1[OpenAI]
    Router --> Provider2[Anthropic]
    Router --> Provider3[Azure OpenAI]
    Router --> ProviderN[其他提供商]
    
    Gateway --> MCP[MCP 服务器]
    MCP --> PublicMCP[公共 MCP]
    MCP --> GroupMCP[组织 MCP]
    MCP --> EmbedMCP[嵌入式 MCP]
    
    Monitor --> Alerts[告警系统]
    Monitor --> Analytics[分析仪表板]
    Monitor --> Logs[审计日志]
```

## 🚀 快速开始

### Docker（推荐）

```bash
# 使用默认配置快速启动
docker run -d \
  --name aiproxy \
  -p 3000:3000 \
  -v $(pwd)/aiproxy:/aiproxy \
  -e ADMIN_KEY=your-admin-key \
  registry.cn-hangzhou.aliyuncs.com/labring/aiproxy:latest

# 夜间构建
docker run -d \
  --name aiproxy \
  -p 3000:3000 \
  -v $(pwd)/aiproxy:/aiproxy \
  -e ADMIN_KEY=your-admin-key \
  registry.cn-hangzhou.aliyuncs.com/labring/aiproxy:main
```

### Docker Compose

```bash
# 下载 docker-compose.yaml
curl -O https://raw.githubusercontent.com/labring/aiproxy/main/docker-compose.yaml

# 启动服务
docker-compose up -d
```

## 🔧 配置说明

### 环境变量

#### **核心设置**

```bash
LISTEN=:3000                    # 服务器监听地址
ADMIN_KEY=your-admin-key        # 管理员 API 密钥
```

#### **数据库配置**

```bash
SQL_DSN=postgres://user:pass@host:5432/db    # 主数据库
LOG_SQL_DSN=postgres://user:pass@host:5432/log_db  # 日志数据库（可选）
REDIS=redis://localhost:6379     # Redis 缓存
```

#### **功能开关**

```bash
BILLING_ENABLED=true           # 启用计费功能
SAVE_ALL_LOG_DETAIL=true     # 记录所有请求详情
```

### 高级配置

<details>
<summary>点击展开高级配置选项</summary>

#### **配额**

```bash
GROUP_MAX_TOKEN_NUM=100        # 每组最大令牌数
```

#### **日志与保留**

```bash
LOG_STORAGE_HOURS=168          # 日志保留时间（0 = 无限制）
LOG_DETAIL_STORAGE_HOURS=72    # 详细日志保留时间
CLEAN_LOG_BATCH_SIZE=5000      # 日志清理批次大小
```

#### **安全与访问控制**

```bash
IP_GROUPS_THRESHOLD=5          # IP 共享告警阈值
IP_GROUPS_BAN_THRESHOLD=10     # IP 共享禁用阈值
```

</details>

## 🔌 插件

AI Proxy 支持插件系统来扩展其功能。当前可用的插件：

### 缓存插件

缓存插件为 AI API 请求提供高性能缓存：

- **双重存储**：支持 Redis 和内存缓存
- **基于内容的键**：使用请求体的 SHA256 哈希
- **可配置 TTL**：缓存项的自定义生存时间
- **大小限制**：通过可配置限制防止内存问题

[查看缓存插件文档](./core/relay/plugin/cache/README.zh.md)

### 网络搜索插件

网络搜索插件添加实时网络搜索功能：

- **多搜索引擎**：支持 Google、Bing 和 Arxiv
- **智能查询重写**：AI 驱动的查询优化
- **引用管理**：自动引用格式化
- **动态控制**：用户可控的搜索深度

[查看网络搜索插件文档](./core/relay/plugin/web-search/README.zh.md)

### 思考模式插件

思考模式插件支持思考模式的内容分割：

- **自动识别**：自动检测响应中的 `<think>...</think>` 标签
- **内容分离**：将思考内容提取到 `reasoning_content` 字段
- **流式支持**：支持流式和非流式响应处理

[查看思考模式插件文档](./core/relay/plugin/thinksplit/README.zh.md)

### 流式伪装插件

流式伪装插件解决非流式请求的超时问题：

- **超时避免**：通过内部流式传输避免长时间等待导致的超时
- **透明转换**：自动将非流式请求转换为流式格式，客户端无感知
- **响应重构**：收集所有流式数据块并重构为完整的非流式响应
- **连接保持**：通过流式传输保持连接活跃，避免网络超时

[查看流式伪装插件文档](./core/relay/plugin/streamfake/README.cn.md)

## 📚 API 文档

### 交互式 API 浏览器

访问 `http://localhost:3000/swagger/index.html` 查看完整的 API 文档和交互示例。

### 快速 API 示例

#### **列出可用模型**

```bash
curl -H "Authorization: Bearer your-token" \
  http://localhost:3000/v1/models
```

#### **聊天补全**

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "你好！"}]
  }'
```

#### **使用 Claude API**

```bash
# 通过 OpenAI API 格式使用 Claude 模型
curl -X POST http://localhost:3000/v1/messages \
  -H "X-Api-Key: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5",
    "messages": [{"role": "user", "content": "你好 Claude！"}]
  }'
```

## 🔌 集成方案

### Sealos 平台

在 Sealos 上一键部署，享受内置模型能力：
[部署到 Sealos](https://hzh.sealos.run/?openapp=system-aiproxy)

### FastGPT 集成

与 FastGPT 无缝集成，增强 AI 工作流：
[FastGPT 文档](https://doc.tryfastgpt.ai/docs/development/modelconfig/ai-proxy/)

### Claude Code 集成

通过配置以下环境变量在 Claude Code 中使用 AI Proxy：

```bash
export ANTHROPIC_BASE_URL=http://127.0.0.1:3000
export ANTHROPIC_AUTH_TOKEN=sk-xxx
export ANTHROPIC_MODEL=gpt-5
export ANTHROPIC_SMALL_FAST_MODEL=gpt-5-nano
```

### Gemini CLI 集成

通过配置以下环境变量在 Gemini CLI 中使用 AI Proxy：

```bash
export GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:3000
export GEMINI_API_KEY=sk-xxx
```

或者在 Gemini CLI 中使用 `/auth` 命令来输出 `GEMINI_API_KEY`

### Codex 集成

通过配置 `~/.codex/config.toml` 在 Codex 中使用 AI Proxy：

```toml
# 请记住，在 TOML 中，根键必须列在表之前。
model = "gpt-4o"
model_provider = "aiproxy"

[model_providers.aiproxy]
# 提供商名称，将显示在 Codex UI 中。
name = "AIProxy"
# 路径 `/chat/completions` 将被追加到此 URL 以发起 POST
# 请求进行聊天补全。
base_url = "http://127.0.0.1:3000/v1"
# 如果设置了 `env_key`，则标识使用此提供商时必须设置的环境变量。
# 环境变量的值必须非空，并将用于 POST 请求的 `Bearer TOKEN` HTTP 头中。
env_key = "AIPROXY_API_KEY"
# wire_api 的有效值为 "chat" 和 "responses"。如果省略，默认为 "chat"。
wire_api = "chat"
```

**协议转换支持**：
- **仅支持 Responses 的模型**：AI Proxy 自动将 Chat/Claude/Gemini 请求转换为 Responses API 格式，支持仅提供 Responses API 的模型
- **多协议访问**：使用任意协议（Chat Completions、Claude Messages 或 Gemini）访问仅支持 Responses 的模型
- **透明转换**：无需客户端修改 - AI Proxy 自动处理协议转换

### MCP (模型上下文协议)

AI Proxy 提供全面的 MCP 支持，扩展 AI 能力：

- **公共 MCP 服务器**：社区维护的集成
- **组织 MCP 服务器**：私有组织工具
- **嵌入式 MCP**：易于配置的内置功能
- **OpenAPI 转 MCP**：从 API 规范自动生成工具

## 🛠️ 开发指南

### 前置要求

- Go 1.24+
- Node.js 22+（前端开发）
- PostgreSQL（可选，默认 SQLite）
- Redis（可选，用于缓存）

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/labring/aiproxy.git
cd aiproxy

# 构建前端（可选）
cd web && npm install -g pnpm && pnpm install && pnpm run build && cp -r dist ../core/public/dist/ && cd ..

# 构建后端
cd core && go build -o aiproxy .

# 运行
./aiproxy
```

## 🤝 参与贡献

我们欢迎贡献！请查看我们的[贡献指南](CONTRIBUTING.md)了解详情。

### 贡献方式

- 🐛 报告错误和问题
- 💡 建议新功能
- 📝 改进文档
- 🔧 提交拉取请求
- ⭐ 为仓库点星

## 📄 开源协议

本项目采用 MIT 协议 - 详见 [LICENSE](LICENSE) 文件。

## 🙏 致谢

- OpenAI 提供的 API 规范
- 开源社区的各种集成贡献
- 所有 AI Proxy 的贡献者和用户
