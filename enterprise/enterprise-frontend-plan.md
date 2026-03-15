# 企业版前端页面开发计划

## Context

后端企业版模块（飞书OAuth、用量分析、额度策略）已全部完成。现在需要添加企业版前端页面，让内部员工通过飞书 OAuth 登录后直接进入企业版 Dashboard，同时提供与现有管理后台的双向导航。

**用户需求：** 内部员工登录后优先看到企业版界面，企业版和管理后台之间有良好的互通导航。

---

## 现有前端架构分析

### 技术栈
- React + Vite + TypeScript
- react-router v7（`createBrowserRouter`）
- Zustand（状态管理，persist 中间件）
- shadcn/ui（Radix UI + TailwindCSS）
- TanStack Query（数据请求）
- i18next（国际化，从 `/locales/{lng}/translation.json` 加载）
- lucide-react（图标）

### 当前认证流程
1. `LoginPage`（`web/src/pages/auth/login.tsx`）— 用户输入 Admin Token
2. `useLoginMutation`（`web/src/feature/auth/hooks.ts`）— 调 `getChannelTypeMetas(token)` 验证
3. 验证通过 → `authStore.login(token)` 存入 Zustand（持久化到 localStorage）
4. `ProtectedRoute`（`web/src/feature/auth/components/ProtectedRoute.tsx`）— 检查 `isAuthenticated`
5. 默认跳转 `/monitor`

### 当前路由结构
```
/login              → LoginPage（lazy loaded）
/                   → Redirect to /monitor
/monitor            → MonitorPage      ┐
/group              → GroupPage         │
/key                → TokenPage         │ 这些路由都在
/channel            → ChannelPage       │ ProtectedRoute → RootLayout → Outlet
/model              → ModelPage         │ 下面
/log                → LogPage           │
/mcp-front          → MCPPage           ┘
```

### 关键文件
| 文件 | 作用 |
|------|------|
| `web/src/routes/config.tsx` | 路由配置，`useRoutes()` 返回 `RouteObject[]` |
| `web/src/routes/constants.ts` | 路由路径常量 `ROUTES` |
| `web/src/store/auth.ts` | Auth Zustand store（`token`, `isAuthenticated`, `login`, `logout`） |
| `web/src/components/layout/RootLayOut.tsx` | 管理后台布局（Sidebar + Outlet） |
| `web/src/components/layout/SideBar.tsx` | 管理后台侧边栏（紫色渐变 + 粒子效果） |
| `web/src/api/index.ts` | Axios 封装（`get`/`post`/`put`/`del`），自动附加 `Authorization` header |
| `web/src/api/services.ts` | API 导出汇总 |
| `web/src/pages/auth/login.tsx` | 登录页 UI |
| `core/router/static.go` | SPA fallback — `NoRoute` handler 对非 `/api`、`/mcp`、`/v1` 路径返回 `index.html` |

### 后端企业版 API（已完成）

> **重要：** 当前 analytics/quota/feishu管理 接口均在 `AdminAuth` 中间件后面（`core/enterprise/router.go:22-24`）。
> `AdminAuth`（`core/middleware/auth.go:41-60`）检查的是 `config.AdminKey`（环境变量 `ADMIN_KEY`），
> 而飞书 OAuth 返回的 `token_key` 是普通 Token，不是 AdminKey。
> **因此飞书用户无法直接调用 analytics 等 admin 接口，需要后端调整（见下方"后端必要修改"章节）。**

