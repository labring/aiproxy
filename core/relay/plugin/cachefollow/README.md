# Cache Follow Plugin

## Overview

`cachefollow` is used to improve upstream cache hit rate.

By default, it enables:

- `prompt_cache_key`-based targeted channel follow
- user-scoped cache-follow

Generic cache-follow is **not** enabled by default. It must be explicitly enabled through `enable_generic_follow`.

The plugin is disabled by default and must be explicitly enabled in the model configuration.

## Configuration Example

Default behavior, with only `prompt_cache_key` and `user` scopes enabled:

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

If you also want generic cache-follow:

```json
{
  "model": "gpt-5",
  "type": 1,
  "plugin": {
    "cachefollow": {
      "enable": true,
      "enable_generic_follow": true,
      "followed_channel_ttl_seconds": 180,
      "recent_channel_update_debounce_seconds": 30
    }
  }
}
```

## Configuration Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enable` | `bool` | `false` | Supports `prompt_cache_key`-based targeted channel follow and user-scoped cache-follow by default, helping improve upstream cache hit rate. Disabled by default. |
| `enable_generic_follow` | `bool` | `false` | Enables generic cache-follow for the model-level scope. This is only used when no `prompt_cache_key` mapping or user-scoped mapping is available. Recommended when each user effectively has an isolated `group` and `token`; not recommended when many users share the same `group` and `token` scope. |
| `followed_channel_ttl_seconds` | `integer` | `180` | Controls how long a remembered cache-effective channel stays valid. It always applies to user-scoped entries, applies to generic entries only when generic follow is enabled, and also applies to `prompt_cache_key` entries when the upstream does not return a more specific retention. |
| `recent_channel_update_debounce_seconds` | `integer` | `30` | Controls the minimum refresh interval for the `recent` channel mapping in the same scope, reducing noisy `recent` updates while still following recent upstream routing changes. |

## Scopes and Priority

When the plugin is enabled, channel selection tries remembered cache-effective channels in this order:

1. `prompt_cache_key` scope
2. `user` scope
3. generic scope, only when `enable_generic_follow = true`
4. normal channel selection if none of the remembered channels can be used

Each scope keeps two remembered channels:

- `stable`: the first cache-effective channel observed in that scope
- `recent`: the most recently observed cache-effective channel in that scope

So a single request can contribute:

- up to 4 preferred channels by default
- up to 6 preferred channels when `enable_generic_follow = true`

Full order when generic follow is enabled:

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

Typical `recent` scenarios:

- the previously preferred channel now has an error rate above the selection threshold, so traffic moves to another healthy cache-effective channel
- the previously preferred channel is disabled or deleted, so selection falls through to another available channel and the `recent` mapping eventually follows it
- the previously preferred channel becomes banned by monitor, so requests stop using it and `recent` can move to the next cache-effective channel once a new successful request is observed

This is why `recent` is debounced instead of frozen:

- it avoids noisy writes under high traffic
- it still allows the remembered routing to adapt when the upstream routing reality changes

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

Generic scope is only active when:

- `enable_generic_follow = true`

When enabled, and the current mode supports `cachefollow`, and the request satisfies the recording conditions, the plugin records:

- `generic stable`
- `generic recent`

This scope also uses:

- `followed_channel_ttl_seconds`

If not configured, it uses the default `180s`.

## When to Enable Generic Follow

Generic follow is keyed by:

- `group`
- `token`
- `model`

That means it works well when this scope is already narrow enough.

Recommended cases:

- public cloud or multi-tenant deployments where each user or tenant has its own `group` and `token`
- environments where a single `group + token + model` scope naturally represents one isolated traffic source

In those cases, generic follow can improve cache affinity without causing unrelated traffic to collapse onto the same channel.

Avoid enabling generic follow when:

- many users share the same `group` and `token`
- traffic is carried through a single global admin key
- you rely on broad load balancing inside one large shared `group + token + model` scope

Typical bad case:

- all traffic uses one shared admin key, with no meaningful `group` or `token` isolation between users

In that setup, enabling generic follow means requests in that shared scope can all converge on the same remembered channel. That weakens load balancing and can over-concentrate traffic on one channel.

## Supported Modes

Modes that support user-scoped cache-follow:

- `responses`
- `chat.completions`
- `gemini`
- `anthropic`

Modes that additionally support `prompt_cache_key`-scoped targeted follow:

- `responses`
- `chat.completions`

Generic scope follows the same mode support as user scope, but only when `enable_generic_follow = true`.

This means:

- `gemini` and `anthropic` can record user-scoped and generic mappings
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

## Notes

- disabling the plugin also disables reading cache-follow mappings during channel selection
- by default, only `prompt_cache_key` and `user` mappings are read and written
- generic mappings are only read and written when `enable_generic_follow = true`
- the plugin only affects channel preference; it does not modify the request body
- `recent_channel_update_debounce_seconds` only affects `recent` refresh frequency and does not affect `stable`
