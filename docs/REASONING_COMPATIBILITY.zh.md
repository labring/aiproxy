# Thinking / Reasoning 参数兼容说明

本文档介绍 aiproxy 当前的“思考 / 推理参数兼容层”功能：

- 不同请求协议如何表达推理参数
- 代理内部如何归一化这些参数
- 转发到不同上游厂商时会如何转换
- 各厂商 / 各模型的已知限制、兜底和降级策略

> 说明：本文档描述的是**参数兼容与转换逻辑**，不是所有厂商完整的 API 文档。

## 1. 目标

这个功能的目标是：

1. 让调用方尽量使用当前请求模式的**原生推理参数**
2. 在“请求格式 A -> 上游格式 B”的转换过程中，自动把推理参数转换成上游能接受的格式
3. 尽可能避免因为模型能力差异或字段限制导致上游直接报错

当前实现明确遵循以下原则：

- **只解析当前请求模式的原生 thinking / reasoning 参数**
  - OpenAI Chat / Completions 只解析 `reasoning_effort`
  - OpenAI Responses 目标格式只写入 `reasoning.effort`
  - Gemini 只解析 `generationConfig.thinkingConfig`
  - Claude / Anthropic 只解析 `thinking` / `output_config`
- **不再做旧版通用 `thinking` 结构的反向兼容解析**
- **只有“转换后的请求体”才会做 thinking 参数兜底与修正**
- **原生请求不会被自动迁移到另一种 thinking 方言**
  - 例如 native Claude 请求不会被自动改写成 OpenAI 的 `reasoning_effort`
  - 但已有的协议级清理逻辑仍可能存在，例如某些上游不允许 `temperature` 与 thinking 同时存在
- 所有**基于模型名**的能力判断都使用：
  1. `OriginModel` 优先
  2. 若未命中，再回退 `ActualModel`

---

## 2. 内部归一化模型

代理内部会先把不同协议的参数归一化成一个统一结构，大致可理解为：

- `Specified`: 是否显式设置了推理参数
- `Disabled`: 是否显式关闭推理
- `Effort`: 统一后的强度枚举
- `BudgetTokens`: 如果原始协议提供了 token 预算，则保留该预算

### 2.1 支持的统一强度枚举

当前统一强度枚举为：

- `none`
- `minimal`
- `low`
- `medium`
- `high`
- `xhigh`

其中也兼容若干别名输入：

- `off` / `disabled` -> `none`
- `med` -> `medium`
- `max` / `maximum` -> `xhigh`

### 2.2 默认 effort <-> budget 映射

当某个上游只支持 token budget、不支持 high / medium 这类离散档位时，会使用以下默认映射：

| effort | budget |
| --- | ---: |
| `none` | `0` |
| `minimal` | `1024` |
| `low` | `2048` |
| `medium` | `8192` |
| `high` | `16384` |
| `xhigh` | `32768` |

反向把 budget 还原为 effort 时，使用下面的区间：

| budget 区间 | 还原 effort |
| --- | --- |
| `<= 0` | `none` |
| `1 ~ 1024` | `minimal` |
| `1025 ~ 4096` | `low` |
| `4097 ~ 12288` | `medium` |
| `12289 ~ 24576` | `high` |
| `> 24576` | `xhigh` |

---

## 3. 各请求模式的入参格式

### 3.1 OpenAI Chat / Completions

当前只解析：

```json
{
  "reasoning_effort": "none|minimal|low|medium|high|xhigh"
}
```

说明：

- 这是当前 OpenAI Chat / Completions 模式下唯一会被兼容层读取的推理参数
- 不再解析旧版通用 `thinking` 结构

### 3.2 OpenAI Responses

当代理需要生成 OpenAI Responses 请求体时，推理参数会写成：

```json
{
  "reasoning": {
    "effort": "none|minimal|low|medium|high|xhigh"
  }
}
```

说明：

- 当前实现中，Responses 主要作为**目标格式**写出
- 即：Chat / Claude / Gemini 等请求在转换成 Responses 时，会写入 `reasoning.effort`

### 3.3 Gemini

