package alipay

import (
	_ "embed"

	mcpservers "github.com/labring/aiproxy/mcp-servers"
)

//go:embed README.md
var readme string

// need import in mcpregister/init.go
func init() {
	mcpservers.Register(
		mcpservers.NewMcp(
			"alipay",
			"Alipay",
			mcpservers.McpTypeDocs,
			mcpservers.WithTags([]string{"pay"}),
			mcpservers.WithReadme(readme),
		),
	)
}