| 路由 | 当前 Auth | 调整后 Auth | 返回 |
|------|-----------|-------------|------|
| `GET /api/enterprise/auth/feishu/login` | 无 | 无（不变） | 302 跳转到飞书授权页 |
| `GET /api/enterprise/auth/feishu/callback?code=xxx` | 无 | 无（不变） | `{token_key, user: {open_id, name, email, avatar}}` |
| `GET /api/enterprise/analytics/department` | **Admin** | **TokenAuth** | `{departments, total}` |
| `GET /api/enterprise/analytics/department/:id/trend` | **Admin** | **TokenAuth** | `{department_id, trend}` |
| `GET /api/enterprise/analytics/user/ranking` | **Admin** | **TokenAuth** | `{ranking, total}` |
| `GET /api/enterprise/analytics/export` | **Admin** | **TokenAuth** | Excel 文件下载 |
| `GET /api/enterprise/feishu/users` | Admin | Admin（不变） | `{users, total}` |
| `GET /api/enterprise/feishu/departments` | Admin | Admin（不变） | `{departments, total}` |
| `POST /api/enterprise/feishu/sync` | Admin | Admin（不变） | `{message}` |
| `GET /api/enterprise/quota/policies` | Admin | Admin（不变） | `{policies, total}` |

---

## 设计方案

### 登录流程

在现有 Login 页面增加「飞书登录」按钮：

```
用户点击「飞书登录」
  → 浏览器跳转 GET /api/enterprise/auth/feishu/login
  → 飞书授权页面
  → 飞书回调到 FEISHU_REDIRECT_URI（前端路由 /feishu/callback?code=xxx）
  → 前端 /feishu/callback 页面提取 code
  → GET /api/enterprise/auth/feishu/callback?code=xxx（不带 Authorization header）
  → 拿到 {token_key, user}
  → authStore.loginWithFeishu(token_key, user)
  → 跳转到 /enterprise
```

管理员用 Admin Token 登录 → 照旧跳 `/monitor`。

**SPA 路由 fallback 已确认：** `core/router/static.go` 中 `NoRoute` handler 对 `/feishu/callback` 路径会返回 `index.html`（因为 `checkNoRouteNotFound` 只排除 `/api`、`/mcp`、`/v1` 前缀），前端 router 可以正常接管。

### 路由结构

```
/login                      — 登录页（新增飞书登录按钮）
/feishu/callback            — 飞书 OAuth 中转页（显示 loading，自动处理 code 交换）
/enterprise                 — 企业版 Dashboard ─┐
/enterprise/ranking         — 员工排行榜        │ ProtectedRoute → EnterpriseLayout
/enterprise/department/:id  — 部门趋势详情      ─┘
/monitor, /group, ...       — 现有管理后台（不变）
```

### 默认跳转逻辑

当前 `routes/config.tsx` 中 `/` 路由硬编码重定向到 `/monitor`。修改为根据用户类型动态跳转：

```typescript
// routes/config.tsx 中 / 路由
{
  path: "/",
  element: <SmartRedirect />  // 新组件，替代 Navigate
}

// SmartRedirect 组件逻辑：
// if (authStore.enterpriseUser) → Navigate to /enterprise
// else → Navigate to /monitor
```

这样飞书用户刷新页面或直接访问 `/` 时会被导向企业版，而非管理后台。

### 布局设计

**EnterpriseLayout** — 新建组件，复用 Sidebar 设计语言（紫色渐变 + 粒子效果）：

```
┌──────────────────────────────────────────┐
│ [紫色 Sidebar]          │  [Main Content]│
│                         │                │
│ 👤 用户名               │                │
│ user@example.com        │    <Outlet/>   │
│                         │                │
│ ── 菜单 ──              │                │
│ 📊 用量概览             │                │
│ 🏆 排行榜               │                │
│ 📥 导出报表             │                │
│                         │                │
│ ── 管理 ──              │                │
│ ⚙️ 管理后台  →          │                │
│                         │                │
│ [退出登录]              │                │
└──────────────────────────────────────────┘
```

管理后台 Sidebar 增加「企业分析」入口（跳转到 `/enterprise`）。

### Auth Store 扩展

```typescript
// 新增字段
enterpriseUser: { name: string; avatar: string; openId: string; email: string } | null

// 新增 action
loginWithFeishu: (token: string, user: EnterpriseUser) => void

// logout 时同时清除 enterpriseUser
// persist partialize 新增 enterpriseUser
```

---

## 后端必要修改