当前解析：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true,
      "thinkingLevel": "minimal|low|medium|high"
    }
  }
}
```

解析优先级：

1. `thinkingLevel`
2. `thinkingBudget`
3. `includeThoughts`

含义：

- `thinkingLevel`：直接映射为统一 effort
- `thinkingBudget`：通过 budget 区间反推 effort
- `includeThoughts=true` 且未给其他字段：按 `medium` 处理
- `thinkingBudget<=0`：按 `none` 处理
- 三者都没给、且 `thinkingConfig` 显式存在但不包含可识别字段：按关闭处理

### 3.4 Claude / Anthropic

当前解析：

```json
{
  "thinking": {
    "type": "disabled|enabled|adaptive",
    "budget_tokens": 2048
  },
  "output_config": {
    "effort": "low|medium|high|max"
  }
}
```

解析规则：

- `thinking.type=disabled` -> `none`
- `thinking.type=enabled` / `adaptive` -> 开启推理
- 若提供了 `budget_tokens`，会同时保留 budget 信息
- 若提供了 `output_config.effort`，会优先据此确定 effort
- 若只给了 `thinking.type=enabled` 但没有 budget / effort，则默认按 `medium` 处理

---

## 4. 各目标格式如何写出

### 4.1 写成 OpenAI Chat / Completions

输出字段：

```json
{
  "reasoning_effort": "..."
}
```

适用场景：

- Gemini -> OpenAI
- Claude -> OpenAI
- 其他请求先归一化后，再输出成 OpenAI 兼容格式

effort 映射：

| 统一 effort | OpenAI Chat / Completions 字段 |
| --- | --- |
| `none` | `reasoning_effort: "none"` |
| `minimal` | `reasoning_effort: "minimal"` |
| `low` | `reasoning_effort: "low"` |
| `medium` | `reasoning_effort: "medium"` |
| `high` | `reasoning_effort: "high"` |
| `xhigh` | `reasoning_effort: "xhigh"` |

### 4.2 写成 OpenAI Responses

输出字段：

```json
{
  "reasoning": {
    "effort": "..."
  }
}
```

适用场景：

- Chat -> Responses
- Claude -> Responses
- Gemini -> Responses

effort 映射：

| 统一 effort | OpenAI Responses 字段 |
| --- | --- |
| `none` | `reasoning.effort: "none"` |
| `minimal` | `reasoning.effort: "minimal"` |
| `low` | `reasoning.effort: "low"` |
| `medium` | `reasoning.effort: "medium"` |
| `high` | `reasoning.effort: "high"` |
| `xhigh` | `reasoning.effort: "xhigh"` |

### 4.3 写成 Gemini

输出位置：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true,
      "thinkingLevel": "low|medium|high"
    }
  }
}
```

规则分两类：

#### A. Gemini 3 / 4 / 5 系列：使用 `thinkingLevel`

模型名命中以下前缀时，优先写 `thinkingLevel`：

- `gemini-3*`
- `gemini-4*`
- `gemini-5*`

映射规则：

- Pro 型号：
  - `high` / `xhigh` -> `high`
  - 其余开启态 -> `low`
- 非 Pro 型号：
  - `none` -> `minimal`
  - `low` -> `low`
  - `medium` -> `medium`
  - `high` / `xhigh` -> `high`
  - 其余 -> `minimal`

关闭规则：

- 这类模型通常不使用 `thinkingBudget=0` 来关闭
- 如果请求显式 `none`，会退化为该模型允许的最小 level，而不是强行写非法关闭参数

精确 level 映射：

| 统一 effort | Gemini 3+ Pro `thinkingLevel` | Gemini 3+ 非 Pro `thinkingLevel` |
| --- | --- | --- |
| `none` | `low` | `minimal` |
| `minimal` | `low` | `minimal` |
| `low` | `low` | `low` |
| `medium` | `low` | `medium` |
| `high` | `high` | `high` |
| `xhigh` | `high` | `high` |

#### B. Gemini 2.5 系列：使用 `thinkingBudget`

模型限制：

| 模型 | budget 范围 | 是否支持关闭 |
| --- | --- | --- |
| `gemini-2.5-pro` | `128 ~ 32768` | 否 |
| `gemini-2.5-flash` | `1 ~ 24576` | 是 |
| `gemini-2.5-flash-lite` | `512 ~ 24576` | 是 |

写出规则：

- 开启推理时：
  - 先按 effort 计算默认 budget
  - 再按模型区间进行 clamp
- 关闭推理时：
  - 对支持关闭的模型写 `thinkingBudget=0`
  - 对不支持关闭的模型写该模型最小 budget
- `includeThoughts`：
  - 开启时为 `true`
  - 关闭时为 `false`

重要说明：

- **不会**因为 `max_tokens` / `maxOutputTokens` 较小，就把 Gemini thinking budget 再向下夹到 `max tokens` 内
- 这是有意设计，避免把合法的 Gemini thinking 配置错误改写成更小值

经过 Gemini 模型区间 clamp 后的精确 budget 映射：

| 统一 effort | `gemini-2.5-pro` | `gemini-2.5-flash` | `gemini-2.5-flash-lite` |
| --- | ---: | ---: | ---: |
| `none` | `128` | `0` | `0` |
| `minimal` | `1024` | `1024` | `1024` |
| `low` | `2048` | `2048` | `2048` |
| `medium` | `8192` | `8192` | `8192` |
| `high` | `16384` | `16384` | `16384` |
| `xhigh` | `32768` | `24576` | `24576` |

开启态行的 `includeThoughts` 为 `true`，`none` 行为 `false`。

### 4.4 写成 Claude / Anthropic

可能输出两种形态：

#### A. 旧式 / budget 模式

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

#### B. adaptive 模式

```json
{
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "low|medium|high|max"
  }
}
```

其中：

- `xhigh` -> Claude `output_config.effort=max`
- `high` -> `high`
- `medium` -> `medium`
- `low` / `minimal` / `none` 的 adaptive 输出强度会落到 `low`

精确输出映射：

