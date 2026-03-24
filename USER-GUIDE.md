# AI Proxy 使用指南

> 面向企业员工的 AI 工具接入手册
>
> 通过飞书登录获取你的专属 API Key，在常用 AI 工具中一键配置，即可使用公司提供的 AI 模型服务。

---

## 一、获取你的 API Key

### 第 1 步：飞书登录

在公司内网打开以下链接，用飞书扫码或点击授权即可登录：

```
https://ai.paigod.work
```

> 💡 需要连接公司内网（办公室 Wi-Fi 或 VPN）才能访问。

### 第 2 步：进入「我的接入」页面

登录成功后，进入「我的接入」页面。页面上会显示以下内容：

- **使用概览**：总请求次数、总消耗额度、可用模型数量
- **Base URL**：AI 服务地址，所有工具都填这个
- **API Key 管理**：创建、查看、启用/禁用你的 API Key
- **快速开始**：各种 AI 工具的配置示例代码
- **可用模型列表**：你可以使用的所有模型

```
┌─────────────────────────────────────────────────┐
│  我的接入                                         │
│                                                   │
│  Base URL   https://apiproxy.paigod.work/v1       │
│                                                   │
│  API Key    sk-xxxxxxxxxxxx        [复制 Key]     │
│                                                   │
└─────────────────────────────────────────────────┘
```

| 名称 | 说明 | 示例 |
|------|------|------|
| **Base URL** | AI 服务地址（已包含 `/v1`） | `https://apiproxy.paigod.work/v1` |
| **API Key** | 你的专属密钥，相当于"通行证" | `sk-xxxxxxxx`（每人不同） |

> ⚠️ **API Key 是你的个人凭证，请勿分享给他人。** 使用量会计入你的账户。

---

## 二、配置 AI 工具

拿到 Base URL 和 API Key 后，在你使用的 AI 工具中填入即可。以下是各工具的详细配置步骤。

---

### 🍒 Cherry Studio

> 桌面端 AI 助手，支持多模型对话、知识库、绘图。

**配置路径：** 设置 → 模型服务商 → 添加自定义服务商

```
┌─ Cherry Studio 设置 ──────────────────────────────┐
│                                                     │
│  服务商名称    公司 AI 服务                           │
│  API 类型     OpenAI 兼容                            │
│  API 地址     https://apiproxy.paigod.work/v1       │
│  API Key      sk-xxxxxxxxxxxx                       │
│                                                     │
│  [获取模型列表]  ← 点击自动拉取可用模型                │
│                                                     │
└─────────────────────────────────────────────────────┘
```

**操作步骤：**

1. 打开 Cherry Studio → 左下角 ⚙️ **设置**
2. 点击 **模型服务商** → **添加自定义服务商**
3. 填写：
   - 服务商名称：`公司 AI 服务`（自定义，方便识别）
   - API 类型：选择 **OpenAI 兼容**
   - API 地址：`https://apiproxy.paigod.work/v1`
   - API Key：粘贴你的 `sk-xxxx`
4. 点击 **获取模型列表**，自动加载可用模型
5. 返回对话页面，选择模型开始使用

---

### 🤖 Claude Code

> Anthropic 官方命令行 AI 编程助手，适合代码开发和自动化任务。

**配置方式：** 通过环境变量设置

**macOS / Linux：**

在终端中执行：

```bash
# 设置环境变量（添加到 ~/.bashrc 或 ~/.zshrc 中，永久生效）
export ANTHROPIC_BASE_URL=https://apiproxy.paigod.work
export ANTHROPIC_API_KEY=sk-xxxxxxxxxxxx
```

或者在启动时指定：

```bash
ANTHROPIC_BASE_URL=https://apiproxy.paigod.work \
ANTHROPIC_API_KEY=sk-xxxxxxxxxxxx \
claude
```

**Windows（PowerShell）：**

```powershell
$env:ANTHROPIC_BASE_URL = "https://apiproxy.paigod.work"
$env:ANTHROPIC_API_KEY = "sk-xxxxxxxxxxxx"
claude
```

**操作步骤：**

1. 确保已安装 Claude Code（`npm install -g @anthropic-ai/claude-code`）
2. 设置上面的两个环境变量
3. 在终端输入 `claude` 启动
4. 如果提示选择模型，选择 `pa/claude-sonnet-4-5-20250929` 或其他可用 Claude 模型

> 💡 **注意**：Claude Code 使用 Anthropic 协议，Base URL **不带** `/v1`（与 OpenAI 兼容工具不同）。
>
> 💡 **建议**把环境变量写入 `~/.zshrc`（macOS）或 `~/.bashrc`（Linux），避免每次手动输入。

---

### ⌨️ Cursor

> AI 驱动的代码编辑器，内置 AI 对话和代码补全。

