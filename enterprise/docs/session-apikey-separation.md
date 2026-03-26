# Session 与 API Key 分离方案

> 状态：**待审阅** | 作者：Claude Code + Ash | 日期：2026-03-26

---

## 1. 问题陈述

当前架构中，飞书 OAuth 登录后返回的 `token_key`（tokens 表的 key 字段）同时承担两个角色：

| 角色 | 用途 | 生命周期 |
|------|------|----------|
| **Web Session 凭证** | 前端 Authorization header，访问企业分析 API | 登录后持续有效 |
| **AI API Key** | 调用 /v1/chat/completions 等 AI 接口 | 用户可随时启用/禁用/删除 |

**核心缺陷**：用户在「我的密钥」页面禁用某个 API Key（常规操作），如果恰好禁用的是飞书登录时 FirstOrCreate 匹配到的那个 token，则 Web Session 立即失效 → 所有企业 API 返回 401 → 前端 logout → 重定向 /login → 再次 OAuth 回调拿到同一个 disabled token → **无限循环**。

**影响范围**：任何创建了多个 Key 并禁用其中之一的飞书用户都可能遇到。

---

## 2. 方案概述

引入 **JWT（JSON Web Token）** 作为 Web Session 凭证，与 tokens 表中的 API Key **完全解耦**。

```
┌─────────────┐     OAuth callback      ┌──────────────┐
│  飞书 OAuth  │ ──────────────────────> │  后端签发 JWT │
└─────────────┘                         └──────┬───────┘
                                               │
                    session_token (JWT)         │
                    ┌──────────────────────────┘
                    ▼
┌─────────────────────────────────────────────────────┐
│  前端 (Zustand)                                      │
│  sessionToken: JWT  ← 用于企业分析 API 鉴权          │
│  apiKeys: []        ← 仅用于展示/管理，不参与登录     │
└─────────────────────────────────────────────────────┘
```

---

## 3. JWT 设计

### 3.1 Payload

```json
{
  "sub": "feishu_user_id (string)",
  "role": "admin | member | viewer",
  "group_id": "对应的 group_id",
  "exp": 1711497600,
  "iat": 1710892800
}
```

### 3.2 签名

- 算法：**HS256**（HMAC-SHA256）
- 密钥：复用现有 `ADMIN_KEY` 环境变量（已存在于 .env）
- 理由：单服务部署，对称签名足够；无需引入非对称密钥管理

### 3.3 有效期

| 参数 | 值 | 说明 |
|------|-----|------|
| 过期时间 | **7 天** | 平衡安全与体验，飞书企业场景可接受 |
| 刷新策略 | **滑动窗口** | 每次请求若剩余 < 1 天，响应头返回新 JWT |
| 强制失效 | 修改 ADMIN_KEY 或用户被移除 | 全局失效，无需维护黑名单 |

### 3.4 依赖

- `github.com/golang-jwt/jwt/v5` — **已在 go.mod 中**，无需新增依赖

---

## 4. 后端改动

### 4.1 新增文件：`core/enterprise/jwt.go`

```go
package enterprise

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
    "github.com/labring/aiproxy/core/common/config"
)

type EnterpriseClaims struct {
    Role    string `json:"role"`
    GroupID string `json:"group_id"`
    jwt.RegisteredClaims
}

func GenerateSessionJWT(feishuUserID, role, groupID string) (string, error) {
    claims := EnterpriseClaims{
        Role:    role,
        GroupID: groupID,
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   feishuUserID,
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(config.GetAdminKey()))
}

func ParseSessionJWT(tokenString string) (*EnterpriseClaims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &EnterpriseClaims{},
        func(token *jwt.Token) (interface{}, error) {
            return []byte(config.GetAdminKey()), nil
        },
    )
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*EnterpriseClaims)
    if !ok {
        return nil, jwt.ErrTokenInvalidClaims
    }
    return claims, nil
}

// ShouldRefresh returns true if token expires within 24 hours
func ShouldRefresh(claims *EnterpriseClaims) bool {
    return time.Until(claims.ExpiresAt.Time) < 24*time.Hour
}
```

### 4.2 修改文件：`core/enterprise/auth.go`

EnterpriseAuth 中间件增加 **JWT 路径**：

