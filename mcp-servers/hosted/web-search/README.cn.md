# 网络搜索 MCP 服务器

一个全面的网络搜索MCP服务器，提供对多个搜索引擎的访问，包括Google、Bing、必应中国(免费)和Arxiv。

## 功能特性

- **多个搜索引擎**: 集成支持Google、Bing、必应中国(免费)和Arxiv
- **灵活配置**: 仅配置您需要的搜索引擎
- **多引擎搜索**: 同时在多个引擎中搜索
- **智能搜索**: 智能查询优化和结果聚合
- **学术搜索**: 通过Arxiv专门支持学术论文
- **语言支持**: 支持不同语言搜索
- **结果控制**: 配置返回结果的最大数量

## 配置

### 必需配置

至少需要配置一个搜索引擎的有效API凭据：

#### Google搜索

- `google_api_key`: 您的Google自定义搜索API密钥
- `google_cx`: 您的Google自定义搜索引擎ID
- `google_base_url`（可选）: Custom Search JSON API 地址，默认 `https://www.googleapis.com/customsearch/v1`

#### Bing搜索

- `bing_api_key`: 您的Bing搜索API密钥
- `bing_base_url`（可选）: Bing Web Search v7 地址，默认 `https://api.bing.microsoft.com/v7.0/search`

#### 必应中国搜索

免费，无需API密钥。

#### Arxiv搜索

无需配置 - Arxiv免费使用。

#### SearchXNG搜索

- `searchxng_base_url`: SearchXNG的基础URL

### 可选配置

- `default_engine`: 要使用的默认搜索引擎 (google, bing, arxiv)
- `max_results`: 返回搜索结果的最大数量 (1-50, 默认: 10)
