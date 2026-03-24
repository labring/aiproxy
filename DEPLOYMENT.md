# AI Proxy 生产部署指南

> 双域名架构 | 单机部署 | 腾讯云 CLB + CVM | 400 人规模
>
> - `ai.paigod.work` — 管理后台（内网），规避等保
> - `apiproxy.paigod.work` — AI API 服务（公网），供客户端调用

---

## 一、架构总览

```
                    ┌──────────────────┐
                    │  腾讯云 CLB       │
                    │  (SSL 终结)       │
                    │                  │
  企业内网            │  ai:443 → :80   │
  ┌──────────────┐  │  apiproxy:443    │
  │ 员工浏览器     │──→│     → :81       │
  │ ai.paigod.work│  └───────┬────────┘
  └──────────────┘          │
                    ┌───────▼─────────────────────────────┐
  公网               │           腾讯云 CVM (8C16G)          │
  ┌───────────────┐ │   ┌───────────┐    ┌──────────────┐ │
  │ AI 客户端       │─→│   │  Nginx    │──→│  AI Proxy     │ │
  │apiproxy.       │ │   │  :80 内网  │    │  :3000       │ │
  │ paigod.work   │ │   │  :81 公网  │    │              │ │
  │ Cursor/ChatBox │ │   │  路径分流   │    │ 单进程，同时   │ │
  └───────────────┘ │   │           │    │ 提供前端 +API  │ │──→ OpenAI / Anthropic
                    │   └───────────┘    └──────────────┘ │    / Gemini 等
                    │         │                │          │
                    │   ┌─────┴──┐    ┌───────┴────────┐ │
                    │   │ Redis  │    │  PostgreSQL    │ │
                    │   │ :6379  │    │  :5432         │ │
                    │   └────────┘    └────────────────┘ │
                    └─────────────────────────────────────┘
```

### 核心设计：CLB SSL 终结 + Nginx 按端口做路径级隔离

| 域名 | 网络 | CLB 监听 | Nginx 端口 | 开放路径 | 用途 |
|------|------|---------|-----------|---------|------|
| `ai.paigod.work` | 内网 | `:443 → CVM:80` | `:80` | 全部（`/`, `/api/*`, `/swagger/*`） | 管理后台、飞书登录、企业分析 |
| `apiproxy.paigod.work` | 公网 | `:443 → CVM:81` | `:81` | 仅 `/v1/*`, `/mcp/*`, `/sse`, `/message` | AI 模型调用、MCP 协议 |

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

| 项目 | 值 |
|------|-----|
| **CVM 公网 IP** | `1.13.81.31` |
| **CVM 内网 IP** | `10.206.0.10` |
| **操作系统** | Ubuntu 22.04.5 LTS |
| **登录用户** | `ppuser`（sudo 权限，密钥认证） |
| **AI Proxy 服务端口** | `3000`（仅监听本地，不直接暴露） |
| **Nginx 端口 80** | 反代至 `127.0.0.1:3000` — 内网管理后台 |
| **Nginx 端口 81** | 反代至 `127.0.0.1:3000` — 公网 AI API（白名单路径） |
| **PostgreSQL** | Docker 容器，`127.0.0.1:5432` |
| **Redis** | Docker 容器，`127.0.0.1:6379` |
| **代码部署方式** | GitHub SSH Deploy Key，仓库 `mashoushan1989/aiproxy` |
| **代码路径** | `/data/aiproxy` |
| **二进制路径** | `/data/aiproxy/aiproxy` |
| **Systemd 服务** | `aiproxy.service`（User=aiproxy） |
| **数据库备份** | 每日 03:00 自动备份至 `/data/backup/`，保留 30 天 |

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

> **[易错点] 两个域名都指向 CLB 的公网 IP（VIP），不是直接指向 CVM！**

在腾讯云 DNSPod 添加以下记录：

| 记录类型 | 主机记录 | 记录值 | 说明 |
|---------|---------|--------|------|
| A | `ai` | `<CLB 公网 VIP>` | 管理后台（通过 CLB 白名单限内网访问） |
| A | `apiproxy` | `<CLB 公网 VIP>` | AI API（公网可达） |

> **内网限制方案：**
>
> - **方案 A（推荐）：** 使用**企业内部 DNS**将 `ai.paigod.work` 指向 CLB 内网 VIP。公网 DNS 中不添加 `ai` 记录，确保外网完全不可达。
> - **方案 B：** 在公网 DNS 添加 `ai` 记录指向 CLB 公网 VIP，然后在 CLB 安全组或 Nginx 中限制来源 IP（仅允许公司出口 IP）。

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