```
请求进入
  │
  ├─ Header 以 "Bearer " 开头？ → JWT 路径
  │   ├─ ParseSessionJWT → 成功 → 查 feishu_users 表确认用户存在
  │   │   ├─ 存在 → set context (feishu_user, role, group_id)
  │   │   │   └─ ShouldRefresh? → 是 → 响应头 X-New-Token 返回新 JWT
  │   │   └─ 不存在（用户被删除）→ 401
  │   └─ 解析失败 / 过期 → 401
  │
  ├─ 等于 ADMIN_KEY？ → Admin 路径（不变）
  │
  └─ 其他 → 旧 token_key 路径（**保留兼容，30 天后移除**）
```

**关键点**：JWT 路径 **不查 tokens 表**，仅查 feishu_users 表确认用户仍然存在。API Key 的禁用/删除对 JWT session 零影响。

### 4.3 修改文件：`core/enterprise/feishu/oauth.go`

HandleCallback 改动：

```go
// 之前：返回 token_key
// redirectURL = fmt.Sprintf("...?token_key=%s&user=...", token.Key)

// 之后：签发 JWT，不再依赖 token
sessionJWT, err := enterprise.GenerateSessionJWT(
    feishuUser.FeishuUserID,
    feishuUser.Role,
    groupID,
)
if err != nil {
    // handle error
}
redirectURL = fmt.Sprintf("...?session_token=%s&user=%s",
    url.QueryEscape(sessionJWT),
    url.QueryEscape(userJSON),
)
```

**新用户首次登录**：仍然自动创建一个 API Key（通过 InsertToken），以便用户立即可用 AI 接口。但这个 Key 仅用于 API 调用，不作为 Session。

**老用户再次登录**：不再调用 InsertToken，直接签发 JWT。

---

## 5. 前端改动

### 5.1 `web/src/store/auth.ts`

```typescript
interface AuthState {
    sessionToken: string | null   // JWT — 新增
    token: string | null           // 保留，用于兼容过渡期
    isAuthenticated: boolean
    enterpriseUser: EnterpriseUser | null

    loginWithFeishu: (sessionToken: string, user: EnterpriseUser) => void
    logout: () => void
}

// persist partialize 改为存储 sessionToken
partialize: (state) => ({
    sessionToken: state.sessionToken,
    isAuthenticated: state.isAuthenticated,
    enterpriseUser: state.enterpriseUser,
})
```

### 5.2 `web/src/api/index.ts`

```typescript
// 请求拦截器
config.headers.Authorization = `Bearer ${sessionToken}`

// 响应拦截器
const newToken = response.headers['x-new-token']
if (newToken) {
    useAuthStore.getState().refreshToken(newToken)
}

// 401 处理 — 只在 JWT 失效时 logout
if (status === 401) {
    useAuthStore.getState().logout()
    window.location.href = '/login'
}
```

### 5.3 `web/src/pages/auth/feishu-callback.tsx`

```typescript
// 之前：读取 token_key
// const tokenKey = searchParams.get('token_key')

// 之后：读取 session_token
const sessionToken = searchParams.get('session_token')
const userStr = searchParams.get('user')
// ...
loginWithFeishu(sessionToken, userData)
```

### 5.4 `web/src/pages/enterprise/my-access.tsx`

API Key 管理页面 **无需改动**。该页面通过企业 API（JWT 鉴权）获取用户的 API Keys 列表，禁用/删除操作仅影响 tokens 表，与 JWT Session 无关。

---

## 6. 迁移影响

### 6.1 对现有用户

| 影响 | 说明 |
|------|------|
| **需要重新登录一次** | 旧 token_key 凭证不再被优先识别。用户下次访问时，前端检测到无 sessionToken → 跳转登录页 → 飞书 OAuth → 获得新 JWT |
| **现有 API Key 完全不受影响** | tokens 表数据保持原样，AI API 调用照常工作 |
| **localStorage 清理** | 新版前端的 persist partialize 不含旧 token 字段，Zustand 自动忽略 |

### 6.2 兼容过渡期（建议 30 天）