### 问题：飞书用户无法通过 AdminAuth 访问 analytics 接口

**根因分析：**
- `core/enterprise/router.go:22-24` 将 analytics 路由注册在 `admin` 组下
- `admin` 组使用 `middleware.AdminAuth`（`core/middleware/auth.go:41`）
- `AdminAuth` 检查 `accessToken == config.AdminKey`
- 飞书 OAuth 返回的 `token_key` 是 `model.Token.Key`，不等于 `config.AdminKey`
- 结果：飞书用户调 analytics 接口会被 401 拦截

**修复方案：** 在 `core/enterprise/router.go` 中新增一个 `tokenAuth` 组，将 analytics 只读接口移入：

```go
// core/enterprise/router.go（修改后）
func RegisterRoutes(router *gin.Engine) {
    enterprise := router.Group("/api/enterprise")

    // Public routes（无需 auth）
    RegisterPublicRoutes(enterprise)

    // Token-authenticated routes（飞书用户可访问）
    tokenAuth := enterprise.Group("")
    tokenAuth.Use(middleware.TokenAuth)   // 使用 TokenAuth 而非 AdminAuth
    RegisterTokenAuthRoutes(tokenAuth)

    // Admin-authenticated routes（仅管理员）
    admin := enterprise.Group("")
    admin.Use(middleware.AdminAuth)
    RegisterAdminRoutes(admin)
}

func RegisterTokenAuthRoutes(tokenAuth *gin.RouterGroup) {
    analytics.RegisterRoutes(tokenAuth)  // analytics 只读接口移到这里
}

func RegisterAdminRoutes(admin *gin.RouterGroup) {
    feishu.RegisterRoutes(nil, admin)    // feishu 管理接口保持 admin
    quota.RegisterRoutes(admin)          // quota 管理接口保持 admin
}
```

**需确认：** `middleware.TokenAuth` 是否存在。如果没有，需要创建一个简化版中间件，仅验证 Token 有效性（从 header 读取 token → 查 cache/DB → 验证 status=enabled）。

**修改文件：**
| 文件 | 修改 |
|------|------|
| `core/enterprise/router.go` | 新增 `tokenAuth` 路由组，analytics 从 admin 移到 tokenAuth |
| `core/middleware/auth.go`（如需） | 新增 `TokenAuth` 中间件（如果不存在） |

---

## 文件变更清单

### 新建文件（7 个）

| 文件 | 说明 |
|------|------|
| `web/src/pages/enterprise/dashboard.tsx` | 企业 Dashboard 页（部门汇总卡片 + 趋势图） |
| `web/src/pages/enterprise/ranking.tsx` | 员工排行榜页（表格 + 筛选） |
| `web/src/pages/enterprise/department.tsx` | 部门趋势详情页 |
| `web/src/components/layout/EnterpriseLayout.tsx` | 企业版布局（Sidebar + 用户信息 + Outlet） |
| `web/src/components/common/SmartRedirect.tsx` | 根据用户类型智能跳转（`/enterprise` 或 `/monitor`） |
| `web/src/api/enterprise.ts` | 企业版 API 调用封装 |
| `web/src/pages/auth/feishu-callback.tsx` | 飞书 OAuth 回调中转页 |

### 修改文件 — 前端（8 个）

| 文件 | 修改说明 |
|------|----------|
| `web/src/routes/constants.ts` | 新增 `ENTERPRISE`, `ENTERPRISE_RANKING`, `ENTERPRISE_DEPARTMENT`, `FEISHU_CALLBACK` |
| `web/src/routes/config.tsx` | 新增企业版路由组 + feishu callback 路由 + `/` 使用 SmartRedirect |
| `web/src/store/auth.ts` | 新增 `enterpriseUser` + `loginWithFeishu` + logout 清除 |
| `web/src/pages/auth/login.tsx` | 新增飞书登录按钮（图标 + 文案） |
| `web/src/components/layout/SideBar.tsx` | 新增「企业分析」菜单项 |
| `web/src/api/services.ts` | 导出 `enterpriseApi` |
| `web/public/locales/zh/translation.json` | 新增企业版中文翻译 |
| `web/public/locales/en/translation.json` | 新增企业版英文翻译 |

