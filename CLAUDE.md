# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AI Proxy is a production-ready AI gateway with OpenAI/Anthropic/Gemini-compatible protocols. It routes requests to 40+ AI providers, handles multi-tenant management (Group/Token), quota enforcement, rate limiting, and usage analytics. Forked from labring/aiproxy for enterprise customization.

## Repository Structure

This is a **Go workspace** with three modules (`go.work`):

- **`core/`** — Main backend (Go 1.26, Gin web framework, GORM ORM)
- **`web/`** — Admin panel frontend (React + Vite + TailwindCSS + Radix UI + Zustand)
- **`mcp-servers/`** — MCP (Model Context Protocol) server implementations
- **`openapi-mcp/`** — OpenAPI-to-MCP converter
- **`core/enterprise/`** — Enterprise module (build tag `enterprise`): Feishu SSO, analytics, quota, notifications
- **`enterprise/docs/`** — Enterprise documentation (progress, architecture)
- **`.githooks/`** — Git hooks (pre-commit: build tag check, sensitive info detection, doc sync reminders)

## Build & Development Commands

### Full Build (Recommended)

```bash
# One-click: frontend → swagger → go build -tags enterprise → post-build verification
bash scripts/build.sh

# Options:
bash scripts/build.sh -o /path/to/output    # custom output path
SKIP_FRONTEND=1 bash scripts/build.sh       # skip frontend rebuild
SKIP_SWAGGER=1 bash scripts/build.sh        # skip swagger regeneration
```

### Backend (Go)

```bash
# Build with enterprise tag (ALWAYS use -tags enterprise for this project)
cd core && go build -tags enterprise -trimpath -ldflags "-s -w" -o aiproxy

# Run tests (with enterprise tag)
cd core && go test -tags enterprise -v -timeout 30s -count=1 ./...

# Run a single test
cd core && go test -tags enterprise -v -timeout 30s -count=1 -run TestFunctionName ./path/to/package/...

# Lint (uses golangci-lint v2 with config at .golangci.yml)
cd core && golangci-lint run --path-mode=abs --build-tags=enterprise

# Lint with auto-fix
cd core && golangci-lint run --path-mode=abs --fix --build-tags=enterprise

# Lint all modules via workspace script
bash scripts/golangci-lint-fix.sh

# Generate Swagger docs (from core/)
cd core && sh scripts/swag.sh
```

### Frontend

```bash
cd web
pnpm install
pnpm run dev      # Dev server
pnpm run build    # Production build (outputs to web/dist/)
pnpm run lint     # ESLint
```

### Docker

```bash
# Full build (frontend + backend, includes enterprise)
docker build -t aiproxy .
# Exposes port 3000, frontend is embedded into the Go binary via core/public/dist/

# Local development with docker-compose
docker compose up -d pgsql redis  # Start PostgreSQL + Redis
```

### Production Deployment (Docker, Recommended)

**Standard deployment method — always use `scripts/deploy.sh`:**

```bash
# Full deploy: git pull → docker build → restart → smoke test
ADMIN_KEY=xxx bash scripts/deploy.sh

# Skip git pull (deploy current code)
ADMIN_KEY=xxx bash scripts/deploy.sh --no-pull

# Build only (no restart)
bash scripts/deploy.sh --build-only

# Restart only (use existing image)
ADMIN_KEY=xxx bash scripts/deploy.sh --restart-only
```

**Why Docker is the mandated deployment method:**
- Dockerfile hardcodes `-tags enterprise` — enterprise module cannot be accidentally omitted
- Multi-stage build always rebuilds frontend fresh — no stale assets
- Swagger docs regenerated in build — API docs always current
- Post-build verification checks enterprise symbols + frontend embedding
- Smoke tests verify the running service after restart
- Actual service downtime: ~5-10 seconds (build happens while old container is running)

**NEVER deploy via bare `go build` on the server.** Always use `deploy.sh` or `docker build`.

