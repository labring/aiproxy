package notion

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// FilterTools filters tools based on enabled tools set
func FilterTools(tools []mcp.Tool, enabledToolsSet map[string]bool) []mcp.Tool {
	if len(enabledToolsSet) == 0 {
		return tools
	}

	var filteredTools []mcp.Tool
	for _, tool := range tools {
		if enabledToolsSet[tool.Name] {
			filteredTools = append(filteredTools, tool)
		}
	}

	return filteredTools
}

// ParseEnabledTools parses enabled tools from comma-separated string
func ParseEnabledTools(enabledToolsStr string) map[string]bool {
	enabledToolsSet := make(map[string]bool)
	if enabledToolsStr == "" {
		return enabledToolsSet
	}

	for tool := range strings.SplitSeq(enabledToolsStr, ",") {
		tool = strings.TrimSpace(tool)
		if tool != "" {
			enabledToolsSet[tool] = true
		}
	}

	return enabledToolsSet
}
