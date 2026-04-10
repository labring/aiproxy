# Cache Follow Plugin

## Overview

`cachefollow` is used to improve upstream cache hit rate.

It supports two main cache-follow behaviors:

- `prompt_cache_key`-based targeted channel follow
- generic cache-follow when no cache key is provided

When the request includes a `user` field, it also records an additional user-scoped cache-follow mapping.

The plugin is disabled by default and must be explicitly enabled in the model configuration.

## Configuration Example

```json
{
  "model": "gpt-5",
  "type": 1,
  "plugin": {
    "cachefollow": {
      "enable": true,
      "followed_channel_ttl_seconds": 180,
      "recent_channel_update_debounce_seconds": 30
    }
  }
}
```

## Configuration Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enable` | `bool` | `false` | Supports `prompt_cache_key`-based targeted channel follow and generic cache-follow when no cache key is provided, helping improve upstream cache hit rate. Disabled by default. |
| `followed_channel_ttl_seconds` | `integer` | `180` | Controls how long a remembered cache-effective channel stays valid. It always applies to user-scoped and generic entries, and also applies to `prompt_cache_key` entries when the upstream does not return a more specific retention. |
| `recent_channel_update_debounce_seconds` | `integer` | `30` | Controls the minimum refresh interval for the `recent` channel mapping in the same scope, reducing noisy `recent` updates while still following recent upstream routing changes. |

## Scopes and Priority

When the plugin is enabled, channel selection tries remembered cache-effective channels in this order:

1. `prompt_cache_key` scope
2. `user` scope
3. generic scope
4. normal channel selection if none of the remembered channels can be used

Each scope keeps two remembered channels:

- `stable`: the first cache-effective channel observed in that scope
- `recent`: the most recently observed cache-effective channel in that scope

So a single request can contribute up to 6 preferred channels:

1. `prompt_cache_key stable`
2. `prompt_cache_key recent`
3. `user stable`
4. `user recent`
5. `generic stable`
6. `generic recent`

Additional notes:

- the same channel ID is only kept once; duplicates are removed
- within the same scope, `stable` is always tried before `recent`
- `user` scope has lower priority than `prompt_cache_key`, and higher priority than generic scope

## `stable` vs `recent`

The two remembered channel types serve different purposes:

- `stable` preserves longer-lived channel affinity once a good cache-effective channel is found
- `recent` tracks more recent upstream routing movement

They are written differently:

- `stable` is only written when that scope does not already have a stable mapping
- `recent` can be refreshed continuously, but is rate-limited by `recent_channel_update_debounce_seconds`

In practice:

- `stable` is better for long-lived cache affinity
- `recent` is better for following recent upstream routing changes

## When a Channel Is Recorded

The plugin only records cache-follow data when all of the following are true:

- the plugin is enabled
- the current request mode supports `cachefollow`
- the upstream request succeeds
- the response status is `2xx`
- the response body is actually written
- the request has cache-related usage

Cache-related usage means either of the following:

- `cached_tokens > 0`
- `cache_creation_tokens > 0`

If both values are `0`, nothing is recorded.

The request must also have valid:

- `channel`
- `model`

Otherwise nothing is recorded.

## Scope-Specific Recording Rules

### `prompt_cache_key` Scope

This scope is only supported in:

- `responses`
- `chat.completions`

When the request includes `prompt_cache_key` and the request has cache-related usage, the plugin records:

- `prompt_cache_key stable`
- `prompt_cache_key recent`

TTL rules for this scope:

- if the upstream returns a valid `prompt_cache_retention`, that value is used first
- otherwise `followed_channel_ttl_seconds` is used
- if `followed_channel_ttl_seconds` is not configured, the built-in default `180s` is used

### `user` Scope

When the request includes `user` and the current mode supports `cachefollow`, the plugin records:

- `user stable`
- `user recent`

This scope always uses:

- `followed_channel_ttl_seconds`

If not configured, it uses the default `180s`.

### Generic Scope

As long as the current mode supports `cachefollow` and the request satisfies the recording conditions, the plugin records:

- `generic stable`
- `generic recent`

This scope also always uses:

- `followed_channel_ttl_seconds`

If not configured, it uses the default `180s`.

## Supported Modes

Modes that support generic and user-scoped cache-follow:

- `responses`
- `chat.completions`
- `gemini`
- `anthropic`

Modes that additionally support `prompt_cache_key`-scoped targeted follow:

- `responses`
- `chat.completions`

This means:

- `gemini` and `anthropic` only record user-scoped and generic mappings
- even if a request includes `prompt_cache_key`, unsupported modes do not record `prompt_cache_key` scope

## Channel Fallback Rules

Remembered channels are only preferences. They still go through normal availability checks.

A preferred channel is skipped if any of the following is true:

- it no longer exists in the model's currently available channel set
- it is disabled
- it does not support the current request mode
- it is currently banned by monitor
- its error rate is higher than `0.75`

If a preferred channel is skipped, the system moves to the next preferred channel. If no preferred channel survives filtering, selection falls back to the normal channel selection flow.

This also means:

- deleted channels do not cause incorrect cache-follow hits
- disabled channels automatically stop being used
- unhealthy channels are not forced back in just because they previously had a cache hit

## Full Selection Order Example

If a request includes both:

- `prompt_cache_key`
- `user`

then the selection order is:

1. `prompt_cache_key stable`
2. `prompt_cache_key recent`
3. `user stable`
4. `user recent`
5. `generic stable`
6. `generic recent`
7. normal channel selection if none of the above can be used

## Notes

- disabling the plugin also disables reading cache-follow mappings during channel selection
- the plugin only affects channel preference; it does not modify the request body
- `recent_channel_update_debounce_seconds` only affects `recent` refresh frequency and does not affect `stable`
