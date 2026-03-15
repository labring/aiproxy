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
- **`enterprise/`** — Enterprise extensions (build tag `enterprise`, planned)

## Build & Development Commands

### Backend (Go)

```bash
# Build (from core/)
cd core && go build -trimpath -ldflags "-s -w" -o aiproxy

# Run tests
cd core && go test -v -timeout 30s -count=1 ./...

# Run a single test
cd core && go test -v -timeout 30s -count=1 -run TestFunctionName ./path/to/package/...

# Lint (uses golangci-lint v2 with config at .golangci.yml)
cd core && golangci-lint run --path-mode=abs

# Lint with auto-fix
cd core && golangci-lint run --path-mode=abs --fix

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
# Full build (frontend + backend)
docker build -t aiproxy .
# Exposes port 3000, frontend is embedded into the Go binary via core/public/dist/
```

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

### Data Model (core/model/)

- **`Group`** — Tenant/organization. Has RPM/TPM ratios, balance, available model sets.
- **`Token`** — API key belonging to a Group. Has quota (total + period), models whitelist, subnet restrictions.
- **`Channel`** — Backend AI provider connection. Has type (ChannelType), base URL, API key, priority, model mappings.
- **`ModelConfig`** — Per-model configuration: pricing, RPM/TPM limits, mode type.
- **`GroupModelConfig`** — Per-group overrides for model config (price, limits, retry).
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

~40 provider adaptors in `core/relay/adaptors/`. Each subfolder implements the `adaptor.Adaptor` interface. Channel types are defined in `core/model/chtype.go`. The relay controller selects an adaptor based on the channel type, then calls the adaptor methods in sequence.

### Notification System

`core/common/notify/notify.go` defines a `Notifier` interface. Default implementation is `StdNotifier` (log). `FeishuNotifier` sends to Feishu/Lark webhooks. Set via `notify.SetDefaultNotifier()`.

### Key Configuration

Runtime config via environment variables. Key ones:
- `SQL_DSN` / `LOG_SQL_DSN` — Database connection
- `REDIS_CONN_STRING` — Redis connection
- `ADMIN_KEY` — Admin API authentication
- `FEISHU_WEBHOOK` — Notification webhook

## Linting Rules

The project uses golangci-lint v2 with a comprehensive config at `.golangci.yml`. Key enabled linters: `errcheck`, `govet`, `staticcheck`, `gosec`, `revive`, `prealloc`, `perfsprint`, `modernize`, `wsl_v5`. Formatters: `gci`, `gofmt`, `gofumpt`, `golines`.

## Enterprise Branch Context

This fork maintains an `enterprise` branch for custom features (Feishu SSO, progressive quota tiers, department analytics). Enterprise code lives in `enterprise/` (top-level, separate from `core/`). See `AI_Proxy_企业版产品目标文档.md` for the full product spec.
