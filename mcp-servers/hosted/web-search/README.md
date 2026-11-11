# Web Search MCP Server

A comprehensive web search MCP server that provides access to multiple search engines including Google, Bing, Bing CN(Free), and Arxiv.

## Features

- **Multiple Search Engines**: Integrated support for Google, Bing, Bing CN(Free), and Arxiv
- **Flexible Configuration**: Configure only the search engines you need
- **Multi-Engine Search**: Search across multiple engines simultaneously
- **Smart Search**: Intelligent query optimization and result aggregation
- **Academic Search**: Specialized support for academic papers through Arxiv
- **Language Support**: Search in different languages
- **Result Control**: Configure the maximum number of results

## Configuration

### Required Configuration

At least one search engine must be configured with valid API credentials:

#### Google Search

- `google_api_key`: Your Google Custom Search API key
- `google_cx`: Your Google Custom Search Engine ID
- `google_base_url` (optional): Custom Search JSON API endpoint, default `https://www.googleapis.com/customsearch/v1`

#### Bing Search

- `bing_api_key`: Your Bing Search API key
- `bing_base_url` (optional): Bing Web Search v7 endpoint, default `https://api.bing.microsoft.com/v7.0/search`

#### Bing CN Search

Free, no API key required.

#### Arxiv Search

No configuration required - Arxiv is free to use.

#### SearchXNG Search

- `searchxng_base_url`: Base URL for SearchXNG

### Optional Configuration

- `default_engine`: Default search engine to use (google, bing, arxiv)
- `max_results`: Maximum number of search results to return (1-50, default: 10)

## How to test in aiproxy

```http
http://localhost:3000/api/test-embedmcp/web-search?key=sealos&config[google_api_key]=google-api-key&config[google_cx]=google-cx
```