**配置路径：** Cursor Settings → Models → OpenAI API Key

```
┌─ Cursor Settings ─────────────────────────────────┐
│                                                     │
│  Models                                             │
│                                                     │
│  ☑ Override OpenAI Base URL                         │
│     https://apiproxy.paigod.work/v1                 │
│                                                     │
│  OpenAI API Key                                     │
│     sk-xxxxxxxxxxxx                                 │
│                                                     │
│  [Verify]  ← 点击验证连接                            │
│                                                     │
└─────────────────────────────────────────────────────┘
```

**操作步骤：**

1. 打开 Cursor → 点击右上角 ⚙️ → **Cursor Settings**
2. 在左侧选择 **Models**
3. 勾选 **Override OpenAI Base URL**
4. 填入：`https://apiproxy.paigod.work/v1`
5. 在 **OpenAI API Key** 中填入你的 `sk-xxxx`
6. 点击 **Verify** 验证连接是否成功
7. 在模型列表中勾选你想使用的模型

> 💡 Cursor 会自动从 API 获取可用模型列表。如果某个模型没显示，可以手动输入模型名称添加。

---

## 三、协议说明

> 如果你只是日常使用，可以跳过这一节。这里是为需要自行开发或集成的同事准备的。

AI Proxy 支持多种协议，不同工具使用不同的协议：

```
┌──────────────────────────────────────────────────────────────┐
│                    AI Proxy 支持的协议                         │
│                                                               │
│  ┌─────────────────┐  ┌──────────────────┐  ┌────────────┐  │
│  │  OpenAI 兼容      │  │  Anthropic 兼容    │  │  MCP 协议   │  │
│  │  /v1/chat/...    │  │  /v1/messages     │  │  /mcp/*    │  │
│  │                  │  │                   │  │  /sse      │  │
│  │ Cherry Studio    │  │  Claude Code      │  │  MCP 客户端 │  │
│  │ Cursor           │  │                   │  │            │  │
│  │ ChatBox          │  │                   │  │            │  │
│  │ LobeChat 等      │  │                   │  │            │  │
│  └─────────────────┘  └──────────────────┘  └────────────┘  │
│                                                               │
│  绝大多数工具选 "OpenAI 兼容" 即可，AI Proxy 会自动转换协议。     │
└──────────────────────────────────────────────────────────────┘
```

| 协议 | Base URL | 适用场景 |
|------|----------|---------|
| **OpenAI 兼容**（推荐） | `https://apiproxy.paigod.work/v1` | 90% 的 AI 工具都支持，首选 |
| **Anthropic 兼容** | `https://apiproxy.paigod.work` | Claude Code 等 Anthropic 原生工具（不带 `/v1`） |
| **MCP 协议** | `https://apiproxy.paigod.work/mcp` | MCP 兼容的 AI Agent 工具 |

> 💡 **不确定选哪个？** 选 **OpenAI 兼容**，工具里选 "OpenAI" 或 "自定义 API"，填入 Base URL 和 API Key 即可。

---

## 四、推荐模型

以下是平台上最值得使用的模型，覆盖日常办公、编程、创作等主要场景。

> 💡 **模型 ID 命名规则：** 闭源商业模型使用 `pa/` 前缀（如 `pa/gpt-5`、`pa/claude-sonnet-4-5-20250929`），社区/开源模型使用 `提供商/模型名/community` 格式（如 `deepseek/deepseek-r1/community`）。具体可用模型以「我的接入」页面的模型列表为准。

### 模型速查表

```
                    性价比                    能力
                 ◄────────►              ◄────────►
                 高        低             通用      专精

  DeepSeek-R1    ██████████             ████████░░   深度推理
  DeepSeek-V3    █████████░             ███████░░░   日常万能
  Claude 4.5     ████░░░░░░             ██████████   编程写作
  GPT-5          ████░░░░░░             █████████░   综合理解
  Claude Haiku   █████████░             █████░░░░░   快速轻量
```

### 详细推荐

#### 1. 🧠 DeepSeek-R1 — 深度思考，复杂推理

| 项目 | 说明 |
|------|------|
| **模型 ID** | `deepseek/deepseek-r1/community` |
| **最佳场景** | 数学推导、逻辑分析、策略规划、复杂问题拆解 |
| **特点** | 会"思考"后再回答，推理能力极强，性价比极高 |
| **适合谁** | 数据分析师、策略岗、需要深度分析的场景 |

> 💡 **适合问题示例：** "帮我分析这组销售数据的趋势，找出异常点并给出改进建议"

#### 2. ⚡ DeepSeek-V3 — 日常万能，响应飞快

