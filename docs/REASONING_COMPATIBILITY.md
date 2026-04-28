# Thinking / Reasoning Compatibility

This document describes the current thinking / reasoning compatibility layer in aiproxy, including:

- how different request protocols express reasoning parameters
- how the proxy normalizes those parameters internally
- how they are converted when forwarding to different upstream vendors
- known vendor- and model-specific limitations, fallbacks, and downgrade behavior

> This document focuses on parameter compatibility and conversion behavior, not the full upstream API surface of each vendor.

## 1. Goals

The purpose of this feature is to:

1. let callers use the **native reasoning format of the current request mode** whenever possible
2. automatically convert reasoning parameters when transforming from request format A to upstream format B
3. reduce upstream validation errors caused by model capability differences or schema mismatches

The implementation follows these rules:

- it only parses the **native thinking / reasoning fields of the current request mode**
  - OpenAI Chat / Completions only parse `reasoning_effort`
  - OpenAI Responses as a target format only writes `reasoning.effort`
  - Gemini only parses `generationConfig.thinkingConfig`
  - Claude / Anthropic only parse `thinking` and `output_config`
- it no longer supports reverse compatibility for the old generic `thinking` structure
- only **converted request bodies** are normalized or constrained for upstream validity
- native requests are not automatically migrated into another thinking dialect
  - for example, a native Claude request is not rewritten into OpenAI `reasoning_effort`
  - existing protocol-level cleanup may still apply, for example when an upstream forbids `temperature` together with thinking
- every **model-name-based capability branch** uses:
  1. `OriginModel` first
  2. `ActualModel` as fallback when origin does not match

---

## 2. Internal normalization model

Internally, the proxy first normalizes different protocol-specific reasoning parameters into one unified structure. Conceptually it contains:

- `Specified`: whether reasoning was explicitly configured
- `Disabled`: whether reasoning was explicitly turned off
- `Effort`: normalized effort level
- `BudgetTokens`: token budget if the original protocol provided one

### 2.1 Supported normalized effort values

The normalized effort enum is:

- `none`
- `minimal`
- `low`
- `medium`
- `high`
- `xhigh`

The parser also accepts several aliases:

- `off` / `disabled` -> `none`
- `med` -> `medium`
- `max` / `maximum` -> `xhigh`

### 2.2 Default effort <-> budget mapping

When an upstream only supports token budgets instead of discrete labels such as `high` or `medium`, the proxy uses this mapping:

| effort | budget |
| --- | ---: |
| `none` | `0` |
| `minimal` | `1024` |
| `low` | `2048` |
| `medium` | `8192` |
| `high` | `16384` |
| `xhigh` | `32768` |

When converting budget back into effort, the proxy uses these ranges:

| budget range | normalized effort |
| --- | --- |
| `<= 0` | `none` |
| `1 ~ 1024` | `minimal` |
| `1025 ~ 4096` | `low` |
| `4097 ~ 12288` | `medium` |
| `12289 ~ 24576` | `high` |
| `> 24576` | `xhigh` |

---

## 3. Supported input formats by request mode

### 3.1 OpenAI Chat / Completions

The compatibility layer currently only parses:

```json
{
  "reasoning_effort": "none|minimal|low|medium|high|xhigh"
}
```

Notes:

- this is the only reasoning field currently consumed in OpenAI Chat / Completions mode
- the old generic `thinking` structure is no longer parsed here

### 3.2 OpenAI Responses

When the proxy needs to build an OpenAI Responses request body, reasoning is written as:

```json
{
  "reasoning": {
    "effort": "none|minimal|low|medium|high|xhigh"
  }
}
```

Notes:

- in the current implementation, Responses is mostly used as a **target format**
- for example, Chat / Claude / Gemini requests converted into Responses will write `reasoning.effort`

### 3.3 Gemini

The proxy currently parses:

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

Parsing priority:

1. `thinkingLevel`
2. `thinkingBudget`
3. `includeThoughts`

Interpretation:

- `thinkingLevel` maps directly to normalized effort
- `thinkingBudget` is converted into effort through the budget ranges
- `includeThoughts=true` with no other recognized fields is treated as `medium`
- `thinkingBudget<=0` is treated as `none`
- if `thinkingConfig` is explicitly present but contains no recognized reasoning fields, it is treated as disabled

### 3.4 Claude / Anthropic

The proxy currently parses:

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

Rules:

- `thinking.type=disabled` -> `none`
- `thinking.type=enabled` or `adaptive` -> reasoning enabled
- if `budget_tokens` is provided, the budget is preserved in normalized form
- if `output_config.effort` is provided, it takes precedence for effort selection
- if only `thinking.type=enabled` is present without budget or effort, the normalized default is `medium`

---

## 4. How normalized reasoning is written to each target format

### 4.1 OpenAI Chat / Completions output

