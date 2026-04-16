# AI Proxy 生产部署指南

> 双域名双节点架构 | 多节点部署（国内腾讯云 + 海外 AWS） | CLB/ALB + Nginx | 410 人规模
>
> 国内域名（`paigod.work`）：
> - `ai.paigod.work` — 管理后台（内网）
> - `apiproxy.paigod.work` — AI API 服务（公网）
>
> 海外域名（`pplabs.tech`）：
> - `ai.pplabs.tech` — 企业前端（公网，Nginx 路径隔离）
> - `apiproxy.pplabs.tech` — AI API 服务（公网）

---

## 一、架构总览

```
  ┌─────────────────────────────────────────────────────────────────────────────┐
  │                    国内节点（主 · 腾讯云）paigod.work                         │
  │                                                                             │
  │   企业内网             腾讯云 CLB (SSL 终结)                                  │
  │   ┌──────────────┐   ai.paigod.work:443 → :80                             │
  │   │ 员工浏览器     │   apiproxy.paigod.work:443 → :81                       │
  │   │ ai.paigod.    │──→ ┌───────────┐    ┌──────────────┐                   │
  │   │  work         │    │  Nginx    │──→│  AI Proxy     │──→ PPIO / 各提供商 │
  │   └──────────────┘    │  :80 内网  │    │  :3000/:3001 │                   │
  │   ┌───────────────┐   │  :81 公网  │    │              │                   │
  │   │ 国内 AI 客户端  │──→│  路径分流   │    │ 单进程，同时   │                   │
  │   │apiproxy.      │   └───────────┘    │ 提供前端+API  │                   │
  │   │ paigod.work   │                    └──────┬───────┘                   │
  │   └───────────────┘   ┌────────┐    ┌────────┴───────┐                   │
  │                        │ Redis  │    │  PostgreSQL    │                   │
  │                        │ :6379  │    │  :5432         │◄────┐             │
  │                        └────────┘    └────────────────┘     │             │
  └─────────────────────────────────────────────────────────────┼─────────────┘
                                                                │
                                                     WireGuard 隧道
                                                     (10.x.x.x 内网)
                                                                │
  ┌─────────────────────────────────────────────────────────────┼─────────────┐
  │                    海外节点（从 · AWS 美西）pplabs.tech         │             │
  │                                                              │             │
  │   ┌───────────────┐   AWS ALB (SSL 终结)                      │             │
  │   │ 海外员工浏览器   │   ai.pplabs.tech:443 → :80              │             │
  │   │ ai.pplabs.tech│   apiproxy.pplabs.tech:443 → :81         │             │
  │   └───────────────┘──→ ┌───────────┐    ┌──────────────┐    │             │
  │   ┌───────────────┐   │  Nginx    │──→│  AI Proxy     │──→ 海外直连渠道   │
  │   │ 海外 AI 客户端  │──→│  :80 前端  │    │  :3000/:3001 │    │             │
  │   │apiproxy.      │   │  :81 API  │    │  NODE_CHANNEL │    │             │
  │   │ pplabs.tech   │   └───────────┘    │  _SET=overseas│────┘             │
  │   └───────────────┘   ┌────────┐       └──────────────┘  连回国内 DB      │
  │                        │ Redis  │                                          │
  │                        │ :6379  │  (本地缓存)                               │
  │                        └────────┘                                          │
  └─────────────────────────────────────────────────────────────────────────────┘
```

### 核心设计：CLB/ALB SSL 终结 + Nginx 按端口做路径级隔离

#### 国内节点（腾讯云，`paigod.work`）

| 域名 | 网络 | LB 监听 | Nginx 端口 | 开放路径 | 用途 |
|------|------|---------|-----------|---------|------|
| `ai.paigod.work` | 内网 | CLB `:443 → CVM:80` | `:80` | 全部（`/`, `/api/*`, `/swagger/*`） | 管理后台、飞书登录、企业分析 |
| `apiproxy.paigod.work` | 公网 | CLB `:443 → CVM:81` | `:81` | 仅 `/v1/*`, `/mcp/*`, `/sse`, `/message` | AI 模型调用、MCP 协议 |

#### 海外节点（AWS 美西，`pplabs.tech`）

| 域名 | 网络 | LB 监听 | Nginx 端口 | 开放路径 | 用途 |
|------|------|---------|-----------|---------|------|
| `ai.pplabs.tech` | 公网（路径隔离） | ALB `:443 → EC2:80` | `:80` | `/api/enterprise/*`, `/api/status`, `/`（SPA）；阻断 `/api/*`、`/swagger/*`、`/v1/*` 等 | 企业前端 + 飞书 SSO（海外员工使用） |
| `apiproxy.pplabs.tech` | 公网 | ALB `:443 → EC2:81` | `:81` | 仅 `/v1/*`, `/v1beta/*`, `/mcp/*`, `/sse`, `/mcp`, `/message` | 海外 AI 模型调用 |

---

## 二、服务器配置采购

### 腾讯云 CVM

| 项目 | 推荐配置 |
|------|---------|
| **机型** | 标准型 S5.2XLARGE16（8 vCPU / 16 GB） |
| **系统盘** | 高性能云硬盘 80 GB |
| **数据盘** | SSD 云硬盘 100 GB（挂载至 `/data`，存放 PostgreSQL + Redis 数据） |
| **操作系统** | Ubuntu 22.04 LTS / Debian 12 |
| **公网带宽** | 按流量计费，带宽上限 100 Mbps |
| **安全组** | 入站开放 80/81（仅来自 CLB），22（SSH 限管理员 IP） |
| **地域** | 上海/北京（离办公地近；出海调 API 可选香港/新加坡） |

### 服务器登录方式

> 服务器通过 JumpServer 堡垒机管理，支持 Web 终端和 SSH 两种登录方式。

**方式一：JumpServer Web 终端（Luna）**

直接浏览器访问：
```
https://jump-new.paigod.work/luna/?login_to=fa179797-95eb-4a78-a4b1-d5621d9cfa17&oid=00000000-0000-0000-0000-000000000002
```

> 需先登录 JumpServer（`https://jump-new.paigod.work`），拥有该资产的连接权限。

**方式二：SSH 直连**

服务器公网 IP `1.13.81.31`，端口 22，仅支持密钥认证（`PermitRootLogin no`，使用 `ppuser` 用户）：

```bash
# 连接服务器（ppuser 拥有 sudo 权限）
ssh ppuser@1.13.81.31

# 需要 root 权限时使用 sudo
ssh ppuser@1.13.81.31 "sudo <command>"
```

> **[安全提醒]**
> - 服务器禁用了 root SSH 登录，所有操作通过 `ppuser` + `sudo` 执行
> - SSH 公钥需提前添加至服务器 `/home/ppuser/.ssh/authorized_keys`
> - 安全组应限制 22 端口仅允许管理员 IP 访问

### 云数据库（推荐，免运维）

| 组件 | 规格 | 说明 |
|------|------|------|
| **PostgreSQL** | 云数据库 2C4G 基础版 | 主库 + 日志库 |
| **Redis** | 标准版 2GB | Token/模型缓存 |

> **省钱方案：** 不购买云数据库，在 CVM 上用 Docker 自建 PostgreSQL + Redis（本文档以此方案为主）。

### 实际部署信息

> 以下为当前生产环境的实际配置，供运维参考。

#### 国内节点（主节点）

| 项目 | 值 |
|------|-----|
| **CVM 公网 IP** | `1.13.81.31` |
| **CVM 内网 IP** | `10.206.0.10` |
| **操作系统** | Ubuntu 22.04.5 LTS |
| **登录用户** | `ppuser`（sudo 权限，密钥认证） |
| **AI Proxy 运行方式** | Docker 容器 `aiproxy-active`（零停机部署，端口交替 3000/3001） |
| **AI Proxy 服务端口** | 当前活跃端口见 `/data/aiproxy/.active-port`（仅监听本地，不直接暴露） |
| **Nginx 端口 80** | 反代至 `upstream aiproxy_backend` — 内网管理后台 |
| **Nginx 端口 81** | 反代至 `upstream aiproxy_backend` — 公网 AI API（白名单路径） |
| **PostgreSQL** | Docker 容器，`127.0.0.1:5432` |
| **Redis** | Docker 容器，`127.0.0.1:6379` |
| **代码部署方式** | GitHub SSH Deploy Key，仓库 `mashoushan1989/aiproxy` |
| **代码路径** | `/data/aiproxy` |
| **Docker 镜像** | `aiproxy:local`（当前），`aiproxy:rollback`（上一版），`aiproxy:sha-<commit>`（Git SHA 追溯） |
| **部署脚本** | `scripts/deploy.sh`（默认零停机，`--legacy` 走旧模式） |
| **数据库备份** | 每日 03:00 自动备份至 `/data/backup/`，保留 30 天 |

#### 海外节点（从节点 · AWS 美西）

| 项目 | 值 |
|------|-----|
| **云平台** | AWS（美西 us-west-2），EC2 实例 |
| **EC2 公网 IP** | `52.35.158.131` |
| **EC2 内网 IP** | `10.195.9.13`（AWS VPC 内网） |
| **WireGuard 内网 IP** | `10.0.0.2`（对端 `10.0.0.1` 为国内节点） |
| **操作系统** | Ubuntu 22.04 LTS |
| **登录用户** | `ppuser`（sudo 权限，密钥认证） |
| **域名** | `ai.pplabs.tech`（企业前端，公网路径隔离）+ `apiproxy.pplabs.tech`（API） |
| **AI Proxy 运行方式** | Docker 容器 `aiproxy-active`（同国内零停机模式） |
| **负载均衡** | AWS ALB（SSL 终结，ACM 证书），转发到 EC2:80（企业前端）/ EC2:81（API） |
| **Nginx 端口 80** | 反代至 `upstream aiproxy_backend` — 企业前端（路径隔离，公网可达） |
| **Nginx 端口 81** | 反代至 `upstream aiproxy_backend` — 公网 AI API（白名单路径） |
| **PostgreSQL** | 通过 WireGuard 隧道连回国内主库（`10.0.0.1:5432`） |
| **Redis** | Docker 容器，`127.0.0.1:6379`（本地缓存） |
| **NODE_CHANNEL_SET** | `overseas`（优先使用海外渠道，无匹配时回落默认） |
| **NODE_TYPE** | `overseas`（构建时使用官方镜像源，非国内加速） |
| **代码路径** | `/data/aiproxy` |
| **WireGuard 健康检查** | `scripts/wireguard-health.sh`，crontab 每分钟执行。隧道恢复后自动重启 aiproxy-active 刷新 DB 连接池。日志：`/var/log/wireguard-health.log` |
| **部署方式** | `scripts/deploy-all.sh` 统一部署，或 SSH 单独部署 |

---

## 三、CLB 负载均衡与 SSL

> **核心思路：** SSL 证书统一托管在腾讯云 CLB（负载均衡）上，Nginx 只处理 HTTP。证书到期由腾讯云自动续签，无需手动运维。

### 域名 → 端口映射关系（申请权限用）

```
请求链路：
  用户浏览器/客户端
       │
       ▼
  CLB (HTTPS:443, SSL 终结)
       │
       ├── ai.paigod.work       → CVM 1.13.81.31:80  (Nginx) → 127.0.0.1:3000 (AI Proxy)
       │                           内网管理后台，开放全部路径
       │
       └── apiproxy.paigod.work  → CVM 1.13.81.31:81  (Nginx) → 127.0.0.1:3000 (AI Proxy)
                                    公网 AI API，仅开放 /v1/* /mcp/* /sse /message
```

