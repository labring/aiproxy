# Cache Follow Plugin Configuration Guide

## Overview

The Cache Follow plugin improves upstream prompt-cache hit rate in two ways:

- when `prompt_cache_key` is present, it remembers the channel bound to that cache key and prefers the same channel on later requests
- when no `prompt_cache_key` is present, it remembers the most recent cache-effective channel for the same request scope and prefers it later

It supports two independent tracking strategies:

- `prompt_cache_key` tracking for OpenAI-compatible `responses` and `chat.completions`
- generic cache-follow tracking for chat-like APIs that report cache usage

The plugin is disabled by default and must be enabled per model.

## Configuration Example

```json
{
  "model": "gpt-5",
  "type": 1,
  "plugin": {
    "cachefollow": {
      "enable": true
    }
  }
}
```

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enable` | bool | Yes | `false` | Whether to enable the Cache Follow plugin |

## How It Works

### Channel Preference Order

When the plugin is enabled, channel selection uses the following preference order:

1. If the request contains `prompt_cache_key`, try the stored channel bound to that key first
2. Otherwise, try the generic cache-follow channel for the same `group + token + model`
3. If no preferred channel is available, fall back to the normal channel selection flow

If `prompt_cache_key` is present, the generic cache-follow preference is ignored.

### Recording Rules

The plugin writes store mappings only when all of the following are true:

- the plugin is enabled
- upstream handling succeeds
- response status is `2xx`
- response body is actually written
- usage contains cache activity:
  - `cached_tokens > 0`, or
  - `cache_creation_tokens > 0`

If both values are `0`, nothing is recorded.

### Store Behavior

The store key scope is:

- `group`
- `token`
- logical store `id`

For `prompt_cache_key` requests:

- only the prompt-cache mapping is written
- the generic cache-follow mapping is skipped
- default TTL is `3m`
- if upstream returns `prompt_cache_retention`, that value overrides the default TTL

For requests without `prompt_cache_key`:

- the generic cache-follow mapping is written
- default TTL is `3m`

### Supported Modes

Prompt-cache mapping:

- `responses`
- `chat.completions`

Generic cache-follow mapping:

- `responses`
- `chat.completions`
- `gemini`
- `anthropic`

## Notes

- Disabling the plugin also disables preferred-channel reads during channel selection
- The plugin only records cache-related channel affinity; it does not change the upstream request body
