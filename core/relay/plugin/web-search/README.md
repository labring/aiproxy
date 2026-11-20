# Web Search Plugin Configuration Guide

## Overview

The Web Search Plugin is a plugin that provides real-time web search capabilities for AI models, supporting multiple search engines (Google, Bing, BingCN, Arxiv, SearchXNG), with automatic search query rewriting and search result formatting.

## Configuration Example

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

## Model Config Field Details

### Basic Configuration

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `model` | string | Yes | - | AI model name to use |
| `retry_times` | int | No | 3 | Number of retries on request failure |
| `type` | int | Yes | - | Model type identifier |

### Web Search Plugin Configuration

#### Main Configuration Items

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enable` | bool | Yes | false | Whether to enable the Web Search plugin |
| `force_search` | bool | No | false | Whether to enable web search by default for all requests. By default, if there's no `web_search_options` field in the user request, web search is not enabled |
| `max_results` | int | No | 10 | Maximum number of results returned per search |
| `need_reference` | bool | No | false | Whether to include reference information in the response |
| `reference_location` | string | No | "content" | Reference position, options: `content`, `references` ... |
| `reference_format` | string | No | "**References:**\n%s" | Reference format template, must include `%s` placeholder |
| `default_language` | string | No | - | Default search language |
| `prompt_template` | string | No | - | Custom prompt template |

#### Search Rewrite Configuration (`search_rewrite`)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enable` | bool | No | false | Whether to enable search query rewriting |
| `model_name` | string | No | - | Model name for query rewriting, uses current request model if empty |
| `timeout_millisecond` | uint32 | No | 10000 | Rewrite request timeout (milliseconds) |
| `max_count` | int | No | 3 | Maximum number of rewritten queries |
| `add_rewrite_usage` | bool | No | false | Whether to add rewrite usage statistics to the response |
| `rewrite_usage_field` | string | No | "rewrite_usage" | Rewrite usage statistics field name |

#### Search Engine Configuration (`search_from`)

Each search engine configuration contains the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Search engine type: `google`, `bing`, `bingcn`, `arxiv`, `searchxng` |
| `max_results` | int | No | Maximum results for this engine |
| `spec` | object | Depends on type | Engine-specific configuration parameters |

##### Google Search Engine Configuration (`spec`)

```json
{
    "type": "google",
    "spec": {
        "api_key": "your_google_api_key",
        "cx": "your_custom_search_engine_id"
    }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `api_key` | string | Yes | Google Custom Search API key |
| `cx` | string | Yes | Google Custom Search Engine ID |
| `base_url` | string | No | Custom Search JSON API endpoint, default `https://www.googleapis.com/customsearch/v1` |

##### Bing Search Engine Configuration (`spec`)

```json
{
    "type": "bing",
    "spec": {
        "api_key": "your_bing_api_key"
    }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `api_key` | string | Yes | Bing Search API key |
| `base_url` | string | No | Bing Web Search v7 endpoint, default `https://api.bing.microsoft.com/v7.0/search` |

##### BingCN Search Engine Configuration (`spec`)

```json

{
    "type": "bingcn",
    "spec": {}
}
```

BingCN search engine requires no additional configuration parameters.

##### Arxiv Search Engine Configuration (`spec`)

```json
{
    "type": "arxiv",
    "spec": {}
}
```

Arxiv search engine requires no additional configuration parameters.

##### SearchXNG Search Engine Configuration (`spec`)

```json
{
    "type": "searchxng",
    "spec": {
        "base_url": "https://searchxng.com"
    }
}
```

## User Request Configuration

### web_search_options Field

Users can add the `web_search_options` field in their requests to control search behavior:

```json
{
    "model": "claude-3-7-sonnet-20250219",
    "messages": [
        {
            "role": "user",
            "content": "Please search for the latest AI technology developments"
        }
    ],
    "web_search_options": {
        "enable": true,
        "search_context_size": "medium"
    }
}
```

#### web_search_options Configuration Items

| Field | Type | Options | Description |
|-------|------|---------|-------------|
| `enable` | bool | - | Whether to enable search, if `false`, search will not be enabled |
| `search_context_size` | string | `low`, `medium`, `high` | Controls the size of search context, affecting the number and depth of search queries |

#### search_context_size Details

The `search_context_size` field controls the breadth and depth of searches:

- **`low`**: Generates 1 search query, suitable for simple, direct questions
- **`medium`**: Generates 3 search queries (default), suitable for most scenarios
- **`high`**: Generates 5 search queries, suitable for complex questions or scenarios requiring comprehensive information

This field overrides the `search_rewrite.max_count` value in the configuration, allowing users to dynamically adjust search strategy based on specific needs.

### Conditions for Enabling Search

The Web Search plugin is enabled under the following conditions:

1. **Default Enable**: When `force_search` is `true` in the configuration, all requests will enable search
2. **On-Demand Enable**: When `force_search` is `false`, only requests containing the `web_search_options` field will enable search

### Usage Examples

#### Basic Search Request

```json
{
    "model": "claude-3-7-sonnet-20250219",
    "messages": [
        {
            "role": "user",
            "content": "What's the weather like today?"
        }
    ],
    "web_search_options": {
        "enable": true
    }
}
```

#### High-Precision Search Request

```json
{
    "model": "claude-3-7-sonnet-20250219",
    "messages": [
        {
            "role": "user",
            "content": "Analyze the latest applications and development trends of artificial intelligence in the medical field"
        }
    ],
    "web_search_options": {
        "enable": true,
        "search_context_size": "high"
    }
}
```

## Usage Instructions

### Basic Usage

1. **Enable Plugin**: Set `enable` to `true`
2. **Configure Search Engines**: Add at least one search engine configuration in the `search_from` array
3. **Set Default Behavior**: Control whether to enable search by default through `force_search`
4. **User Control**: Users can control search behavior through the `web_search_options` field

### Advanced Features

#### Search Query Rewriting

When `search_rewrite.enable` is enabled, the plugin will use AI models to automatically optimize user search queries, improving the relevance of search results.

#### Reference Management

When `need_reference` is `true`:

- Search results will include numbered references `[1]`, `[2]`, etc.
- Reference display position can be controlled through `reference_location`
- Reference format can be customized through `reference_format`

#### Dynamic Search Control

Users can dynamically adjust search depth through the `search_context_size` parameter:

- Use `low` for simple questions to reduce latency
- Use `high` for complex questions to get more comprehensive information
- Use `medium` by default to balance performance and quality

## Important Notes

1. **API Keys**: Ensure valid API keys are provided for selected search engines
2. **Quota Limits**: Be aware of API call quota limits for various search engines
3. **Performance Impact**: Enabling search functionality will increase response time; higher `search_context_size` settings result in greater latency
4. **Cost Considerations**: Search API calls and additional AI model calls will incur costs
5. **Request Cleanup**: The `web_search_options` field is automatically removed from requests after processing

## Troubleshooting

- If search functionality is not working, check if `enable` is set to `true`
- Verify that search engine API keys are correctly configured
- Ensure the model supports Chat Completions mode
- Check network connectivity and API service availability
- Ensure the request contains the `web_search_options` field (when `force_search` is `false`)
