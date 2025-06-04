package aiproxyopenapi

import mcpservers "github.com/labring/aiproxy/mcp-servers"

// need import in mcpregister/init.go
func init() {
	mcpservers.Register(
		mcpservers.NewMcp(
			"aiproxy-openapi",
			"AI Proxy OpenAPI",
			mcpservers.McpTypeEmbed,
			mcpservers.WithNewServerFunc(NewServer),
			mcpservers.WithConfigTemplates(configTemplates),
		),
	)
}