| 统一 effort | 旧式 / budget Claude 输出 | adaptive Claude 输出 |
| --- | --- | --- |
| `none` | `thinking.type=disabled` | `thinking.type=disabled`；对 adaptive-only / Mythos 模型可能被移除 |
| `minimal` | `thinking.type=enabled`, `budget_tokens=1024` | `thinking.type=adaptive`, `output_config.effort=low` |
| `low` | `thinking.type=enabled`, `budget_tokens=2048` | `thinking.type=adaptive`, `output_config.effort=low` |
| `medium` | `thinking.type=enabled`, `budget_tokens=8192` | `thinking.type=adaptive`, `output_config.effort=medium` |
| `high` | `thinking.type=enabled`, `budget_tokens=16384` | `thinking.type=adaptive`, `output_config.effort=high` |
| `xhigh` | `thinking.type=enabled`, `budget_tokens=32768` | `thinking.type=adaptive`, `output_config.effort=max` |

budget 模式下的约束：

- 最小 `budget_tokens=1024`
- 如果显式 budget 小于 `1024`，会被提升到 `1024`
- 如果同时存在 `max_tokens`，会保证：
  - `max_tokens >= max(budget_tokens + 1, 2048)`
  - `budget_tokens < max_tokens`
- 如果 budget 不合法，会自动调整成上游可接受的值

adaptive 能力判断：

- 旧模型继续使用 `enabled + budget_tokens`
- 支持 adaptive 的模型会写成 `thinking.type=adaptive + output_config.effort`
- 在 Claude 系列中，模型能力判断会优先看 `OriginModel`，未命中时再看 `ActualModel`

### 4.5 写成 Ali DashScope 兼容格式

输出字段：

```json
{
  "enable_thinking": true,
  "thinking_budget": 2048
}
```

规则：

- `none` -> `enable_thinking=false`，并移除 `thinking_budget`
- 开启推理 -> `enable_thinking=true`
- 若模型支持 budget，再写 `thinking_budget`

精确映射：

| 统一 effort | 支持 `thinking_budget` 的 Ali 模型输出 | 不支持 budget 的 Ali 模型输出 |
| --- | --- | --- |
| `none` | `enable_thinking=false`；无 `thinking_budget` | `enable_thinking=false`；无 `thinking_budget` |
| `minimal` | `enable_thinking=true`, `thinking_budget=1024` | `enable_thinking=true`；无 `thinking_budget` |
| `low` | `enable_thinking=true`, `thinking_budget=2048` | `enable_thinking=true`；无 `thinking_budget` |
| `medium` | `enable_thinking=true`, `thinking_budget=8192` | `enable_thinking=true`；无 `thinking_budget` |
| `high` | `enable_thinking=true`, `thinking_budget=16384` | `enable_thinking=true`；无 `thinking_budget` |
| `xhigh` | `enable_thinking=true`, `thinking_budget=32768` | `enable_thinking=true`；无 `thinking_budget` |

当前认为支持 `thinking_budget` 的模型包括：

- `qwen3-*`
- `qwq-*`
- 包含 `glm`
- 包含 `kimi`

Ali 特殊规则：

- **不会**按 `max_tokens` 夹紧 `thinking_budget`
- `qwen3-*`：非流式请求会强制 `enable_thinking=false`
- `qwq-*`：会强制 `stream=true`

### 4.6 写成 Zhipu / DeepSeek / Doubao 的 thinking 对象

输出字段统一为：

```json
{
  "thinking": {
    "type": "enabled|disabled"
  }
}
```

规则：

- `none` -> `thinking.type=disabled`
- 其余开启态 -> `thinking.type=enabled`
- 这几个上游当前**不保留 budget 细节**，只保留“开 / 关”语义

这意味着：

- `minimal` / `low` / `medium` / `high` / `xhigh`
- 最终都会降级成同一个“enabled”状态

精确映射：

| 统一 effort | Zhipu / DeepSeek / Doubao 输出 |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

### 4.7 写成 Moonshot / Kimi

Moonshot / Kimi 只会对支持 thinking 开关的上游模型写 Kimi `thinking` 对象：

```json
{
  "thinking": {
    "type": "enabled|disabled"
  }
}
```

支持开关的 Kimi 模型精确映射：

| 统一 effort | Kimi 输出 |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

对于不支持开关的 Kimi 模型，adaptor 会移除 `reasoning_effort`，并且不发送
`thinking`。Kimi 输出当前只保留开关语义，不保留 budget / 细粒度 effort。

---

## 5. 厂商 / 适配器支持矩阵

下面只列出当前已经接入 thinking / reasoning 兼容层的主要适配器。
表格描述的是：请求先被解析成统一 reasoning 结构后，各个 adaptor 会写成什么上游字段。

## 5.1 OpenAI / Azure / OpenAI 兼容上游

原生字段：

| 模式 | 原生输入字段 | 写给上游的字段 | effort 精确映射 |
| --- | --- | --- | --- |
| Chat Completions | `reasoning_effort` | `reasoning_effort` | 统一 effort 原样写出 |
| Completions | `reasoning_effort` | `reasoning_effort` | 统一 effort 原样写出 |
| Responses | `reasoning.effort` | `reasoning.effort` | 统一 effort 原样写出 |

跨协议转换：

| 输入请求模式 | 目标 OpenAI 模式 | 转换方式 |
| --- | --- | --- |
| Gemini -> Chat / Completions | OpenAI-compatible chat payload | `generationConfig.thinkingConfig` -> 统一 effort -> `reasoning_effort` |
| Claude / Anthropic -> Chat / Completions | OpenAI-compatible chat payload | `thinking` / `output_config` -> 统一 effort -> `reasoning_effort` |
| OpenAI Chat -> Responses | OpenAI Responses payload | `reasoning_effort` -> 统一 effort -> `reasoning.effort` |
| Gemini -> Responses | OpenAI Responses payload | `thinkingConfig` -> 统一 effort -> `reasoning.effort` |
| Claude / Anthropic -> Responses | OpenAI Responses payload | `thinking` / `output_config` -> OpenAI chat request -> `reasoning.effort` |

