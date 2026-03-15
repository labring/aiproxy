# 企业版架构

## 后端模块结构 (`core/enterprise/`)

```
core/enterprise/
├── init.go              # 企业版初始化入口，注册所有子模块
├── router.go            # 企业版路由注册 /api/enterprise/*
├── auth.go              # 企业版认证中间件（从飞书 token 验证）
├── models/
│   ├── feishu.go        # FeishuUser 模型（open_id, tenant_id, group_id, department_id...）
│   ├── quota.go         # QuotaTier 模型（渐进式额度策略）
│   └── migrate.go       # 企业版表自动迁移
├── feishu/
│   ├── client.go        # 飞书 API 客户端 + 租户白名单校验
│   ├── oauth.go         # 飞书 OAuth 登录/回调（含租户校验）
│   ├── sync.go          # 飞书组织架构同步（定时任务）
│   └── register.go      # 飞书模块路由注册
├── analytics/
│   ├── register.go      # 分析模块路由注册（9 个路由）
│   ├── handler.go       # HTTP handlers
│   ├── department.go    # 部门汇总查询（GetDepartmentSummary, GetDepartmentTrend）
│   ├── ranking.go       # 排行榜查询（GetUserRanking, GetDepartmentRanking）
│   ├── comparison.go    # 环比对比（GetPeriodComparison: daily/weekly/monthly）
│   ├── model_distribution.go  # 模型用量分布（GetModelDistribution）
│   ├── custom_report.go # 自定义报表（多维度聚合查询 + 字段目录）
│   └── export.go        # Excel 导出（4 Sheet: 汇总/部门/用户/模型）
├── quota/
│   ├── handler.go       # 额度策略 CRUD handlers
│   ├── hook.go          # 请求前额度检查 hook
│   ├── cache.go         # 额度策略缓存
│   └── register.go      # 额度模块路由注册
└── notify/
    ├── feishu_p2p.go    # 飞书私聊通知
    ├── quota_alert.go   # 额度告警通知
    └── register.go      # 通知模块注册
```

## API 路由

```
# 认证
POST /api/enterprise/auth/feishu/login         # 飞书 OAuth 跳转
GET  /api/enterprise/auth/feishu/callback      # 飞书 OAuth 回调 → {token_key, user}

# 分析报表（9 个接口）
GET  /api/enterprise/analytics/department           # 部门汇总
GET  /api/enterprise/analytics/department/:id/trend # 部门趋势（按小时）
GET  /api/enterprise/analytics/department/ranking   # 部门排行
GET  /api/enterprise/analytics/user/ranking         # 用户排行
GET  /api/enterprise/analytics/model/distribution   # 模型用量分布
GET  /api/enterprise/analytics/comparison           # 环比对比
POST /api/enterprise/analytics/custom-report        # 自定义报表（多维度聚合）
GET  /api/enterprise/analytics/custom-report/fields # 自定义报表字段目录
GET  /api/enterprise/analytics/export               # Excel 报表导出

# 额度策略
GET    /api/enterprise/quota/tiers             # 获取所有策略
POST   /api/enterprise/quota/tiers             # 创建策略
PUT    /api/enterprise/quota/tiers/:id         # 更新策略
DELETE /api/enterprise/quota/tiers/:id         # 删除策略
POST   /api/enterprise/quota/assign            # 分配策略给用户/部门
```

## 前端结构

```
web/src/
├── store/auth.ts                    # Zustand auth store（含 enterpriseUser 字段）
├── api/enterprise.ts                # 企业版 API 封装（所有接口类型 + 调用方法）
├── lib/enterprise.ts                # 工具函数（TimeRange, getTimeRange, formatNumber, formatAmount）
├── routes/constants.ts              # 路由常量
│   ├── ENTERPRISE: "/enterprise"
│   ├── ENTERPRISE_RANKING: "/enterprise/ranking"
│   ├── ENTERPRISE_DEPARTMENT: "/enterprise/department"
│   ├── ENTERPRISE_QUOTA: "/enterprise/quota"
│   └── ENTERPRISE_CUSTOM_REPORT: "/enterprise/custom-report"
├── routes/config.tsx                # 路由配置（EnterpriseLayout 嵌套路由）
├── components/layout/
│   ├── EnterpriseLayout.tsx         # 企业版布局（紫色 Sidebar + Outlet）
│   └── SideBar.tsx                  # 管理后台侧边栏（含"企业分析"入口）
├── pages/enterprise/
│   ├── dashboard.tsx                # 企业概览（指标卡片 + 部门表 + 饼图 + 模型分布）
│   ├── ranking.tsx                  # 员工排行榜（9 列表格 + 筛选 + 排序 + 列选择）
│   ├── department.tsx               # 部门趋势详情（ECharts 折线/柱状图）
│   ├── quota.tsx                    # 额度策略管理（CRUD + 分配）
│   └── custom-report.tsx            # 自定义报表（维度/度量/模板/筛选/透视表/图表/CSV导出）
└── pages/auth/
    ├── login.tsx                    # 登录页（含飞书登录按钮）
    └── feishu-callback.tsx          # 飞书 OAuth 中转页（含错误处理）
```

## 自定义报表架构

