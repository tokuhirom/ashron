package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolInfo represents the structure of a tool's metadata
type ToolInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Required    []string          `json:"required,omitempty"`
}

// AllTools contains the metadata for all available tools
var AllTools = []ToolInfo{
	{
		Name:        "read_file",
		Description: "Read the contents of a file",
		Parameters: map[string]string{
			"path": "The file path to read",
		},
		Required: []string{"path"},
	},
	{
		Name:        "write_file",
		Description: "Write content to a file",
		Parameters: map[string]string{
			"path":    "The file path to write",
			"content": "The content to write",
		},
		Required: []string{"path", "content"},
	},
	{
		Name:        "execute_command",
		Description: "Execute a shell command",
		Parameters: map[string]string{
			"command":     "The command to execute",
			"working_dir": "Working directory for the command (optional)",
		},
		Required: []string{"command"},
	},
	{
		Name:        "list_directory",
		Description: "List files in a directory",
		Parameters: map[string]string{
			"path": "The directory path to list",
		},
		Required: []string{"path"},
	},
	{
		Name:        "list_tools",
		Description: "List all available tools and their descriptions",
		Parameters:  map[string]string{},
		Required:    []string{},
	},
	{
		Name:        "git_grep",
		Description: "Search for a pattern in git repository files",
		Parameters: map[string]string{
			"pattern":          "The pattern to search for",
			"path":             "Limit search to specific path or file pattern",
			"case_insensitive": "Perform case-insensitive search",
			"line_number":      "Show line numbers in output",
			"count":            "Show only count of matching lines",
		},
		Required: []string{"pattern"},
	},
	{
		Name:        "git_ls_files",
		Description: "List files in git repository",
		Parameters: map[string]string{
			"cached":           "Show cached files",
			"deleted":          "Show deleted files",
			"modified":         "Show modified files",
			"others":           "Show other (untracked) files",
			"ignored":          "Show ignored files",
			"stage":            "Show staged contents' object name",
			"unmerged":         "Show unmerged files",
			"killed":           "Show files that git checkout would overwrite",
			"exclude_standard": "Use standard git exclusions",
			"full_name":        "Show full path from repository root",
			"path":             "Limit to specific path or file pattern",
		},
		Required: []string{},
	},
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

// FormatAsJSON formats the tools list as JSON
func FormatAsJSON(tools []ToolInfo) (string, error) {
	jsonData, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal tools to JSON: %w", err)
	}

	return string(jsonData), nil
}

// ListTools generates a formatted list of available tools in text format
func ListTools() (string, error) {
	return FormatAsText(AllTools), nil
}

// ListToolsJSON returns the tool list as JSON
func ListToolsJSON() (string, error) {
	return FormatAsJSON(AllTools)
}
