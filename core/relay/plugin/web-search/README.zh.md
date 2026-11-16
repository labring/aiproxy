# Web Search Plugin 配置说明

## 概述

Web Search Plugin 是一个为 AI 模型提供实时网络搜索能力的插件，支持多种搜索引擎（Google、Bing、BingCN、Arxiv、SearchXNG），能够自动重写搜索查询并格式化搜索结果。

## 配置示例

```json
{
    "model": "claude-3-7-sonnet-20250219",
    "retry_times": 5,
    "owner": "anthropic",
    "type": 1,
    "plugin": {
        "web-search": {
            "enable": true,
            "force_search": true,
            "search_rewrite": {
                "enable": true,
                "add_rewrite_usage": true,
                "rewrite_usage_field": "rewrite_usage"
            },
            "need_reference": true,
            "reference_location": "content",
            "search_from": [
                {
                    "type": "google",
                    "spec": {
                        "api_key": "api key",
                        "cx": "cx"
                    }
                }
            ]
        }
    }
}
```

## Model Config 配置字段详解

### 基础配置

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `model` | string | 是 | - | 使用的 AI 模型名称 |
| `retry_times` | int | 否 | 3 | 请求失败时的重试次数 |
| `type` | int | 是 | - | 模型类型标识 |

### Web Search 插件配置

#### 主要配置项

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `enable` | bool | 是 | false | 是否启用 Web Search 插件 |
| `force_search` | bool | 否 | false | 是否默认为所有请求启用网络搜索，默认情况下，如果用户请求中没有 `web_search_options` 字段，则不启用网络搜索 |
| `max_results` | int | 否 | 10 | 每次搜索返回的最大结果数量 |
| `need_reference` | bool | 否 | false | 是否在回答中包含引用信息 |
| `reference_location` | string | 否 | "content" | 引用位置，可选值：`content`、`references` ... 等 |
| `reference_format` | string | 否 | "**References:**\n%s" | 引用格式模板，必须包含 `%s` 占位符 |
| `default_language` | string | 否 | - | 默认搜索语言 |
| `prompt_template` | string | 否 | - | 自定义提示词模板 |

#### 搜索重写配置 (`search_rewrite`)

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `enable` | bool | 否 | false | 是否启用搜索查询重写功能 |
| `model_name` | string | 否 | - | 用于重写查询的模型名称，为空时使用当前请求的模型 |
| `timeout_millisecond` | uint32 | 否 | 10000 | 重写请求超时时间（毫秒） |
| `max_count` | int | 否 | 3 | 最大重写查询数量 |
| `add_rewrite_usage` | bool | 否 | false | 是否在响应中添加重写使用统计信息 |
| `rewrite_usage_field` | string | 否 | "rewrite_usage" | 重写使用统计信息字段名称 |

#### 搜索引擎配置 (`search_from`)

每个搜索引擎配置包含以下字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 搜索引擎类型：`google`、`bing`、`bingcn`、`arxiv`、`searchxng` |
| `max_results` | int | 否 | 该引擎的最大结果数量 |
| `spec` | object | 视类型而定 | 引擎特定的配置参数 |

##### Google 搜索引擎配置 (`spec`)

```json
{
    "type": "google",
    "spec": {
        "api_key": "your_google_api_key",
        "cx": "your_custom_search_engine_id"
    }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `api_key` | string | 是 | Google Custom Search API 密钥 |
| `cx` | string | 是 | Google 自定义搜索引擎 ID |
| `base_url` | string | 否 | Custom Search JSON API 地址，默认 `https://www.googleapis.com/customsearch/v1` |

##### Bing 搜索引擎配置 (`spec`)

```json
{
    "type": "bing",
    "spec": {
        "api_key": "your_bing_api_key"
    }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `api_key` | string | 是 | Bing Search API 密钥 |
| `base_url` | string | 否 | Bing Web Search v7 地址，默认 `https://api.bing.microsoft.com/v7.0/search` |


##### BingCN 搜索引擎配置 (`spec`)

```json
{
    "type": "bingcn",
    "spec": {}
}
```

BingCN 搜索引擎无需额外配置参数，使用默认配置。

##### Arxiv 搜索引擎配置 (`spec`)

```json
{
    "type": "arxiv",
    "spec": {}
}
```