### 用户使用（公网 `apiproxy.paigod.work`）

| 用途 | 地址 |
|------|------|
| **AI API 基础地址** | `https://apiproxy.paigod.work/v1` |
| **MCP 协议** | `https://apiproxy.paigod.work/mcp/*` |
| **SSE 端点** | `https://apiproxy.paigod.work/sse` |
| **健康检查** | `https://apiproxy.paigod.work/v1/models`（Token 认证） |

用户在 AI 客户端（Cursor、ChatBox、LobeChat、Cherry Studio 等）中配置：

```
API Base URL: https://apiproxy.paigod.work
API Key:      sk-xxxx（管理员通过内网后台分发的 Token Key）
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
ENTERPRISE_BASE_URL=https://apiproxy.paigod.work/v1

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
```

### 6.4 创建专用系统用户

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

创建 `/etc/nginx/sites-available/ai.paigod.work`：

```nginx
# 内网管理后台 — CLB(443) → CVM(80)，开放全部路径
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
    location / {
        proxy_pass http://127.0.0.1:3000;
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
        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }
}
```

### 7.2 公网域名配置（端口 81）：`apiproxy.paigod.work`

创建 `/etc/nginx/sites-available/apiproxy.paigod.work`：

```nginx
# 公网 AI API — CLB(443) → CVM(81)，仅开放白名单路径
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
    proxy_connect_timeout 60s;
    proxy_send_timeout 300s;
    proxy_read_timeout 300s;

    # ============================================================
    # 白名单路径：仅开放 AI API 和 MCP 相关端点
    # 这些端点都有 Token 认证保护（middleware.TokenAuth / MCPAuth）
    # ============================================================

    # OpenAI 兼容 API（/v1/chat/completions, /v1/models, /v1/responses 等）
    location /v1/ {
        proxy_pass http://127.0.0.1:3000;
    }

    # Gemini 兼容 API
    location /v1beta/ {
        proxy_pass http://127.0.0.1:3000;
    }

    # MCP 协议端点
    location /mcp/ {
        proxy_pass http://127.0.0.1:3000;
    }

    # MCP SSE 端点
    location = /sse {
        proxy_pass http://127.0.0.1:3000;
    }

    # MCP Streamable 端点
    location = /mcp {
        proxy_pass http://127.0.0.1:3000;
    }

    # MCP Message 端点
    location = /message {
        proxy_pass http://127.0.0.1:3000;
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
- Token 级别启用 RPM/TPM 限流，防滥用
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

---

## 十、监控与运维

### 10.1 日志查看

```bash
# AI Proxy 日志
sudo journalctl -u aiproxy -f

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

### 10.4 更新升级

> **[易错点] 更新时必须重新编译前端（因为 `VITE_API_BASE_URL` 是构建时写入的），否则前端会调用错误的 API 地址。**

#### 标准更新流程

```bash
# ============================================================
# 第 1 步：拉取代码（服务不中断，0 影响）
# ============================================================
# 目录归属给 aiproxy 用户，需临时切回 ppuser 操作 git
sudo chown -R ppuser:ppuser /data/aiproxy
cd /data/aiproxy && git pull

# ============================================================
# 第 2 步：编译前端（服务不中断，~15 秒）
# ============================================================
cd /data/aiproxy/web
echo 'VITE_API_BASE_URL=https://ai.paigod.work/api' > .env.production
pnpm install && pnpm run build

# ============================================================
# 第 3 步：嵌入前端 + 编译后端（服务不中断，~30 秒）
# ============================================================
cd /data/aiproxy
rsync -a --delete web/dist/ core/public/dist/
cd core
export PATH=$PATH:/usr/local/go/bin GOPROXY=https://goproxy.cn,direct
go build -tags enterprise -trimpath -ldflags "-s -w" -o /data/aiproxy/aiproxy

# ============================================================
# 第 4 步：重启服务（唯一中断窗口，~2-3 秒）
# ============================================================
sudo chown -R aiproxy:aiproxy /data/aiproxy
sudo chmod 755 /data/aiproxy
sudo systemctl restart aiproxy

# 验证
curl -s http://127.0.0.1:3000/api/status
sudo journalctl -u aiproxy -n 20 --no-pager
```

#### 用户影响评估

| 阶段 | 耗时 | 服务影响 |
|------|------|---------|
| git pull | ~3 秒 | 无，服务持续运行 |
| 前端编译 | ~15 秒 | 无，服务持续运行 |
| 后端编译 | ~30 秒 | 无，服务持续运行 |
| **systemctl restart** | **~2-3 秒** | **服务中断** |