### MCP Servers

```bash
cd mcp-servers && go test -v -timeout 30s -count=1 ./...
cd mcp-servers && golangci-lint run --path-mode=abs
```

## Architecture

### Request Flow

```
Client → Gin Router → IPBlock → TokenAuth → Distribute → Relay Controller → Adaptor → Provider
                                    ↓              ↓              ↓
                              Group/Token     Rate Limit     Plugin Chain
                              Validation     (RPM/TPM)    (cache, search,
                              + Balance                    thinksplit, etc.)
```

1. **`core/router/`** — Route registration. `relay.go` maps OpenAI-compatible endpoints to controllers.
2. **`core/middleware/auth.go`** — `TokenAuth` validates API key → loads `TokenCache` + `GroupCache` from Redis/DB. `AdminAuth` for `/api/` admin endpoints.
3. **`core/middleware/distributor.go`** — `distribute()` is the central orchestrator: checks group balance, resolves model, enforces RPM/TPM limits via `reqlimit`, adjusts config per group via `GetGroupAdjustedModelConfig()`.
4. **`core/relay/controller/`** — Per-mode handlers (chat, completions, anthropic, gemini, etc.). Calls `Handle()` → `DoHelper()` which orchestrates the adaptor lifecycle.
5. **`core/relay/adaptor/`** — Provider interface. Each provider (openai, anthropic, aws, gemini, etc.) implements the `Adaptor` interface: `GetRequestURL`, `SetupRequestHeader`, `ConvertRequest`, `DoRequest`, `DoResponse`.
6. **`core/relay/plugin/`** — Request/response plugins: `cache`, `web-search`, `thinksplit`, `streamfake`, `patch`, `monitor`, `timeout`.
7. **`core/common/consume/`** — Post-request consumption recording: updates token/group usage, writes logs and summaries.

### Plugin System

Plugins extend request/response processing. Located in `core/relay/plugin/`, they run in a chain during relay.

**Available Plugins:**
- **cache** (`core/relay/plugin/cache/`) — Response caching with Redis/memory backend. SHA256-based cache keys.
- **web-search** (`core/relay/plugin/web-search/`) — Real-time web search (Google/Bing/Arxiv) with AI-powered query rewriting and citation management.
- **thinksplit** (`core/relay/plugin/thinksplit/`) — Extracts `<think>...</think>` tags to `reasoning_content` field for reasoning models.
- **streamfake** (`core/relay/plugin/streamfake/`) — Converts non-streaming to internal streaming to avoid timeout issues.
- **monitor** — Request metrics collection and performance monitoring.
- **timeout** — Request timeout enforcement.
- **patch** — Response patching/modification for compatibility fixes.

**Integration:** Plugins are invoked in `core/relay/controller/` via plugin chain execution. Configuration is per-group via `GroupModelConfig`. Each plugin implements `Plugin` interface with `PreRequest` and `PostResponse` hooks.

### Data Model (core/model/)

- **`Group`** — Tenant/organization. Has RPM/TPM ratios, balance, available model sets.
- **`Token`** — API key belonging to a Group. Has quota (total + period), models whitelist, subnet restrictions.
- **`Channel`** — Backend AI provider connection. Has type (ChannelType), base URL, proxy URL, API key, priority, model mappings.
- **`ModelConfig`** — Per-model configuration: pricing, RPM/TPM limits, mode type, request/response body storage size limits.
- **`GroupModelConfig`** — Per-group overrides for model config (price, limits, retry, timeout, body storage size).
- **`GroupSummary`** — Hourly usage aggregation by (group_id, token_name, model).
- **`Log`** — Individual request log with full details.

### Caching Layer (core/model/cache.go)

Two-tier cache: Redis (primary) + in-memory fallback. Key patterns:
- `token:<key>` → TokenCache
- `group:<id>` → GroupCache
- Model configs loaded in bulk and cached in `ModelCaches` (atomic pointer swap every 3 min).