说明：

- OpenAI Chat / Completions 模式只解析 `reasoning_effort`。
- OpenAI Chat / Completions 不解析 Gemini 风格 `thinkingConfig`、Claude 风格 `thinking`、Ali `enable_thinking` 或 Ali `thinking_budget`。

## 5.2 Google Gemini

支持 reasoning 转换的模式：

| 输入请求模式 | Gemini adaptor 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | `generationConfig.thinkingConfig` |
| Claude / Anthropic | 通过 Claude converter 解析 `thinking` / `output_config` | `generationConfig.thinkingConfig` |
| Gemini native | 使用原生 `generationConfig.thinkingConfig` | `generationConfig.thinkingConfig` |
| OpenAI Responses | 不是 Gemini adaptor 的输入模式 | N/A |
| OpenAI Completions | 不是 Gemini adaptor 的输入模式 | N/A |

具体输出取决于目标 Gemini 模型系列：

| 统一 effort | Gemini 3+ Pro | Gemini 3+ 非 Pro | `gemini-2.5-pro` | `gemini-2.5-flash` | `gemini-2.5-flash-lite` |
| --- | --- | --- | --- | --- | --- |
| `none` | `thinkingLevel=low` | `thinkingLevel=minimal` | `thinkingBudget=128`, `includeThoughts=false` | `thinkingBudget=0`, `includeThoughts=false` | `thinkingBudget=0`, `includeThoughts=false` |
| `minimal` | `thinkingLevel=low` | `thinkingLevel=minimal` | `thinkingBudget=1024`, `includeThoughts=true` | `thinkingBudget=1024`, `includeThoughts=true` | `thinkingBudget=1024`, `includeThoughts=true` |
| `low` | `thinkingLevel=low` | `thinkingLevel=low` | `thinkingBudget=2048`, `includeThoughts=true` | `thinkingBudget=2048`, `includeThoughts=true` | `thinkingBudget=2048`, `includeThoughts=true` |
| `medium` | `thinkingLevel=low` | `thinkingLevel=medium` | `thinkingBudget=8192`, `includeThoughts=true` | `thinkingBudget=8192`, `includeThoughts=true` | `thinkingBudget=8192`, `includeThoughts=true` |
| `high` | `thinkingLevel=high` | `thinkingLevel=high` | `thinkingBudget=16384`, `includeThoughts=true` | `thinkingBudget=16384`, `includeThoughts=true` | `thinkingBudget=16384`, `includeThoughts=true` |
| `xhigh` | `thinkingLevel=high` | `thinkingLevel=high` | `thinkingBudget=32768`, `includeThoughts=true` | `thinkingBudget=24576`, `includeThoughts=true` | `thinkingBudget=24576`, `includeThoughts=true` |

说明：

- 这个精确映射表适用于 adaptor 从 OpenAI Chat 或 Claude / Anthropic 转成 Gemini 的场景。
- Gemini native 请求会保留自己的 `generationConfig.thinkingConfig`；adaptor 不会把一种 Gemini thinking 方言再改写成另一种 Gemini thinking 方言。
- Gemini 2.5 系列按 budget 输出，并做模型范围约束。
- Gemini 3 / 4 / 5 系列按 `thinkingLevel` 输出。
- 某些模型不能真正关闭 thinking，`none` 会退化为最小允许 level 或 budget。

## 5.3 Anthropic 官方

支持 reasoning 转换的模式：

| 输入请求模式 | Anthropic adaptor 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | `thinking`，必要时带 `output_config` |
| Gemini native | 解析 `generationConfig.thinkingConfig` | `thinking`，必要时带 `output_config` |
| Anthropic native | 保留 Anthropic 原生字段 | 按输入保留 `thinking` / `output_config`，并做原生清理 |
| OpenAI Responses | 不是 Anthropic adaptor 的输入模式 | N/A |
| OpenAI Completions | 不是 Anthropic adaptor 的输入模式 | N/A |

按 Claude 能力分支的精确输出：

| 统一 effort | 旧式 / budget Claude 输出 | adaptive Claude 输出 |
| --- | --- | --- |
| `none` | `thinking.type=disabled` | `thinking.type=disabled`；在 adaptive-only / Mythos 模型上可能会被移除 |
| `minimal` | `thinking.type=enabled`, `budget_tokens=1024` | `thinking.type=adaptive`, `output_config.effort=low` |
| `low` | `thinking.type=enabled`, `budget_tokens=2048` | `thinking.type=adaptive`, `output_config.effort=low` |
| `medium` | `thinking.type=enabled`, `budget_tokens=8192` | `thinking.type=adaptive`, `output_config.effort=medium` |
| `high` | `thinking.type=enabled`, `budget_tokens=16384` | `thinking.type=adaptive`, `output_config.effort=high` |
| `xhigh` | `thinking.type=enabled`, `budget_tokens=32768` | `thinking.type=adaptive`, `output_config.effort=max` |

说明：