### 修改文件 — 后端（1-2 个）

| 文件 | 修改说明 |
|------|----------|
| `core/enterprise/router.go` | 新增 tokenAuth 组，analytics 从 admin 移入 |
| `core/middleware/auth.go`（如需） | 新增 `TokenAuth` 中间件（如果现有 middleware 中不存在） |

### 配置变更

| 文件 | 说明 |
|------|------|
| `core/.env.local` | 将 `FEISHU_REDIRECT_URI` 改为 `http://localhost:3001/feishu/callback` |

---

## 关键实现细节

### 1. 飞书 OAuth 回调中转页

**路径：** `web/src/pages/auth/feishu-callback.tsx`

**流程：**
1. 组件挂载时从 `window.location.search` 提取 `code` 参数
2. 显示 Loading 动画（"正在登录..."）
3. 调用 `GET /api/enterprise/auth/feishu/callback?code={code}`（**显式清空 Authorization header**，避免旧 session 的 token 干扰）
4. 成功：调 `authStore.loginWithFeishu(token_key, user)` → `navigate('/enterprise')`
5. 失败：显示错误信息 + "重试" 按钮（跳回 `/login`）

**注意：** 飞书 redirect_uri 配置需要在飞书开放平台「安全设置」中更新为 `http://localhost:3001/feishu/callback`。

### 2. Enterprise API 封装

**路径：** `web/src/api/enterprise.ts`

复用现有 `get` 函数（`web/src/api/index.ts`），注意 baseURL 已是 `/api`：

```typescript
import { get } from './index'
import apiClient from './index'  // 用于 blob 下载

export const enterpriseApi = {
  // 飞书 OAuth callback（显式清空 Authorization，避免旧 token 干扰）
  feishuCallback: (code: string) =>
    get<{ token_key: string; user: { open_id: string; name: string; email: string; avatar: string } }>(
      `/enterprise/auth/feishu/callback?code=${encodeURIComponent(code)}`,
      { headers: { Authorization: '' } }
    ),

  // 部门汇总
  getDepartmentSummary: (params?: { start_timestamp?: number; end_timestamp?: number }) =>
    get<{ departments: DepartmentSummary[]; total: number }>('/enterprise/analytics/department', { params }),

  // 部门趋势
  getDepartmentTrend: (id: string, params?: { start_timestamp?: number; end_timestamp?: number }) =>
    get<{ department_id: string; trend: TrendPoint[] }>(`/enterprise/analytics/department/${id}/trend`, { params }),

  // 用户排行
  getUserRanking: (params?: { department_id?: string; limit?: number; start_timestamp?: number; end_timestamp?: number }) =>
    get<{ ranking: UserRankingEntry[]; total: number }>('/enterprise/analytics/user/ranking', { params }),

  // Excel 导出（使用 apiClient 走统一拦截器，responseType: 'blob'）
  exportReport: (params?: { start_timestamp?: number; end_timestamp?: number }) =>
    apiClient.get('/enterprise/analytics/export', { params, responseType: 'blob' })
      .then(response => response.data as Blob),
}
```

**注意：** `exportReport` 使用 `apiClient`（而非裸 `axios`），这样会走请求拦截器附加 Authorization header，且 baseURL 为 `/api`，最终请求路径为 `/api/enterprise/analytics/export`。

### 3. Enterprise Dashboard

**路径：** `web/src/pages/enterprise/dashboard.tsx`

**布局：**
- 顶部：标题 + 时间范围选择器（DateRangePicker，默认最近7天）
- 指标卡片行（4 列）：总请求数 / 总用量（金额） / 总 Token 数 / 活跃部门数
- 下方左侧：部门汇总表格（TanStack Table 或简单 table，支持排序，点击行跳转到趋势页）
- 下方右侧：部门用量分布（简单 bar chart 或 recharts）