| 域名 | 网络 | CLB 入站 | CVM 目标端口 | 服务端口 | 用途 | 开放路径 |
|------|------|---------|-------------|---------|------|---------|
| `ai.paigod.work` | **内网** | HTTPS:443 | **80** | 3000 | 管理后台、飞书登录、Swagger | 全部 |
| `apiproxy.paigod.work` | **公网** | HTTPS:443 | **81** | 3000 | AI 模型调用、MCP 协议 | `/v1/*` `/v1beta/*` `/mcp/*` `/sse` `/mcp` `/message` |

**需申请的权限清单：**

1. **CLB 创建**：应用型 CLB，同 VPC（CVM 所在 VPC），需公网 EIP
2. **SSL 证书**：`*.paigod.work` 通配符证书（或分别申请 `ai.paigod.work` + `apiproxy.paigod.work`）
3. **DNS 解析**：`ai.paigod.work` 和 `apiproxy.paigod.work` 两条 A 记录指向 CLB 公网 VIP
4. **CVM 安全组**：入站放通 CLB 内网 IP 段访问 80/81 端口
5. **CLB 健康检查**：80 端口用 HTTP（`/api/status`），**81 端口必须用 TCP**（`/` 返回 403 会导致 HTTP 健康检查失败）

### 3.1 购买 CLB

在腾讯云控制台创建 **应用型 CLB**（同地域同 VPC），获得 CLB VIP（如 `10.0.1.200` 内网 + `119.x.x.x` 公网 EIP）。

### 3.2 SSL 证书

在腾讯云「SSL 证书」中申请/上传证书：

| 证书 | 类型 | 说明 |
|------|------|------|
| `ai.paigod.work` | 免费单域名 / 付费通配符 | 内网管理后台 |
| `apiproxy.paigod.work` | 免费单域名 / 付费通配符 | 公网 AI API |

> **推荐购买 `*.paigod.work` 通配符证书**，一张证书覆盖两个子域名，管理更简单。腾讯云支持证书到期自动替换（需开启「托管」功能）。

### 3.3 CLB 监听器配置

| 监听器 | 协议 | CLB 端口 | SSL 证书 | 后端协议 | 后端端口 | 转发域名 |
|--------|------|---------|---------|---------|---------|---------|
| ai-https | HTTPS | 443 | `ai.paigod.work` | HTTP | 80 | `ai.paigod.work` |
| apiproxy-https | HTTPS | 443 | `apiproxy.paigod.work` | HTTP | 81 | `apiproxy.paigod.work` |

> **[易错点] 两个监听器共用 CLB 端口 443，靠 SNI（域名）区分。在 CLB「七层监听器」中按转发域名绑定不同的后端端口。**

配置步骤：
1. 创建 HTTPS:443 监听器，绑定 `ai.paigod.work` 证书
2. 添加转发规则：域名 `ai.paigod.work`，路径 `/`，后端 CVM:80
3. 在同一监听器添加转发规则：域名 `apiproxy.paigod.work`，绑定 `apiproxy.paigod.work` 证书，路径 `/`，后端 CVM:81

### 3.5 CLB 健康检查配置（P0 必须修改）

> **[易错点] `apiproxy.paigod.work` 的兜底路径返回 403，如果 CLB 沿用默认 HTTP 健康检查打 `/`，会持续判为不健康，导致线上 502！必须改为 TCP 健康检查。**

| 后端端口 | 健康检查类型 | 说明 |
|---------|------------|------|
| CVM:80 | TCP（或 HTTP 打 `/api/status`，期望 200） | ai.paigod.work 全路径开放，HTTP 健康检查可用 |
| CVM:81 | **TCP** | apiproxy.paigod.work 的 `/` 返回 403，必须用 TCP，否则 CLB 判死 |

**CLB 后端服务 81 端口健康检查设置：**
- 协议：**TCP**（不是 HTTP）
- 检查端口：81
- 检查间隔：5 秒，超时 2 秒，连续 3 次成功即为健康

### 3.4 DNS 解析

> **[易错点] 域名指向 LB 的公网 IP/DNS，不是直接指向 CVM/EC2！**

#### 国内域名（`paigod.work`，DNSPod 管理）

| 记录类型 | 主机记录 | 记录值 | 说明 |
|---------|---------|--------|------|
| A | `ai` | `<国内 CLB 公网 VIP>` | 管理后台（通过 CLB 白名单限内网访问） |
| A | `apiproxy` | `<国内 CLB 公网 VIP>` | AI API — 国内员工 |

#### 海外域名（`pplabs.tech`，AWS Route 53 或其他 DNS 管理）

| 记录类型 | 主机记录 | 记录值 | 说明 |
|---------|---------|--------|------|
| A / CNAME | `ai` | `<AWS ALB 公网 DNS/IP>` | 企业前端（公网，路径隔离） |
| A / CNAME | `apiproxy` | `<AWS ALB 公网 DNS/IP>` | AI API — 海外员工 |

> **注意：** 如使用 AWS ALB 的 DNS 名称（如 `xxx.us-west-2.elb.amazonaws.com`），需用 CNAME 记录而非 A 记录。

> **内网限制方案（国内 `ai.paigod.work`）：**
>
> - **方案 A（推荐）：** 使用**企业内部 DNS**将 `ai.paigod.work` 指向 CLB 内网 VIP。公网 DNS 中不添加 `ai` 记录，确保外网完全不可达。
> - **方案 B：** 在公网 DNS 添加 `ai` 记录指向 CLB 公网 VIP，然后在 CLB 安全组或 Nginx 中限制来源 IP（仅允许公司出口 IP）。
>
> **海外 `ai.pplabs.tech` 访问控制（已实施：Nginx 路径级隔离）：**
>
> `ai.pplabs.tech` 公网可达，通过 Nginx 路径白名单实现安全隔离：
>
> - **放通**：`/api/enterprise/*`（飞书 SSO 保护）、`/api/status`（健康检查）、`/`（前端 SPA）
> - **阻断**：`/api/*`（管理 API）、`/swagger/*`（API 文档）、`/v1/*`、`/v1beta/*`、`/mcp/*`、`/sse`、`/message`（Relay API）
> - **ALB Security Group**：`:80` 和 `:81` 均为 `0.0.0.0/0` 公网开放
> - **安全保障**：企业 API 需飞书 SSO 登录，管理 API 在 nginx 层直接返回 403
>
> 配置文件：`deploy/nginx/overseas/ai.pplabs.tech.conf`

### 3.6 CLB 超时配置（P0 必须修改）