- 这个精确映射表适用于 adaptor 从 OpenAI Chat 或 Gemini 转成 Claude 的场景。
- Anthropic native 请求会保留原生 `thinking` / `output_config` 字段，只做 adaptor 需要的原生清理。
- budget 模式会保证 `budget_tokens < max_tokens`。
- 旧模型使用 `enabled + budget_tokens`。
- 支持 adaptive 的模型使用 `adaptive + output_config.effort`。

## 5.4 AWS Bedrock Claude

支持 reasoning 转换的模式：

| 输入请求模式 | Bedrock Claude 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | Bedrock 包装后的 Claude `thinking` |
| Gemini native | 解析 `generationConfig.thinkingConfig` | Bedrock 包装后的 Claude `thinking` |
| Anthropic native | 保留 Anthropic 原生字段 | Bedrock 包装后的 Claude `thinking` |
| OpenAI Responses | 不是 Bedrock Claude 的输入模式 | N/A |
| OpenAI Completions | 不是 Bedrock Claude 的输入模式 | N/A |

effort 映射与 Anthropic 官方一致：旧模型使用 `enabled + budget_tokens`，支持 adaptive 的模型使用 `adaptive + output_config.effort`。
之后再包装成 Bedrock runtime 请求。

## 5.5 Vertex AI Claude

支持 reasoning 转换的模式：

| 输入请求模式 | Vertex Claude 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | Vertex 包装后的 Claude `thinking` |
| Gemini native | 解析 `generationConfig.thinkingConfig` | Vertex 包装后的 Claude `thinking` |
| Anthropic native | 保留 Anthropic 原生字段 | Vertex 包装后的 Claude `thinking` |
| OpenAI Responses | 不是 Vertex Claude 的输入模式 | N/A |
| OpenAI Completions | 不是 Vertex Claude 的输入模式 | N/A |

effort 映射与 Anthropic 官方一致：旧模型使用 `enabled + budget_tokens`，支持 adaptive 的模型使用 `adaptive + output_config.effort`。
之后再包装成 Vertex AI 请求。

## 5.6 Ali DashScope

支持 reasoning 转换的模式：

| 输入请求模式 | Ali adaptor 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | `enable_thinking`；可选 `thinking_budget` |
| OpenAI Completions | 解析 `reasoning_effort` | `enable_thinking`；可选 `thinking_budget` |
| Gemini native | 先通过 OpenAI-compatible 转换解析 `generationConfig.thinkingConfig` | `enable_thinking`；可选 `thinking_budget` |
| Anthropic native | 走 Ali Claude Code Proxy 原生请求格式 | 不做 thinking 方言迁移 |
| OpenAI Responses | 走 OpenAI-compatible Responses 转换 | 没有 Ali 专用 reasoning hook |

Ali 各档 effort 的精确映射：

| 统一 effort | 支持 budget 的 Ali 模型输出 | 不支持 budget 的 Ali 模型输出 |
| --- | --- | --- |
| `none` | `enable_thinking=false`；移除 `thinking_budget` | `enable_thinking=false`；移除 `thinking_budget` |
| `minimal` | `enable_thinking=true`；`thinking_budget=1024` | `enable_thinking=true`；无 `thinking_budget` |
| `low` | `enable_thinking=true`；`thinking_budget=2048` | `enable_thinking=true`；无 `thinking_budget` |
| `medium` | `enable_thinking=true`；`thinking_budget=8192` | `enable_thinking=true`；无 `thinking_budget` |
| `high` | `enable_thinking=true`；`thinking_budget=16384` | `enable_thinking=true`；无 `thinking_budget` |
| `xhigh` | `enable_thinking=true`；`thinking_budget=32768` | `enable_thinking=true`；无 `thinking_budget` |

Ali 支持 `thinking_budget` 的模型判断：

| 模型规则 | 是否写 `thinking_budget` |
| --- | --- |
| 模型名以 `qwen3-` 开头 | 是 |
| 模型名以 `qwq-` 开头 | 是 |
| 模型名包含 `glm` | 是 |
| 模型名包含 `kimi` | 是 |
| 其他 Ali-compatible 模型 | 否，只写 `enable_thinking` |

Ali 特殊规则：

- `thinking_budget` 不会按 `max_tokens` 夹紧。
- `qwen3-*` 非流式 Chat / Completions 请求会在 reasoning hook 后强制 `enable_thinking=false`。
- `qwen3-*` 非流式 Gemini-mode 请求会强制 `enable_thinking=false`，并移除 `thinking_budget`。
- `qwq-*` 请求会强制 `stream=true`。

## 5.7 Doubao

支持 reasoning 转换的模式：

| 输入请求模式 | Doubao adaptor 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | `thinking.type` |
| Gemini native | 先通过 OpenAI-compatible 转换解析 `generationConfig.thinkingConfig` | `thinking.type` |
| Anthropic native | 先通过 OpenAI-compatible 转换解析 `thinking` / `output_config` | `thinking.type` |
| OpenAI Responses | 走 OpenAI-compatible Responses 原生转换 | 没有 Doubao 专用 reasoning hook |
| OpenAI Completions | Doubao adaptor 不支持 | N/A |

精确 effort 映射：

| 统一 effort | Doubao 输出 |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

说明：