Output field:

```json
{
  "reasoning_effort": "..."
}
```

Typical use cases:

- Gemini -> OpenAI
- Claude -> OpenAI
- any other request normalized first, then emitted as OpenAI-compatible reasoning

Effort mapping:

| normalized effort | OpenAI Chat / Completions field |
| --- | --- |
| `none` | `reasoning_effort: "none"` |
| `minimal` | `reasoning_effort: "minimal"` |
| `low` | `reasoning_effort: "low"` |
| `medium` | `reasoning_effort: "medium"` |
| `high` | `reasoning_effort: "high"` |
| `xhigh` | `reasoning_effort: "xhigh"` |

### 4.2 OpenAI Responses output

Output field:

```json
{
  "reasoning": {
    "effort": "..."
  }
}
```

Typical use cases:

- Chat -> Responses
- Claude -> Responses
- Gemini -> Responses

Effort mapping:

| normalized effort | OpenAI Responses field |
| --- | --- |
| `none` | `reasoning.effort: "none"` |
| `minimal` | `reasoning.effort: "minimal"` |
| `low` | `reasoning.effort: "low"` |
| `medium` | `reasoning.effort: "medium"` |
| `high` | `reasoning.effort: "high"` |
| `xhigh` | `reasoning.effort: "xhigh"` |

### 4.3 Gemini output

Output location:

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

There are two main branches.

#### A. Gemini 3 / 4 / 5 families: use `thinkingLevel`

If the model name matches one of these families, the proxy prefers `thinkingLevel`:

- `gemini-3*`
- `gemini-4*`
- `gemini-5*`

Mapping rules:

- Pro models:
  - `high` / `xhigh` -> `high`
  - all other enabled states -> `low`
- non-Pro models:
  - `none` -> `minimal`
  - `low` -> `low`
  - `medium` -> `medium`
  - `high` / `xhigh` -> `high`
  - everything else -> `minimal`

Disable behavior:

- these models generally do not use `thinkingBudget=0` as the disable path
- when the caller explicitly sends `none`, the proxy degrades to the minimum valid level for that model instead of forcing an invalid disable payload

Exact level mapping:

| normalized effort | Gemini 3+ Pro `thinkingLevel` | Gemini 3+ non-Pro `thinkingLevel` |
| --- | --- | --- |
| `none` | `low` | `minimal` |
| `minimal` | `low` | `minimal` |
| `low` | `low` | `low` |
| `medium` | `low` | `medium` |
| `high` | `high` | `high` |
| `xhigh` | `high` | `high` |

#### B. Gemini 2.5 family: use `thinkingBudget`

Model limits:

| model | budget range | disable supported |
| --- | --- | --- |
| `gemini-2.5-pro` | `128 ~ 32768` | no |
| `gemini-2.5-flash` | `1 ~ 24576` | yes |
| `gemini-2.5-flash-lite` | `512 ~ 24576` | yes |

Write rules:

- when reasoning is enabled:
  - first derive a default budget from effort
  - then clamp it to the model-specific allowed range
- when reasoning is disabled:
  - models that support disabling receive `thinkingBudget=0`
  - models that do not support disabling receive the minimum allowed budget
- `includeThoughts`:
  - `true` when reasoning is enabled
  - `false` when reasoning is disabled

Important note:

- Gemini thinking budgets are **not** additionally clamped by `max_tokens` / `maxOutputTokens`
- this is intentional, to avoid incorrectly shrinking an otherwise valid Gemini reasoning configuration

Exact budget mapping after Gemini model-range clamping:

| normalized effort | `gemini-2.5-pro` | `gemini-2.5-flash` | `gemini-2.5-flash-lite` |
| --- | ---: | ---: | ---: |
| `none` | `128` | `0` | `0` |
| `minimal` | `1024` | `1024` | `1024` |
| `low` | `2048` | `2048` | `2048` |
| `medium` | `8192` | `8192` | `8192` |
| `high` | `16384` | `16384` | `16384` |
| `xhigh` | `32768` | `24576` | `24576` |

`includeThoughts` is `true` for enabled rows and `false` for the `none` row.

### 4.4 Claude / Anthropic output

The proxy may emit two shapes.

#### A. Legacy / budget mode

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

#### B. Adaptive mode

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

Mapping details:

- `xhigh` -> Claude `output_config.effort=max`
- `high` -> `high`
- `medium` -> `medium`
- `low`, `minimal`, and `none` map to `low` when adaptive output is used

Exact output mapping:

| normalized effort | legacy / budget Claude output | adaptive Claude output |
| --- | --- | --- |
| `none` | `thinking.type=disabled` | `thinking.type=disabled`; may be removed for adaptive-only / Mythos models |
| `minimal` | `thinking.type=enabled`, `budget_tokens=1024` | `thinking.type=adaptive`, `output_config.effort=low` |
| `low` | `thinking.type=enabled`, `budget_tokens=2048` | `thinking.type=adaptive`, `output_config.effort=low` |
| `medium` | `thinking.type=enabled`, `budget_tokens=8192` | `thinking.type=adaptive`, `output_config.effort=medium` |
| `high` | `thinking.type=enabled`, `budget_tokens=16384` | `thinking.type=adaptive`, `output_config.effort=high` |
| `xhigh` | `thinking.type=enabled`, `budget_tokens=32768` | `thinking.type=adaptive`, `output_config.effort=max` |

Budget-mode constraints:

- minimum `budget_tokens=1024`
- any explicit budget below `1024` is raised to `1024`
- when `max_tokens` exists, the proxy guarantees:
  - `max_tokens >= max(budget_tokens + 1, 2048)`
  - `budget_tokens < max_tokens`
- invalid budgets are adjusted into an upstream-valid value

Adaptive capability behavior:

- older models continue to use `enabled + budget_tokens`
- models that support adaptive thinking are emitted as `thinking.type=adaptive + output_config.effort`
- Claude capability detection uses `OriginModel` first, then `ActualModel`

### 4.5 Ali DashScope-compatible output

Output fields:

```json
{
  "enable_thinking": true,
  "thinking_budget": 2048
}
```

Rules:

- `none` -> `enable_thinking=false`, and `thinking_budget` is removed
- enabled reasoning -> `enable_thinking=true`
- if the model supports budgets, `thinking_budget` is written

Exact mapping:

| normalized effort | Ali output for models supporting `thinking_budget` | Ali output for models without budget support |
| --- | --- | --- |
| `none` | `enable_thinking=false`; no `thinking_budget` | `enable_thinking=false`; no `thinking_budget` |
| `minimal` | `enable_thinking=true`, `thinking_budget=1024` | `enable_thinking=true`; no `thinking_budget` |
| `low` | `enable_thinking=true`, `thinking_budget=2048` | `enable_thinking=true`; no `thinking_budget` |
| `medium` | `enable_thinking=true`, `thinking_budget=8192` | `enable_thinking=true`; no `thinking_budget` |
| `high` | `enable_thinking=true`, `thinking_budget=16384` | `enable_thinking=true`; no `thinking_budget` |
| `xhigh` | `enable_thinking=true`, `thinking_budget=32768` | `enable_thinking=true`; no `thinking_budget` |

Models currently considered to support `thinking_budget` include:

- `qwen3-*`
- `qwq-*`
- models containing `glm`
- models containing `kimi`

Ali-specific behavior:

- `thinking_budget` is **not** clamped by `max_tokens`
- `qwen3-*`: non-streaming requests are forced to `enable_thinking=false`
- `qwq-*`: requests are forced to `stream=true`

### 4.6 Zhipu / DeepSeek / Doubao thinking output

These vendors currently use a simplified thinking object:

```json
{
  "thinking": {
    "type": "enabled|disabled"
  }
}
```

Rules:

- `none` -> `thinking.type=disabled`
- every other enabled state -> `thinking.type=enabled`
- these vendors currently preserve only the on/off meaning, not detailed budget information

That means:

- `minimal`, `low`, `medium`, `high`, and `xhigh`
- all collapse into the same upstream `enabled` state

Exact mapping:

| normalized effort | Zhipu / DeepSeek / Doubao output |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

### 4.7 Moonshot / Kimi output

Moonshot / Kimi uses Kimi's `thinking` object only for upstream models that
support thinking toggling:

```json
{
  "thinking": {
    "type": "enabled|disabled"
  }
}
```

Exact mapping for toggle-capable Kimi models:

| normalized effort | Kimi output |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

For non-toggle Kimi models, the adaptor removes `reasoning_effort` and does not
send `thinking`. Kimi output currently preserves only on/off semantics, not
budget or fine-grained effort.

---

## 5. Vendor / adaptor support matrix

This section focuses on the major adaptors that currently participate in the reasoning compatibility layer.
The tables describe what happens after the source request has been parsed into the normalized reasoning model.

## 5.1 OpenAI / Azure / OpenAI-compatible upstreams

Native fields:

| mode | native source field | emitted upstream field | exact effort mapping |
| --- | --- | --- | --- |
| Chat Completions | `reasoning_effort` | `reasoning_effort` | unchanged normalized effort |
| Completions | `reasoning_effort` | `reasoning_effort` | unchanged normalized effort |
| Responses | `reasoning.effort` | `reasoning.effort` | unchanged normalized effort |

Cross-protocol conversions:

