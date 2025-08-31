package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func ListTools(_ *config.ToolsConfig, toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	// Check if JSON format is requested
	format := ""
	if f, ok := args["format"].(string); ok {
		format = f
	}

	var output string
	var err error

	if format == "json" {
		output, err = FormatAsJSON(GetAllTools())
	} else {
		output = FormatAsText(GetAllTools())
	}

	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error listing tools: %v", err)
		return result
	}

	result.Output = output
	return result
}

// FormatAsText formats the tools list as human-readable text
func FormatAsText(tools []ToolInfo) string {
	var sb strings.Builder

	sb.WriteString("Available Tools:\n")
	sb.WriteString("================\n\n")

	// Format each tool
	for i, tool := range tools {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, tool.Name))
		sb.WriteString(fmt.Sprintf("   Description: %s\n", tool.Description))

		if len(tool.Parameters) > 0 {
			sb.WriteString("   Parameters:\n")
			for param, desc := range tool.Parameters {
				required := ""
				for _, req := range tool.Required {
					if req == param {
						required = " (required)"
						break
					}
				}
				sb.WriteString(fmt.Sprintf("     - %s: %s%s\n", param, desc, required))
			}
		} else {
			sb.WriteString("   Parameters: None\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func FormatAsJSON(tools []ToolInfo) (string, error) {
	jsonData, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal tools to JSON: %w", err)
	}

	return string(jsonData), nil
}