- 只保留开关语义，不保留 budget / 细粒度 effort。
- `deepseek-reasoner` 额外注入系统提示，模型匹配同样遵循 origin-first, actual-fallback。

## 5.8 DeepSeek

支持 reasoning 转换的模式：

| 输入请求模式 | DeepSeek adaptor 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | `thinking.type` |
| Gemini native | 先通过 OpenAI-compatible 转换解析 `generationConfig.thinkingConfig` | `thinking.type` |
| Anthropic native | 走 DeepSeek `/anthropic/v1/messages` | 不做 thinking 方言迁移 |
| OpenAI Responses | DeepSeek adaptor 不支持 | N/A |
| OpenAI Completions | OpenAI-compatible 透传 | 没有 DeepSeek 专用 reasoning hook |

Chat / Gemini 两条 hook 路径的精确 effort 映射：

| 统一 effort | DeepSeek 输出 |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

DeepSeek 当前在 hooked 的 OpenAI Chat 与 Gemini 路径只保留 enabled / disabled。

## 5.9 Zhipu

支持 reasoning 转换的模式：

| 输入请求模式 | Zhipu adaptor 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | `thinking.type` |
| Gemini native | 先通过 OpenAI-compatible 转换解析 `generationConfig.thinkingConfig` | `thinking.type` |
| Anthropic native | 先通过 OpenAI-compatible 转换解析 `thinking` / `output_config` | `thinking.type` |
| OpenAI Responses | 当前 adaptor 不支持 | N/A |
| OpenAI Completions | OpenAI-compatible 透传 | 没有 Zhipu 专用 reasoning hook |

Chat / Gemini / Anthropic 三条 hook 路径的精确 effort 映射：

| 统一 effort | Zhipu 输出 |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

Zhipu 当前在 hooked 路径只保留 enabled / disabled。

## 5.10 Moonshot / Kimi

支持 reasoning 转换的模式：

| 输入请求模式 | Moonshot adaptor 行为 | 写给上游的字段 |
| --- | --- | --- |
| OpenAI Chat | 解析 `reasoning_effort` | 对支持开关的模型写 Kimi `thinking.type` |
| Gemini native | 先通过 OpenAI-compatible 转换解析 `generationConfig.thinkingConfig` | 对支持开关的模型写 Kimi `thinking.type` |
| Anthropic native | 先通过 OpenAI-compatible 转换解析 `thinking` / `output_config` | 对支持开关的模型写 Kimi `thinking.type` |
| OpenAI Completions | OpenAI-compatible payload 透传 | 没有 Moonshot 专用 reasoning hook |
| OpenAI Responses | Moonshot adaptor 不支持 | N/A |

Moonshot adaptor 当前把以下 actual upstream model name 视为支持 thinking 开关：

| actual upstream model 规则 | 是否支持写 Kimi `thinking.type` |
| --- | --- |
| `kimi-k2.5*` | 是 |
| `kimi-k2.6*` | 是 |
| 其他 Kimi 模型名 | 否；移除 `reasoning_effort`，并省略 `thinking` |

支持开关的 Kimi 模型精确映射：

| 统一 effort | Kimi 输出 |
| --- | --- |
| `none` | `thinking.type=disabled`；移除 `reasoning_effort` |
| `minimal` | `thinking.type=enabled`；移除 `reasoning_effort` |
| `low` | `thinking.type=enabled`；移除 `reasoning_effort` |
| `medium` | `thinking.type=enabled`；移除 `reasoning_effort` |
| `high` | `thinking.type=enabled`；移除 `reasoning_effort` |
| `xhigh` | `thinking.type=enabled`；移除 `reasoning_effort` |

对于不支持切换的 Kimi 模型，例如专用 thinking 模型，adaptor 会移除 `reasoning_effort` 并省略 `thinking`。
它不会向不能关闭 thinking 的模型发送 `thinking.type=disabled`。

说明：

- 只保留开关语义，不保留 budget / 细粒度 effort。
- 模型能力判断使用 `ActualModel`，因为渠道映射后的最终 Kimi 上游模型名决定 `thinking` 字段是否合法。

---

## 6. 模型名匹配策略

大多数“按模型能力分支”的逻辑，都遵循统一策略：

1. 先使用 `OriginModel`
2. 如果 `OriginModel` 没命中规则，再使用 `ActualModel`

这样做的原因是：

- 用户侧可能传的是更有业务含义的原始模型名
- 渠道映射后 `ActualModel` 可能是上游真实模型名
- 某些能力判断只在其中一个名字上才能命中

这个策略已经用于：

- Claude adaptive 能力判断
- Gemini thinking level / budget 路径判断
- Ali budget 能力判断
- Doubao bot / vision / deepseek-reasoner 特殊逻辑
- 其他基于模型名的 thinking 能力分支

例外：

- Moonshot / Kimi thinking 开关能力判断优先使用 `ActualModel`，因为
  `thinking` 字段是否合法取决于渠道映射后的最终 Kimi 上游模型。

---

## 7. 完整转换示例

这一节会尽量覆盖当前代码里所有已经实现的 reasoning / thinking 转换路径。

## 7.1 以 OpenAI Chat / Completions 作为输入格式

### 7.1.1 OpenAI Chat -> OpenAI Responses

输入：