| source request mode | target OpenAI mode | conversion |
| --- | --- | --- |
| Gemini -> Chat / Completions | OpenAI-compatible chat payload | `generationConfig.thinkingConfig` -> normalized effort -> `reasoning_effort` |
| Claude / Anthropic -> Chat / Completions | OpenAI-compatible chat payload | `thinking` / `output_config` -> normalized effort -> `reasoning_effort` |
| OpenAI Chat -> Responses | OpenAI Responses payload | `reasoning_effort` -> normalized effort -> `reasoning.effort` |
| Gemini -> Responses | OpenAI Responses payload | `thinkingConfig` -> normalized effort -> `reasoning.effort` |
| Claude / Anthropic -> Responses | OpenAI Responses payload | `thinking` / `output_config` -> OpenAI chat request -> `reasoning.effort` |

Notes:

- OpenAI Chat / Completions only parse `reasoning_effort`.
- OpenAI Chat / Completions do not parse Gemini-style `thinkingConfig`, Claude-style `thinking`, Ali `enable_thinking`, or Ali `thinking_budget` in this mode.

## 5.2 Google Gemini

Supported reasoning conversion modes:

| source request mode | Gemini adaptor behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | `generationConfig.thinkingConfig` |
| Claude / Anthropic | parses `thinking` / `output_config` through the Claude converter | `generationConfig.thinkingConfig` |
| Gemini native | uses `generationConfig.thinkingConfig` as the native request field | `generationConfig.thinkingConfig` |
| OpenAI Responses | not a Gemini adaptor input mode | N/A |
| OpenAI Completions | not a Gemini adaptor input mode | N/A |

Exact output depends on the target Gemini model family:

| normalized effort | Gemini 3+ Pro | Gemini 3+ non-Pro | `gemini-2.5-pro` | `gemini-2.5-flash` | `gemini-2.5-flash-lite` |
| --- | --- | --- | --- | --- | --- |
| `none` | `thinkingLevel=low` | `thinkingLevel=minimal` | `thinkingBudget=128`, `includeThoughts=false` | `thinkingBudget=0`, `includeThoughts=false` | `thinkingBudget=0`, `includeThoughts=false` |
| `minimal` | `thinkingLevel=low` | `thinkingLevel=minimal` | `thinkingBudget=1024`, `includeThoughts=true` | `thinkingBudget=1024`, `includeThoughts=true` | `thinkingBudget=1024`, `includeThoughts=true` |
| `low` | `thinkingLevel=low` | `thinkingLevel=low` | `thinkingBudget=2048`, `includeThoughts=true` | `thinkingBudget=2048`, `includeThoughts=true` | `thinkingBudget=2048`, `includeThoughts=true` |
| `medium` | `thinkingLevel=low` | `thinkingLevel=medium` | `thinkingBudget=8192`, `includeThoughts=true` | `thinkingBudget=8192`, `includeThoughts=true` | `thinkingBudget=8192`, `includeThoughts=true` |
| `high` | `thinkingLevel=high` | `thinkingLevel=high` | `thinkingBudget=16384`, `includeThoughts=true` | `thinkingBudget=16384`, `includeThoughts=true` | `thinkingBudget=16384`, `includeThoughts=true` |
| `xhigh` | `thinkingLevel=high` | `thinkingLevel=high` | `thinkingBudget=32768`, `includeThoughts=true` | `thinkingBudget=24576`, `includeThoughts=true` | `thinkingBudget=24576`, `includeThoughts=true` |

Notes:

- The exact mapping table applies when the adaptor is converting from OpenAI Chat or Claude / Anthropic into Gemini.
- Native Gemini requests keep their own `generationConfig.thinkingConfig`; the adaptor does not rewrite a native Gemini thinking dialect into another Gemini thinking dialect.
- Gemini 2.5 uses budget-based output with model-specific range enforcement.
- Gemini 3 / 4 / 5 use `thinkingLevel`.
- Some models cannot truly disable thinking, so `none` degrades to the minimum valid level or budget.

## 5.3 Anthropic official

Supported reasoning conversion modes:

| source request mode | Anthropic adaptor behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | `thinking` and optionally `output_config` |
| Gemini native | parses `generationConfig.thinkingConfig` | `thinking` and optionally `output_config` |
| Anthropic native | preserves native Anthropic fields | `thinking` and `output_config` as provided, with native cleanup |
| OpenAI Responses | not an Anthropic adaptor input mode | N/A |
| OpenAI Completions | not an Anthropic adaptor input mode | N/A |

Exact output by Claude capability:

| normalized effort | legacy / budget Claude output | adaptive Claude output |
| --- | --- | --- |
| `none` | `thinking.type=disabled` | `thinking.type=disabled`; may be removed for adaptive-only / Mythos models |
| `minimal` | `thinking.type=enabled`, `budget_tokens=1024` | `thinking.type=adaptive`, `output_config.effort=low` |
| `low` | `thinking.type=enabled`, `budget_tokens=2048` | `thinking.type=adaptive`, `output_config.effort=low` |
| `medium` | `thinking.type=enabled`, `budget_tokens=8192` | `thinking.type=adaptive`, `output_config.effort=medium` |
| `high` | `thinking.type=enabled`, `budget_tokens=16384` | `thinking.type=adaptive`, `output_config.effort=high` |
| `xhigh` | `thinking.type=enabled`, `budget_tokens=32768` | `thinking.type=adaptive`, `output_config.effort=max` |

