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

---

## 5. 厂商 / 适配器支持矩阵

下面只列出当前已经接入 thinking / reasoning 兼容层的主要适配器。

## 5.1 OpenAI / Azure / OpenAI 兼容上游

### 原生支持

- Chat / Completions: `reasoning_effort`
- Responses: `reasoning.effort`

### 转换支持

- Gemini 请求 -> OpenAI `reasoning_effort`
- Claude 请求 -> OpenAI `reasoning_effort`
- Chat / Claude / Gemini -> Responses `reasoning.effort`

### 说明

- OpenAI Chat / Completions 模式当前只解析 `reasoning_effort`
- 不解析 Gemini / Claude 风格 thinking 参数

## 5.2 Google Gemini

### 输入模式

- OpenAI Chat -> Gemini thinkingConfig
- Claude -> Gemini thinkingConfig
- Gemini native -> 原样使用 native 字段

### 输出字段

- `generationConfig.thinkingConfig`

### 限制

- 2.5 系列按 budget 处理，并做范围约束
- 3 / 4 / 5 系列按 `thinkingLevel` 处理
- 某些模型不能真正关闭 thinking，`none` 会退化为最小允许等级

## 5.3 Anthropic 官方

### 输入模式

- OpenAI Chat -> Claude thinking
- Gemini -> Claude thinking
- Anthropic native -> 保持 native thinking 字段

### 输出字段

- `thinking`
- `output_config`

### 限制

- budget 模式下会自动保证 `budget_tokens < max_tokens`
- 旧模型使用 `enabled + budget_tokens`
- 支持 adaptive 的模型使用 `adaptive + output_config.effort`

## 5.4 AWS Bedrock Claude

### 输入模式

- OpenAI Chat -> Claude thinking
- Gemini -> Claude thinking
- Anthropic native -> 保持 native thinking 字段

### 限制

- 继承 Claude 系列的能力判断与 budget 约束
- native Anthropic 请求不会被自动迁移成另一种 thinking 方言

## 5.5 Vertex AI Claude

### 输入模式

- OpenAI Chat -> Claude thinking
- Gemini -> Claude thinking
- Anthropic native -> 保持 native thinking 字段

### 限制

- 继承 Claude 系列的能力判断与 budget 约束

## 5.6 Ali DashScope

### 输入模式

- OpenAI Chat -> `enable_thinking` / `thinking_budget`
- OpenAI Completions -> `enable_thinking` / `thinking_budget`
- Gemini -> `enable_thinking` / `thinking_budget`
- Anthropic native -> 走 Ali 的 Claude Code Proxy 原生请求格式，不做 thinking 方言迁移

### 限制

- 预算只在部分模型上写出
- `qwen3-*` 非流式请求强制关闭思考
- `qwq-*` 强制流式

## 5.7 Doubao

### 输入模式

- OpenAI Chat -> `thinking.type`
- Gemini -> `thinking.type`
- Anthropic -> `thinking.type`

### 输出字段

- `thinking.type=enabled|disabled`

### 限制

- 只保留开关语义，不保留 budget / effort 细节
- `deepseek-reasoner` 额外注入系统提示，模型匹配同样遵循 origin-first, actual-fallback

## 5.8 DeepSeek

### 输入模式

- OpenAI Chat -> `thinking.type`
- Gemini -> `thinking.type`
- Anthropic native -> 走 DeepSeek `/anthropic/v1/messages`，不做 thinking 方言迁移

### 限制

- 当前只保留 enabled / disabled 两态
- Completions 当前不做 reasoning 兼容转换

## 5.9 Zhipu

### 输入模式

- OpenAI Chat -> `thinking.type`
- Gemini -> `thinking.type`
- Anthropic -> `thinking.type`

### 输出字段

- `thinking.type=enabled|disabled`

### 限制

- 只保留开关语义，不保留 budget / effort 细节
- Completions 当前不做 reasoning 兼容转换

---

## 6. 模型名匹配策略

所有“按模型能力分支”的逻辑，都遵循统一策略：

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

### 7.1.17 OpenAI Completions -> Ali

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
5. 所有模型能力分支都使用 origin-first, actual-fallback
6. 为以下情况补测试：
   - 显式关闭
   - 老模型 / 新模型差异
   - budget 上下限
   - `max_tokens` / `maxOutputTokens` 相关限制
   - `OriginModel` 命中、`ActualModel` 回退命中