### Database

Supports PostgreSQL (primary) and SQLite (default fallback). Set via `SQL_DSN` env var. Log data can use a separate DB via `LOG_SQL_DSN`. Code checks `common.UsingSQLite` for SQL dialect differences (ILIKE vs LIKE).

### Multi-Provider Adaptor System

~40 provider adaptors in `core/relay/adaptor/`. Each subfolder implements the `adaptor.Adaptor` interface and self-registers via `init()` using `registry.Register(channelType, &Adaptor{})`. Channel types are defined in `core/model/chtype.go`. The relay controller selects an adaptor based on the channel type, then calls the adaptor methods in sequence.

**Adaptor Registration Pattern** (since upstream #503):
```go
// core/relay/adaptor/myprovider/adaptor.go
func init() {
    registry.Register(model.ChannelTypeMyProvider, &Adaptor{})
}
```
All adaptors are imported as blank imports (`_ "..."`) in `core/relay/adaptors/register.go`, which triggers their `init()`. The `registry` package (`core/relay/adaptor/registry/`) provides `Register()`, `Get()`, `Snapshot()`, and `SortedTypes()`.

### Protocol Conversion

AI Proxy transparently converts between multiple AI API protocols:
- **OpenAI Chat Completions** ↔ **Claude Messages** ↔ **Gemini** ↔ **OpenAI Responses API**

Handled in `core/relay/adaptor/`. This enables:
- Using responses-only models (e.g., `gpt-4.5-*`) with any protocol
- Accessing Claude models via OpenAI SDK
- Unified interface for multi-model applications

Conversion logic is implemented in each provider's adaptor (`ConvertRequest`/`DoResponse` methods).

**Native Mode Preference:** When a model exists in multiple channels (e.g., both an OpenAI-type and Anthropic-type channel), the channel selector prefers channels that natively handle the request protocol without conversion. Adaptors implement the optional `NativeModeChecker` interface (`core/relay/adaptor/interface.go`) to declare which modes they handle natively. For example, an Anthropic protocol request will prefer an Anthropic-type channel (passthrough) over an OpenAI-type channel (which would require Anthropic→OpenAI conversion). Channels requiring conversion are used as fallback when no native channel is available. See `filterNativeChannels()` in `core/controller/relay-channel.go`.

### Notification System

`core/common/notify/notify.go` defines a `Notifier` interface. Default implementation is `StdNotifier` (log). `FeishuNotifier` sends to Feishu/Lark webhooks. Set via `notify.SetDefaultNotifier()`.

### MCP (Model Context Protocol) Support

AI Proxy provides comprehensive MCP server support. See `mcp-servers/README.md` for full details.

**Three MCP Types:**
1. **Embedded MCP** — Native Go implementations in `core/mcpservers/`. Registered via `init()`, run in-process with zero network latency. High performance, type-safe.
2. **Public MCP** — Community-maintained servers in `mcp-servers/hosted/`.
3. **Organization MCP** — Private organizational servers in `mcp-servers/local/`.

**Creating an Embedded MCP Server:**

```go
// core/mcpservers/my-server/server.go
func init() {
    mcpservers.Register(mcpservers.EmbedMcp{
        ID:              "my-server",
        Name:            "My Server",
        NewServer:       NewServer,
        ConfigTemplates: configTemplates, // Configuration schema with validators
        Tags:            []string{"example"},
        Readme:          "Server description",
    })
}

func NewServer(config map[string]string, reusingConfig map[string]string) (*server.MCPServer, error) {
    // config: init-time configuration (set once globally)
    // reusingConfig: per-group configuration (can vary by tenant)
    mcpServer := server.NewMCPServer("my-server", server.WithMCPCapabilities(...))
    mcpServer.AddTool(server.Tool{...}, handler)
    return mcpServer, nil
}
```

Register in `core/mcpservers/mcpregister/init.go` by adding import.

**Key APIs:**
- `GET /api/embedmcp/` — List all embedded MCP servers with config templates
- `POST /api/embedmcp/` — Configure/enable a server
- `GET /api/test-embedmcp/{id}/sse` — Test SSE connection with query params `config[key]=value`

**Configuration Types:**
- `ConfigRequiredTypeInitOnly` — Required once at server initialization
- `ConfigRequiredTypeReusingOnly` — Required per-group, varies by tenant
- `ConfigRequiredTypeInitOrReusingOnly` — Mutually exclusive: either init or reusing
- `ConfigRequiredTypeInitOptional` / `ConfigRequiredTypeReusingOptional` — Optional configs

**Example:** `aiproxy-openapi` server exposes AI Proxy's REST API as MCP tools (get_channels, create_token, etc.).

### Frontend Architecture (web/)

Built with **React 19 + Vite + TailwindCSS + Radix UI + Zustand**.

**Key Structure:**
- `src/routes/` — React Router v7 config. `config.tsx` defines all routes, `index.tsx` creates router with error boundaries.
- `src/pages/` — Page components organized by feature:
  - `token/`, `group/`, `channel/`, `model/` — Core admin pages
  - `enterprise/` — 5 enterprise pages (dashboard, ranking, department, quota, custom-report)
  - `mcp/` — MCP server management (public/org/embedded with config UI)
  - `log/`, `monitor/` — Request logs and monitoring
  - `auth/` — Login page and Feishu OAuth callback
- `src/store/` — Zustand stores with persistence (`auth.ts` contains user session + enterprise user info)
- `src/api/` — API client functions organized by module (e.g., `enterprise.ts` has all 9 analytics APIs)
- `src/components/` — Reusable UI components built on Radix UI design system
  - `layout/` — `SideBar`, `EnterpriseLayout` (purple theme for enterprise pages)
  - `ui/` — shadcn-style components (Button, Table, Dialog, etc.)
- `src/lib/` — Utility functions (time formatting, number formatting, API helpers)

**Routing:** Uses `createBrowserRouter` with recursive error boundary injection. Admin pages use default layout, enterprise pages use `EnterpriseLayout`.

**State Management:** Minimal Zustand usage - currently only `auth.ts` with `persist` middleware. Most state is server-driven via React Query.

**Styling:** TailwindCSS + CSS variables for theming. Enterprise pages use purple gradient (`bg-gradient-to-br from-purple-50 to-blue-50`).

### Key Configuration

Runtime config via environment variables. Key ones:
- `SQL_DSN` / `LOG_SQL_DSN` — Database connection
- `REDIS_CONN_STRING` — Redis connection
- `ADMIN_KEY` — Admin API authentication
- `FEISHU_WEBHOOK` — Notification webhook

## Linting Rules

The project uses golangci-lint v2 with a comprehensive config at `.golangci.yml`. Key enabled linters: `errcheck`, `govet`, `staticcheck`, `gosec`, `revive`, `prealloc`, `perfsprint`, `modernize`, `wsl_v5`. Formatters: `gci`, `gofmt`, `gofumpt`, `golines`.

## Enterprise Module

**IMPORTANT:** Enterprise features require build tag: `go build -tags enterprise -o aiproxy`

Located in `core/enterprise/` with dedicated frontend pages in `web/src/pages/enterprise/`. See `enterprise/docs/` for detailed architecture and progress tracking.

**Key Subsystems:**
- **feishu/** — Feishu/Lark OAuth SSO, tenant whitelist validation (`FEISHU_ALLOWED_TENANTS`), organization sync (scheduled task).
- **analytics/** — 9 API endpoints: department summaries, user/department ranking, trends, model distribution, period comparison, custom reports (multi-dimension aggregation), Excel export (4 sheets).
- **quota/** — Progressive quota tier policies with request hook enforcement. CRUD APIs + user/department assignment.
- **notify/** — Feishu P2P notifications and quota alerts integration.

**Frontend Routes:**
- `/enterprise` — Dashboard (metrics cards, department table, model distribution pie chart, period-over-period indicators)
- `/enterprise/ranking` — Employee ranking (9-column table with filters, sorting, column visibility, export)
- `/enterprise/department` — Department trend details (ECharts line/bar charts)
- `/enterprise/quota` — Quota policy management (CRUD + assignment)
- `/enterprise/custom-report` — Custom reports (dimension/measure selector, pivot table, charts, templates, CSV export)

**Critical Build Requirements:**
- All `core/enterprise/*.go` files MUST have `//go:build enterprise` as the first line (enforced by pre-commit hook)
- Frontend pages use `EnterpriseLayout` (purple gradient sidebar)
- Data queries: `model.LogDB` for GroupSummary, `model.DB` for FeishuUser/QuotaTier
- `/api/status` returns `isEnterprise: true` when built with enterprise tag (see `core/controller/misc_enterprise.go`)

**Environment Variables:**
- `FEISHU_APP_ID`, `FEISHU_APP_SECRET` — Feishu app credentials
- `FEISHU_REDIRECT_URI` — OAuth callback URL (e.g., `https://api.example.com/api/enterprise/auth/feishu/callback`)
- `FEISHU_FRONTEND_URL` — Frontend base URL for redirects
- `FEISHU_ALLOWED_TENANTS` — Comma-separated tenant whitelist (e.g., `tenant_abc,tenant_def`)

**Key Gotchas:**
- Auth store field is `isAuthenticated`, persist key is `auth-storage`
- `hour_timestamp` is Unix seconds, frontend must `* 1000` for JS Date
- Zero division protection required for all percentage calculations
- TFunction type from `i18next` (not `react-i18next`), use `as never` for dynamic keys

## User Preferences

- **Language**: 中文交流，代码注释用英文
- **Commit Style**: 中文 commit message，遵循 Conventional Commits
- **Code Style**: 遵循 golangci-lint 规则，自动格式化
- **Testing**: 修改代码后运行相关测试验证
- **Compliance**: 系统组件、依赖环境、第三方库等，均需要评估商业应用的合规性（许可证兼容性、安全性、长期维护性）

## Git Hooks

Git hooks are in `.githooks/` (activated via `git config core.hooksPath .githooks`).

**Setup (run once after clone):**
```bash
bash scripts/setup-hooks.sh
```

**Pre-commit checks:**
1. **Enterprise build tag** (BLOCKING) — `core/enterprise/*.go` must have `//go:build enterprise` as first line
2. **Sensitive info detection** (BLOCKING) — Blocks hardcoded API keys (`sk-` 20+ chars), passwords, internal IPs
3. **Documentation sync** (non-blocking) — Reminds when code changes may require doc updates

**Code → Document mapping:**
- `core/router/`, `core/middleware/`, `core/relay/`, `core/model/`, `web/src/` → `CLAUDE.md`
- `core/enterprise/` → `enterprise/docs/progress.md`
- `Dockerfile`, `docker-compose.yaml` → `DEPLOYMENT.md`
- `core/common/config/` → `config.md`
- `web/src/pages/enterprise/`, `core/enterprise/auth|feishu|quota/`, `core/router/relay.go`, `core/model/chtype.go` → `USER-GUIDE.md`

## Quick Commands

```bash
# 一键完整构建（本地开发）
bash scripts/build.sh

# 一键启动开发环境
docker compose up -d pgsql redis && cd core && go run -tags enterprise .

# 快速测试
cd core && go test -tags enterprise -v -run TestXxx ./...

# 提交前检查
cd core && golangci-lint run --fix --build-tags=enterprise && go test -tags enterprise ./...

# 生产部署（唯一标准方式）
ADMIN_KEY=xxx bash scripts/deploy.sh

# 部署后验证
ADMIN_KEY=xxx bash scripts/smoke-test.sh https://your-server
```
