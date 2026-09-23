# Patch Plugin

The Patch Plugin provides powerful JSON request modification capabilities using sonic for high-performance JSON processing. It allows you to automatically modify API requests based on model types, field values, or custom conditions.

## Features

- **High Performance**: Uses ByteDance's sonic library for fast JSON parsing and manipulation
- **Predefined Patches**: Built-in patches for common scenarios (DeepSeek max_tokens limits, GPT-5/GPT-6 compatibility, etc.)
- **User-Defined Patches**: Flexible configuration system for custom patches
- **Conditional Logic**: Apply patches based on model types, field values, or complex conditions
- **Multiple Operations**: Set, delete, add, and limit operations on JSON fields
- **Nested Field Support**: Use dot notation to modify nested JSON structures
- **Placeholder Support**: Dynamic value replacement using `{{field_name}}` syntax

## Predefined Patches

The plugin comes with several built-in patches:

### 1. DeepSeek Max Tokens Limit
- **Purpose**: Limits `max_tokens` to 16000 for DeepSeek models
- **Condition**: Model name contains "deepseek"
- **Operation**: Limits `max_tokens` field to maximum 16000

### 2. GPT-5/GPT-6 Max Tokens Conversion
- **Purpose**: Converts `max_tokens` to `max_completion_tokens` for GPT-5 and GPT-6 models
- **Condition**: Model name starts with `gpt-5` or `gpt-6` and `max_tokens` field exists
- **Operation**: 
  - Sets `max_completion_tokens` to the value of `max_tokens`
  - Removes the `max_tokens` field

### 3. O1 Models Max Tokens Conversion
- **Purpose**: Converts `max_tokens` to `max_completion_tokens` for o1 models
- **Condition**: Model matches o1, o1-preview, or o1-mini
- **Operation**: Same as GPT-5 conversion

### 4. Claude Max Tokens Limit
- **Purpose**: Limits `max_tokens` to 8192 for Claude models
- **Condition**: Model name contains "claude"
- **Operation**: Limits `max_tokens` field to maximum 8192

### 5. Remove Unsupported Stream Options
- **Purpose**: Removes `stream_options` for older GPT models that don't support it
- **Condition**: Model matches older GPT patterns and `stream_options` exists
- **Operation**: Removes `stream_options` field

## Configuration

### Basic Usage

```go
import (
    "github.com/labring/aiproxy/core/relay/plugin/patch"
)

// Create plugin - configuration is loaded from model config
plugin := patch.New()
```

### Configuration

The patch plugin loads configuration from the model's plugin configuration in the database. The configuration should be stored in the model config's `plugin` field under the key `"patch"`.

Example model config plugin configuration:

```json
{
  "patch": {
    "enable": true,
    "user_patches": [
      {
        "name": "custom_temperature_limit",
        "description": "Limit temperature for specific models",
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

### Predefined Patches

The plugin comes with built-in predefined patches that are always enabled:

- **DeepSeek max_tokens limit**: Automatically limits `max_tokens` to 16000 for DeepSeek models
- **GPT-5/GPT-6 compatibility**: Converts `max_tokens` to `max_completion_tokens` for GPT-5 and GPT-6 models
- **O1 models compatibility**: Same conversion for o1, o1-preview, and o1-mini models
- **Claude max_tokens limit**: Limits `max_tokens` to 8192 for Claude models
- **Stream options cleanup**: Removes unsupported `stream_options` for older GPT models

These predefined patches run automatically and cannot be disabled.

## Condition Operators

- `equals`: Exact string match
- `not_equals`: Not equal to string
- `contains`: String contains substring
- `not_contains`: String does not contain substring
- `regex`: Regular expression match
- `exists`: Field exists (non-nil)
- `not_exists`: Field does not exist (nil)

## Operation Types

- `set`: Set field to a specific value
- `delete`: Remove field from JSON
- `add`: Add field only if it doesn't exist
- `limit`: Limit numeric field to maximum value

## Special Keys

- `model`: References the actual model name from meta
- `original_model`: References the original model name from meta
- Any other key: References JSON field (supports dot notation)

## Placeholder Syntax

Use `{{field_name}}` to reference values from the JSON data:

```go
{
    Op:    patch.OpSet,
    Key:   "max_completion_tokens",
    Value: "{{max_tokens}}", // Will be replaced with actual max_tokens value
}
```

## Nested Field Access

Use dot notation to access nested fields:

```go
{
    Key: "parameters.max_tokens",  // Accesses parameters.max_tokens
    // ...
}
```

## Integration with Plugin System

```go
import (
    "github.com/labring/aiproxy/core/relay/plugin"
    "github.com/labring/aiproxy/core/relay/plugin/patch"
)

// Create patch plugin
patchPlugin := patch.New()

// Wrap adaptor with plugin
adaptor = plugin.WrapperAdaptor(adaptor, patchPlugin)
```

## Performance Considerations

- Uses sonic library for high-performance JSON processing
- Efficient condition evaluation with early termination
- Minimal memory allocation for unchanged requests
- Lazy evaluation of patches (only applied when conditions match)

## Error Handling

- Graceful degradation: if patching fails, original request is preserved
- Detailed logging of patch failures
- Type-safe operations with proper error checking

## Examples

### Example 1: Model-specific Max Tokens

```json
{
    "name": "anthropic_max_tokens",
    "description": "Set appropriate max_tokens for Anthropic models",
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

### Example 2: Add Default Parameters

```json
{
    "name": "add_default_temperature",
    "description": "Add default temperature if not specified",
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

### Example 3: Complex Conditional Logic

```json
{
    "name": "streaming_optimization",
    "description": "Optimize streaming for specific models",
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