Arxiv 搜索引擎无需额外配置参数。

##### SearchXNG 搜索引擎配置 (`spec`)

```json
{
    "type": "searchxng",
    "spec": {
        "base_url": "https://searchxng.com"
    }
}
```

## 用户请求配置

### web_search_options 字段

用户可以在请求中添加 `web_search_options` 字段来控制搜索行为：

```json
{
    "model": "claude-3-7-sonnet-20250219",
    "messages": [
        {
            "role": "user",
            "content": "请搜索最新的AI技术发展"
        }
    ],
    "web_search_options": {
        "enable": true,
        "search_context_size": "medium"
    }
}
```

#### web_search_options 配置项

| 字段 | 类型 | 可选值 | 说明 |
|------|------|--------|------|
| `enable` | bool | - | 是否启用搜索，如果为 `false`，则不启用搜索 |
| `search_context_size` | string | `low`、`medium`、`high` | 控制搜索上下文的大小，影响搜索查询的数量和深度 |

#### search_context_size 详解

`search_context_size` 字段用于控制搜索的广度和深度：

- **`low`**：生成 1 个搜索查询，适合简单、直接的问题
- **`medium`**：生成 3 个搜索查询（默认值），适合大多数场景
- **`high`**：生成 5 个搜索查询，适合复杂问题或需要全面信息的场景

该字段会覆盖配置中 `search_rewrite.max_count` 的值，允许用户根据具体需求动态调整搜索策略。

### 启用搜索的条件

Web Search 插件在以下情况下会被启用：

1. **默认启用**：当配置中 `force_search` 为 `true` 时，所有请求都会启用搜索
2. **按需启用**：当 `force_search` 为 `false` 时，只有包含 `web_search_options` 字段的请求才会启用搜索

### 使用示例

#### 基础搜索请求

```json
{
    "model": "claude-3-7-sonnet-20250219",
    "messages": [
        {
            "role": "user",
            "content": "今天的天气如何？"
        }
    ],
    "web_search_options": {
        "enable": true
    }
}
```

#### 高精度搜索请求

```json
{
    "model": "claude-3-7-sonnet-20250219",
    "messages": [
        {
            "role": "user",
            "content": "分析当前人工智能在医疗领域的最新应用和发展趋势"
        }
    ],
    "web_search_options": {
        "enable": true,
        "search_context_size": "high"
    }
}
```

## 使用说明

### 基本使用

1. **启用插件**：将 `enable` 设置为 `true`
2. **配置搜索引擎**：在 `search_from` 数组中添加至少一个搜索引擎配置
3. **设置默认行为**：通过 `force_search` 控制是否默认启用搜索
4. **用户控制**：用户可通过 `web_search_options` 字段控制搜索行为

### 高级功能

#### 搜索查询重写

启用 `search_rewrite.enable` 后，插件会使用 AI 模型自动优化用户的搜索查询，提高搜索结果的相关性。

#### 引用管理

当 `need_reference` 为 `true` 时：

- 搜索结果会包含编号引用 `[1]`、`[2]` 等
- 可通过 `reference_location` 控制引用显示位置
- 可通过 `reference_format` 自定义引用格式

#### 动态搜索控制

用户可以通过 `search_context_size` 参数动态调整搜索的深度：

- 简单问题使用 `low` 减少延迟
- 复杂问题使用 `high` 获取更全面的信息
- 默认使用 `medium` 平衡性能和质量

## 注意事项

1. **API 密钥**：确保为所选搜索引擎提供有效的 API 密钥
2. **配额限制**：注意各搜索引擎的 API 调用配额限制
3. **性能影响**：启用搜索功能会增加响应时间，`search_context_size` 设置越高，延迟越大
4. **成本考虑**：搜索 API 调用和额外的 AI 模型调用会产生费用
5. **请求清理**：`web_search_options` 字段会在处理后自动从请求中移除

## 故障排除

- 如果搜索功能未生效，检查 `enable` 是否为 `true`
- 验证搜索引擎 API 密钥是否正确配置
- 确认模型支持 Chat Completions 模式
- 检查网络连接和 API 服务可用性
- 确保请求中包含 `web_search_options` 字段（当 `force_search` 为 `false` 时）