Notes:

- The exact mapping table applies when the adaptor is converting from OpenAI Chat or Gemini into Claude.
- Native Anthropic requests keep native `thinking` / `output_config` fields, apart from native cleanup required by the adaptor.
- Budget mode enforces `budget_tokens < max_tokens`.
- Older models use `enabled + budget_tokens`.
- Adaptive-capable models use `adaptive + output_config.effort`.

## 5.4 AWS Bedrock Claude

Supported reasoning conversion modes:

| source request mode | Bedrock Claude behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | Claude `thinking` after Bedrock wrapping |
| Gemini native | parses `generationConfig.thinkingConfig` | Claude `thinking` after Bedrock wrapping |
| Anthropic native | preserves native Anthropic fields | Claude `thinking` after Bedrock wrapping |
| OpenAI Responses | not a Bedrock Claude input mode | N/A |
| OpenAI Completions | not a Bedrock Claude input mode | N/A |

The effort mapping is the same as Anthropic official: legacy models use `enabled + budget_tokens`; adaptive-capable models use `adaptive + output_config.effort`.
Bedrock then wraps the Claude request for the Bedrock runtime.

## 5.5 Vertex AI Claude

Supported reasoning conversion modes:

| source request mode | Vertex Claude behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | Claude `thinking` after Vertex wrapping |
| Gemini native | parses `generationConfig.thinkingConfig` | Claude `thinking` after Vertex wrapping |
| Anthropic native | preserves native Anthropic fields | Claude `thinking` after Vertex wrapping |
| OpenAI Responses | not a Vertex Claude input mode | N/A |
| OpenAI Completions | not a Vertex Claude input mode | N/A |

The effort mapping is the same as Anthropic official: legacy models use `enabled + budget_tokens`; adaptive-capable models use `adaptive + output_config.effort`.
Vertex then wraps the Claude request for Vertex AI.

## 5.6 Ali DashScope

Supported reasoning conversion modes:

| source request mode | Ali adaptor behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | `enable_thinking`; optional `thinking_budget` |
| OpenAI Completions | parses `reasoning_effort` | `enable_thinking`; optional `thinking_budget` |
| Gemini native | parses `generationConfig.thinkingConfig` through OpenAI-compatible conversion | `enable_thinking`; optional `thinking_budget` |
| Anthropic native | uses Ali Claude Code Proxy native request format | no cross-dialect reasoning migration |
| OpenAI Responses | passed through OpenAI-compatible Responses conversion | no Ali-specific reasoning hook |

Ali exact effort mapping:

| normalized effort | output on budget-capable Ali models | output on non-budget Ali models |
| --- | --- | --- |
| `none` | `enable_thinking=false`; `thinking_budget` removed | `enable_thinking=false`; `thinking_budget` removed |
| `minimal` | `enable_thinking=true`; `thinking_budget=1024` | `enable_thinking=true`; no `thinking_budget` |
| `low` | `enable_thinking=true`; `thinking_budget=2048` | `enable_thinking=true`; no `thinking_budget` |
| `medium` | `enable_thinking=true`; `thinking_budget=8192` | `enable_thinking=true`; no `thinking_budget` |
| `high` | `enable_thinking=true`; `thinking_budget=16384` | `enable_thinking=true`; no `thinking_budget` |
| `xhigh` | `enable_thinking=true`; `thinking_budget=32768` | `enable_thinking=true`; no `thinking_budget` |

Ali budget-capable model detection:

| model rule | writes `thinking_budget` |
| --- | --- |
| model starts with `qwen3-` | yes |
| model starts with `qwq-` | yes |
| model name contains `glm` | yes |
| model name contains `kimi` | yes |
| other Ali-compatible models | no; only `enable_thinking` is written |

Ali-specific behavior:

- `thinking_budget` is not clamped by `max_tokens`.
- `qwen3-*` non-streaming Chat / Completions requests force `enable_thinking=false` after the reasoning hook.
- `qwen3-*` non-streaming Gemini-mode requests force `enable_thinking=false` and remove `thinking_budget`.
- `qwq-*` requests force `stream=true`.

## 5.7 Doubao

Supported reasoning conversion modes:

| source request mode | Doubao adaptor behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | `thinking.type` |
| Gemini native | parses `generationConfig.thinkingConfig` through OpenAI-compatible conversion | `thinking.type` |
| Anthropic native | parses `thinking` / `output_config` through OpenAI-compatible conversion | `thinking.type` |
| OpenAI Responses | passed through native OpenAI-compatible Responses conversion | no Doubao-specific reasoning hook |
| OpenAI Completions | not supported by Doubao adaptor | N/A |

Exact effort mapping:

| normalized effort | Doubao output |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

Notes:

- Only on/off semantics are preserved; budget and fine-grained effort are dropped.
- `deepseek-reasoner` also injects a system prompt, using the same origin-first, actual-fallback model match strategy.

## 5.8 DeepSeek

Supported reasoning conversion modes:

| source request mode | DeepSeek adaptor behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | `thinking.type` |
| Gemini native | parses `generationConfig.thinkingConfig` through OpenAI-compatible conversion | `thinking.type` |
| Anthropic native | uses DeepSeek `/anthropic/v1/messages` | no cross-dialect reasoning migration |
| OpenAI Responses | unsupported by the DeepSeek adaptor | N/A |
| OpenAI Completions | OpenAI-compatible pass-through | no DeepSeek-specific reasoning hook |

Exact effort mapping for the hooked Chat / Gemini paths:

| normalized effort | DeepSeek output |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

DeepSeek currently preserves only enabled / disabled for the hooked OpenAI Chat and Gemini paths.

## 5.9 Zhipu

Supported reasoning conversion modes:

| source request mode | Zhipu adaptor behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | `thinking.type` |
| Gemini native | parses `generationConfig.thinkingConfig` through OpenAI-compatible conversion | `thinking.type` |
| Anthropic native | parses `thinking` / `output_config` through OpenAI-compatible conversion | `thinking.type` |
| OpenAI Responses | unsupported by this adaptor | N/A |
| OpenAI Completions | OpenAI-compatible pass-through | no Zhipu-specific reasoning hook |

Exact effort mapping for the hooked Chat / Gemini / Anthropic paths:

| normalized effort | Zhipu output |
| --- | --- |
| `none` | `thinking.type=disabled` |
| `minimal` | `thinking.type=enabled` |
| `low` | `thinking.type=enabled` |
| `medium` | `thinking.type=enabled` |
| `high` | `thinking.type=enabled` |
| `xhigh` | `thinking.type=enabled` |

Zhipu currently preserves only enabled / disabled for the hooked paths.

## 5.10 Moonshot / Kimi

Supported reasoning conversion modes:

| source request mode | Moonshot adaptor behavior | emitted upstream field |
| --- | --- | --- |
| OpenAI Chat | parses `reasoning_effort` | Kimi `thinking.type` for toggle-capable models |
| Gemini native | parses `generationConfig.thinkingConfig` through OpenAI-compatible conversion | Kimi `thinking.type` for toggle-capable models |
| Anthropic native | parses `thinking` / `output_config` through OpenAI-compatible conversion | Kimi `thinking.type` for toggle-capable models |
| OpenAI Completions | OpenAI-compatible payload is passed through | no Moonshot-specific reasoning hook |
| OpenAI Responses | unsupported by the Moonshot adaptor | N/A |

The Moonshot adaptor currently treats these actual upstream model names as thinking-toggle capable:

| actual upstream model rule | supports emitted Kimi `thinking.type` |
| --- | --- |
| `kimi-k2.5*` | yes |
| `kimi-k2.6*` | yes |
| other Kimi model names | no; `reasoning_effort` is removed and `thinking` is omitted |

Exact effort mapping for toggle-capable Kimi models:

| normalized effort | Kimi output |
| --- | --- |
| `none` | `thinking.type=disabled`; `reasoning_effort` removed |
| `minimal` | `thinking.type=enabled`; `reasoning_effort` removed |
| `low` | `thinking.type=enabled`; `reasoning_effort` removed |
| `medium` | `thinking.type=enabled`; `reasoning_effort` removed |
| `high` | `thinking.type=enabled`; `reasoning_effort` removed |
| `xhigh` | `thinking.type=enabled`; `reasoning_effort` removed |

For non-toggle Kimi models, such as dedicated thinking models, the adaptor removes `reasoning_effort` and omits `thinking`.
It does not send `thinking.type=disabled` to a model that cannot be disabled.

Notes:

- Only on/off semantics are preserved; budget and fine-grained effort are dropped.
- Model support is checked against `ActualModel`, because the upstream Kimi model name after channel mapping determines whether `thinking` is valid.

---

## 6. Model matching strategy

Most model-capability branches follow the same rule:

1. use `OriginModel` first
2. if `OriginModel` does not match, fall back to `ActualModel`

Why this exists:

- callers may use a business-facing original model name
- channel mapping may rewrite to the real upstream model name in `ActualModel`
- some capability checks only match one of those names

This strategy is already used in:

- Claude adaptive capability detection
- Gemini thinking-level vs thinking-budget path selection
- Ali budget support detection
- Doubao bot / vision / deepseek-reasoner special routing
- other model-name-based reasoning capability branches

Exception:

- Moonshot / Kimi thinking-toggle detection uses `ActualModel` first, because
  `thinking` validity depends on the final upstream Kimi model after channel
  mapping.

---

## 7. Complete conversion examples

This section is intentionally broad. It tries to cover every reasoning-related conversion path currently implemented in code.

## 7.1 OpenAI Chat / Completions as the source format

### 7.1.1 OpenAI Chat -> OpenAI Responses

Input:

```json
{
  "model": "gpt-4o",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "model": "gpt-4o",
  "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "hello"}]}],
  "reasoning": {
    "effort": "high"
  }
}
```

Notes:

- the same reasoning payload shape is used when Chat is converted to Responses for OpenAI-compatible upstreams
- the same principle also applies to Azure when the request is routed to Responses

### 7.1.2 OpenAI Chat -> Gemini 2.5 Pro

Input:

```json
{
  "model": "gemini-2.5-pro",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

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

### 7.1.3 OpenAI Chat -> Gemini 2.5 Flash with explicit disable

Input:

```json
{
  "model": "gemini-2.5-flash",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

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

### 7.1.4 OpenAI Chat -> Gemini 3 Pro with explicit disable

Input:

```json
{
  "model": "gemini-3-pro",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

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

Notes:

- the proxy does not force an invalid disable payload here
- it degrades to the minimum valid level for the model

### 7.1.5 OpenAI Chat -> Anthropic Claude Sonnet 4.5

Input:

```json
{
  "model": "claude-sonnet-4-5",
  "reasoning_effort": "low",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

### 7.1.6 OpenAI Chat -> Anthropic Claude 3.7 Sonnet

Input:

```json
{
  "model": "claude-3-7-sonnet-20250219",
  "reasoning_effort": "medium",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 8192
  }
}
```

### 7.1.7 OpenAI Chat -> Anthropic Claude Opus 4.7

Input:

```json
{
  "model": "claude-opus-4-7",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

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

Input:

```json
{
  "model": "claude-opus-4-7",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Representative output body:

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

Notes:

- AWS wraps the Claude request with Bedrock-specific fields
- the inner Claude thinking shape still follows the Anthropic conversion rules

### 7.1.9 OpenAI Chat -> Vertex AI Claude

Input:

```json
{
  "model": "claude-sonnet-4-5",
  "reasoning_effort": "low",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Representative output body:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

Notes:

- Vertex AI uses a different transport path such as `rawPredict` / `streamRawPredict`
- the request body still follows the Claude reasoning schema

### 7.1.10 OpenAI Chat -> Ali compatible chat

Input:

```json
{
  "model": "glm-4.5",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "enable_thinking": true,
  "thinking_budget": 16384
}
```

### 7.1.11 OpenAI Chat -> Ali `qwen3-*` non-stream request

Input:

```json
{
  "model": "qwen3-32b",
  "reasoning_effort": "high",
  "stream": false,
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "enable_thinking": false,
  "thinking_budget": 16384
}
```

Notes:

- the model-specific qwen3 patch forces `enable_thinking=false` for non-streaming requests
- the budget field may still remain in the converted body because the override only changes the enable flag

### 7.1.12 OpenAI Chat -> Ali `qwq-*`

Input:

```json
{
  "model": "qwq-plus",
  "reasoning_effort": "low",
  "stream": false,
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "enable_thinking": true,
  "thinking_budget": 2048,
  "stream": true
}
```

### 7.1.13 OpenAI Chat -> Zhipu

Input:

```json
{
  "model": "glm-5.1",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "thinking": {
    "type": "disabled"
  }
}
```

### 7.1.14 OpenAI Chat -> DeepSeek

Input:

```json
{
  "model": "deepseek-chat",
  "reasoning_effort": "high",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "thinking": {
    "type": "enabled"
  }
}
```

### 7.1.15 OpenAI Chat -> Doubao

Input:

```json
{
  "model": "doubao-seed-1-6",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "thinking": {
    "type": "disabled"
  }
}
```

### 7.1.16 OpenAI Chat -> Doubao with `deepseek-reasoner`

Input:

```json
{
  "model": "deepseek-reasoner",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

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

Notes:

- this is not an effort conversion example
- it is still part of the implemented reasoning-related behavior in the Doubao adaptor

### 7.1.17 OpenAI Chat -> Moonshot / Kimi K2.6

Input:

```json
{
  "model": "kimi-k2.6",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "thinking": {
    "type": "disabled"
  }
}
```

Notes:

- `reasoning_effort` is removed
- enabled efforts become `thinking.type=enabled`
- budget and fine-grained effort are not preserved

### 7.1.18 OpenAI Chat -> Moonshot / Kimi non-toggle model

Input:

```json
{
  "model": "kimi-k2-thinking",
  "reasoning_effort": "none",
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "messages": [{"role": "user", "content": "hello"}]
}
```

Notes:

- `reasoning_effort` is removed
- `thinking` is omitted because this model family does not support toggling via
  request parameter

### 7.1.19 OpenAI Completions -> Ali

Input:

```json
{
  "model": "glm-4.5",
  "reasoning_effort": "low",
  "prompt": "hello"
}
```

Output:

```json
{
  "enable_thinking": true,
  "thinking_budget": 2048
}
```

## 7.2 Gemini native requests as the source format

### 7.2.1 Gemini -> OpenAI Chat / Completions

Input:

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

Output:

```json
{
  "reasoning_effort": "low",
  "messages": [{"role": "user", "content": "hello"}]
}
```

### 7.2.2 Gemini -> OpenAI Responses

Input:

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

Output:

```json
{
  "reasoning": {
    "effort": "high"
  }
}
```

### 7.2.3 Gemini -> Anthropic official

Input:

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

Output for an older Claude model:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

### 7.2.4 Gemini -> Anthropic adaptive Claude

Input:

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

Output for `claude-opus-4-7`:

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

Representative output body:

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

Representative output body:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

### 7.2.7 Gemini -> Ali

Input:

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

Output:

```json
{
  "enable_thinking": true,
  "thinking_budget": 2048
}
```

### 7.2.8 Gemini -> Zhipu / DeepSeek / Doubao

Input:

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

Output:

```json
{
  "thinking": {
    "type": "enabled"
  }
}
```

Notes:

- budget details are not preserved
- the payload degrades to pure on/off semantics

### 7.2.9 Gemini -> Moonshot / Kimi K2.6

Input:

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

Output:

```json
{
  "thinking": {
    "type": "enabled"
  }
}
```

Notes:

- Gemini `thinkingConfig` is first normalized through the OpenAI-compatible
  `reasoning_effort` path, then the Moonshot hook writes Kimi `thinking`
- budget details are not preserved

## 7.3 Claude / Anthropic requests as the source format

### 7.3.1 Claude -> OpenAI Chat / Completions

Input:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "reasoning_effort": "low",
  "messages": [{"role": "user", "content": "hello"}]
}
```

### 7.3.2 Claude adaptive -> OpenAI Chat / Completions

Input:

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

Output:

```json
{
  "reasoning_effort": "high"
}
```

### 7.3.3 Claude -> OpenAI Responses

Input:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "reasoning": {
    "effort": "low"
  }
}
```

### 7.3.4 Claude -> Gemini

Input:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 16384
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output for `gemini-2.5-pro`:

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

### 7.3.5 Native Anthropic -> Anthropic official

Input:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 2048
  }
}
```

Notes:

- this path preserves native Claude thinking fields
- it does not rewrite them into another thinking dialect

### 7.3.6 Native Anthropic -> AWS / Vertex Claude wrappers

Input:

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

Representative AWS wrapper:

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

Representative Vertex body:

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

Input:

```json
{
  "thinking": {
    "type": "disabled"
  },
  "messages": [{"role": "user", "content": "hello"}]
}
```

Output:

```json
{
  "thinking": {
    "type": "disabled"
  }
}
```

Notes:

- Claude `thinking` is first normalized through the OpenAI-compatible
  `reasoning_effort` path, then the Moonshot hook writes Kimi `thinking`
- budget and adaptive effort details are not preserved by the Kimi target

---

## 8. What this feature does not do

This feature intentionally does **not** do the following:

- it does not parse thinking dialects that do not belong to the current request mode
  - for example, OpenAI Chat mode does not parse Gemini `thinkingConfig`
  - Gemini mode does not parse Claude `thinking`
- it does not migrate every native request into a different thinking dialect
- it does not guarantee that every upstream preserves detailed budget and effort semantics
  - especially for Zhipu / Doubao / DeepSeek, which currently preserve only enabled / disabled
- it does not automatically add reasoning compatibility to adaptors that do not yet install the required hooks

---

## 9. Maintenance guidance

If a new vendor or request format needs thinking compatibility in the future, the recommended workflow is:

1. define the **native parsing entry** for that request mode
2. normalize it into `NormalizedReasoning`
3. write it back using the actual upstream-supported schema
4. only apply constraints and validity fixes to the **converted request body**
5. document the model-name matching order for capability checks; use
   origin-first / actual-fallback by default, but use `ActualModel` when the
   final upstream model name determines parameter validity
6. add tests for at least:
   - explicit disable
   - old-model vs new-model behavior
   - budget minimum and maximum limits
   - `max_tokens` / `maxOutputTokens` interactions
   - `OriginModel` match and `ActualModel` fallback match