> **[易错点#22] CLB 默认后端超时通常只有 60 秒。Claude Code 使用 extended thinking、长上下文对话等场景，单次请求可能持续 3-10 分钟。如果 CLB 超时太短，会导致 `504 Gateway Time-out`（错误页来自 stgw 腾讯云网关），进而引发 Claude Code 客户端报 `Cannot read properties of undefined (reading 'trim')` 错误。**

**必须修改的 CLB 超时参数：**

| 参数 | 默认值 | 推荐值 | 说明 |
|------|--------|--------|------|
| **后端响应超时** | 60s | **900s** | 等待后端返回首字节的最长时间 |
| **空闲连接超时** | 60s | **900s** | SSE 流式响应两次数据之间的最大间隔 |

**腾讯云控制台操作路径：**

1. 进入「负载均衡」→ 选择 CLB 实例
2. 点击「监听器管理」→ 找到 HTTPS:443 监听器
3. 点击 `apiproxy.paigod.work` 的转发规则 → 「编辑」
4. 修改「后端超时」为 **900** 秒
5. 如有「空闲超时」选项，也改为 **900** 秒
6. 同样修改 `ai.paigod.work` 的转发规则（管理后台也可能有长请求）
7. 保存

> **验证方法：** 修改后可使用超时测试脚本验证配置是否生效：
> ```bash
> # 从公网测试完整链路（需要 API Key）
> API_KEY=sk-xxx bash scripts/test-timeout-config.sh remote
>
> # 从服务器本地测试（绕过 CLB）
> API_KEY=sk-xxx bash scripts/test-timeout-config.sh local
> ```

**错误现象与排查：**

```
# 典型错误 1: 504 HTML 页面（来自腾讯云 STGW 网关）
API Error: 504 <html>
<head><title>504 Gateway Time-out</title></head>
<body><center><h1>504 Gateway Time-out</h1></center>
<hr><center>stgw</center></body></html>

# 典型错误 2: Claude Code 客户端解析 504 HTML 时的连锁错误
API Error: Cannot read properties of undefined (reading 'trim')
```

> **根因链条：** CLB/Nginx 超时 → 返回 504 HTML 页面 → Claude Code 期望 JSON/SSE 响应 → 解析 HTML 失败 → `trim` 报错。**修复超时配置后两个错误都会消失。**

---

## 四、关键地址一览

### 管理员使用（内网 `ai.paigod.work`）

| 用途 | 地址 |
|------|------|
| **管理后台（前端）** | `https://ai.paigod.work` |
| **管理 API** | `https://ai.paigod.work/api/*` |
| **Swagger 文档** | `https://ai.paigod.work/swagger/index.html` |
| **飞书登录入口** | `https://ai.paigod.work/api/enterprise/auth/feishu/login` |
| **飞书 OAuth 回调** | `https://ai.paigod.work/api/enterprise/auth/feishu/callback` |
| **健康检查** | `https://ai.paigod.work/api/status` |

### 用户使用（公网 API）

| 用途 | 国内地址（`paigod.work`） | 海外地址（`pplabs.tech`） |
|------|---------|---------|
| **AI API 基础地址** | `https://apiproxy.paigod.work/v1` | `https://apiproxy.pplabs.tech/v1` |
| **MCP 协议** | `https://apiproxy.paigod.work/mcp/*` | `https://apiproxy.pplabs.tech/mcp/*` |
| **SSE 端点** | `https://apiproxy.paigod.work/sse` | `https://apiproxy.pplabs.tech/sse` |
| **健康检查** | `https://apiproxy.paigod.work/v1/models`（Token 认证） | `https://apiproxy.pplabs.tech/v1/models` |

> **域名选择：**
> - **国内员工：** 使用 `apiproxy.paigod.work`
> - **海外员工：** 使用 `apiproxy.pplabs.tech`（直连 AWS 美西节点）
> - 两个域名使用同一套 Token Key，无需重新申请

用户在 AI 客户端（Cursor、ChatBox、LobeChat、Cherry Studio 等）中配置：

```
# 国内员工
API Base URL: https://apiproxy.paigod.work
API Key:      sk-xxxx（管理员通过内网后台分发的 Token Key）

# 海外员工
API Base URL: https://apiproxy.pplabs.tech
API Key:      sk-xxxx（同一个 Token Key）
```

### 飞书登录流程

> **[易错点] 飞书 OAuth 全程走内网域名。用户必须在可访问 `ai.paigod.work` 的网络环境下操作。**

1. 用户在内网打开 `https://ai.paigod.work`
2. 点击「飞书登录」→ 跳转飞书授权页（飞书域名，公网）
3. 用户授权 → 飞书回调至 `https://ai.paigod.work/api/enterprise/auth/feishu/callback`（内网）
4. 后端创建 Group + Token → 重定向至 `https://ai.paigod.work/feishu/callback?token_key=...`
5. 用户拿到 Token Key 后，在 AI 客户端中配置 `https://apiproxy.paigod.work` + Token Key 即可使用

> 分享给同事的飞书登录链接（内网可达）：`https://ai.paigod.work/api/enterprise/auth/feishu/login`

---

## 五、服务器环境搭建

### 5.1 基础环境

```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装依赖
sudo apt install -y curl wget git nginx

# 安装 Docker（使用腾讯云镜像源，清华源可能返回 403）
curl -fsSL https://mirrors.cloud.tencent.com/docker-ce/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-ce.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-ce.gpg] https://mirrors.cloud.tencent.com/docker-ce/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker-ce.list > /dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo usermod -aG docker $USER

# 配置 Docker Hub 镜像加速（腾讯云内网加速）
sudo mkdir -p /etc/docker
sudo tee /etc/docker/daemon.json <<'EOF'
{
  "registry-mirrors": [
    "https://mirror.ccs.tencentyun.com",
    "https://docker.mirrors.tuna.tsinghua.edu.cn"
  ]
}
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker

# 创建数据目录
sudo mkdir -p /data/{postgres,redis,aiproxy,backup}
sudo chown -R $USER:$USER /data
```

### 5.2 安装 Go 1.26+（编译用）

> **⚠️ Docker 部署（标准方式）无需在服务器上安装 Go 和 Node.js，Dockerfile 中已包含多阶段构建。以下仅供非 Docker 紧急调试场景参考。**

```bash
# 使用 Go 官方中国镜像站下载
wget https://golang.google.cn/dl/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
# 配置 Go 模块代理（七牛云镜像，国内最快）
echo 'export GOPROXY=https://goproxy.cn,direct' >> ~/.bashrc
source ~/.bashrc
go version
```

### 5.3 安装 Node.js + pnpm（前端编译用）

> **⚠️ Docker 部署（标准方式）无需在服务器上安装 Node.js，以下仅供非 Docker 紧急调试场景参考。**

```bash
# 使用清华镜像安装 Node.js 22
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs

# 配置 npm 国内镜像（npmmirror，原淘宝镜像）
npm config set registry https://registry.npmmirror.com
sudo npm install -g pnpm
```

---

## 六、组件部署

### 6.1 PostgreSQL + Redis（Docker）

创建 `/data/docker-compose.yml`：

```yaml
version: "3.8"
services:
  postgres:
    image: postgres:16-alpine
    container_name: aiproxy-postgres
    restart: unless-stopped
    volumes:
      - /data/postgres:/var/lib/postgresql/data
    environment:
      POSTGRES_USER: aiproxy
      POSTGRES_PASSWORD: <生成一个强密码>  # 例如: openssl rand -base64 32
      POSTGRES_DB: aiproxy
      TZ: Asia/Shanghai
    ports:
      - "127.0.0.1:5432:5432"
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "aiproxy"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: aiproxy-redis
    restart: unless-stopped
    command: redis-server --requirepass <Redis密码> --maxmemory 512mb --maxmemory-policy allkeys-lru
    volumes:
      - /data/redis:/data
    ports:
      - "127.0.0.1:6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "<Redis密码>", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
```

```bash
cd /data && docker compose up -d
docker compose ps  # 确认 healthy
```

> **安全提醒：** PostgreSQL 和 Redis 仅监听 `127.0.0.1`，不暴露公网。

### 6.2 编译 AI Proxy

> **[易错点] 前端构建时必须设置 `VITE_API_BASE_URL` 指向内网域名。这决定了前端页面调用哪个地址的管理 API。如果设错，前端会调不通后端 API。**

```bash
cd /data
git clone https://github.com/<your-org>/aiproxy.git
cd aiproxy

# 1. 编译前端（关键：指定内网管理 API 地址）
cd web
echo 'VITE_API_BASE_URL=https://ai.paigod.work/api' > .env.production
pnpm install
pnpm run build

# 2. 将前端产物嵌入后端（用 rsync --delete 避免旧文件残留）
rsync -a --delete dist/ ../core/public/dist/

# 3. 编译后端（含企业模块）
cd ../core
go build -tags enterprise -trimpath -ldflags "-s -w" -o /data/aiproxy/aiproxy
```

> **[易错点] 不要漏掉 `-tags enterprise`，否则飞书登录、企业分析、配额管理等功能全部不可用，且不会报错——只是路由不存在，返回 404。**

### 6.3 配置 AI Proxy 环境变量

创建 `/data/aiproxy/.env`：

```bash
# ============================
# 核心配置
# ============================

# 管理员密钥（用于管理后台 API 认证，务必使用强密码）
ADMIN_KEY=<生成强密码: openssl rand -base64 32>

# 数据库连接
SQL_DSN=postgres://aiproxy:<PostgreSQL密码>@127.0.0.1:5432/aiproxy?sslmode=disable

# 日志库：400 人规模（日均 2-6 万请求）不需要拆库，共用主库即可。
# 原因：logs 表月增 ~1 GB，request_details 月增 ~5-15 GB（受 LOG_DETAIL_STORAGE_HOURS 自动清理），
#       聚合表（group_summaries 等）增长极慢；8C16G + SSD 100GB 完全够用。
# 何时考虑拆库：用户超 2000+ 或日请求 > 30 万，或企业报表查询开始影响 API 延迟。
# LOG_SQL_DSN=postgres://aiproxy:<密码>@127.0.0.1:5432/aiproxy_log?sslmode=disable

# Redis 连接
REDIS=redis://:<Redis密码>@127.0.0.1:6379

# ============================
# 飞书 SSO 配置
# ============================

# 飞书开放平台 App 凭证
FEISHU_APP_ID=<你的飞书应用 App ID>
FEISHU_APP_SECRET=<你的飞书应用 App Secret>

# [易错点] 这两个地址必须指向内网域名 ai.paigod.work，不是 apiproxy.paigod.work
# OAuth 回调地址（必须与飞书开放平台中配置的一致）
FEISHU_REDIRECT_URI=https://ai.paigod.work/api/enterprise/auth/feishu/callback

# 前端基础 URL（OAuth 成功后重定向）
FEISHU_FRONTEND_URL=https://ai.paigod.work

# 允许的飞书租户（* 表示允许所有，多个用逗号分隔）
FEISHU_ALLOWED_TENANTS=*

# ============================
# 企业版「我的接入」页面配置
# ============================

# [重要] 用户在「我的接入」页面看到的 Base URL。
# 不设置时回退到请求 Host（即 ai.paigod.work/v1），这是内网地址，用户无法在公网使用！
# 必须设置为公网 API 地址。
# ⚠ 国内节点和海外节点需要设置不同的值！
#   国内：ENTERPRISE_BASE_URL=https://apiproxy.paigod.work/v1
#   海外：ENTERPRISE_BASE_URL=https://apiproxy.pplabs.tech/v1
ENTERPRISE_BASE_URL=https://apiproxy.paigod.work/v1

# [可选] 多地域接入地址（两个节点配置相同值）
# 设置后「我的接入」页面各渠道分组显示对应 Base URL，方便用户就近选择。
# 格式：JSON，key=渠道 owner（与模型分组名称一致），value=Base URL
# ENTERPRISE_BASE_URLS={"ppio":"https://apiproxy.paigod.work/v1","海外":"https://apiproxy.pplabs.tech/v1"}

# ============================
# 可选配置
# ============================

# 飞书 Webhook 通知
# NOTIFY_FEISHU_WEBHOOK=https://open.feishu.cn/open-apis/bot/v2/hook/<webhook-id>

# 请求详情保留 30 天（720h），超期由系统每次启动时自动批量清理。
# 这是控制磁盘增长最关键的配置——不设则 request_details 表无限膨胀。
# 注意：只清理明细（request_details / logs），不影响聚合表（group_summaries），企业分析报表不受影响。
LOG_DETAIL_STORAGE_HOURS=720

# 请求/响应 body 最大存储大小
# LOG_DETAIL_REQUEST_BODY_MAX_SIZE=4096
# LOG_DETAIL_RESPONSE_BODY_MAX_SIZE=4096

# 开启 ffmpeg（用于音视频处理）
FFMPEG_ENABLED=true

# 开启 gzip 压缩
GZIP_ENABLED=true

# 时区
TZ=Asia/Shanghai

# ============================
# 多节点部署配置（海外节点专用）
# ============================

# 节点渠道集：overseas = 优先使用"海外"渠道集的 Channel，无匹配时自动回落默认渠道
# 国内节点无需设置（使用默认渠道）
# NODE_CHANNEL_SET=overseas

# 节点类型：影响 Docker 构建时的镜像源选择
# domestic（默认）= 使用国内加速镜像源（goproxy.cn、npmmirror 等）
# overseas = 使用官方镜像源（适合海外服务器）
# NODE_TYPE=overseas
```

### 6.4 创建专用系统用户

> **[Legacy] 以下 §6.4–6.5 为 Systemd 裸机部署方式，已被 Docker 零停机部署（§10.4）取代。仅供首次从 Systemd 迁移到 Docker 时参考，新部署请直接使用 `scripts/deploy.sh`。**

> **[安全] 不要用 root 运行 AI Proxy。该服务处理外部请求、文件上传、MCP、第三方模型调用，一旦被攻破直接获得 root 权限。**

```bash
# 创建无登录 Shell 的系统用户
sudo useradd -r -s /sbin/nologin -d /data/aiproxy aiproxy

# 授予必要目录权限
sudo chown -R aiproxy:aiproxy /data/aiproxy /data/backup
sudo chmod 750 /data/aiproxy

# /tmp 由系统默认开放，aiproxy 用户可写（音频临时文件需要）
```

### 6.5 Systemd 服务

创建 `/etc/systemd/system/aiproxy.service`：

```ini
[Unit]
Description=AI Proxy Service
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
User=aiproxy
Group=aiproxy
WorkingDirectory=/data/aiproxy
EnvironmentFile=/data/aiproxy/.env
ExecStart=/data/aiproxy/aiproxy
Restart=always
RestartSec=5
LimitNOFILE=65536

# 安全加固
NoNewPrivileges=true
ProtectSystem=strict
# [易错点] 必须同时允许 /data/aiproxy 和 /tmp，否则 /v1/audio/transcriptions
# 链路中 os.CreateTemp("", "audio") 会写 /tmp 导致 500 错误。
ReadWritePaths=/data/aiproxy /tmp

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable aiproxy
sudo systemctl start aiproxy

# 检查状态
sudo systemctl status aiproxy
curl http://127.0.0.1:3000/api/status
```

---

## 七、Nginx 反向代理（核心：按端口做路径隔离）

> **Nginx 不再处理 SSL，仅做 HTTP 反向代理。SSL 终结由 CLB 完成。两个端口分别对应两个域名。**

### 7.1 内网域名配置（端口 80）：`ai.paigod.work`

> **前置条件：** 需先安装 `deploy/nginx/aiproxy-upstream.conf` 到 `/etc/nginx/conf.d/`，定义 `upstream aiproxy_backend`。详见 §10.4 一次性初始化。

创建 `/etc/nginx/sites-available/ai.paigod.work`（或直接使用 `deploy/nginx/ai.paigod.work.conf`）：

```nginx
# 内网管理后台 — CLB(443) → CVM(80)，开放全部路径
# 需要 /etc/nginx/conf.d/aiproxy-upstream.conf（定义 upstream aiproxy_backend）
server {
    listen 80;
    server_name ai.paigod.work;

    # 安全头
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";

    # 客户端请求大小（文件分析场景需要较大 body）
    client_max_body_size 100M;

    # ============================================================
    # [易错点] 如果使用方案 B（DNS 指向 CLB 公网 VIP + IP 白名单），
    # 取消下面的注释，将 IP 替换为公司办公网出口 IP。
    # 如果使用方案 A（内网 DNS），则无需此配置。
    # ============================================================
    # allow 202.x.x.x/32;   # 公司出口 IP 1
    # allow 116.x.x.x/32;   # 公司出口 IP 2
    # deny all;

    # 反向代理到 AI Proxy — 全部路径
    # [重要] 使用 upstream 名称（非硬编码端口），支持零停机部署端口切换
    location / {
        proxy_pass http://aiproxy_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;

        # SSE 流式响应支持
        proxy_set_header Connection '';
        proxy_buffering off;
        proxy_cache off;
        chunked_transfer_encoding on;

        # 超时设置
        # [易错点#22] Claude Code 等 AI 工具使用 extended thinking 时，
        # 单次请求可能持续 3-10 分钟。超时必须 ≥ 900s，否则会出现：
        #   - 504 Gateway Time-out（来自 stgw 腾讯云网关）
        #   - Claude Code 报 "Cannot read properties of undefined (reading 'trim')"
        proxy_connect_timeout 60s;
        proxy_send_timeout 900s;
        proxy_read_timeout 900s;
    }
}
```

### 7.2 公网域名配置（端口 81）：`apiproxy.paigod.work`

创建 `/etc/nginx/sites-available/apiproxy.paigod.work`（或直接使用 `deploy/nginx/apiproxy.paigod.work.conf`）：

```nginx
# 公网 AI API — CLB(443) → CVM(81)，仅开放白名单路径
# 需要 /etc/nginx/conf.d/aiproxy-upstream.conf（定义 upstream aiproxy_backend）
server {
    listen 81;
    server_name apiproxy.paigod.work;

    # 安全头
    add_header X-Content-Type-Options nosniff;

    # 客户端请求大小（文件分析需要较大 body）
    client_max_body_size 100M;

    # ============================================================
    # 公共代理参数（复用）
    # ============================================================
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;

    # SSE 流式响应支持（关键！）
    proxy_set_header Connection '';
    proxy_buffering off;
    proxy_cache off;
    chunked_transfer_encoding on;

    # 超时设置（AI 响应可能较慢）
    # [易错点#22] 必须 ≥ 900s，详见端口 80 注释
    proxy_connect_timeout 60s;
    proxy_send_timeout 900s;
    proxy_read_timeout 900s;

    # ============================================================
    # 白名单路径：仅开放 AI API 和 MCP 相关端点
    # 这些端点都有 Token 认证保护（middleware.TokenAuth / MCPAuth）
    # [重要] 使用 upstream 名称（非硬编码端口），支持零停机部署端口切换
    # ============================================================

    # OpenAI 兼容 API（/v1/chat/completions, /v1/models, /v1/responses 等）
    location /v1/ {
        proxy_pass http://aiproxy_backend;
    }

    # Gemini 兼容 API
    location /v1beta/ {
        proxy_pass http://aiproxy_backend;
    }

    # MCP 协议端点
    location /mcp/ {
        proxy_pass http://aiproxy_backend;
    }

    # MCP SSE 端点
    location = /sse {
        proxy_pass http://aiproxy_backend;
    }

    # MCP Streamable 端点
    location = /mcp {
        proxy_pass http://aiproxy_backend;
    }

    # MCP Message 端点
    location = /message {
        proxy_pass http://aiproxy_backend;
    }

    # ============================================================
    # [易错点] 下面这条规则至关重要！
    # 所有不在白名单中的路径一律返回 403，确保管理后台、
    # Swagger、/api/* 等管理接口在公网完全不可达。
    # ============================================================
    location / {
        return 403 '{"error": "forbidden"}';
        add_header Content-Type application/json;
    }
}
```

### 7.3 启用配置

```bash
sudo ln -s /etc/nginx/sites-available/ai.paigod.work /etc/nginx/sites-enabled/
sudo ln -s /etc/nginx/sites-available/apiproxy.paigod.work /etc/nginx/sites-enabled/

# [易错点] 删除默认配置，避免冲突
sudo rm -f /etc/nginx/sites-enabled/default

sudo nginx -t    # 必须显示 "test is successful"
sudo systemctl reload nginx
```

> **[易错点] Nginx 启动后检查两个端口都在监听：`ss -tlnp | grep nginx` 应看到 `:80` 和 `:81`。**

---

## 八、飞书开放平台配置

### 8.1 创建飞书应用

1. 前往 [飞书开放平台](https://open.feishu.cn/app/) → 创建企业自建应用
2. 记录 `App ID` 和 `App Secret`

### 8.2 配置权限

在「权限管理」中申请以下权限并发布版本：

| 权限 | 说明 |
|------|------|
| `contact:user.employee_id:readonly` | 获取用户 employee_id |
| `contact:user.base:readonly` | 获取用户基本信息 |
| `contact:user.email:readonly` | 获取用户邮箱 |
| `contact:department.base:readonly` | 获取部门基本信息 |
| `tenant:tenant:readonly` | 获取企业信息（组织同步用） |

### 8.3 配置重定向 URL

> **[易错点] 必须填内网域名 `ai.paigod.work`，不是公网域名。飞书开放平台的「安全设置」→「重定向 URL」不校验域名是否公网可达，所以内网地址可以正常配置。**

在「安全设置」→「重定向 URL」中添加：

```
https://ai.paigod.work/api/enterprise/auth/feishu/callback
```

> **[易错点] 飞书 OAuth 回调流程：飞书服务器不直接调用这个 URL，而是通过 302 重定向让用户的浏览器访问。因此只要用户浏览器能在内网访问到 `ai.paigod.work` 即可。**

### 8.4 配置应用可用范围

在「可用范围」中选择「全体员工」或指定需要使用的部门/人员。

### 8.5 发布应用

完成以上配置后，创建版本并提交审核发布。

---

## 九、安全加固

### 9.1 腾讯云安全组规则

> **[易错点] CVM 安全组只允许 CLB 访问 80/81，不直接暴露给公网！**

| 方向 | 协议 | 端口 | 来源 | 说明 |
|------|------|------|------|------|
| 入站 | TCP | 80 | CLB 内网 IP 段 | ai.paigod.work（管理后台） |
| 入站 | TCP | 81 | CLB 内网 IP 段 | apiproxy.paigod.work（AI API） |
| 入站 | TCP | 22 | 管理员 IP | SSH |
| 出站 | ALL | ALL | 0.0.0.0/0 | 允许所有出站（调用 AI Provider） |

> **不要开放** 3000（AI Proxy）、5432（PostgreSQL）、6379（Redis）端口。
> **不要把 80/81 开放给 0.0.0.0/0**，应只允许 CLB 所在的 VPC 子网访问。

### 9.2 AI Proxy 安全配置

- `ADMIN_KEY` 使用 32+ 字符强密码
- Token 级别启用 RPM/TPM 限流，防滥用（详见 §9.5）
- 启用 IP 限制（可选，配合企业出口 IP 白名单）
- 定期检查日志中的异常请求

### 9.3 防火墙（UFW）

```bash
sudo ufw allow 22/tcp    # SSH
sudo ufw allow from <CLB内网IP段> to any port 80    # CLB → Nginx(ai)
sudo ufw allow from <CLB内网IP段> to any port 81    # CLB → Nginx(apiproxy)
sudo ufw enable
```

> **[易错点] 不要 `ufw allow 80` / `ufw allow 81`，这会开放给所有来源。只允许 CLB 子网访问。**

### 9.4 验证公网隔离

> **[易错点] 部署完成后务必从公网验证以下地址不可访问！如果任何一个返回了非 403 内容，说明 Nginx 配置有误。**

```bash
# 从公网（非公司网络）执行，以下应全部返回 403：
curl -s https://apiproxy.paigod.work/                         # 403 ✓
curl -s https://apiproxy.paigod.work/api/status                # 403 ✓
curl -s https://apiproxy.paigod.work/swagger/index.html        # 403 ✓
curl -s https://apiproxy.paigod.work/api/enterprise/auth/feishu/login  # 403 ✓

# 以下应正常返回（需要带 Token）：
curl -s https://apiproxy.paigod.work/v1/models \
  -H "Authorization: Bearer sk-xxx"                       # 200 ✓

# 验证 CVM 端口不直接公网可达（应超时或拒绝）：
curl -s --connect-timeout 3 http://<CVM公网IP>:80/         # 超时 ✓
curl -s --connect-timeout 3 http://<CVM公网IP>:81/         # 超时 ✓
```

### 9.5 RPM / TPM 限流配置

> **RPM（Requests Per Minute）** 和 **TPM（Tokens Per Minute）** 是 AI Proxy 内置的按 Group+Model 粒度的滑动窗口限流机制，用于防止单个用户短时间内大量消耗资源。

#### 概念说明

| 指标 | 全称 | 窗口 | 含义 |
|------|------|------|------|
| **RPM** | Requests Per Minute | 滑动 1 分钟 | 每分钟最大请求次数 |
| **TPM** | Tokens Per Minute | 滑动 1 分钟 | 每分钟最大 Token 消耗量（含输入+输出） |

#### 工作原理

1. 每个请求进入时，系统按 `Group + Model` 查询最近 1 分钟的累计 RPM/TPM
2. 如果超过 ModelConfig 中设定的阈值，返回 `429` 错误
3. 请求完成后，实际消耗的 Token 数会计入 TPM 滑动窗口
4. **设为 0 表示不限制**（代码检查 `mc.TPM > 0` / `mc.RPM > 0`，为 0 时跳过检查）

#### 配置方式

RPM/TPM 在 **模型配置（ModelConfig）** 中按模型维度设置：

```
管理后台 → 模型管理 → 选择模型 → 编辑 → RPM / TPM 字段
```

#### 用户侧报错

当触发限流时，用户会收到 HTTP 429 响应：

```json
// RPM 超限
{"type":"error","error":{"type":"aiproxy_error","message":"request rpm limit exceeded, please try again later"}}

// TPM 超限
{"type":"error","error":{"type":"aiproxy_error","message":"request tpm limit exceeded, please try again later"}}
```

> **建议：** 对高成本模型（如 Claude Opus）设置较低的 RPM/TPM，对轻量模型（如 Haiku）可以放宽或设为 0（不限制）。

### 9.6 额度策略（Quota Policy）管理

> 额度策略是企业版功能，用于为员工分配周期性使用配额，并在消耗达到不同阶梯时自动调整 RPM/TPM 限制。**额度策略与 RPM/TPM 基础限流完全独立**：基础限流作用于 ModelConfig（模型维度），额度策略通过乘数动态调整已有限流。

#### 核心概念

| 概念 | 说明 |
|------|------|
| **周期配额（PeriodQuota）** | 每个计费周期（月/周等）分配给用户的最大金额 |
| **阶梯（Tier）** | 消耗占配额比例达到阈值时触发，共 3 级 |
| **TPM/RPM 乘数** | 触发阶梯后，将 ModelConfig 中的 TPM/RPM 乘以此值。如 `tpm_multiplier=0.5` 表示 TPM 减半 |
| **使用比例** | `(已用金额 - 上次周期快照) / 周期配额`，决定当前处于哪个阶梯 |

#### 阶梯机制示例

假设某模型 ModelConfig 设置 `TPM=100000`，额度策略配置如下：

| 阶梯 | 触发条件（使用比例） | TPM 乘数 | 实际 TPM |
|------|---------------------|---------|---------|
| 正常 | < 70% | 1.0 | 100,000 |
| Tier 1 | ≥ 70% | 0.8 | 80,000 |
| Tier 2 | ≥ 90% | 0.5 | 50,000 |
| Tier 3 | ≥ 100% | 0.0（封禁） | 0（请求被拒绝） |

#### 重要注意事项

- **TPM/RPM 基础值为 0 时，乘数无效**：`0 × 任何乘数 = 0`，原本不限流的模型不受额度策略影响
- **额度策略不影响余额系统**：余额扣费和额度策略是两套独立系统
- **乘数范围**：管理页面支持 0 ~ 10，支持小数（如 0.5、1.5），设为 0 表示完全封禁该阶梯

---

## 十、监控与运维

### 10.1 日志查看

```bash
# AI Proxy 日志（Docker 零停机部署）
sudo docker logs -f aiproxy-active

# AI Proxy 日志（Legacy Systemd 部署）
# sudo journalctl -u aiproxy -f

# Nginx 访问日志
sudo tail -f /var/log/nginx/access.log

# Docker 容器日志
docker logs -f aiproxy-postgres
docker logs -f aiproxy-redis
```

### 10.2 健康检查

```bash
# API 健康（从服务器本地）
curl -s http://127.0.0.1:3000/api/status | jq .

# PostgreSQL
docker exec aiproxy-postgres pg_isready -U aiproxy

# Redis
docker exec aiproxy-redis redis-cli -a <Redis密码> ping
```

### 10.3 数据备份

```bash
# 主库每日备份（添加到 crontab）
0 3 * * * docker exec aiproxy-postgres pg_dump -U aiproxy aiproxy | gzip > /data/backup/pg_$(date +\%Y\%m\%d).sql.gz

# 如果配置了 LOG_SQL_DSN 分离日志库（数据库名 aiproxy_log），单独备份：
# 0 3 * * * docker exec aiproxy-postgres pg_dump -U aiproxy aiproxy_log | gzip > /data/backup/pg_log_$(date +\%Y\%m\%d).sql.gz

# 保留最近 30 天备份
0 4 * * * find /data/backup -name "pg_*.sql.gz" -mtime +30 -delete
```

**恢复命令（仅供参考）：**

```bash
# 恢复主库
gunzip -c /data/backup/pg_20260323.sql.gz | docker exec -i aiproxy-postgres psql -U aiproxy aiproxy

# 恢复日志库（如有分离）
gunzip -c /data/backup/pg_log_20260323.sql.gz | docker exec -i aiproxy-postgres psql -U aiproxy aiproxy_log
```

> **[易错点] 如果生产环境启用了 `LOG_SQL_DSN` 分离日志库，必须同时备份两个数据库，否则恢复时会丢失请求日志和审计数据。**

### 10.4 更新升级（零停机 Docker 部署）

> **推荐方式：** 使用 `scripts/deploy.sh`（默认零停机模式）。Docker 多阶段构建自动完成前端编译 + Swagger 生成 + enterprise 标签编译，消除手动构建的易错点。

#### 工作原理

```
Phase 1: Docker 构建新镜像（旧容器持续服务，0 影响）
Phase 2: 新容器启动在备用端口（如 :3001），健康检查 + smoke test
Phase 3: Nginx upstream 切换到新端口 → nginx reload（零断连，SSE 不中断）
Phase 4: 旧容器收到 SIGTERM → graceful drain（最长等 600s，在途请求自然完成）
Phase 5: 更新状态文件，清理旧容器
```

端口交替策略：每次部署在 3000/3001 之间交替，通过 `/data/aiproxy/.active-port` 跟踪。

#### 标准部署命令（单节点）

```bash
# 完整部署（拉代码 + 构建 + 零停机切换 + smoke test）
ADMIN_KEY=xxx bash scripts/deploy.sh

# 跳过 git pull（部署当前代码）
ADMIN_KEY=xxx bash scripts/deploy.sh --no-pull

# 仅构建镜像（不切换流量）
bash scripts/deploy.sh --build-only

# 紧急回滚（使用上次部署前的镜像）
ADMIN_KEY=xxx bash scripts/deploy.sh --rollback
```

#### 多节点部署命令

```bash
# 一键部署所有节点（国内 + 海外，顺序执行）
ADMIN_KEY=xxx bash scripts/deploy-all.sh

# 带飞书通知
ADMIN_KEY=xxx FEISHU_WEBHOOK=https://open.feishu.cn/... bash scripts/deploy-all.sh

# 跳过 git pull
ADMIN_KEY=xxx DEPLOY_ARGS="--no-pull" bash scripts/deploy-all.sh

# 仅构建
bash scripts/deploy-all.sh --build-only

# 紧急回滚所有节点
ADMIN_KEY=xxx bash scripts/deploy-all.sh --rollback
```

> **多节点部署说明：**
> - `deploy-all.sh` 按顺序部署各节点（先国内，后海外），任一节点失败会输出手动回滚命令
> - 海外节点使用 `NODE_TYPE=overseas` 构建（使用官方镜像源），并设置 `NODE_CHANNEL_SET=overseas` 运行
> - 单独部署海外节点：`ssh ppuser@52.35.158.131 "cd /data/aiproxy && sudo bash -c 'export ADMIN_KEY=xxx NODE_TYPE=overseas && bash scripts/deploy.sh'"`
> - 海外节点部署后自动执行外部 ALB 冒烟测试（通过公网 `https://apiproxy.pplabs.tech` 验证端到端可达性）
> - 部署成功/失败都会自动发送飞书通知（需设置 `FEISHU_WEBHOOK` 环境变量），包含 Git SHA 版本号和耗时
> - Docker 镜像同时打 `aiproxy:sha-<commit>` 标签，便于追溯线上运行版本
>
> **[重要] DB 迁移向后兼容要求：**
> 多节点共享同一数据库，国内节点先部署会率先执行 GORM AutoMigrate。在海外节点升级完成前，**旧版代码仍在运行**。因此：
> - 新增列可以设 `NOT NULL DEFAULT`，但不能删除旧代码仍在使用的列
> - 表/列重命名必须拆成两次发布：第一次添加新列 + 双写，第二次删除旧列
> - 如需不兼容的 schema 变更，应先部署海外节点（从节点）暂停服务，再部署国内节点

#### 用户影响评估

| 阶段 | 耗时 | 服务影响 |
|------|------|---------|
| Docker build（前端 + Go） | ~10-15 分钟（`--no-cache`） | 无，旧容器持续服务 |
| Canary 启动 + 健康检查 | ~10-30 秒 | 无，旧容器持续服务 |
| **Nginx reload** | **~0 秒** | **无中断，已有连接继续走旧 worker** |
| 旧容器 drain | 0-600 秒 | 无，在途 SSE 流自然完成 |

> **总影响：0 秒服务中断。** 正在进行的 SSE 流式响应通过 Nginx 旧 worker 继续完成，新请求由 Nginx 新 worker 路由到新容器。用户完全无感。

#### 快速回滚

```bash
# 方式一：脚本回滚（推荐，使用上次部署前自动保存的镜像）
ADMIN_KEY=xxx bash scripts/deploy.sh --rollback

# 方式二：手动回滚
# 1. 查看可用镜像
docker images | grep aiproxy

# 2. 用 rollback 镜像启动容器，切换 Nginx（脚本会自动处理）
```

> 每次部署前脚本自动执行 `docker tag aiproxy:local aiproxy:rollback`，保留上一版镜像。回滚无需重新编译，秒级恢复。

#### Legacy 模式（兜底）

如果 Nginx upstream 配置尚未部署，可使用 legacy 模式（直接容器替换，5-10 秒中断）：

```bash
ADMIN_KEY=xxx bash scripts/deploy.sh --legacy
```

#### 服务器一次性初始化（首次启用零停机部署时执行）

```bash
# 1. 部署 Nginx upstream 配置文件
sudo cp deploy/nginx/aiproxy-upstream.conf /etc/nginx/conf.d/

# 2. 修改现有站点配置：proxy_pass 改为使用 upstream 名称
#    （不要直接替换站点配置文件 — 生产环境可能有额外的自定义规则）
sudo sed -i 's|proxy_pass http://127\.0\.0\.1:3000|proxy_pass http://aiproxy_backend|g' \
  /etc/nginx/sites-available/ai.paigod.work \
  /etc/nginx/sites-available/apiproxy.paigod.work

# 3.（推荐）将超时从 300s 升级到 900s（支持 extended thinking）
sudo sed -i 's|proxy_send_timeout 300s|proxy_send_timeout 900s|g' \
  /etc/nginx/sites-available/ai.paigod.work \
  /etc/nginx/sites-available/apiproxy.paigod.work
sudo sed -i 's|proxy_read_timeout 300s|proxy_read_timeout 900s|g' \
  /etc/nginx/sites-available/ai.paigod.work \
  /etc/nginx/sites-available/apiproxy.paigod.work

# 4. 验证 Nginx 配置
sudo nginx -t

# 5. 初始化状态文件（当前服务运行在哪个端口就写哪个）
echo 3000 | sudo tee /data/aiproxy/.active-port

# 6. 重载 Nginx
sudo nginx -s reload

# 7. 验证服务正常（切换到 upstream 后应无感知）
curl -s http://localhost:80/api/status
curl -s http://localhost:81/v1/models -H "Authorization: Bearer sk-xxx"
```

---

## 十一、远程服务器操作手册

> 本节记录在生产服务器上执行日常运维操作的完整步骤和注意事项，避免重复踩坑。

### 11.1 登录信息速查

#### 国内节点

| 项目 | 值 |
|------|-----|
| **服务器** | `1.13.81.31`（CVM 公网 IP） |
| **登录用户** | `ppuser`（SSH 密钥认证，有 sudo 权限） |
| **SSH 私钥** | `/home/ppuser/.ssh/id_ed25519`（服务器上，用于 git 操作） |
| **AI Proxy 运行方式** | Docker 容器 `aiproxy-active`（零停机部署，端口交替 3000/3001），镜像 `aiproxy:local` |
| **代码路径** | `/data/aiproxy` |
| **环境变量文件** | `/data/aiproxy/.env` |
| **活跃端口状态文件** | `/data/aiproxy/.active-port` |
| **部署脚本** | `scripts/deploy.sh`（默认零停机，`--legacy` 走旧模式） |
| **Admin Key** | 见 `/data/aiproxy/.env` 中的 `ADMIN_KEY` |
| **PostgreSQL** | Docker 容器 `aiproxy-postgres`，用户 `aiproxy`，库 `aiproxy`，端口 `5432` |
| **Redis** | Docker 容器 `aiproxy-redis`，端口 `6379` |

#### 海外节点（AWS 美西）

| 项目 | 值 |
|------|-----|
| **服务器** | `52.35.158.131`（AWS EC2 公网 IP，us-west-2） |
| **内网 IP** | `10.195.9.13`（AWS VPC） |
| **登录用户** | `ppuser`（SSH 密钥认证，有 sudo 权限） |
| **域名** | `ai.pplabs.tech`（企业前端）+ `apiproxy.pplabs.tech`（API） |
| **AI Proxy 运行方式** | Docker 容器 `aiproxy-active`（同国内零停机模式） |
| **代码路径** | `/data/aiproxy` |
| **环境变量文件** | `/data/aiproxy/.env`（含 `NODE_CHANNEL_SET=overseas`） |
| **PostgreSQL** | 通过 WireGuard 连回国内主库（`10.0.0.1:5432`） |
| **Redis** | Docker 容器，`127.0.0.1:6379`（本地缓存） |
| **WireGuard** | 接口 `wg0`，本端 `10.0.0.2`，对端 `10.0.0.1` |

### 11.2 SSH 登录

```bash
# 国内节点
ssh ppuser@1.13.81.31

# 海外节点
ssh ppuser@52.35.158.131
```

> **注意：** 服务器禁用了 root SSH 登录，所有操作通过 `ppuser` + `sudo` 执行。

### 11.3 Git 操作

> **[易错点]** `/data/aiproxy` 目录归属 `aiproxy` 用户（服务运行需要），但 git 操作需要 SSH 密钥（存在 `ppuser` home 下）。因此 git 命令必须通过 `sudo` + 指定 SSH 密钥执行。

```bash
# 拉取最新代码
cd /data/aiproxy
sudo GIT_SSH_COMMAND="ssh -i /home/ppuser/.ssh/id_ed25519 -o StrictHostKeyChecking=no" git pull origin main

# 查看状态
sudo git status
sudo git log --oneline -5
```

### 11.4 前端编译（有前端改动时）

> **[Legacy] §11.4–11.6 为手动裸机编译部署流程。当前推荐使用 `ADMIN_KEY=xxx bash scripts/deploy.sh`（Docker 零停机部署），Dockerfile 自动完成前端编译、Swagger 生成、Go 构建和前端嵌入，无需手动操作。以下仅供无 Docker 环境的紧急场景参考。**

> **[P0 易错点#21]** 前端编译输出到 `web/dist/`，但 Go 的 `go:embed` 读取的是 `core/public/dist/`。**编译前端后必须同步到 embed 目录**，否则 Go 二进制嵌入的仍是旧版前端，新功能在浏览器中不可用且无任何报错。

```bash
# 1. 编译前端
cd /data/aiproxy/web
sudo npm run build

# 2. [关键] 同步到 go:embed 目录
sudo rsync -a --delete /data/aiproxy/web/dist/ /data/aiproxy/core/public/dist/

# 3. 验证同步（两边 hash 应一致）
ls /data/aiproxy/web/dist/assets/ | grep "index-"
ls /data/aiproxy/core/public/dist/assets/ | grep "index-"
```

### 11.5 后端编译

> **[易错点]** Go 安装在 `/usr/local/go/bin`，ppuser 的环境变量可能不包含此路径，必须通过 `sudo env` 显式传递。

```bash
cd /data/aiproxy/core

# 编译（含企业模块）
sudo env "PATH=/usr/local/go/bin:$PATH" "GOPATH=/home/ppuser/go" "GOCACHE=/tmp/go-cache" \
  go build -tags enterprise -trimpath -ldflags "-s -w" -o aiproxy

# 验证编译产物
ls -la /data/aiproxy/core/aiproxy
```

### 11.6 部署（替换二进制并重启）

> **[P0 易错点]** `go build` 输出到 `/data/aiproxy/core/aiproxy`，但 systemd `ExecStart` 指向 `/data/aiproxy/aiproxy`。如果只重启服务而不复制二进制，会继续运行旧版本！

```bash
# 必须先停服务，否则复制时报 "Text file busy"
sudo systemctl stop aiproxy

# 复制新二进制到服务路径
sudo cp /data/aiproxy/core/aiproxy /data/aiproxy/aiproxy
sudo chown aiproxy:aiproxy /data/aiproxy/aiproxy

# 启动服务
sudo systemctl start aiproxy

# 验证
sudo systemctl status aiproxy
curl -s http://127.0.0.1:3000/api/status
sudo journalctl -u aiproxy -n 20 --no-pager
```

> **确认正在运行新版本的方法：** 检查二进制文件的修改时间是否与编译时间一致：
> ```bash
> stat /data/aiproxy/aiproxy | grep Modify
> stat /data/aiproxy/core/aiproxy | grep Modify
> # 两者应相同，且为刚才的编译时间
> ```

### 11.7 数据库操作

```bash
# 进入 PostgreSQL 交互式 Shell
sudo docker exec -it aiproxy-postgres psql -U aiproxy -d aiproxy

# 常用查询示例
# 查看模型总数
SELECT COUNT(*) FROM model_configs;

# 查看某个模型的配置
SELECT model, owner, type, config FROM model_configs WHERE model = 'pa/gpt-5.3-codex';

# 查看 Channel 列表
SELECT id, name, type, base_url, status FROM channels;

# 查看最近的同步历史
SELECT * FROM sync_histories ORDER BY synced_at DESC LIMIT 5;
```

```bash
# Redis 操作
sudo docker exec -it aiproxy-redis redis-cli
# 如有密码：
# sudo docker exec -it aiproxy-redis redis-cli -a <Redis密码>
```

### 11.8 日志排查

```bash
# 实时查看 AI Proxy 日志（Docker 零停机部署）
sudo docker logs -f aiproxy-active

# 查看最近 100 行日志
sudo docker logs --tail 100 aiproxy-active

# 按时间查看日志
sudo docker logs --since "2026-03-25T14:00:00" --until "2026-03-25T15:00:00" aiproxy-active

# Legacy Systemd 部署日志（如仍使用）
# sudo journalctl -u aiproxy -f
# sudo journalctl -u aiproxy -n 100 --no-pager

# Nginx 日志
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log

# Docker 容器日志
sudo docker logs -f aiproxy-postgres
sudo docker logs -f aiproxy-redis
```

### 11.9 服务管理

```bash
# === Docker 零停机部署（当前标准） ===

# 查看运行状态
sudo docker ps | grep aiproxy
cat /data/aiproxy/.active-port

# 查看日志
sudo docker logs -f aiproxy-active

# 重新部署（零停机）
cd /data/aiproxy && ADMIN_KEY=xxx sudo bash scripts/deploy.sh --no-pull

# 紧急回滚
cd /data/aiproxy && ADMIN_KEY=xxx sudo bash scripts/deploy.sh --rollback

# 手动停止（会中断服务！）
sudo docker stop aiproxy-active

# === Legacy Systemd 部署 ===
# sudo systemctl start aiproxy
# sudo systemctl stop aiproxy
# sudo systemctl restart aiproxy
# sudo systemctl status aiproxy

# === 基础设施容器管理 ===
sudo docker compose -f /data/docker-compose.yml ps
sudo docker compose -f /data/docker-compose.yml restart postgres
sudo docker compose -f /data/docker-compose.yml restart redis
```

### 11.10 完整快速部署流程（一键 Copy）

#### 方案 0：Docker 零停机部署（推荐）

> **标准部署方式。** 自动完成 git pull → Docker 构建（前端 + Swagger + Go） → canary 健康检查 → Nginx 切换 → 旧容器优雅退出。全程零中断。

```bash
# === 单节点部署（国内） ===
ssh ppuser@1.13.81.31
cd /data/aiproxy && sudo bash -c 'export ADMIN_KEY=xxx && bash scripts/deploy.sh'

# === 单节点部署（海外） ===
ssh ppuser@52.35.158.131
cd /data/aiproxy && sudo bash -c 'export ADMIN_KEY=xxx NODE_TYPE=overseas && bash scripts/deploy.sh'

# === 多节点一键部署（国内 + 海外，顺序执行） ===
ADMIN_KEY=xxx bash scripts/deploy-all.sh

# === 跳过 git pull（部署当前代码） ===
# ADMIN_KEY=xxx bash scripts/deploy-all.sh --no-pull

# === 紧急回滚（所有节点） ===
# ADMIN_KEY=xxx bash scripts/deploy-all.sh --rollback
```

---

> **以下方案 A/B 为 Legacy 裸机部署方式（需停机 5-10 秒），仅供无 Docker 环境的紧急场景。新部署请使用方案 0。**

> **先判断变更范围再选方案：** `git log --oneline origin/main..main` 查看待部署 commit，`git diff origin/main --stat | grep "web/src/"` 判断是否含前端改动。

#### 方案 A（Legacy）：含前端改动（完整部署，约 1.5 分钟）

```bash
# === 登录服务器 ===
ssh ppuser@1.13.81.31

# === 拉取 + 前端 + 后端 + 部署 ===
cd /data/aiproxy && \
sudo GIT_SSH_COMMAND="ssh -i /home/ppuser/.ssh/id_ed25519 -o StrictHostKeyChecking=no" git pull origin main && \
cd web && sudo npm run build && \
sudo rsync -a --delete /data/aiproxy/web/dist/ /data/aiproxy/core/public/dist/ && \
cd /data/aiproxy/core && \
sudo env "PATH=/usr/local/go/bin:$PATH" "GOPATH=/home/ppuser/go" "GOCACHE=/tmp/go-cache" \
  go build -tags enterprise -trimpath -ldflags "-s -w" -o aiproxy && \
sudo systemctl stop aiproxy && \
sudo cp /data/aiproxy/core/aiproxy /data/aiproxy/aiproxy && \
sudo chown aiproxy:aiproxy /data/aiproxy/aiproxy && \
sudo systemctl start aiproxy && \
echo "=== 部署完成 ===" && \
sudo systemctl status aiproxy --no-pager
```

#### 方案 B（Legacy）：仅后端改动（快速部署，约 1 分钟）

> ⚠️ **确认无前端改动后才能用此方案！** 否则会导致易错点 #21：前端代码不生效。

```bash
# === 登录服务器 ===
ssh ppuser@1.13.81.31

# === 拉取 + 编译 + 部署 ===
cd /data/aiproxy && \
sudo GIT_SSH_COMMAND="ssh -i /home/ppuser/.ssh/id_ed25519 -o StrictHostKeyChecking=no" git pull origin main && \
cd core && \
sudo env "PATH=/usr/local/go/bin:$PATH" "GOPATH=/home/ppuser/go" "GOCACHE=/tmp/go-cache" \
  go build -tags enterprise -trimpath -ldflags "-s -w" -o aiproxy && \
sudo systemctl stop aiproxy && \
sudo cp /data/aiproxy/core/aiproxy /data/aiproxy/aiproxy && \
sudo chown aiproxy:aiproxy /data/aiproxy/aiproxy && \
sudo systemctl start aiproxy && \
echo "=== 部署完成 ===" && \
sudo systemctl status aiproxy --no-pager
```

---

## 十二、部署检查清单

### 基础设施（国内节点）

- [ ] 服务器已购买并初始化（8C16G，Ubuntu 22.04）
- [ ] 数据盘已挂载至 `/data`
- [ ] Docker + Nginx 已安装（Docker 部署无需 Go/Node.js）
- [ ] PostgreSQL + Redis 容器已启动且 healthy

### Docker 部署

- [ ] `/data/aiproxy/.env` 配置完整（参见 §6.3 环境变量模板）
- [ ] `/data/aiproxy/.env` 中 `FEISHU_REDIRECT_URI` 和 `FEISHU_FRONTEND_URL` 都指向 `ai.paigod.work`
- [ ] `/data/aiproxy/.env` 中 `ENTERPRISE_BASE_URL=https://apiproxy.paigod.work/v1`（否则「我的接入」页面显示内网地址）
- [ ] Nginx `aiproxy-upstream.conf` 已安装到 `/etc/nginx/conf.d/`（零停机部署必需）
- [ ] `ADMIN_KEY=xxx bash scripts/deploy.sh` 部署成功，smoke test 通过

### CLB & 域名 & 网络

- [ ] CLB 已创建，HTTPS:443 监听器已配置
- [ ] `ai.paigod.work` 转发规则 → CVM:80
- [ ] `apiproxy.paigod.work` 转发规则 → CVM:81
- [ ] SSL 证书已上传至 CLB 并绑定（推荐通配符 `*.paigod.work`）
- [ ] `ai.paigod.work` DNS 解析到 CLB VIP（内网 DNS 或公网 + IP 限制）
- [ ] `apiproxy.paigod.work` DNS 解析到国内 CLB 公网 VIP（默认线路）
- [ ] ~~`apiproxy.paigod.work` 不再需要境外智能解析（海外使用独立域名 `pplabs.tech`）~~
- [ ] `apiproxy.pplabs.tech` DNS 解析到 AWS ALB
- [ ] `ai.pplabs.tech` DNS 解析到 AWS ALB
- [ ] Nginx 监听 80（ai）和 81（apiproxy），`nginx -t` 通过
- [ ] CVM 安全组仅允许 CLB 访问 80/81 + 管理员 SSH 22
- [ ] CLB 后端超时已修改为 900s（参见 §3.6）
- [ ] UFW 防火墙已启用

### 公网隔离验证

- [ ] 从公网访问 `https://apiproxy.paigod.work/` 返回 403
- [ ] 从公网访问 `https://apiproxy.paigod.work/api/status` 返回 403
- [ ] 从公网访问 `https://apiproxy.paigod.work/swagger/index.html` 返回 403
- [ ] 从公网访问 `https://apiproxy.paigod.work/v1/models`（带 Token）返回 200
- [ ] 直接访问 CVM 公网 IP:80 和 :81 不可达

### 海外节点

- [ ] AWS EC2 已购买并初始化（Ubuntu 22.04，us-west-2）
- [ ] Docker + Nginx 已安装
- [ ] WireGuard 隧道已配置（本端 `10.0.0.2` ↔ 对端 `10.0.0.1`），`ping 10.0.0.1` 正常
- [ ] EC2 Security Group：ALB 安全组访问 81 + 管理员 SSH 22 + WireGuard UDP 51820
- [ ] PostgreSQL 连接通过 WireGuard 到国内主库（`10.0.0.1:5432`），连接正常
- [ ] Redis 本地 Docker 容器已启动
- [ ] `/data/aiproxy/.env` 包含 `NODE_CHANNEL_SET=overseas`
- [ ] `/data/aiproxy/.env` 中 `ENTERPRISE_BASE_URL=https://apiproxy.pplabs.tech/v1`（海外节点必须用 pplabs.tech 域名）
- [ ] `/data/aiproxy/.env` 中 `FEISHU_REDIRECT_URI` / `FEISHU_FRONTEND_URL` 指向 `ai.pplabs.tech`（如海外支持飞书登录）
- [ ] Nginx `aiproxy-upstream.conf` 已安装到 `/etc/nginx/conf.d/`
- [ ] Nginx `apiproxy.pplabs.tech.conf` + `ai.pplabs.tech.conf` 已安装到 `/etc/nginx/sites-available/` 并启用
- [ ] AWS ALB 已创建，两个 Target Group：`:80`（admin）和 `:81`（API）
- [ ] ACM 证书（`*.pplabs.tech`）已申请并绑定到 ALB
- [ ] ALB Security Group 已限制 `:80`（`ai.pplabs.tech`）仅允许公司 IP 访问
- [ ] `sudo bash -c 'export NODE_TYPE=overseas ADMIN_KEY=xxx && bash scripts/deploy.sh'` 部署成功（HEALTH_TIMEOUT 自动 600s）
- [ ] WireGuard 健康检查 crontab 已配置：`*/1 * * * * bash /data/aiproxy/scripts/wireguard-health.sh`
- [ ] `/var/log/wireguard-health.log` 有定期输出（确认 crontab 生效）
- [ ] 从海外访问 `https://apiproxy.pplabs.tech/v1/models`（带 Token）返回 200
- [ ] 从海外访问 `https://ai.pplabs.tech`（企业前端）正常加载

### 飞书 & 基础功能验证

- [ ] 飞书应用已创建，权限已审批，重定向 URL 配置为 `https://ai.paigod.work/...`
- [ ] 在内网访问飞书登录链接测试通过
- [ ] 管理后台可正常访问（内网）
- [ ] 至少添加一个 Channel（AI Provider），从公网 API 测试调用正常
- [ ] **PPIO Channel base_url 使用 `api.ppinfra.com`**（非 `api.ppio.com`）：OpenAI 通道 → `https://api.ppinfra.com/v3/openai`，Anthropic 通道 → `https://api.ppinfra.com/anthropic`
- [ ] **Anthropic 通道仅包含 Claude 系模型**（`pa/*`、`claude-*`），非 Claude 模型只放在 OpenAI 通道，避免路由冲突导致 404

### 发布前高风险功能 Smoke 验证

> 根据本次发布范围选择，**加粗项无论范围如何都必须验证**。

- [ ] **文本主链路**：`POST /v1/chat/completions` 正常返回
- [ ] **计费主链路**：请求后 Log 记录、Group 用量、Token 用量三者一致
- [ ] **模型可见性**：管理后台的可用模型中，无已知不可用模型（如 ernie-* 系列，仅在启用 Baidu Channel 时才能出现）
- [ ] MCP（如本次上线包含）：`GET /sse` 建立连接，`initialize` 不返回 502；`tools/list` 能稳定返回结果
- [ ] 多模态（如本次上线承诺）：`POST /v1/audio/transcriptions`、`POST /v1/embeddings` 各至少一条成功
- [ ] Responses 协议（如本次上线承诺）：`POST /v1/responses` 至少 1 个可用模型正常响应

### 运维

- [ ] 数据库备份 cron 已配置（如启用日志库分离，两个库都已加入备份）
- [ ] CLB 证书托管已开启自动续签
- [ ] CLB 后端 81 端口健康检查已改为 TCP

### 超时配置（Claude Code / Extended Thinking）

- [ ] **Nginx `proxy_read_timeout` 和 `proxy_send_timeout` ≥ 900s**（端口 80 和 81 都要改）
- [ ] **CLB 后端响应超时 ≥ 900s**（控制台 → 负载均衡 → 监听器 → 转发规则 → 编辑）
- [ ] CLB 空闲连接超时 ≥ 900s（如有此选项）
- [ ] Nginx `proxy_buffering off`（SSE 流式响应必须关闭缓冲）
- [ ] 验证：`API_KEY=sk-xxx bash scripts/test-timeout-config.sh remote` 无 504 错误

---

## 十三、易错点汇总

> **Docker 部署消除的易错点** 标记为 ~~删除线~~，表示使用 `scripts/deploy.sh` Docker 构建后不再可能发生。

| # | 优先级 | 位置 | 易错描述 | 后果 |
|---|--------|------|---------|------|
| 1 | P0 | CLB 健康检查 | 81 端口健康检查未改为 TCP，沿用默认 HTTP 打 `/` | `/` 返回 403，CLB 持续判后端不健康，线上 502 |
| ~~5~~ | ~~P2~~ | ~~前端部署~~ | ~~更新时用 `cp -r` 而非 `rsync --delete`~~ | ~~Docker 构建自动处理，不再需要手动同步~~ |
| 6 | P2 | CLB 监听器 | 两个转发规则后端端口搞反（ai→81, apiproxy→80） | 管理后台暴露公网，或 API 端口不通 |
| 7 | P2 | DNS 解析 | 域名直接指向 CVM 公网 IP 而非 CLB VIP | SSL 不生效，直连 HTTP 暴露端口 |
| ~~8~~ | ~~P2~~ | ~~前端编译~~ | ~~忘记设置 `VITE_API_BASE_URL`~~ | ~~Docker 构建内置前端编译，不再手动设置~~ |
| ~~9~~ | ~~P2~~ | ~~后端编译~~ | ~~忘记 `-tags enterprise`~~ | ~~Dockerfile 硬编码 `-tags enterprise`，不可能遗漏~~ |
| 10 | P2 | 环境变量 | `FEISHU_REDIRECT_URI` 或 `FEISHU_FRONTEND_URL` 写成了 `apiproxy.paigod.work` | OAuth 回调指向公网域名，被 Nginx 403 拦截，登录失败 |
| 11 | P2 | 飞书平台 | 开放平台的重定向 URL 与 `.env` 中的 `FEISHU_REDIRECT_URI` 不一致 | 飞书报"重定向 URI 不匹配"错误 |
| 12 | P2 | Nginx | `apiproxy.paigod.work` 的 `location /` 兜底规则缺失 | 公网可访问管理后台和 Swagger，安全漏洞 |
| 13 | P2 | Nginx | 没有删除 `/etc/nginx/sites-enabled/default` | default server 可能拦截请求，导致两个域名都 404 |
| 14 | P2 | 安全组 | CVM 安全组把 80/81 开放给 0.0.0.0/0 | 可绕过 CLB 直连 CVM，绕过 SSL 和 CLB 安全策略 |
| 15 | P3 | docker-compose | PostgreSQL/Redis 密码中包含特殊字符（`@`, `#`, `%`） | 连接串解析失败，AI Proxy 启动报数据库连接错误 |
| 16 | P1 | Channel 配置 | PPIO 默认域名已迁移至 `api.ppinfra.com`，旧域名 `api.ppio.com` 不可用 | Channel base_url 使用旧域名，所有 PPIO 请求返回 404 |
| 17 | P1 | Channel 配置 | Anthropic 通道（base_url 含 `/anthropic`）中包含非 Claude 模型 | 非 Claude 模型被随机路由到 Anthropic 通道，拼接 `/chat/completions` 后 URL 不兼容，约 50% 请求 404 |
| 18 | P1 | 环境变量 | 未设置 `ENTERPRISE_BASE_URL` | 「我的接入」页面显示内网地址 `ai.paigod.work/v1`，用户在公网无法使用 |
| ~~19~~ | ~~P0~~ | ~~部署~~ | ~~`go build` 输出路径 vs systemd 运行路径不一致~~ | ~~Docker 容器内路径固定，不再需要复制二进制~~ |
| ~~20~~ | ~~P1~~ | ~~部署~~ | ~~运行时 `cp` 覆盖二进制报 "Text file busy"~~ | ~~Docker 容器替换，不涉及文件覆盖~~ |
| ~~21~~ | ~~P0~~ | ~~前端部署~~ | ~~`go:embed` 目录未同步~~ | ~~Dockerfile `COPY --from` 自动完成，不可能遗漏~~ |
| 22 | P0 | Nginx / CLB 超时 | Nginx `proxy_read_timeout` 或 CLB 后端超时 < 900s | Claude Code 等使用 extended thinking 的工具报 504 Gateway Time-out（stgw），以及连锁错误 `Cannot read properties of undefined (reading 'trim')`。详见 §3.6 和 §7.1 |
| 23 | P1 | 零停机部署 | Nginx 未安装 `aiproxy-upstream.conf`，`proxy_pass` 仍硬编码 `127.0.0.1:3000` | 零停机部署脚本无法切换流量，部署失败。需先执行 §10.4 的一次性初始化 |
| 24 | P2 | 零停机部署 | `/data/aiproxy/.active-port` 状态文件内容与实际运行端口不一致 | 下次部署会尝试在已占用的端口启动 canary，启动失败 |
| 25 | P1 | Docker 构建 | `.dockerignore` 排除了 `mcp-servers/**/*.md` | `go:embed` 找不到 `README.md` / `README.cn.md`，`go build` 失败。MCP server 的 `init.go` 用 `go:embed` 嵌入这些文件 |
| 26 | P2 | Docker 构建 | Dockerfile 中 `pnpm install` 未设置 `CI=true` | pnpm 在无 TTY 的 Docker build 环境中检测到需要删除 `node_modules` 时会中止，报 `ERR_PNPM_ABORTED_REMOVE_MODULES_DIR_NO_TTY` |
| 27 | P2 | Docker 构建 | ~~已修复~~ Dockerfile 现通过 `ARG USE_CN_MIRROR=true` 条件化设置 `GOPROXY`。国内构建默认使用 `goproxy.cn`，海外构建（`--build-arg USE_CN_MIRROR=false`）使用官方源。`deploy.sh` 根据 `NODE_TYPE` 自动传递此参数 | ~~不再是易错点~~ |
| 28 | P1 | 多节点 | 海外 `.env` 中 `ENTERPRISE_BASE_URL` 或 `FEISHU_REDIRECT_URI` 使用了国内域名 `paigod.work` | 海外员工看到国内域名的 Base URL / OAuth 回调指向国内。`deploy.sh` 预检会发出警告 |
| 29 | P2 | 多节点 | DB schema 变更未向后兼容（如删列、改名） | 国内先部署后海外旧代码仍在跑，查询崩溃导致海外服务中断 |
| 30 | P1 | Docker 网络 | `iptables` NAT MASQUERADE 规则丢失（防火墙变更、WireGuard 重配等触发） | Docker 容器内 DNS 解析失败，构建和运行均报 `temporary error` / `bad address`。宿主机网络正常但容器不通。**修复**：`sudo iptables -t nat -A POSTROUTING -s 172.17.0.0/16 ! -o docker0 -j MASQUERADE`，或重启 Docker daemon。`deploy.sh` 预检已自动检测并修复 |
| 31 | P2 | 部署脚本 | `sudo bash scripts/deploy.sh` 执行 `git pull` 时 SSH key 无权限 | `sudo` 丢失 SSH agent forwarding，`git pull` 报 `Permission denied (publickey)`。**修复**：脚本已自动回退到显式 SSH key，或手动 `--no-pull` 后单独拉取 |
| 4 | P1 | 备份 | 启用 `LOG_SQL_DSN` 后只备份主库 | 恢复时丢失请求日志和审计数据 |

---

## 多节点部署（国内 + 海外）

### 架构概述

| 节点 | 云平台 | 服务器 | 用途 | 域名 |
|------|--------|--------|------|------|
| 国内（主） | 腾讯云 | 1.13.81.31 | 全功能：API + 管理后台 + DB | `ai.paigod.work` + `apiproxy.paigod.work` |
| 海外（从） | AWS us-west-2 | 52.35.158.131 | 全功能：API + 管理后台（无本地 DB） | `ai.pplabs.tech` + `apiproxy.pplabs.tech` |

- **数据库共享**：海外节点通过 WireGuard 隧道连回国内 PostgreSQL，无独立 DB
- **渠道分流**：海外节点通过 `NODE_CHANNEL_SET=overseas` 环境变量自动优先使用海外渠道，无匹配时 fallback 到 default（PPIO）
- **管理后台**：国内 `ai.paigod.work`，海外 `ai.pplabs.tech`（同一 DB，数据互通）

### 渠道分流机制

优先级（高→低）：
1. **Group.AvailableSets**（显式配置）— 管理员手动为某个 Group 指定渠道集
2. **NODE_CHANNEL_SET 环境变量**（服务器级）— 海外节点设为 `overseas`，自动优先海外渠道 + default 兜底
3. **默认值** `["default"]` — 国内节点无需配置，保持原有行为

### 多节点一键部署

```bash
# 全量部署（两个节点依次执行）
ADMIN_KEY=xxx bash scripts/deploy-all.sh

# 跳过 git pull
ADMIN_KEY=xxx DEPLOY_ARGS="--no-pull" bash scripts/deploy-all.sh

# 仅构建
bash scripts/deploy-all.sh --build-only

# 单节点部署（直接 SSH）
ssh ppuser@52.35.158.131 "cd /data/aiproxy && sudo bash -c 'export ADMIN_KEY=xxx NODE_TYPE=overseas && bash scripts/deploy.sh'"
```

`NODE_TYPE` 控制 Dockerfile 镜像源：
- `domestic`（默认）：使用国内镜像，APK 回退链：腾讯云 → 清华 → 阿里云（任一不可用自动切换下一个）；npm 用 npmmirror，Go 用 goproxy.cn
- `overseas`：使用官方镜像（npmjs.org、proxy.golang.org、dl-cdn.alpinelinux.org）

### 海外服务器首次初始化清单

> 以下步骤在海外服务器上**仅执行一次**。

#### 1. 基础设置

```bash
# 创建 ppuser 用户（如尚未存在）
sudo adduser ppuser
sudo usermod -aG sudo ppuser

# 配置 SSH key 登录
sudo mkdir -p /home/ppuser/.ssh
sudo cp ~/.ssh/authorized_keys /home/ppuser/.ssh/
sudo chown -R ppuser:ppuser /home/ppuser/.ssh

# 时区
sudo timedatectl set-timezone America/Los_Angeles
```

#### 2. 安装 Docker

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker ppuser
```

#### 3. 安装 WireGuard 并配置隧道

```bash
sudo apt install -y wireguard
# 编辑 /etc/wireguard/wg0.conf（参考 VPN 文档配置对端地址）
# 确保 PersistentKeepalive = 25
sudo systemctl enable --now wg-quick@wg0
```

#### 4. 验证隧道和 DB 连通性

```bash
ping -c 3 10.x.x.1                          # WireGuard 对端
psql -h 10.x.x.1 -U postgres -d aiproxy -c "SELECT 1"   # DB
```

#### 5. 安装 Nginx

```bash
sudo apt install -y nginx
sudo cp deploy/nginx/overseas/aiproxy-upstream.conf /etc/nginx/conf.d/
# 安装海外节点 Nginx 站点配置（需根据实际域名调整 server_name）
sudo cp deploy/nginx/overseas/apiproxy.pplabs.tech.conf /etc/nginx/sites-available/
sudo ln -sf /etc/nginx/sites-available/apiproxy.pplabs.tech.conf /etc/nginx/sites-enabled/
# 如需管理后台，同样安装 ai.pplabs.tech.conf
sudo cp deploy/nginx/overseas/ai.pplabs.tech.conf /etc/nginx/sites-available/
sudo ln -sf /etc/nginx/sites-available/ai.pplabs.tech.conf /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx
```

#### 6. AWS ALB + ACM 证书

- 在 AWS us-west-2 创建 Application Load Balancer（ALB）
- 在 ACM（AWS Certificate Manager）申请 `*.pplabs.tech` 证书（免费，自动续签）
- ALB 监听器：HTTPS:443，按 Host Header 转发：
  - `ai.pplabs.tech` → Target Group（EC2:80，HTTP）
  - `apiproxy.pplabs.tech` → Target Group（EC2:81，HTTP）
- ALB Security Group：仅允许入站 443（公网）
- EC2 Security Group：仅允许 ALB 安全组访问 80/81 + 管理员 SSH 22 + WireGuard UDP 51820

#### 7. DNS 记录

| 域名 | 记录类型 | 值 | 说明 |
|------|---------|-----|------|
| `ai.pplabs.tech` | CNAME（或 A） | AWS ALB DNS 名称（或 EIP） | 海外企业前端（公网，路径隔离） |
| `apiproxy.pplabs.tech` | CNAME（或 A） | AWS ALB DNS 名称（或 EIP） | 海外 AI API |

#### 8. Clone 代码

```bash
sudo mkdir -p /data/aiproxy
sudo chown ppuser:ppuser /data/aiproxy
git clone git@github.com:your-org/aiproxy.git /data/aiproxy
```

#### 9. 创建 .env 文件

```bash
cat > /data/aiproxy/.env << 'EOF'
SQL_DSN=postgres://postgres:xxx@10.x.x.1:5432/aiproxy
LOG_SQL_DSN=postgres://postgres:xxx@10.x.x.1:5432/aiproxy
REDIS_CONN_STRING=redis://127.0.0.1:6379

NODE_CHANNEL_SET=overseas
BATCH_UPDATE_INTERVAL_SECONDS=10

FEISHU_APP_ID=xxx
FEISHU_APP_SECRET=xxx
FEISHU_REDIRECT_URI=https://ai.pplabs.tech/api/enterprise/auth/feishu/callback
FEISHU_FRONTEND_URL=https://ai.pplabs.tech

ENTERPRISE_BASE_URL=https://apiproxy.pplabs.tech/v1

ADMIN_KEY=xxx
TZ=America/Los_Angeles
EOF

chmod 600 /data/aiproxy/.env
```

#### 10. 启动 Redis

```bash
docker run -d --name redis --restart unless-stopped \
  --network host \
  -v /data/redis:/data \
  redis:latest --bind 127.0.0.1
```

#### 11. 首次部署

```bash
cd /data/aiproxy
sudo bash -c 'export NODE_TYPE=overseas ADMIN_KEY=xxx && bash scripts/deploy.sh'
```

> **注意：** 海外节点 DB 查询通过 WireGuard 隧道延迟较高（1-3s/查询），部署脚本在 `NODE_TYPE=overseas` 时自动将健康检查超时从 90s 延长至 600s。如需手动覆盖：`export HEALTH_TIMEOUT=xxx`。

#### 12. 配置 WireGuard 健康监控

```bash
# 编辑 crontab
crontab -e
# 添加：
# */1 * * * * PEER_IP=10.0.0.1 FEISHU_WEBHOOK=https://open.feishu.cn/... bash /data/aiproxy/scripts/wireguard-health.sh 2>&1
```

#### 13. 管理后台配置渠道

在 `ai.paigod.work` 管理后台中：
1. 启用海外直连渠道
2. 给海外渠道的 Sets 字段设置为 `["overseas"]`
3. 验证：海外节点请求 → 优先 overseas 渠道；国内节点请求 → default（PPIO）

### WireGuard 隧道故障处理

如果海外节点报告 DB 连接失败：

```bash
# 检查隧道状态
sudo wg show
ping -c 3 10.x.x.1

# 重启隧道
sudo systemctl restart wg-quick@wg0

# 检查 DB 连通性
psql -h 10.x.x.1 -U postgres -d aiproxy -c "SELECT 1"

# 查看健康检查日志
tail -50 /var/log/wireguard-health.log
```

---

## 附录：环境变量完整参考

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `ADMIN_KEY` | 是 | - | 管理员 API 密钥 |
| `SQL_DSN` | 否 | SQLite `./aiproxy.db` | PostgreSQL 连接串 |
| `LOG_SQL_DSN` | 否 | 与 `SQL_DSN` 相同 | 日志独立数据库 |
| `REDIS` / `REDIS_CONN_STRING` | 否 | 内存缓存 | Redis 连接串 |
| `REDIS_KEY_PREFIX` | 否 | 空 | Redis key 前缀 |
| `FEISHU_APP_ID` | 企业版必填 | - | 飞书应用 App ID |
| `FEISHU_APP_SECRET` | 企业版必填 | - | 飞书应用 App Secret |
| `FEISHU_REDIRECT_URI` | 企业版必填 | - | OAuth 回调 URL（必须 `ai.paigod.work`） |
| `FEISHU_FRONTEND_URL` | 企业版必填 | - | 前端基础 URL（必须 `ai.paigod.work`） |
| `FEISHU_ALLOWED_TENANTS` | 否 | 允许所有 | 租户白名单，`*` 或逗号分隔 |
| `ENTERPRISE_BASE_URL` | 企业版必填 | 请求 Host + `/v1` | 「我的接入」页面展示的公网 Base URL（如 `https://apiproxy.paigod.work/v1`） |
| `ENTERPRISE_BASE_URLS` | 否 | - | 多地域接入地址 JSON，key 为渠道 owner（即 `ChannelType.String()`），value 为 Base URL（如 `{"ppio":"https://apiproxy.paigod.work/v1","海外":"https://apiproxy.pplabs.tech/v1"}`）。设置后「我的接入」页面各渠道分组显示对应地址。**两个节点必须配置相同值** |
| `NOTIFY_FEISHU_WEBHOOK` | 否 | - | 飞书 Bot Webhook URL |
| `NODE_CHANNEL_SET` | 否 | 空 | 服务器级默认渠道集（如 `overseas`），优先匹配该 Set 的渠道，fallback 到 default |
| `NODE_TYPE` | 否 | `domestic` | 构建时指定节点类型，控制 Dockerfile 镜像源（`domestic`/`overseas`）+ 海外自动 HEALTH_TIMEOUT=600s |
| `HEALTH_TIMEOUT` | 否 | 90（国内）/ 600（海外） | 部署脚本健康检查超时秒数，`NODE_TYPE=overseas` 时自动 600s |
| `BATCH_UPDATE_INTERVAL_SECONDS` | 否 | `5` | 批处理刷新间隔（秒）。海外高延迟节点建议设为 `10`~`15`，减少跨洲 DB 写入频率 |
| `FFMPEG_ENABLED` | 否 | `false` | 启用 ffmpeg |
| `GZIP_ENABLED` | 否 | `false` | 启用 gzip 压缩 |
| `LOG_DETAIL_STORAGE_HOURS` | 否 | 不限 | 日志详情保留时长（小时） |
| `DEBUG` | 否 | `false` | 调试模式 |
| `TZ` | 否 | UTC | 时区 |