**数据获取：** 使用 TanStack Query：
```typescript
const { data, isLoading } = useQuery({
  queryKey: ['enterprise', 'department-summary', startTime, endTime],
  queryFn: () => enterpriseApi.getDepartmentSummary({ start_timestamp: startTime, end_timestamp: endTime })
})
```

### 4. 排行榜页

**路径：** `web/src/pages/enterprise/ranking.tsx`

- 表格：排名 / 用户名 / 部门 / 用量金额 / 请求数 / Token 数
- 筛选：部门下拉选择 + 条目数量
- 导出按钮
- 使用 TanStack Query 请求数据

### 5. EnterpriseLayout

**路径：** `web/src/components/layout/EnterpriseLayout.tsx`

复用 `SideBar.tsx` 的视觉设计（紫色渐变、粒子效果、响应式折叠），但：
- 顶部区域显示用户信息（从 `authStore.enterpriseUser` 读取，若为 null 则不显示头像区域）
- 菜单项不同：Dashboard / 排行榜 / 导出
- 增加「管理后台」链接（跳到 `/monitor`，使用 `<Link>` 内部导航）
- 底部退出按钮

### 6. SmartRedirect 组件

**路径：** `web/src/components/common/SmartRedirect.tsx`

```typescript
import { Navigate } from 'react-router'
import useAuthStore from '@/store/auth'

export function SmartRedirect() {
  const enterpriseUser = useAuthStore((s) => s.enterpriseUser)
  return <Navigate to={enterpriseUser ? '/enterprise' : '/monitor'} replace />
}
```

用于替代 `routes/config.tsx` 中 `/` 路径原本硬编码的 `<Navigate to="/monitor" />`。

### 7. 管理后台 Sidebar 添加企业入口

在 `web/src/components/layout/SideBar.tsx` 的 `createSidebarConfig` 数组中添加：
```typescript
{
  title: t("sidebar.enterprise"),
  icon: Building2,  // from lucide-react
  href: ROUTES.ENTERPRISE,
  display: true,
}
```

在 `SidebarDisplayConfig` 接口中新增 `enterprise?: boolean`。

### 8. 国际化翻译

在 `web/public/locales/zh/translation.json` 中新增：
```json
{
  "sidebar": {
    "enterprise": "企业分析"
  },
  "enterprise": {
    "dashboard": {
      "title": "企业用量概览",
      "totalRequests": "总请求数",
      "totalUsage": "总用量",
      "totalTokens": "总Token数",
      "activeDepartments": "活跃部门"
    },
    "ranking": {
      "title": "员工用量排行",
      "rank": "排名",
      "userName": "用户",
      "department": "部门",
      "usedAmount": "用量",
      "requestCount": "请求数"
    },
    "department": {
      "title": "部门趋势"
    },
    "export": "导出报表",
    "adminBackend": "管理后台"
  },
  "auth": {
    "login": {
      "feishu": "飞书登录",
      "feishuDesc": "使用飞书账号登录"
    }
  },
  "feishuCallback": {
    "loading": "正在登录...",
    "error": "登录失败",
    "retry": "重试"
  }
}
```

---

## 后端配置调整

1. **`core/.env.local`** 中 `FEISHU_REDIRECT_URI` 改为 `http://localhost:3001/feishu/callback`
2. **飞书开放平台**「安全设置 → 重定向URL」中更新为 `http://localhost:3001/feishu/callback`
3. **后端路由调整**（详见"后端必要修改"章节）— analytics 接口从 AdminAuth 移到 TokenAuth

---

## 执行策略

### 第0步：后端路由鉴权修复（前置，必须先完成）
1. 确认 `middleware.TokenAuth` 是否存在
2. 修改 `core/enterprise/router.go` — 新增 tokenAuth 组
3. 编译验证 `go build -tags enterprise`