```json
{
  "model": "gpt-4o",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "model": "gpt-4o",
  "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "hello"}]}],
  "reasoning": {
    "effort": "high"
  }
}
```

说明：

- 当 Chat 被转换成 Responses 时，都会写成 `reasoning.effort`
- Azure 在走 Responses 路由时，本质上也是同样的参数形态

### 7.1.2 OpenAI Chat -> Gemini 2.5 Pro

输入：

```json
{
  "model": "gemini-2.5-pro",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 16384,
      "includeThoughts": true
    }
  }
}
```

### 7.1.3 OpenAI Chat -> Gemini 2.5 Flash，显式关闭 thinking

输入：

```json
{
  "model": "gemini-2.5-flash",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 0,
      "includeThoughts": false
    }
  }
}
```

### 7.1.4 OpenAI Chat -> Gemini 3 Pro，显式关闭 thinking

输入：

```json
{
  "model": "gemini-3-pro",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingLevel": "low",
      "includeThoughts": false
    }
  }
}
```

说明：

- 这里不会强写非法关闭态
- 会退化到该模型允许的最小 thinking level

### 7.1.5 OpenAI Chat -> Anthropic Claude Sonnet 4.5

输入：

```json
{
  "model": "claude-sonnet-4-5",
  "reasoning_effort": "low",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

### 7.1.6 OpenAI Chat -> Anthropic Claude 3.7 Sonnet

输入：

```json
{
  "model": "claude-3-7-sonnet-20250219",
  "reasoning_effort": "medium",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 8192
  }
}
```

### 7.1.7 OpenAI Chat -> Anthropic Claude Opus 4.7

输入：

```json
{
  "model": "claude-opus-4-7",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "high"
  }
}
```

### 7.1.8 OpenAI Chat -> AWS Bedrock Claude

输入：

```json
{
  "model": "claude-opus-4-7",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

代表性输出体：

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "high"
  }
}
```

说明：

- AWS 会在 Claude 请求外再包一层 Bedrock 字段
- 内层 thinking 结构仍然遵循 Claude 的转换规则

### 7.1.9 OpenAI Chat -> Vertex AI Claude

输入：

```json
{
  "model": "claude-sonnet-4-5",
  "reasoning_effort": "low",
  "messages": [{"role": "user", "content": "hello"}]
}
```

代表性输出体：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

说明：

- Vertex AI 的传输路径会走 `rawPredict` / `streamRawPredict`
- 但 body 内部仍然是 Claude 的 thinking 结构

### 7.1.10 OpenAI Chat -> Ali 兼容 Chat

输入：

```json
{
  "model": "glm-4.5",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "enable_thinking": true,
  "thinking_budget": 16384
}
```

### 7.1.11 OpenAI Chat -> Ali `qwen3-*` 非流式请求

输入：

```json
{
  "model": "qwen3-32b",
  "reasoning_effort": "high",
  "stream": false,
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "enable_thinking": false,
  "thinking_budget": 16384
}
```

说明：

- `qwen3-*` 的补丁会把非流式请求强制改成 `enable_thinking=false`
- 这里只覆盖开关位，预算字段仍可能保留在转换结果里

### 7.1.12 OpenAI Chat -> Ali `qwq-*`

输入：

```json
{
  "model": "qwq-plus",
  "reasoning_effort": "low",
  "stream": false,
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "enable_thinking": true,
  "thinking_budget": 2048,
  "stream": true
}
```

### 7.1.13 OpenAI Chat -> Zhipu

输入：

```json
{
  "model": "glm-5.1",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "disabled"
  }
}
```

### 7.1.14 OpenAI Chat -> DeepSeek

输入：

```json
{
  "model": "deepseek-chat",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "enabled"
  }
}
```

### 7.1.15 OpenAI Chat -> Doubao

输入：

```json
{
  "model": "doubao-seed-1-6",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "disabled"
  }
}
```

### 7.1.16 OpenAI Chat -> Doubao，模型为 `deepseek-reasoner`

输入：

```json
{
  "model": "deepseek-reasoner",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "messages": [
    {
      "role": "system",
      "content": "回答前，都先用 <think></think> 输出你的思考过程。"
    },
    {
      "role": "user",
      "content": "hello"
    }
  ]
}
```

说明：

- 这不是 effort 到 effort 的字段转换
- 但它是当前 Doubao adaptor 中与 reasoning 相关的特殊兼容逻辑

### 7.1.17 OpenAI Chat -> Moonshot / Kimi K2.6

输入：

```json
{
  "model": "kimi-k2.6",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "disabled"
  }
}
```

说明：

- 转换后移除 `reasoning_effort`
- 开启态 effort 会写成 `thinking.type=enabled`
- 不保留 budget / 细粒度 effort 语义

### 7.1.18 OpenAI Chat -> Moonshot / Kimi 不支持开关的模型

输入：

```json
{
  "model": "kimi-k2-thinking",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "messages": [{"role": "user", "content": "hello"}]
}
```

说明：

- 转换后移除 `reasoning_effort`
- 因为该模型族不支持通过请求参数切换 thinking，所以省略 `thinking`

### 7.1.19 OpenAI Completions -> Ali

输入：

```json
{
  "model": "glm-4.5",
  "reasoning_effort": "low",
  "prompt": "hello"
}
```

输出：

```json
{
  "enable_thinking": true,
  "thinking_budget": 2048
}
```

## 7.2 以 Gemini Native Request 作为输入格式

### 7.2.1 Gemini -> OpenAI Chat / Completions

输入：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true
    }
  },
  "contents": [{"role": "user", "parts": [{"text": "hello"}]}]
}
```

