package websearch

import (
	_ "embed"

	mcpservers "github.com/labring/aiproxy/mcp-servers"
)

//go:embed features.md
var readme string

// Register the server
func init() {
	mcpservers.Register(
		mcpservers.NewMcp(
			"web-search",
			"Web Search",
			mcpservers.McpTypeEmbed,
			mcpservers.WithNewServerFunc(NewServer),
			mcpservers.WithConfigTemplates(configTemplates),
			mcpservers.WithTags([]string{"search", "web", "google", "bing", "arxiv", "searchxng"}),
			mcpservers.WithReadme(readme),
		),
	)
}