### 第1步：前端基础设施（串行）
1. 修改 `store/auth.ts` — 新增 `enterpriseUser` 相关字段
2. 新建 `api/enterprise.ts` — API 封装
3. 新建 `SmartRedirect.tsx`
4. 修改 `routes/constants.ts` — 新增路由常量
5. 修改 `api/services.ts` — 导出
6. 更新 locale 翻译文件

### 第2步：页面实现（可 2 个 agent 并行）
- **Agent A**: `feishu-callback.tsx` + `login.tsx` 修改 + `EnterpriseLayout.tsx`
- **Agent B**: `dashboard.tsx` + `ranking.tsx` + `department.tsx`

### 第3步：路由整合 + 导航互通（串行）
1. 修改 `routes/config.tsx` — 添加企业版路由组 + SmartRedirect
2. 修改 `SideBar.tsx` — 添加企业分析入口
3. 更新 `.env.local` 的 `FEISHU_REDIRECT_URI`

### 第4步：构建验证
```bash
cd web && pnpm build
cp -r dist/ ../core/public/dist/
cd ../core && PATH="/usr/local/go/bin:$PATH" GOPROXY=https://goproxy.cn,direct go build -tags enterprise -o aiproxy
# 启动并测试
```

---

## Review 修复记录

以下是 Review 中发现并已修复的问题：

### ✅ 已修复：鉴权矛盾（严重）
**问题：** 飞书用户的 `token_key` 是普通 Token，无法通过 `AdminAuth` 访问 analytics 接口。
**修复：** 新增"后端必要修改"章节，将 analytics 接口从 AdminAuth 组移到 TokenAuth 组。

### ✅ 已修复：feishuCallback 请求可能携带旧 Authorization header
**问题：** Axios 拦截器会自动附加 `authStore.token`，若用户之前登录过未 logout，旧 token 会被附加到 feishu callback 请求上。
**修复：** `feishuCallback` 调用时显式传 `{ headers: { Authorization: '' } }` 清空 header。

### ✅ 已确认无问题：SPA 路由 fallback
**验证结果：** `core/router/static.go` 的 `NoRoute` handler 中，`checkNoRouteNotFound` 仅排除 `/api`、`/mcp`、`/v1` 前缀。`/feishu/callback` 不匹配任何排除规则，会正确返回 `index.html`，前端 router 可以接管。

### ✅ 已修复：导出 API URL 路径不一致
**问题：** `exportReport` 使用裸 `axios.get('/api/enterprise/...')`（绕过 apiClient），与其他 API 的 baseURL 机制不一致。
**修复：** 改为使用 `apiClient.get('/enterprise/...')`，走统一拦截器和 baseURL。

### ✅ 已修复：默认跳转逻辑未区分用户类型
**问题：** `/` 路由硬编码 `<Navigate to="/monitor">`，飞书用户刷新或访问 `/` 会被导向管理后台。
**修复：** 新增 `SmartRedirect` 组件，根据 `enterpriseUser` 是否存在决定跳转到 `/enterprise` 或 `/monitor`。

---

## 验证清单

- [ ] 后端 `go build -tags enterprise` 编译通过
- [ ] 飞书用户的 token 可以访问 `/api/enterprise/analytics/*` 接口
- [ ] 访问 `/login` → 能看到飞书登录按钮和原有 Token 登录
- [ ] 点击飞书登录 → 跳转飞书授权 → 回调 → 自动跳转 `/enterprise`
- [ ] 飞书用户访问 `/` 时跳转到 `/enterprise`（而非 `/monitor`）
- [ ] Admin Token 登录后访问 `/` 跳转到 `/monitor`
- [ ] Enterprise Dashboard 渲染正常（空数据不报错）
- [ ] 排行榜页渲染正常
- [ ] 导出按钮下载 Excel 文件
- [ ] 企业版侧边栏「管理后台」跳转到 `/monitor` 正常
- [ ] 管理后台侧边栏「企业分析」跳转到 `/enterprise` 正常
- [ ] 退出登录清除所有状态（包括 enterpriseUser），回到 `/login`
- [ ] 中英文切换正常