### 后端：custom_report.go
- **字段目录 API** (`GET /fields`): 返回可选维度、度量、计算度量
- **报表生成 API** (`POST /custom-report`): 接受 dimensions + measures + filters + sort + limit
- 维度: `department`, `model`, `user_name`, `time_hour`, `time_day`, `time_week`
- 度量: `request_count`, `used_amount`, `total_tokens`, `input_tokens`, `output_tokens`
- 计算度量: `active_users`, `unique_models`, `success_rate`, `error_rate`, `avg_latency`
- 筛选: `department_ids[]`, `models[]`, `user_names[]`

### 前端：custom-report.tsx
- **视图模式**: table / chart / pivot（2 维度时可用）
- **图表类型**: bar / line / pie（时间维度自动切折线图）
- **透视表**: 第一维度→行头，第二维度→列头，支持切换展示度量
- **预设模板**: 5 个快速开始（点击→填充→自动生成）
- **筛选器**: 部门多选 Popover + 模型 TagInput + 用户名 TagInput
- **导出**: CSV 格式

### 关键技术决策
- `mutateRef` 模式: 解决 `useMutation.mutate` 引用不稳定导致 `useCallback` 失效
- `useEffect` 守卫: viewMode 从 pivot 自动降级（当维度数 ≠ 2 时）
- `TFunction` 从 `i18next` 导入（非 `react-i18next`），动态 i18n key 用 `as never`

## 飞书 OAuth 认证流程

```
┌─────────────────────────────────────────────────────────────────────┐
│                        用户点击"飞书登录"                            │
└─────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────┐
│  POST /api/enterprise/auth/feishu/login → 重定向到飞书授权页         │
└─────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────┐
│  用户在飞书授权 → 飞书回调到 /callback?code=xxx                      │
└─────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────┐
│  后端用 code 换取 user_access_token → 获取用户信息（含 TenantID）    │
└─────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────┐
│  校验 TenantID 是否在 FEISHU_ALLOWED_TENANTS 白名单中                │
│  ├─ 不在白名单 → 返回 403 / 重定向到前端错误页                       │
│  └─ 在白名单 → 继续                                                  │
└─────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────┐
│  创建/更新 FeishuUser、Group、Token → 返回 token_key                │
└─────────────────────────────────────────────────────────────────────┘
```

## 数据流

```
GroupSummary 表（已有日志数据）
  ┌─────────────────────────────────────┐
  │ group_id | token_name | model |     │
  │ hour_timestamp | request_count |    │
  │ status_2xx_count | input_tokens |   │
  │ output_tokens | total_tokens |      │
  │ used_amount | ...                   │
  └─────────────────────────────────────┘
         ↓ GORM 聚合查询
  FeishuUser 表 → group_id ↔ department_id ↔ tenant_id 映射
         ↓
  按部门/用户/模型/时间维度聚合 → API 返回 JSON → React Query 缓存 → ECharts 渲染
```

## 关键数据模型

### FeishuUser 字段
| 字段 | 类型 | 说明 |
|------|-----|------|
| OpenID | string | 用户在应用中的唯一标识 |
| UnionID | string | 用户在开发者所有应用中的标识 |
| UserID | string | 用户在企业内部的标识（仅企业用户有） |
| TenantID | string | 企业租户标识（仅企业用户有） |
| GroupID | string | 关联的 AI Proxy Group ID |
| DepartmentID | string | 飞书部门 ID |

### 关键计算字段
| 概念 | 后端字段 | 前端类型 |
|------|---------|---------|
| 活跃用户 | `COUNT(DISTINCT group_id)` | `active_users: number` |
| 成功率 | `SUM(status_2xx_count)/SUM(request_count)*100` | `success_rate: number` |
| 模型占比 | `model_amount/total_amount*100` | `percentage: number` |
| 环比变化 | `(current-previous)/previous*100` | `PeriodChanges` 各 `_pct` 字段 |
| 时间戳 | `hour_timestamp` (Unix 秒) | `hour_timestamp: number`（前端 `*1000`） |

## 排行榜功能

### 可用列（SortField）
| 字段 | 对齐 | 默认显示 | 格式化 |
|------|-----|---------|--------|
| rank | left | ✅ | 金银铜徽章 |
| user_name | left | ✅ | 粗体 |
| department_name | left | ✅ | 灰色 |
| request_count | right | ✅ | formatNumber |
| used_amount | right | ✅ | formatAmount |
| total_tokens | right | ✅ | formatNumber |
| input_tokens | right | ❌ | formatNumber |
| output_tokens | right | ❌ | formatNumber |
| unique_models | right | ✅ | formatNumber |

### 筛选器
- 时间范围：7d / 30d / 本月 / 自定义
- 部门：多选 Popover
- 数量：Top 20/50/100 / 自定义 / 全量

## 环境变量

| 变量 | 必填 | 说明 |
|------|-----|------|
| `FEISHU_APP_ID` | ✅ | 飞书应用 App ID |
| `FEISHU_APP_SECRET` | ✅ | 飞书应用 App Secret |
| `FEISHU_REDIRECT_URI` | ✅ | OAuth 回调地址 |
| `FEISHU_FRONTEND_URL` | ❌ | 前端基础 URL，默认 `http://localhost:5173` |
| `FEISHU_ALLOWED_TENANTS` | ❌ | 允许的企业租户白名单，逗号分隔，空则允许所有 |

## Build Tag

所有 `core/enterprise/` 下的 `.go` 文件首行必须有：
```go
//go:build enterprise
```
编译时：`go build -tags enterprise -o aiproxy`
