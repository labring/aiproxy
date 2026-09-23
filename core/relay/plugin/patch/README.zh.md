# Patch 插件

Patch 插件提供了强大的 JSON 请求修改功能，使用 sonic 库实现高性能 JSON 处理。它允许您基于模型类型、字段值或自定义条件自动修改 API 请求。

## 功能特性

- **高性能**: 使用字节跳动的 sonic 库进行快速 JSON 解析和操作
- **预定义补丁**: 内置常见场景的补丁（DeepSeek max_tokens 限制、GPT-5/GPT-6 兼容性等）
- **用户自定义补丁**: 灵活的配置系统支持自定义补丁
- **条件逻辑**: 基于模型类型、字段值或复杂条件应用补丁
- **多种操作**: 支持设置、删除、添加和限制 JSON 字段的操作
- **嵌套字段支持**: 使用点语法修改嵌套 JSON 结构
- **占位符支持**: 使用 `{{field_name}}` 语法进行动态值替换

## 预定义补丁

插件包含几个内置补丁：

### 1. DeepSeek Max Tokens 限制
- **目的**: 将 DeepSeek 模型的 `max_tokens` 限制为 16000
- **条件**: 模型名称包含 "deepseek"
- **操作**: 将 `max_tokens` 字段限制为最大 16000

### 2. GPT-5/GPT-6 Max Tokens 转换
- **目的**: 为 GPT-5 和 GPT-6 模型将 `max_tokens` 转换为 `max_completion_tokens`
- **条件**: 模型名称以 `gpt-5` 或 `gpt-6` 开头且存在 `max_tokens` 字段
- **操作**: 
  - 将 `max_completion_tokens` 设置为 `max_tokens` 的值
  - 删除 `max_tokens` 字段

### 3. O1 模型 Max Tokens 转换
- **目的**: 为 o1 模型将 `max_tokens` 转换为 `max_completion_tokens`
- **条件**: 模型匹配 o1、o1-preview 或 o1-mini
- **操作**: 与 GPT-5 转换相同

### 4. Claude Max Tokens 限制
- **目的**: 将 Claude 模型的 `max_tokens` 限制为 8192
- **条件**: 模型名称包含 "claude"
- **操作**: 将 `max_tokens` 字段限制为最大 8192

### 5. 移除不支持的 Stream Options
- **目的**: 为不支持的较旧 GPT 模型移除 `stream_options`
- **条件**: 模型匹配较旧的 GPT 模式且存在 `stream_options`
- **操作**: 删除 `stream_options` 字段

## 配置

### 基本用法

```go
import (
    "github.com/labring/aiproxy/core/relay/plugin/patch"
)

// 创建插件 - 配置从模型配置中加载
plugin := patch.New()
```

### 配置

patch插件从数据库中模型的插件配置中加载配置。配置应存储在模型配置的`plugin`字段中，键名为`"patch"`。

模型配置插件配置示例：

```json
{
  "patch": {
    "enable": true,
    "user_patches": [
      {
        "name": "custom_temperature_limit",
        "description": "为特定模型限制温度值",
        "conditions": [
          {
            "key": "model", 
            "operator": "contains",
            "value": "gpt-4"
          }
        ],
        "operations": [
          {
            "op": "limit",
            "key": "temperature", 
            "value": 1.0
          }
        ]
      }
    ]
  }
}
```

### 预定义补丁

插件包含内置的预定义补丁，这些补丁始终启用：

- **DeepSeek max_tokens限制**: 自动将DeepSeek模型的`max_tokens`限制为16000
- **GPT-5/GPT-6兼容性**: 为GPT-5和GPT-6模型将`max_tokens`转换为`max_completion_tokens`
- **O1模型兼容性**: 为o1、o1-preview和o1-mini模型进行相同转换
- **Claude max_tokens限制**: 将Claude模型的`max_tokens`限制为8192
- **Stream选项清理**: 为较旧的GPT模型移除不支持的`stream_options`

这些预定义补丁自动运行且无法禁用。

## 条件操作符

- `equals`: 精确字符串匹配
- `not_equals`: 不等于字符串
- `contains`: 字符串包含子字符串
- `not_contains`: 字符串不包含子字符串
- `regex`: 正则表达式匹配
- `exists`: 字段存在（非空）
- `not_exists`: 字段不存在（空）

## 操作类型

- `set`: 将字段设置为特定值
- `delete`: 从 JSON 中删除字段
- `add`: 仅当字段不存在时添加字段
- `limit`: 将数值字段限制为最大值

## 特殊键

- `model`: 引用 meta 中的实际模型名称
- `original_model`: 引用 meta 中的原始模型名称
- 任何其他键: 引用 JSON 字段（支持点语法）

## 占位符语法

使用 `{{field_name}}` 引用 JSON 数据中的值：

```go
{
    Op:    patch.OpSet,
    Key:   "max_completion_tokens",
    Value: "{{max_tokens}}", // 将被替换为实际的 max_tokens 值
}
```

## 嵌套字段访问

使用点语法访问嵌套字段：

```go
{
    Key: "parameters.max_tokens",  // 访问 parameters.max_tokens
    // ...
}
```

## 与插件系统集成

```go
import (
    "github.com/labring/aiproxy/core/relay/plugin"
    "github.com/labring/aiproxy/core/relay/plugin/patch"
)

// 创建 patch 插件
patchPlugin := patch.New()

// 用插件包装适配器
adaptor = plugin.WrapperAdaptor(adaptor, patchPlugin)
```

## 性能考虑

- 使用 sonic 库进行高性能 JSON 处理
- 高效的条件评估，支持早期终止
- 对未更改的请求最小化内存分配
- 延迟评估补丁（仅在条件匹配时应用）

## 错误处理

- 优雅降级：如果补丁失败，保留原始请求
- 详细的补丁失败日志记录
- 具有适当错误检查的类型安全操作

## 示例

### 示例 1: 模型特定的 Max Tokens

```json
{
    "name": "anthropic_max_tokens",
    "description": "为 Anthropic 模型设置适当的 max_tokens",
    "conditions": [
        {
            "key": "model",
            "operator": "contains",
            "value": "claude"
        }
    ],
    "operations": [
        {
            "op": "limit",
            "key": "max_tokens",
            "value": 4096
        }
    ]
}
```

### 示例 2: 添加默认参数

```json
{
    "name": "add_default_temperature",
    "description": "如果未指定则添加默认温度值",
    "conditions": [
        {
            "key": "temperature",
            "operator": "not_exists",
            "value": ""
        }
    ],
    "operations": [
        {
            "op": "add",
            "key": "temperature",
            "value": 0.7
        }
    ]
}
```

### 示例 3: 复杂条件逻辑

```json
{
    "name": "streaming_optimization",
    "description": "为特定模型优化流式处理",
    "conditions": [
        {
            "key": "stream",
            "operator": "equals",
            "value": "true"
        },
        {
            "key": "model",
            "operator": "regex",
            "value": "^gpt-4"
        }
    ],
    "operations": [
        {
            "op": "set",
            "key": "stream_options.include_usage",
            "value": true
        }
    ]
}
```

## 使用场景

1. **模型兼容性**: 自动转换不同 API 之间的参数格式
2. **令牌限制**: 基于模型能力限制 max_tokens
3. **参数清理**: 移除特定模型不支持的参数
4. **默认值设置**: 为缺失的参数添加合理默认值
5. **API 版本适配**: 处理不同 API 版本之间的差异

这个插件设计简洁而强大，可以轻松扩展以支持新的补丁规则和操作类型。
