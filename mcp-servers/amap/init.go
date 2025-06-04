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
			mcpservers.WithReadme(
				`# AMAP MCP Server

https://lbs.amap.com/api/mcp-server/gettingstarted
`),
		),
	)
}