> **总影响：约 2-3 秒断线。** 正在进行的 API 请求和 SSE 流式响应会被中断，新请求短暂拒绝。服务启动后自动恢复，大部分 AI 客户端（Cursor、ChatBox 等）有自动重试机制，用户基本无感。
>
> **建议更新时间：** 工作日午休（12:00-13:00）或非工作时间，避开使用高峰。

#### 快速回滚

如果更新后发现问题，可快速回滚到上一版本：

```bash
# 回退到上一个 commit
sudo chown -R ppuser:ppuser /data/aiproxy
cd /data/aiproxy && git log --oneline -5   # 确认要回退到的版本
git checkout <上一个commit-hash>

# 重新编译（同上述第 2-4 步）
cd web && pnpm run build
cd .. && rsync -a --delete web/dist/ core/public/dist/
cd core && go build -tags enterprise -trimpath -ldflags "-s -w" -o /data/aiproxy/aiproxy
sudo chown -R aiproxy:aiproxy /data/aiproxy
sudo chmod 755 /data/aiproxy
sudo systemctl restart aiproxy
```

#### 未来零停机升级方案（按需）

当前单机架构重启时有 2-3 秒中断。如业务增长后需要零停机：

| 方案 | 停机时间 | 额外成本 | 复杂度 |
|------|---------|---------|--------|
| **双进程热切换** | < 0.5 秒 | 无 | 中等（新二进制用临时端口启动，Nginx upstream 切换后关闭旧进程） |
| **双 CVM + CLB 滚动更新** | 0 秒 | 服务器成本翻倍 | 低（CLB 摘流量 → 更新 → 挂回，逐台操作） |

---

## 十一、部署检查清单

### 基础设施

- [ ] 服务器已购买并初始化（8C16G，Ubuntu 22.04）
- [ ] 数据盘已挂载至 `/data`
- [ ] Docker、Go 1.26+、Node.js 22、pnpm 已安装
- [ ] PostgreSQL + Redis 容器已启动且 healthy

### 编译部署

- [ ] 前端编译时 `.env.production` 设置了 `VITE_API_BASE_URL=https://ai.paigod.work/api`
- [ ] 后端编译包含 `-tags enterprise`
- [ ] `/data/aiproxy/.env` 中 `FEISHU_REDIRECT_URI` 和 `FEISHU_FRONTEND_URL` 都指向 `ai.paigod.work`
- [ ] `/data/aiproxy/.env` 中 `ENTERPRISE_BASE_URL=https://apiproxy.paigod.work/v1`（否则「我的接入」页面显示内网地址）
- [ ] 专用 `aiproxy` 系统用户已创建，`/data/aiproxy` 目录归属正确
- [ ] `aiproxy.service` 中 `User=aiproxy`，`ReadWritePaths` 包含 `/data/aiproxy /tmp`
- [ ] AI Proxy systemd 服务已启动，`curl http://127.0.0.1:3000/api/status` 返回正常

### CLB & 域名 & 网络

- [ ] CLB 已创建，HTTPS:443 监听器已配置
- [ ] `ai.paigod.work` 转发规则 → CVM:80
- [ ] `apiproxy.paigod.work` 转发规则 → CVM:81
- [ ] SSL 证书已上传至 CLB 并绑定（推荐通配符 `*.paigod.work`）
- [ ] `ai.paigod.work` DNS 解析到 CLB VIP（内网 DNS 或公网 + IP 限制）
- [ ] `apiproxy.paigod.work` DNS 解析到 CLB 公网 VIP
- [ ] Nginx 监听 80（ai）和 81（apiproxy），`nginx -t` 通过
- [ ] CVM 安全组仅允许 CLB 访问 80/81 + 管理员 SSH 22
- [ ] UFW 防火墙已启用

### 公网隔离验证

- [ ] 从公网访问 `https://apiproxy.paigod.work/` 返回 403
- [ ] 从公网访问 `https://apiproxy.paigod.work/api/status` 返回 403
- [ ] 从公网访问 `https://apiproxy.paigod.work/swagger/index.html` 返回 403
- [ ] 从公网访问 `https://apiproxy.paigod.work/v1/models`（带 Token）返回 200
- [ ] 直接访问 CVM 公网 IP:80 和 :81 不可达

### 飞书 & 基础功能验证