保留旧 token_key 路径在 EnterpriseAuth 中作为 fallback：
- 如果 Authorization header **不含 "Bearer " 前缀**，走旧 token_key 路径
- 30 天后（或确认无旧版前端流量后）移除

### 6.3 回滚方案

如果发布后出现问题：
1. 回滚后端代码到上一版本（恢复旧 EnterpriseAuth）
2. 前端回滚到旧版本
3. 用户 token_key 仍在 localStorage（persist 中），可继续使用
4. 不需要数据库操作

---

## 7. 安全评估

### 7.1 优势

| 项目 | 说明 |
|------|------|
| **Session 不可被用户操作影响** | 禁用/删除 API Key 不会导致登出 |
| **无状态验证** | JWT 本地解析，不查 DB，降低数据库压力 |
| **自动续期** | 滑动窗口避免用户频繁重新登录 |
| **单点失效** | 修改 ADMIN_KEY 即可让所有 JWT 失效 |

### 7.2 风险与缓解

| 风险 | 等级 | 缓解措施 |
|------|------|----------|
| JWT 被窃取 | 中 | HTTPS only；7 天过期；前端 localStorage（非 cookie，避免 CSRF） |
| 用户被删除后 JWT 仍有效 | 低 | EnterpriseAuth 每次查 feishu_users 确认用户存在（已在方案中） |
| ADMIN_KEY 泄露 | 高 | 与现有风险等级一致（ADMIN_KEY 已是最高权限密钥） |
| 时钟偏移导致 JWT 验证失败 | 极低 | 单服务部署，无分布式时钟问题 |

### 7.3 未纳入本次的优化（未来考虑）

- **Refresh Token + Access Token 双 token**：当前单 JWT 已足够，企业内网场景无需双 token 复杂度
- **JWT 黑名单（Redis）**：当前通过 feishu_users 表存在性检查 + ADMIN_KEY 全局失效已足够
- **HttpOnly Cookie 存储**：需要处理 CSRF，当前 localStorage + Bearer header 更简单

---

## 8. 实施步骤

```
Phase 1: 后端（约 2 小时）
  ├─ 1. 新增 core/enterprise/jwt.go
  ├─ 2. 修改 core/enterprise/auth.go（三路径鉴权）
  └─ 3. 修改 core/enterprise/feishu/oauth.go（签发 JWT）

Phase 2: 前端（约 1 小时）
  ├─ 4. 修改 web/src/store/auth.ts
  ├─ 5. 修改 web/src/api/index.ts
  └─ 6. 修改 web/src/pages/auth/feishu-callback.tsx

Phase 3: 测试（约 1 小时）
  ├─ 7. 本地测试：飞书登录 → JWT 签发 → 企业 API 访问
  ├─ 8. 测试：禁用 API Key → 企业 Dashboard 仍可访问
  ├─ 9. 测试：JWT 过期 → 重定向登录 → 重新获取 JWT
  └─ 10. 测试：兼容旧 token_key（过渡期）

Phase 4: 部署
  ├─ 11. 编译 go build -tags enterprise
  ├─ 12. 部署后端，重启服务
  └─ 13. 部署前端（npm run build + Nginx）
```

---

## 9. 验证清单

- [ ] 飞书新用户首次登录：自动创建 Group + API Key + 签发 JWT
- [ ] 飞书老用户再次登录：直接签发 JWT，不创建新 Key
- [ ] 禁用用户的所有 API Key → 企业 Dashboard 仍可正常访问
- [ ] 删除用户的所有 API Key → 企业 Dashboard 仍可正常访问
- [ ] JWT 过期 → 401 → 重定向登录页 → 重新 OAuth
- [ ] JWT 临近过期 → X-New-Token 自动续期
- [ ] Admin Key 登录 → 不受影响
- [ ] 旧版前端（兼容期）→ token_key 仍可访问
- [ ] 修改 ADMIN_KEY → 所有 JWT 失效

---

## 10. 结论

本方案通过引入 JWT 作为独立的 Web Session 凭证，**彻底解决了 Token=Session=API Key 的架构耦合问题**。改动范围可控（3 个后端文件 + 3 个前端文件），对现有用户影响最小（仅需重新登录一次），且提供了 30 天兼容过渡期和完整的回滚方案。

建议在审阅通过后立即实施。