| 项目 | 说明 |
|------|------|
| **模型 ID** | `deepseek/deepseek-v3/community` |
| **最佳场景** | 日常问答、文案撰写、邮件润色、翻译、摘要 |
| **特点** | 速度快、价格低、中文理解能力优秀 |
| **适合谁** | 所有人的日常首选 |

> 💡 **适合问题示例：** "把这段会议纪要整理成结构化的待办事项" "帮我写一封催促进度的邮件，语气委婉"

#### 3. 💻 Claude Sonnet 4.5 — 编程写作，精准可靠

| 项目 | 说明 |
|------|------|
| **模型 ID** | `pa/claude-sonnet-4-5-20250929` |
| **最佳场景** | 代码编写/审查、技术文档、长文写作、复杂指令跟随 |
| **特点** | 代码能力顶级，写作风格自然，对长上下文理解强 |
| **适合谁** | 开发工程师、技术写作、需要高质量长文的场景 |

> 💡 **适合问题示例：** "Review 这段代码并指出潜在的性能问题" "帮我写一份技术方案文档"
>
> 💡 **更强选择：** `pa/claude-opus-4-5-20251101`（Claude Opus 4.5），能力更强但速度稍慢、成本更高，适合最复杂的任务。

#### 4. 🌐 GPT-5 — 综合全能，视觉理解

| 项目 | 说明 |
|------|------|
| **模型 ID** | `pa/gpt-5` |
| **最佳场景** | 多模态（图片理解）、跨语言翻译、创意生成、综合分析 |
| **特点** | 综合能力均衡，支持图片输入，生态成熟 |
| **适合谁** | 产品经理、设计师、需要图片分析或创意灵感的场景 |

> 💡 **适合问题示例：** "看看这个竞品截图，分析它的设计亮点" "帮我用中英双语写一段产品介绍"
>
> 💡 **其他 GPT 系列：** `pa/gpt-5.1`（更新版）、`pa/gpt-5-mini`（轻量快速版）

#### 5. 🚀 Claude Haiku 4.5 — 极速响应，轻量任务

| 项目 | 说明 |
|------|------|
| **模型 ID** | `pa/claude-haiku-4-5-20251001` |
| **最佳场景** | 简单问答、格式转换、快速分类、批量处理 |
| **特点** | 速度极快、成本极低，适合不需要深度思考的任务 |
| **适合谁** | 需要快速出结果、高频轻量使用的场景 |

> 💡 **适合问题示例：** "把这个 JSON 转成表格" "这段文字是正面还是负面评价？"

### 怎么选？一句话决策

```
需要深度分析/推理？      → DeepSeek-R1
日常聊天/写邮件/翻译？    → DeepSeek-V3
写代码/技术文档？        → Claude Sonnet 4.5
需要看图/创意/综合？     → GPT-5
追求速度/简单任务？      → Claude Haiku 4.5
```

> 💡 平台上还有更多模型（Gemini、Grok、通义千问、文心一言等），完整列表请查看「我的接入」页面。

---

## 五、常见问题

### 连接不上 / 报错 401

- 检查 API Key 是否正确复制（前后不要有空格）
- 确认 Base URL 是 `https://apiproxy.paigod.work/v1`（OpenAI 兼容工具）或 `https://apiproxy.paigod.work`（Anthropic 工具，不带 `/v1`）
- 注意 **不是** `ai.paigod.work`（那是内网管理后台地址）
- API Key 过期？重新登录 `https://ai.paigod.work` 获取新 Key

### 提示模型不存在

- 确认模型 ID 拼写正确（区分大小写）
- 闭源模型需要带 `pa/` 前缀（如 `pa/gpt-5` 而非 `gpt-5`）
- 社区模型使用完整路径（如 `deepseek/deepseek-r1/community`）
- 部分模型可能未开放，联系管理员确认

### 响应很慢

- AI 模型首次响应需要 2-5 秒属于正常
- 复杂问题 + 长上下文会更慢
- 尝试换用更快的模型（如 Claude Haiku 4.5 或 DeepSeek-V3）

### 其他 AI 工具怎么配？

大部分 AI 工具都支持 **OpenAI 兼容** 模式，通用配置方法：

1. 在工具设置中找到 "API" 或 "模型提供商" 相关选项
2. 选择 "OpenAI" 或 "自定义 API"
3. 填入 Base URL：`https://apiproxy.paigod.work/v1`
4. 填入 API Key：`sk-xxxx`
5. 保存并测试

---

## 六、获取帮助

- **管理后台：** [https://ai.paigod.work](https://ai.paigod.work)（内网）
- **飞书登录：** [https://ai.paigod.work/api/enterprise/auth/feishu/login](https://ai.paigod.work/api/enterprise/auth/feishu/login)（内网，可分享给同事）
- **问题反馈：** 联系 IT 管理员或在飞书群中提问