- [ ] 飞书应用已创建，权限已审批，重定向 URL 配置为 `https://ai.paigod.work/...`
- [ ] 在内网访问飞书登录链接测试通过
- [ ] 管理后台可正常访问（内网）
- [ ] 至少添加一个 Channel（AI Provider），从公网 API 测试调用正常
- [ ] **PPIO Channel base_url 使用 `api.ppinfra.com`**（非 `api.ppio.com`）：OpenAI 通道 → `https://api.ppinfra.com/v3/openai`，Anthropic 通道 → `https://api.ppinfra.com/v3/anthropic`
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

---

## 十二、易错点汇总

| # | 优先级 | 位置 | 易错描述 | 后果 |
|---|--------|------|---------|------|
| 1 | P0 | CLB 健康检查 | 81 端口健康检查未改为 TCP，沿用默认 HTTP 打 `/` | `/` 返回 403，CLB 持续判后端不健康，线上 502 |
| 2 | P1 | systemd | `ReadWritePaths` 未包含 `/tmp` | `/v1/audio/transcriptions` 写临时文件失败，返回 500 |
| 3 | P1 | systemd | `User=root` 运行 | 服务被攻破直接获得 root 权限，安全加固形同虚设 |
| 4 | P1 | 备份 | 启用 `LOG_SQL_DSN` 后只备份主库 | 恢复时丢失请求日志和审计数据 |
| 5 | P2 | 前端部署 | 更新时用 `cp -r` 而非 `rsync --delete` | 旧资源文件残留，升级后出现静态资源错配 |
| 6 | P2 | CLB 监听器 | 两个转发规则后端端口搞反（ai→81, apiproxy→80） | 管理后台暴露公网，或 API 端口不通 |
| 7 | P2 | DNS 解析 | 域名直接指向 CVM 公网 IP 而非 CLB VIP | SSL 不生效，直连 HTTP 暴露端口 |
| 8 | P2 | 前端编译 | 忘记设置 `VITE_API_BASE_URL`，使用了默认的 `localhost:3000` | 前端页面加载后调不通后端 API，所有操作失败 |
| 9 | P2 | 后端编译 | 忘记 `-tags enterprise` | 飞书登录、企业分析等路由不存在，返回 404，无报错 |
| 10 | P2 | 环境变量 | `FEISHU_REDIRECT_URI` 或 `FEISHU_FRONTEND_URL` 写成了 `apiproxy.paigod.work` | OAuth 回调指向公网域名，被 Nginx 403 拦截，登录失败 |
| 11 | P2 | 飞书平台 | 开放平台的重定向 URL 与 `.env` 中的 `FEISHU_REDIRECT_URI` 不一致 | 飞书报"重定向 URI 不匹配"错误 |
| 12 | P2 | Nginx | `apiproxy.paigod.work` 的 `location /` 兜底规则缺失 | 公网可访问管理后台和 Swagger，安全漏洞 |
| 13 | P2 | Nginx | 没有删除 `/etc/nginx/sites-enabled/default` | default server 可能拦截请求，导致两个域名都 404 |
| 14 | P2 | 安全组 | CVM 安全组把 80/81 开放给 0.0.0.0/0 | 可绕过 CLB 直连 CVM，绕过 SSL 和 CLB 安全策略 |
| 18 | P1 | 环境变量 | 未设置 `ENTERPRISE_BASE_URL` | 「我的接入」页面显示内网地址 `ai.paigod.work/v1`，用户在公网无法使用 |
| 15 | P3 | docker-compose | PostgreSQL/Redis 密码中包含特殊字符（`@`, `#`, `%`） | 连接串解析失败，AI Proxy 启动报数据库连接错误 |
| 16 | P1 | Channel 配置 | PPIO 默认域名已迁移至 `api.ppinfra.com`，旧域名 `api.ppio.com` 不可用 | Channel base_url 使用旧域名，所有 PPIO 请求返回 404 |
| 17 | P1 | Channel 配置 | Anthropic 通道（base_url 含 `/anthropic`）中包含非 Claude 模型 | 非 Claude 模型被随机路由到 Anthropic 通道，拼接 `/chat/completions` 后 URL 不兼容，约 50% 请求 404 |

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
| `NOTIFY_FEISHU_WEBHOOK` | 否 | - | 飞书 Bot Webhook URL |
| `FFMPEG_ENABLED` | 否 | `false` | 启用 ffmpeg |
| `GZIP_ENABLED` | 否 | `false` | 启用 gzip 压缩 |
| `LOG_DETAIL_STORAGE_HOURS` | 否 | 不限 | 日志详情保留时长（小时） |
| `DEBUG` | 否 | `false` | 调试模式 |
| `TZ` | 否 | UTC | 时区 |
