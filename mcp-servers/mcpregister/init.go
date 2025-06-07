package mcpregister

import (
	// register embed mcp
	_ "github.com/labring/aiproxy/mcp-servers/12306"
	_ "github.com/labring/aiproxy/mcp-servers/aiproxy-openapi"
	_ "github.com/labring/aiproxy/mcp-servers/alipay"
	_ "github.com/labring/aiproxy/mcp-servers/amap"
	_ "github.com/labring/aiproxy/mcp-servers/baidu-map"
	_ "github.com/labring/aiproxy/mcp-servers/bingcn"
	_ "github.com/labring/aiproxy/mcp-servers/fetch"
	_ "github.com/labring/aiproxy/mcp-servers/firecrawl"
	_ "github.com/labring/aiproxy/mcp-servers/flomo"
	_ "github.com/labring/aiproxy/mcp-servers/gpt-vis"
	_ "github.com/labring/aiproxy/mcp-servers/hefeng-weather"
	_ "github.com/labring/aiproxy/mcp-servers/jina-tools"
	_ "github.com/labring/aiproxy/mcp-servers/time"
	_ "github.com/labring/aiproxy/mcp-servers/web-search"
)
