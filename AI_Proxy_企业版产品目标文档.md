# AI Proxy 企业版产品目标文档

## Context

基于开源项目 [labring/aiproxy](https://github.com/labring/aiproxy) 进行企业级定制开发。原项目是一个功能完善的 AI 网关，支持 40+ 提供商适配、多租户（Group/Token 体系）、额度管理（总额度 + 周期额度）、RPM/TPM 限速、飞书 Webhook 通知和基于小时粒度的多维 Summary 统计。

**开发动机：** 企业需要统一管理 AI API 调用，对接内部身份系统（飞书），实现精细化额度管控（渐进式降级而非硬切断），并提供部门/员工维度的用量分析能力。

**核心约束：** 自定义代码必须与上游保持最大程度分离，便于持续合并上游更新。优先复用上游已有基础设施（Quota 体系、Notify 接口、Summary 聚合、Usage Alert），避免平行新建。

---

## 1. 架构设计原则

### 1.1 代码分离策略（Fork + 分支 + 顶层目录隔离）

```
aiproxy/                          # Fork 自 labring/aiproxy
├── core/                         # 上游核心代码，尽量不修改
│   ├── middleware/               # 认证、分发、限速（现有）
│   ├── model/                    # 数据模型和缓存（现有）
│   ├── common/notify/            # 通知接口和飞书 Webhook（现有）
│   ├── common/consume/           # 消费记录和汇总（现有）
│   └── ...
├── enterprise/                   # 企业版后端扩展（与 core/ 平级，新建）
│   ├── feishu/                   # 飞书身份集成
│   ├── quota/                    # 渐进式额度策略
│   ├── analytics/                # 增强分析（部门聚合、排行榜、导出）
│   ├── notify/                   # 点对点飞书消息推送
│   └── init.go                   # build tag 注册，统一注入所有扩展
├── web/src/
│   ├── ...                       # 上游前端代码
│   └── enterprise/               # 企业版前端扩展
└── scripts/
    └── migrate_enterprise.sql    # 企业版数据库迁移
```

**分支策略：**
- `main` 分支：跟踪上游 `labring/aiproxy` 的 main，仅做同步，不提交自有代码
- `enterprise` 分支：企业定制主开发分支，所有自有代码在此

**同步策略：**
1. 定期 `git fetch upstream && git merge upstream/main` 到 enterprise 分支（使用 merge 而非 rebase，避免多人协作时的 force-push 问题）
2. 企业代码集中在顶层 `enterprise/` 目录和 `web/src/enterprise/`，与上游目录结构无交叉，合并冲突概率极低
3. 对 `core/` 的修改仅限于**最小化扩展点注入**（如在 `core/router/api.go` 中注册企业路由），通过 Go build tags 控制

### 1.2 扩展点设计（复用上游模式）

上游已有的扩展模式是通过全局变量 + 注册函数（如 `notify.SetDefaultNotifier()`），企业版沿用此模式：

| 扩展点 | 上游现有机制 | 企业版注入方式 |
|--------|-------------|---------------|
| 通知系统 | `notify.Notifier` 接口 + `SetDefaultNotifier()` | 实现支持点对点推送的 `EnterpriseFeishuNotifier`，替换默认 Notifier |
| 额度检查 | `Token.GetEffectiveQuotaStatus()` + `distributor.go` 中的 `checkGroupBalance()` | 在 `distribute()` 流程中注入阶梯检查钩子（通过 build tag 编译） |
| RPM/TPM 调整 | `GetGroupAdjustedModelConfig()` 动态调整 RPM/TPM ratio | 在额度阶梯触发时动态修改 `GroupCache.RPMRatio` / `TPMRatio` |
| 用量告警 | `GetGroupUsageAlert()` 动态阈值检测 | 直接复用，扩展通知渠道为飞书点对点推送 |
| 统计聚合 | `GroupSummary`（group_id + token_name + model + hour 维度） | 通过查询层聚合（飞书部门→Group 映射），无需新建汇总表 |

### 1.3 Build Tag 机制

```go
// enterprise/init.go
//go:build enterprise

package enterprise

import (
    "github.com/labring/aiproxy/enterprise/feishu"
    "github.com/labring/aiproxy/enterprise/quota"
    "github.com/labring/aiproxy/enterprise/notify"
)

func init() {
    // 注册飞书认证路由
    feishu.Register()
    // 注册渐进式额度检查钩子
    quota.Register()
    // 替换通知器为支持点对点推送的版本
    notify.Register()
}
```

编译命令：
```bash
# 开源版（与上游一致）
go build ./core/...

# 企业版
go build -tags enterprise ./core/... ./enterprise/...
```

---

## 2. 功能模块详细设计

### 2.1 飞书身份管理（实时同步 + 权限继承）

#### 与上游认证模型的对接关系

上游认证体系：`Group`（租户/组织）→ `Token`（API 密钥，属于某个 Group）→ `TokenAuth` 中间件校验。

企业版映射关系：
```
飞书部门  →  AI Proxy Group    （1:1 自动创建）
飞书员工  →  AI Proxy Token    （1:1 自动创建，归属主部门对应的 Group）
```

员工通过飞书 OAuth 登录后，系统自动为其在对应部门 Group 下创建专属 Token。该 Token 的 `Name` 绑定飞书 OpenID，后续 API 调用使用此 Token，复用上游全部鉴权、限速和计费逻辑。

#### 数据模型扩展

```go
// enterprise/feishu/models.go

// FeishuUser 员工信息（与 Token 1:1 关联）
type FeishuUser struct {
    OpenID          string    `gorm:"primaryKey;size:64"`
    UnionID         string    `gorm:"size:64;index"`
    Name            string    `gorm:"size:128"`
    Email           string    `gorm:"size:256"`
    DepartmentID    string    `gorm:"size:64;index"`          // 主部门 ID
    DepartmentIDs   []string  `gorm:"serializer:json;type:text"` // 所有部门（飞书支持多部门）
    TokenID         int       `gorm:"index"`                  // 关联的 AI Proxy Token ID
    GroupID         string    `gorm:"size:64;index"`          // 关联的 AI Proxy Group ID（主部门对应）
    Status          int       `gorm:"default:1"`              // 1=在职, 2=离职
    IsAdmin         bool      `gorm:"default:false"`          // 是否为管理员
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// FeishuDepartment 部门信息（与 Group 1:1 关联）
type FeishuDepartment struct {
    DepartmentID    string  `gorm:"primaryKey;size:64"`
    ParentID        string  `gorm:"size:64;index"`
    Name            string  `gorm:"size:256"`
    GroupID         string  `gorm:"size:64;uniqueIndex"`  // 关联的 AI Proxy Group ID
    Level           int     `gorm:"default:0"`            // 部门层级深度
    Path            string  `gorm:"size:1024"`            // 部门路径 "root/dept1/dept2"
    ManagerOpenID   string  `gorm:"size:64"`              // 部门负责人
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

#### 实现要点

1. **OAuth 登录流程**
   - 新增路由 `/api/auth/feishu/login`（重定向到飞书授权页） 和 `/api/auth/feishu/callback`（处理回调）
   - 回调成功后：查询或创建 `FeishuUser` → 查询或创建对应部门的 `Group` → 查询或创建该用户在 Group 下的 `Token`
   - 返回 Token key 供前端后续 API 调用使用
   - 复用上游 `model.InsertToken()` 创建 Token（设置 `autoCreateGroup=true`）

2. **事件订阅同步**
   - 使用 `larksuite/oapi-sdk-go/v3`（项目已有依赖，见 go.mod:25）处理飞书事件
   - 监听用户事件：`user.created` → 创建 FeishuUser + Token；`user.updated` → 更新部门归属；`user.deleted` → 禁用 Token（调用 `model.UpdateTokenStatus(id, TokenStatusDisabled)`）
   - 监听部门事件：`department.created` → 创建 Group；`department.updated` → 更新映射；`department.deleted` → 禁用 Group
   - Webhook 端点：`/api/enterprise/feishu/webhook`

3. **全量同步（兜底）**
   - 定时任务（如每 6 小时）全量拉取飞书组织架构，与本地数据对账
   - 处理事件丢失、网络中断等异常场景
   - 使用飞书 API 分页接口批量获取，注意 API 限流（通过 go-cache 缓存 tenant_access_token）

#### 关键文件

- `enterprise/feishu/oauth.go` - OAuth 登录和回调
- `enterprise/feishu/sync.go` - 事件订阅处理 + 全量同步
- `enterprise/feishu/models.go` - 数据模型定义
- `enterprise/feishu/register.go` - 路由注册

---

### 2.2 渐进式额度管控（三阶梯策略）

#### 设计原则：扩展现有 Quota 体系，而非新建

上游 Token 已有完整的额度体系：
- `Token.Quota` — 总额度上限
- `Token.PeriodQuota` — 周期额度上限（对应企业版 Tier3 硬上限）
- `Token.PeriodType` — 周期类型（daily / weekly / monthly）
- `Token.GetEffectiveQuotaStatus()` — 额度校验逻辑
- `Group.RPMRatio` / `TPMRatio` — 速率调整倍率
- `GetGroupAdjustedModelConfig()` — 动态调整模型 RPM/TPM

企业版在此基础上增加 Tier1（模型降级）和 Tier2（限速）两个中间阶梯，复用 `PeriodQuota` 作为 Tier3。

#### 额度策略配置（新增表）

```go
// enterprise/quota/config.go

// QuotaPolicy 额度策略（绑定到 Group）
type QuotaPolicy struct {
    ID               int       `gorm:"primaryKey"`
    Name             string    `gorm:"size:64;uniqueIndex"`  // "试用", "标准", "高级"
    Description      string    `gorm:"size:256"`

    // 阶梯阈值（占 PeriodQuota 的比例，0.0-1.0）
    Tier1Ratio       float64   // 模型降级触发比例（如 0.6 = 周期额度 60%）
    Tier2Ratio       float64   // 限速触发比例（如 0.8 = 周期额度 80%）
    // Tier3 = PeriodQuota 本身（100%），复用上游逻辑

    // 模型降级映射
    DegradeMapping   map[string]string `gorm:"serializer:json;type:text"` // {"gpt-4o": "gpt-4o-mini"}

    // 限速配置（倍率，叠加到 Group.RPMRatio 上）
    DegradeRPMRatio  float64   `gorm:"default:0.3"` // 限速后 RPM 变为原来的 30%
    DegradeTPMRatio  float64   `gorm:"default:0.3"` // 限速后 TPM 变为原来的 30%

    CreatedAt        time.Time
    UpdatedAt        time.Time
}

// GroupQuotaPolicy 部门/Group 的策略绑定
type GroupQuotaPolicy struct {
    GroupID        string `gorm:"primaryKey;size:64"`
    QuotaPolicyID  int    `gorm:"index"`
    InheritParent  bool   `gorm:"default:true"` // 是否继承父部门策略
}
```

#### 三阶梯逻辑

```
当前周期用量 = Token.UsedAmount - Token.PeriodLastUpdateAmount
周期额度 = Token.PeriodQuota

Tier0: 用量 < PeriodQuota × Tier1Ratio   → 正常使用所有模型
Tier1: 用量 >= PeriodQuota × Tier1Ratio   → 模型自动降级（如 GPT-4o → GPT-4o-mini）
Tier2: 用量 >= PeriodQuota × Tier2Ratio   → 降级 + 限速（RPM/TPM 降至 30%）
Tier3: 用量 >= PeriodQuota                 → 完全阻止（上游已实现）
```

#### 注入方式

通过 build tag 在 `distribute()` 流程中注入阶梯检查，不修改上游 `distributor.go` 源码：

```go
// enterprise/quota/hook.go

// CheckQuotaTier 在 distribute() 的模型选择阶段调用
// 返回: 实际模型名、RPM/TPM 调整倍率、是否阻止
func CheckQuotaTier(
    group model.GroupCache,
    token model.TokenCache,
    requestModel string,
) (effectiveModel string, rpmRatio, tpmRatio float64, blocked bool) {
    policy := getCachedQuotaPolicy(group.ID) // Redis 缓存
    if policy == nil {
        return requestModel, 1.0, 1.0, false // 无策略，正常通过
    }

    periodUsage := token.UsedAmount - token.PeriodLastUpdateAmount
    periodQuota := token.PeriodQuota
    if periodQuota <= 0 {
        return requestModel, 1.0, 1.0, false
    }

    usageRatio := periodUsage / periodQuota

    switch {
    case usageRatio >= 1.0:
        return "", 0, 0, true // Tier3: 上游已处理，这里做兜底
    case usageRatio >= policy.Tier2Ratio:
        // Tier2: 降级 + 限速
        model := degradeModel(policy.DegradeMapping, requestModel)
        return model, policy.DegradeRPMRatio, policy.DegradeTPMRatio, false
    case usageRatio >= policy.Tier1Ratio:
        // Tier1: 仅降级
        model := degradeModel(policy.DegradeMapping, requestModel)
        return model, 1.0, 1.0, false
    default:
        return requestModel, 1.0, 1.0, false
    }
}
```

#### 降级透明度设计

模型降级时，API 响应中通过自定义 Header 告知调用方：
```
X-Quota-Tier: 1
X-Quota-Original-Model: gpt-4o
X-Quota-Effective-Model: gpt-4o-mini
X-Quota-Usage-Ratio: 0.65
```

调用方（如内部 AI 应用）可据此在 UI 中展示提示信息。

#### 关键文件

- `enterprise/quota/config.go` - 策略模型定义
- `enterprise/quota/hook.go` - 阶梯检查逻辑（注入 distribute 流程）
- `enterprise/quota/cache.go` - 策略 Redis 缓存
- `enterprise/quota/register.go` - 钩子注册

---

### 2.3 用量分析增强

#### 设计原则：查询层聚合，不新建汇总表

上游 `GroupSummary` 已按 `(group_id, token_name, model, hour_timestamp)` 维度存储完整统计数据（30+ 字段：request_count、tokens、amount、latency、cache 命中等）。

企业版通过 `FeishuDepartment.GroupID` 映射，在 API 查询层将同一部门下所有 Group 的 Summary 聚合，无需冗余存储。

#### 分析 API 设计

| API 端点 | 数据来源 | 说明 |
|----------|---------|------|
| `GET /api/enterprise/analytics/department` | `GroupSummary` JOIN `feishu_departments`（按 group_id） | 部门维度汇总 |
| `GET /api/enterprise/analytics/department/:id/trend` | `GroupSummary` WHERE group_id IN (部门所有 Group) | 部门用量趋势 |
| `GET /api/enterprise/analytics/user/ranking` | `GroupSummary` GROUP BY token_name JOIN `feishu_users` | 员工排行榜 |
| `GET /api/enterprise/analytics/model` | `GroupSummary` GROUP BY model | 复用上游已有，增加部门筛选参数 |
| `GET /api/enterprise/analytics/trend` | `GroupSummary` GROUP BY hour | 复用上游已有，增加部门/员工筛选 |
| `GET /api/enterprise/analytics/export` | 组合以上查询，生成 Excel/CSV | 报表导出 |

#### 排行榜查询示例

```go
// enterprise/analytics/ranking.go

func GetUserRanking(departmentID string, startTime, endTime int64, limit int) ([]UserRankItem, error) {
    // 获取该部门下所有 Group ID
    groupIDs := getGroupIDsByDepartment(departmentID)

    var results []UserRankItem
    err := model.LogDB.
        Model(&model.GroupSummary{}).
        Select("token_name, SUM(used_amount) as total_amount, SUM(request_count) as total_requests, SUM(total_tokens) as total_tokens").
        Where("group_id IN ? AND hour_timestamp BETWEEN ? AND ?", groupIDs, startTime, endTime).
        Group("token_name").
        Order("total_amount DESC").
        Limit(limit).
        Find(&results).Error

    // 关联飞书用户名（token_name = open_id）
    enrichWithFeishuUserInfo(&results)
    return results, err
}
```

#### 报表导出

```go
// enterprise/analytics/export.go

type ExportRequest struct {
    StartTime    int64    `json:"start_time"`
    EndTime      int64    `json:"end_time"`
    Dimensions   []string `json:"dimensions"` // ["department", "user", "model"]
    Format       string   `json:"format"`     // "xlsx", "csv"
    GroupBy      string   `json:"group_by"`   // "hour", "day", "week", "month"
    DepartmentID string   `json:"department_id,omitempty"` // 可选：筛选特定部门
}

// ExportToExcel 使用 excelize/v2 生成 Excel
func ExportToExcel(req ExportRequest) (*excelize.File, error)
```

#### 关键文件

- `enterprise/analytics/department.go` - 部门维度聚合查询
- `enterprise/analytics/ranking.go` - 排行榜查询
- `enterprise/analytics/export.go` - 报表导出
- `enterprise/analytics/handler.go` - HTTP handler

---

### 2.4 消息通知（飞书点对点推送）

#### 设计原则：扩展现有 Notify 接口

上游已有：
- `notify.Notifier` 接口（`Notify` + `NotifyThrottle` 方法）
- `FeishuNotifier`（通过 Webhook 发群消息）
- `GetGroupUsageAlert()` 异常用量检测（基于前三天均值的动态阈值）

企业版扩展：将 Webhook 群通知升级为**飞书消息 API 点对点推送**（给具体用户发消息），并增加额度阶梯触发通知。

#### 通知场景

| 场景 | 触发条件 | 通知对象 | 实现方式 |
|------|----------|----------|----------|
| 额度预警 | 周期用量达到 Tier1Ratio | 用户本人 | 飞书点对点消息 |
| Tier1 触发 | 模型降级生效 | 用户本人 | 飞书点对点消息 |
| Tier2 触发 | 限速生效 | 用户本人 + 部门负责人 | 飞书点对点消息 |
| Tier3 触发 | 完全阻止 | 用户本人 + 管理员 | 飞书点对点消息 + Webhook 群通知 |
| 异常用量 | `GetGroupUsageAlert()` 检测到异常 | 管理员 | 复用上游告警 + 飞书点对点推送 |

#### 实现

```go
// enterprise/notify/feishu_p2p.go

// EnterpriseFeishuNotifier 实现 notify.Notifier 接口
// 在上游 FeishuNotifier（Webhook）基础上增加点对点推送能力
type EnterpriseFeishuNotifier struct {
    webhookURL  string              // 群 Webhook（复用上游）
    larkClient  *lark.Client        // 飞书 API 客户端（新增）
}

func (n *EnterpriseFeishuNotifier) Notify(level notify.Level, title, message string) {
    // 1. 群 Webhook 通知（复用上游 PostToFeiShuv2）
    go notify.PostToFeiShuv2(context.Background(), level2Color(level), title, message, n.webhookURL)
}

// NotifyUser 向指定飞书用户发送消息（企业版新增）
func (n *EnterpriseFeishuNotifier) NotifyUser(openID string, card *larkcard.MessageCard) error {
    // 使用 larksuite/oapi-sdk-go 发送点对点消息
}

// NotifyQuotaTierChange 额度阶梯变更通知
func (n *EnterpriseFeishuNotifier) NotifyQuotaTierChange(
    user FeishuUser,
    tier int,
    usageRatio float64,
    originalModel, effectiveModel string,
) error {
    // 构造卡片消息，包含用量信息和降级说明
}
```

#### 关键文件

- `enterprise/notify/feishu_p2p.go` - 飞书点对点推送
- `enterprise/notify/quota_alert.go` - 额度阶梯通知逻辑
- `enterprise/notify/register.go` - 注册为默认 Notifier

---

## 3. 前端扩展

### 3.1 新增页面

| 页面 | 路径 | 功能 |
|------|------|------|
| 飞书登录 | `/login` | 飞书 OAuth 扫码/网页登录 |
| 部门管理 | `/admin/departments` | 部门列表、部门-Group 映射查看 |
| 额度策略配置 | `/admin/quota-policies` | 创建/编辑额度策略，绑定到部门 |
| 用量分析仪表盘 | `/analytics` | 多维度分析（部门/员工/模型/时间） |
| 排行榜 | `/analytics/ranking` | 员工/部门用量排行 |
| 报表导出 | `/analytics/export` | 选择维度和时间范围，生成下载 |

### 3.2 目录结构

```
web/src/enterprise/
├── components/
│   ├── FeishuLogin.tsx           # 飞书登录组件
│   ├── QuotaPolicyEditor.tsx     # 额度策略配置编辑器
│   ├── DepartmentTree.tsx        # 部门树形组件
│   └── AnalyticsChart.tsx        # 分析图表（复用 echarts）
├── pages/
│   ├── Login.tsx
│   ├── DepartmentManage.tsx
│   ├── QuotaPolicyManage.tsx
│   ├── Analytics.tsx
│   ├── Ranking.tsx
│   └── Export.tsx
├── api/
│   └── enterprise.ts             # 企业版 API 调用封装
└── store/
    └── feishuUser.ts             # 飞书用户状态管理（Zustand）
```

---

## 4. 数据库迁移

### 新增表（仅 3 张，复用上游 groups/tokens/group_summaries）

```sql
-- 飞书用户表
CREATE TABLE feishu_users (
    open_id VARCHAR(64) PRIMARY KEY,
    union_id VARCHAR(64),
    name VARCHAR(128),
    email VARCHAR(256),
    department_id VARCHAR(64),
    department_ids JSONB DEFAULT '[]',         -- 多部门支持
    token_id INT REFERENCES tokens(id),        -- 关联 Token
    group_id VARCHAR(64) REFERENCES groups(id),-- 关联主部门 Group
    status INT DEFAULT 1,
    is_admin BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_feishu_users_dept ON feishu_users(department_id);
CREATE INDEX idx_feishu_users_token ON feishu_users(token_id);
CREATE INDEX idx_feishu_users_group ON feishu_users(group_id);

-- 飞书部门表
CREATE TABLE feishu_departments (
    department_id VARCHAR(64) PRIMARY KEY,
    parent_id VARCHAR(64),
    name VARCHAR(256),
    group_id VARCHAR(64) UNIQUE REFERENCES groups(id), -- 关联 Group
    level INT DEFAULT 0,
    path VARCHAR(1024),
    manager_open_id VARCHAR(64),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_feishu_depts_parent ON feishu_departments(parent_id);
CREATE INDEX idx_feishu_depts_group ON feishu_departments(group_id);

-- 额度策略配置表
CREATE TABLE quota_policies (
    id SERIAL PRIMARY KEY,
    name VARCHAR(64) UNIQUE NOT NULL,
    description VARCHAR(256),
    tier1_ratio DECIMAL(5,4) DEFAULT 0.6,     -- 模型降级阈值比例
    tier2_ratio DECIMAL(5,4) DEFAULT 0.8,     -- 限速阈值比例
    degrade_mapping JSONB DEFAULT '{}',
    degrade_rpm_ratio DECIMAL(5,4) DEFAULT 0.3,
    degrade_tpm_ratio DECIMAL(5,4) DEFAULT 0.3,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Group 额度策略绑定表
CREATE TABLE group_quota_policies (
    group_id VARCHAR(64) PRIMARY KEY REFERENCES groups(id),
    quota_policy_id INT REFERENCES quota_policies(id),
    inherit_parent BOOLEAN DEFAULT TRUE
);
```

**注意：** 不新建 `department_summaries` 汇总表。部门维度的统计通过 `group_summaries` + `feishu_departments` 的 JOIN 查询实现。

---

## 5. 配置项

### 环境变量

```bash
# 飞书配置
FEISHU_APP_ID=cli_xxx               # 飞书自建应用 App ID
FEISHU_APP_SECRET=xxx               # 飞书自建应用 App Secret
FEISHU_ENCRYPT_KEY=xxx              # 事件订阅加密密钥
FEISHU_VERIFICATION_TOKEN=xxx       # 事件订阅验证 token

# 企业版功能开关（build tag 已做编译隔离，此处为运行时精细控制）
ENTERPRISE_QUOTA_CHECK_ENABLED=true  # 是否启用渐进式额度检查
ENTERPRISE_DEFAULT_POLICY_ID=1       # 新部门默认额度策略 ID
ENTERPRISE_FEISHU_SYNC_INTERVAL=6h   # 全量同步间隔

# 通知配置（复用上游 NOTIFY_FEISHU_WEBHOOK，额外增加以下）
ENTERPRISE_ADMIN_OPEN_IDS=ou_xxx,ou_yyy  # 管理员飞书 OpenID（用于接收告警）
```

---

## 6. 部署架构

### 开发环境（Docker Compose）

```yaml
services:
  aiproxy:
    build:
      context: .
      args:
        - BUILD_TAGS=enterprise      # 编译时启用企业版
    ports:
      - "3000:3000"
    environment:
      - FEISHU_APP_ID=${FEISHU_APP_ID}
      - FEISHU_APP_SECRET=${FEISHU_APP_SECRET}
      # ...
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
```

### 生产环境（Kubernetes）

- Deployment: aiproxy (2+ replicas)
- Service: ClusterIP
- Ingress: 对外暴露（需配置飞书 OAuth 回调 URL）
- ConfigMap: 非敏感配置（功能开关、同步间隔等）
- Secret: 飞书 App Secret、数据库密码等
- CronJob: 飞书全量同步（或内置定时器）

### 回滚方案

企业版出问题时可快速切回开源版：
1. 重新部署不带 `enterprise` build tag 的镜像
2. 上游 Token 和 Group 数据不受影响（企业版只增加了扩展表）
3. 飞书登录不可用时，管理员可通过上游 Admin API 手动管理 Token

---

## 7. 实施计划

### Phase 1: 基础架构 + 飞书登录（1.5 周）
- [ ] 建立 enterprise 分支和 `enterprise/` 目录结构
- [ ] 实现 build tag 注入机制和 `enterprise/init.go`
- [ ] 数据库迁移脚本（3 张新表）
- [ ] 飞书 OAuth 登录流程（登录 → 创建 Group + Token → 返回 Token key）
- [ ] 前端飞书登录页面

### Phase 2: 飞书组织同步（1 周）
- [ ] 飞书事件订阅处理（用户/部门 CRUD）
- [ ] 全量同步定时任务
- [ ] 前端部门管理页面
- [ ] 离职员工自动禁用

### Phase 3: 渐进式额度管控（1.5 周）
- [ ] 额度策略 CRUD API
- [ ] 三阶梯检查逻辑注入 distribute 流程
- [ ] 模型降级和限速实现
- [ ] 降级 Header 信息传递
- [ ] 策略 Redis 缓存
- [ ] 前端策略配置页面

### Phase 4: 通知告警（1 周）
- [ ] 飞书点对点消息推送（实现 `EnterpriseFeishuNotifier`）
- [ ] 额度阶梯触发通知
- [ ] 复用上游 `GetGroupUsageAlert()` + 飞书推送

### Phase 5: 用量分析（1.5 周）
- [ ] 部门维度聚合查询 API
- [ ] 员工/部门排行榜 API
- [ ] Excel 报表导出
- [ ] 前端分析仪表盘和排行榜页面

### Phase 6: 测试和文档（1 周）
- [ ] 单元测试（额度策略、降级逻辑）
- [ ] 集成测试（飞书登录→额度触发→通知 端到端）
- [ ] 部署文档
- [ ] 用户手册

**总工期：约 7.5 周**

---

## 8. 验证方案

### 功能验证

1. **飞书登录**
   - 飞书扫码/网页登录成功
   - 自动创建 FeishuUser + Token + Group
   - 使用返回的 Token key 调用 AI API 成功
   - 离职员工 Token 自动禁用，API 调用返回 401

2. **额度管控**
   - 正常使用 → Tier1 模型降级 → Tier2 限速 → Tier3 阻止的完整流程
   - 验证降级后 API 返回的 Header 信息正确
   - 周期重置后额度恢复，模型和速率恢复正常
   - 无策略的 Group 不受影响（bypass）

3. **用量分析**
   - 部门维度汇总数据准确（与直接查询 GroupSummary 对账）
   - 排行榜排序正确
   - Excel 导出内容与页面数据一致

4. **通知告警**
   - 各阶梯触发条件正确发送飞书点对点消息
   - 节流机制生效（同一用户短时间内不重复通知）

### 性能验证

- 1000 QPS 下响应时间 < 100ms（排除上游 AI 调用时间）
- 额度阶梯检查增加延迟 < 5ms（Redis 缓存命中场景）
- 部门聚合查询响应时间 < 500ms（100 个部门、30 天范围）

---

## 9. 关键依赖

| 依赖 | 用途 | 版本 | 备注 |
|------|------|------|------|
| larksuite/oapi-sdk-go/v3 | 飞书 OAuth + 事件订阅 + 消息 API | v3.5.3 | 项目已有依赖 |
| excelize/v2 | Excel 报表生成 | v2.8+ | 需新增 |
| redis/go-redis/v9 | 策略缓存 | v9.18.0 | 项目已有依赖 |
| 其他上游依赖 | 保持一致 | - | 不引入额外依赖 |

---

## 10. 上游兼容性评估

### 10.1 整体评估：兼容风险低

企业版采用顶层 `enterprise/` 目录隔离 + build tag 编译隔离，与上游 `core/` 目录无文件级冲突。

### 10.2 冲突风险矩阵

| 上游变更类型 | 冲突概率 | 影响范围 | 应对策略 |
|-------------|---------|---------|---------|
| **新增文件/功能** | 极低 | 无冲突 | 直接 merge |
| **修改 `model/token.go` 数据结构** | 低 | 如果 Token 字段名变更，影响 `enterprise/quota/hook.go` 中的字段引用 | 跟踪 TokenCache 结构变化，及时适配 |
| **修改 `middleware/distributor.go` 流程** | 中 | 如果 `distribute()` 函数签名或流程大幅变更，影响企业版钩子注入点 | 这是唯一的高风险点，需要在 core 中保留一个最小化的钩子调用（约 5 行代码） |
| **修改 `common/notify/notify.go` 接口** | 低 | 如果 Notifier 接口签名变更，影响企业版实现 | 接口稳定，变更概率极低 |
| **修改 `model/group.go` / `model/cache.go`** | 低 | 如果 GroupCache 结构变更，影响额度策略缓存 | 跟踪 GroupCache 字段变化 |
| **前端重构** | 中 | `web/src/` 结构变化可能需要调整企业版前端引用路径 | 企业版前端独立于 `web/src/enterprise/`，仅 import 上游组件 |
| **数据库 schema 变更** | 低 | 上游可能修改 groups/tokens 表结构 | 企业版扩展表通过外键关联，上游表结构变更需检查外键兼容性 |

### 10.3 对 core/ 的最小化修改清单

企业版需要对上游 core/ 做以下最小化修改（总计约 15-20 行），这些是 merge 时唯一可能产生冲突的地方：

| 文件 | 修改内容 | 行数 | 冲突缓解 |
|------|---------|------|---------|
| `core/router/api.go` | 增加 `enterprise.RegisterRoutes(r)` 调用 | ~3 行 | 用 build tag 包裹，上游无此代码段 |
| `core/middleware/distributor.go` | 在 `distribute()` 中 `findModel` 之后增加钩子调用 | ~8 行 | 用 build tag 包裹，位置固定在模型确定后 |
| `core/model/main.go` | 在 `AutoMigrate` 中增加企业版表 | ~3 行 | 用 build tag 包裹 |

每个修改点都用 build tag 包裹，示例：
```go
//go:build enterprise

// core/middleware/distributor_enterprise.go（新文件，不修改上游文件）
package middleware

// EnterpriseQuotaHook 企业版额度检查钩子
var EnterpriseQuotaHook func(group model.GroupCache, token model.TokenCache, model string) (string, float64, float64, bool)
```

**策略优化**：如果上游的 `distributor.go` 发生冲突，可以退化为"仅在 Tier3 时由上游阻止"，企业版的 Tier1/Tier2 功能通过独立中间件实现，完全不修改 `distributor.go`。

### 10.4 同步操作规范

1. **频率**：每 1-2 周同步一次上游 main
2. **流程**：
   ```bash
   git fetch upstream
   git checkout enterprise
   git merge upstream/main
   # 解决冲突（如有），重点检查 10.3 中列出的文件
   # 运行测试
   go test -tags enterprise ./...
   ```
3. **CI 自动化**：GitHub Actions 定期检测上游更新，自动创建 merge PR，CI 跑测试
4. **破坏性变更应对**：如果上游做了大规模重构（如包路径变更），创建专门的适配 PR 处理

---

## 11. 开源许可证合规性分析

### 11.1 整体评估：商业使用安全

项目及主要依赖均采用商业友好许可证（MIT / Apache 2.0 / BSD），**可以安全用于企业内部部署和商业使用**。

### 11.2 核心组件许可证

| 组件 | 许可证 | 商业使用 | 备注 |
|------|--------|----------|------|
| **labring/aiproxy** | MIT | 完全允许 | 主项目，可 Fork、修改、内部部署、商业分发 |
| gin-gonic/gin | MIT | 允许 | Web 框架 |
| gorm.io/gorm | MIT | 允许 | ORM 框架 |
| bytedance/sonic | Apache 2.0 | 允许 | JSON 库，需保留 NOTICE |
| larksuite/oapi-sdk-go/v3 | MIT | 允许 | 飞书 SDK，已在 go.mod 中 |
| redis/go-redis/v9 | BSD-2-Clause | 允许 | Redis 客户端 |
| glebarez/sqlite | BSD-3-Clause | 允许 | SQLite 驱动 |
| sirupsen/logrus | MIT | 允许 | 日志库 |
| shopspring/decimal | MIT | 允许 | 精确计算库 |
| jackc/pgx/v5 | MIT | 允许 | PostgreSQL 驱动 |
| golang-jwt/jwt/v5 | MIT | 允许 | JWT 库 |
| aws/aws-sdk-go-v2 | Apache 2.0 | 允许 | AWS SDK |
| google.golang.org/api | BSD-3-Clause | 允许 | Google API |

### 11.3 前端依赖

| 组件 | 许可证 | 商业使用 |
|------|--------|----------|
| React | MIT | 允许 |
| echarts | Apache 2.0 | 允许 |
| Radix UI | MIT | 允许 |
| TailwindCSS | MIT | 允许 |
| Vite | MIT | 允许 |
| Zustand | MIT | 允许 |

### 11.4 新增依赖

| 组件 | 许可证 | 商业使用 | 备注 |
|------|--------|----------|------|
| excelize/v2 | BSD-3-Clause | 允许 | Excel 生成，需保留版权声明 |

### 11.5 需关注的组件

| 组件 | 风险点 | 风险等级 | 建议 |
|------|--------|----------|------|
| **tiktoken-go/tokenizer** (v0.7.0) | 内嵌 OpenAI 词汇表文件。tokenizer 库本身为 MIT，但词汇表来源于 OpenAI tiktoken（MIT 许可），社区普遍认为可用。严格来说，词汇表的版权归属未明确声明 | 中低 | **方案 A（推荐）**：继续使用，OpenAI 的 tiktoken 本身为 MIT 许可，词汇表作为其一部分分发。社区广泛使用，无已知法律纠纷。**方案 B**：如需完全规避，改用 API 端计费（上游 provider 返回的 usage 字段），不依赖本地 tokenizer |
| **OpenAI API 兼容协议** | API 格式（JSON schema）本身不受版权保护。但 "OpenAI" 是注册商标 | 低 | 避免在产品名称、营销材料、对外文档中使用 "OpenAI" 字样。代码中的 "openai" 包名属于技术描述性使用，风险极低 |
| **bytedance/sonic** | Apache 2.0 需遵守 NOTICE 文件义务 | 低 | 检查是否有 NOTICE 文件需包含 |

### 11.6 许可证合规义务

#### MIT 许可证（主项目和大部分依赖）
- 在分发的软件副本中保留原始版权声明和许可证文本
- 对于企业内部部署（不对外分发二进制），义务极轻

#### Apache 2.0（sonic、echarts、AWS SDK 等）
- 保留版权声明和许可证文本
- 如修改了源代码，需在修改的文件中添加变更说明
- 包含 NOTICE 文件（如果原项目提供）

#### BSD（go-redis、sqlite、excelize、Google API 等）
- 保留版权声明
- 不得使用原项目名称/贡献者名称进行推广（BSD-3-Clause 附加条款）

#### 实施清单

```bash
# 1. 自动生成依赖许可证清单
go install github.com/google/go-licenses@latest
go-licenses csv github.com/labring/aiproxy/core > THIRD_PARTY_LICENSES.csv

# 2. 在构建产物中包含许可证信息
# Dockerfile 中：
COPY LICENSE THIRD_PARTY_LICENSES.csv /app/

# 3. 前端依赖
npx license-checker --csv > web/THIRD_PARTY_LICENSES.csv
```

### 11.7 企业版代码的许可证定位

| 场景 | 建议 |
|------|------|
| 企业内部部署（不对外分发） | 无额外许可证义务，保留上游 LICENSE 文件即可 |
| 对外分发二进制 | 需包含 THIRD_PARTY_LICENSES，保留所有版权声明 |
| 对外分发源码 | 上游部分保持 MIT，`enterprise/` 目录可选择私有许可证 |
| SaaS 服务 | MIT/Apache 2.0/BSD 均允许，无 copyleft 条款，安全 |

---

## 12. 风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| 上游 `distributor.go` 大幅重构 | 企业版钩子注入失效 | 低 | 1. 钩子设计为可降级（失败时 bypass）2. CI 自动检测上游变更 3. 备选方案：用独立中间件实现 |
| 上游新增与企业版功能重叠的特性 | 功能冲突 | 中 | 持续关注上游 roadmap，如上游实现了更好的方案则迁移复用 |
| 飞书 API 限流 | 大规模同步延迟 | 中 | 全量同步使用分页 + 退避重试；增量同步使用事件订阅（实时） |
| 额度检查性能 | 请求延迟 | 低 | Redis 缓存策略（TTL 5min），热路径检查 < 5ms |
| 飞书 API 服务条款变更 | 集成功能受限 | 低 | 飞书 API 为企业自建应用标准能力，条款稳定。关注飞书开放平台公告 |
| 开源许可证合规 | 法律风险 | 极低 | 自动化许可证扫描 + THIRD_PARTY_LICENSES 文件 |
| 现有数据迁移 | 已有 Group/Token 与飞书数据关联 | 中 | 提供一次性迁移脚本：根据 Group ID / Token Name 匹配飞书用户，需人工确认映射关系 |

---

## 附录 A：上游现有基础设施复用清单

| 上游模块 | 文件位置 | 企业版复用方式 |
|---------|---------|---------------|
| Token Quota 体系 | `core/model/token.go` | PeriodQuota 作为 Tier3 硬上限 |
| Group RPM/TPM ratio | `core/model/group.go` | 限速阶梯动态调整 ratio |
| GroupModelConfig | `core/model/groupmodel.go` | 模型级别配置覆盖 |
| 动态 RPM/TPM 调整 | `core/middleware/distributor.go` `GetGroupAdjustedModelConfig()` | 注入额度阶梯的 ratio 计算 |
| 消费级别 ratio | `core/middleware/distributor.go` `calculateGroupConsumeLevelRatio()` | 参考其阶梯设计模式 |
| Notifier 接口 | `core/common/notify/notify.go` | 实现企业版 Notifier |
| 飞书 Webhook | `core/common/notify/feishu.go` | 复用 `PostToFeiShuv2()` |
| 异常用量检测 | `core/model/usage_alert.go` | 直接复用 `GetGroupUsageAlert()` |
| GroupSummary 聚合 | `core/model/groupsummary.go` | 部门维度查询层聚合 |
| Token 缓存 | `core/model/cache.go` | 额度信息已在 TokenCache 中（含 PeriodQuota、UsedAmount） |
| 飞书 SDK | `go.mod` 已引入 `larksuite/oapi-sdk-go/v3` | 直接使用 |