输出：

```json
{
  "reasoning_effort": "low",
  "messages": [{"role": "user", "content": "hello"}]
}
```

### 7.2.2 Gemini -> OpenAI Responses

输入：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingLevel": "high"
    }
  },
  "contents": [{"role": "user", "parts": [{"text": "hello"}]}]
}
```

输出：

```json
{
  "reasoning": {
    "effort": "high"
  }
}
```

### 7.2.3 Gemini -> Anthropic 官方

输入：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true
    }
  },
  "contents": [{"role": "user", "parts": [{"text": "hello"}]}]
}
```

输出（旧 Claude 模型）：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

### 7.2.4 Gemini -> Anthropic Adaptive Claude

输入：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true
    }
  },
  "contents": [{"role": "user", "parts": [{"text": "hello"}]}]
}
```

输出（`claude-opus-4-7`）：

```json
{
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "low"
  }
}
```

### 7.2.5 Gemini -> AWS Bedrock Claude

代表性输出体：

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

### 7.2.6 Gemini -> Vertex AI Claude

代表性输出体：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

### 7.2.7 Gemini -> Ali

输入：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true
    }
  },
  "contents": [{"role": "user", "parts": [{"text": "hello"}]}]
}
```

输出：

```json
{
  "enable_thinking": true,
  "thinking_budget": 2048
}
```

### 7.2.8 Gemini -> Zhipu / DeepSeek / Doubao

输入：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true
    }
  }
}
```

输出：

```json
{
  "thinking": {
    "type": "enabled"
  }
}
```

说明：

- budget 细节不会保留
- 会降级成纯开关语义

### 7.2.9 Gemini -> Moonshot / Kimi K2.6

输入：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true
    }
  }
}
```

输出：

```json
{
  "thinking": {
    "type": "enabled"
  }
}
```

说明：

- Gemini `thinkingConfig` 会先通过 OpenAI-compatible `reasoning_effort`
  路径归一化，然后由 Moonshot hook 写成 Kimi `thinking`
- budget 细节不会保留

## 7.3 以 Claude / Anthropic Request 作为输入格式

### 7.3.1 Claude -> OpenAI Chat / Completions

输入：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "reasoning_effort": "low",
  "messages": [{"role": "user", "content": "hello"}]
}
```

### 7.3.2 Claude Adaptive -> OpenAI Chat / Completions

输入：

```json
{
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "high"
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "reasoning_effort": "high"
}
```

### 7.3.3 Claude -> OpenAI Responses

输入：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "reasoning": {
    "effort": "low"
  }
}
```

### 7.3.4 Claude -> Gemini

输入：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 16384
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出（`gemini-2.5-pro`）：

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 16384,
      "includeThoughts": true
    }
  }
}
```

### 7.3.5 Native Anthropic -> Anthropic 官方

输入：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

说明：

- 该路径会保留原生 Claude thinking 字段
- 不会迁移成别的 thinking 方言

### 7.3.6 Native Anthropic -> AWS / Vertex Claude 包装层

输入：

```json
{
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "low"
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

代表性 AWS 包装：

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "low"
  }
}
```

代表性 Vertex body：

```json
{
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "low"
  }
}
```

### 7.3.7 Claude -> Moonshot / Kimi K2.6

输入：

```json
{
  "thinking": {
    "type": "disabled"
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

输出：

```json
{
  "thinking": {
    "type": "disabled"
  }
}
```

说明：

- Claude `thinking` 会先通过 OpenAI-compatible `reasoning_effort` 路径归一化，
  然后由 Moonshot hook 写成 Kimi `thinking`
- Kimi 目标不保留 budget / adaptive effort 细节

---

## 8. 当前不做的事情

当前功能**不负责**以下事项：

- 不解析当前请求模式之外的 thinking 方言
  - 例如 OpenAI Chat 请求中不会解析 Gemini `thinkingConfig`
  - 例如 Gemini 请求中不会解析 Claude `thinking`
- 不对所有 native 请求做 thinking 方言迁移
- 不保证每个厂商都能完整保留 budget / effort 细节
  - 尤其是 Zhipu / Doubao / DeepSeek 当前只保留 enabled / disabled
- 不为未接入 reasoning hook 的适配器自动增加推理兼容能力

---

## 9. 维护建议

如果后续要新增某个厂商或某种请求格式的 thinking 兼容，建议遵循下面的流程：

1. 先定义该请求模式的**原生解析入口**
2. 归一化为统一的 `NormalizedReasoning`
3. 按上游实际支持的字段写回
4. 只在“转换后的请求体”做约束与合法化
5. 为模型能力判断明确记录模型名匹配顺序；默认使用 origin-first /
   actual-fallback，但当最终上游模型名决定参数合法性时，使用 `ActualModel`
6. 为以下情况补测试：
   - 显式关闭
   - 老模型 / 新模型差异
   - budget 上下限
   - `max_tokens` / `maxOutputTokens` 相关限制
   - `OriginModel` 命中、`ActualModel` 回退命中
