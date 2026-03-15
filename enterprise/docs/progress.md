# 企业版开发进度

## 已完成功能 ✅

### 第 1 阶段：后端基础（commit 1d4e8ee）
- [x] 飞书 OAuth 登录/回调
- [x] 飞书组织架构同步（定时任务）
- [x] FeishuUser / QuotaTier 数据模型 + 自动迁移
- [x] 企业版认证中间件
- [x] 部门汇总 / 部门趋势 API
- [x] 用户排行 API
- [x] 渐进式额度策略 CRUD + 请求 hook
- [x] 飞书私聊通知 / 额度告警
- [x] Excel 基础导出

### 第 2 阶段：前端页面（commit e635d8d）
- [x] Login 页飞书登录按钮
- [x] 飞书 OAuth 回调中转页 `/feishu/callback`
- [x] EnterpriseLayout（紫色渐变 Sidebar）
- [x] Enterprise Dashboard（指标卡片 + 部门表 + 饼图）
- [x] 员工排行榜页
- [x] 部门趋势详情页
- [x] 管理后台 ↔ 企业分析双向导航
- [x] auth store 扩展（enterpriseUser 字段）
- [x] 中英文国际化翻译

### 第 3 阶段：报表增强（commit dfb3821）
- [x] 模型用量分布接口 + 前端图表
- [x] 环比对比接口（日/周/月） + 前端指标卡片变化指示器
- [x] 部门排行接口
- [x] 部门汇总增加：活跃用户、成功率、平均成本、模型数
- [x] 用户排行增加：排名、输入/输出 Token、成功率、模型数
- [x] Excel 导出重构为 4 Sheet（汇总、部门明细、用户排行、模型分布）
- [x] 前端 Playwright UI 自测通过

### 第 4 阶段：额度策略 UI（commit 205ef42, ed11ab5）
- [x] 额度策略管理页面（CRUD 表格）
- [x] 额度策略表单（渐进式配额配置）
- [x] 策略分配给用户/部门
- [x] 用户当前额度状态展示
- [x] 飞书 OAuth 流程优化

### 第 5 阶段：排行榜增强（commit 72e9286, 00c00f7, d6bc83b）
- [x] 筛选功能增强（时间范围、部门多选、Top N / 全量）
- [x] 列可见性自定义选择
- [x] 点击列头排序（升序/降序切换）
- [x] 排序状态与列可见性联动修复
- [x] 管理后台链接改为新标签页打开

### 第 6 阶段：企业租户白名单（commit 8c4c846）
- [x] `FEISHU_ALLOWED_TENANTS` 环境变量支持多租户白名单
- [x] OAuth 回调时校验用户 TenantID
- [x] 非白名单用户返回 403 + 友好错误页面
- [x] FeishuUser 模型新增 TenantID 字段
- [x] 浏览器流程错误处理优化

### 第 7 阶段：自定义报表（commit 809329f）
- [x] 后端 custom_report.go（多维度聚合 API + 字段目录接口）
- [x] 前端 custom-report.tsx 完整实现
- [x] 透视表模式（2 维度行列交叉展示，支持切换度量）
- [x] 时间维度自动折线图（选 time_* 维度自动切换 line chart）
- [x] 5 个预设报表模板（部门费用、模型趋势、用户排行、交叉分析、每日性能）
- [x] 筛选器 UI（部门多选 Popover、模型 Tag 输入、用户名 Tag 输入）
- [x] CSV 导出
- [x] viewMode 维度变化自动降级修复
- [x] handleGenerate useCallback 稳定性修复（mutateRef 模式）
- [x] 路由、导航栏、i18n（中/英）配置

## 待开发功能 📋

### 第 8 阶段：通知与告警 UI
- [ ] 飞书通知配置页面
- [ ] 额度告警阈值设置
- [ ] 告警历史记录查看

### 第 9 阶段：集成与优化
- [ ] 前端嵌入 Go 二进制（`cp -r web/dist/ core/public/dist/`）
- [ ] 飞书 REDIRECT_URI 配置说明文档
- [ ] 生产环境部署测试
- [ ] 性能优化（大数据量分页、图表懒加载）

## 前端页面清单

| 页面 | 路由 | 状态 | 功能 |
|-----|------|-----|------|
| 企业概览 | `/enterprise` | ✅ | 指标卡片、部门表、模型饼图、环比变化 |
| 员工排行 | `/enterprise/ranking` | ✅ | 9列表格、筛选、排序、列选择、导出 |
| 部门趋势 | `/enterprise/department` | ✅ | ECharts 折线/柱状图 |
| 额度策略 | `/enterprise/quota` | ✅ | 策略 CRUD、分配用户/部门 |
| 自定义报表 | `/enterprise/custom-report` | ✅ | 维度/度量选择、表格/图表/透视表、模板、筛选器、CSV 导出 |

## 环境变量配置

| 变量 | 说明 | 示例 |
|------|-----|------|
| `FEISHU_APP_ID` | 飞书应用 App ID | `cli_xxxxx` |
| `FEISHU_APP_SECRET` | 飞书应用 App Secret | `xxxxx` |
| `FEISHU_REDIRECT_URI` | OAuth 回调地址 | `https://api.example.com/api/enterprise/auth/feishu/callback` |
| `FEISHU_FRONTEND_URL` | 前端基础 URL | `https://app.example.com` |
| `FEISHU_ALLOWED_TENANTS` | 允许的企业租户白名单（逗号分隔） | `tenant_abc,tenant_def` |

## 易错点与注意事项 ⚠️

### 后端
1. **Build Tag**: 所有 `core/enterprise/*.go` 文件首行必须有 `//go:build enterprise`
2. **Go PATH**: 本机 go 不在默认 PATH，需要 `PATH="/usr/local/go/bin:$PATH"`
3. **数据库选择**: `model.LogDB` 查询 GroupSummary，`model.DB` 查询 FeishuUser 等业务表
4. **零除保护**: 所有百分比计算都必须检查分母为零
5. **golangci-lint**: 本机未安装，依赖 `go build -tags enterprise` 验证编译

### 前端
1. **Auth Store**: 字段 `isAuthenticated`，persist key `auth-storage`，格式 `{state: {...}, version: 0}`
2. **时间戳**: `hour_timestamp` 是 Unix 秒，前端必须 `* 1000` 转 JS Date
3. **Playwright**: 使用 `channel="chrome"` 用本地 Chrome，不要下载浏览器
4. **ECharts**: useEffect cleanup 中必须 dispose 实例
5. **React Query Key**: 企业版查询 key 以 `["enterprise", ...]` 开头
6. **TFunction 类型**: 从 `i18next` 导入（非 `react-i18next`），动态 key 用 `as never` 转型
7. **useCallback + mutation**: mutation 不稳定，用 `mutateRef` 模式避免无限重渲染

### 数据模型
1. **活跃用户** = `COUNT(DISTINCT group_id)`
2. **UniqueModels** = `COUNT(DISTINCT model)`
3. **FeishuUser.GroupID** 连接 GroupSummary 和飞书用户
4. **FeishuUser.TenantID** 记录用户所属企业租户
5. **DepartmentID** 来自飞书组织架构同步

## 参考项目
- `/Users/ash/AI/ai-api-gateway/gateway-ext` — 字段设计和报表格式参考
