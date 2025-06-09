package amap

import mcpservers "github.com/labring/aiproxy/mcp-servers"

// need import in mcpregister/init.go
func init() {
	mcpservers.Register(
		mcpservers.NewMcp(
			"amap",
			"AMAP",
			mcpservers.McpTypeEmbed,
			mcpservers.WithNewServerFunc(NewServer),
			mcpservers.WithConfigTemplates(configTemplates),
			mcpservers.WithTags([]string{"map"}),
			mcpservers.WithDescription(
				"AMAP (AutoNavi) Map MCP Server provides comprehensive location-based services including geocoding, place search, route planning, and more through AMAP's APIs.",
			),
			mcpservers.WithDescriptionCN("高德地图MCP服务器通过高德地图API提供全面的基于位置的服务，包括地理编码、地点搜索、路线规划等。"),
			mcpservers.WithReadme(
				`# AMAP MCP Server

https://lbs.amap.com/api/mcp-server/gettingstarted
`),
		),
	)
}
