package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ListTools generates a formatted list of available tools
func ListTools() (string, error) {
	var sb strings.Builder

	sb.WriteString("Available Tools:\n")
	sb.WriteString("================\n\n")

	// Define tool information
	tools := []struct {
		Name        string
		Description string
		Parameters  map[string]string
		Required    []string
	}{
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
			Name:        "init",
			Description: "Generate an AGENTS.md file for the project",
			Parameters:  map[string]string{},
			Required:    []string{},
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
	}

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

	return sb.String(), nil
}

// ListToolsJSON returns the tool list as JSON
func ListToolsJSON() (string, error) {
	tools := []map[string]interface{}{
		{
			"name":        "read_file",
			"description": "Read the contents of a file",
			"parameters": map[string]string{
				"path": "The file path to read",
			},
			"required": []string{"path"},
		},
		{
			"name":        "write_file",
			"description": "Write content to a file",
			"parameters": map[string]string{
				"path":    "The file path to write",
				"content": "The content to write",
			},
			"required": []string{"path", "content"},
		},
		{
			"name":        "execute_command",
			"description": "Execute a shell command",
			"parameters": map[string]string{
				"command":     "The command to execute",
				"working_dir": "Working directory for the command",
			},
			"required": []string{"command"},
		},
		{
			"name":        "list_directory",
			"description": "List files in a directory",
			"parameters": map[string]string{
				"path": "The directory path to list",
			},
			"required": []string{"path"},
		},
		{
			"name":        "list_tools",
			"description": "List all available tools and their descriptions",
			"parameters":  map[string]string{},
			"required":    []string{},
		},
		{
			"name":        "git_grep",
			"description": "Search for a pattern in git repository files",
			"parameters": map[string]string{
				"pattern":          "The pattern to search for",
				"path":             "Limit search to specific path or file pattern",
				"case_insensitive": "Perform case-insensitive search",
				"line_number":      "Show line numbers in output",
				"count":            "Show only count of matching lines",
			},
			"required": []string{"pattern"},
		},
	}

	jsonData, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal tools to JSON: %w", err)
	}

	return string(jsonData), nil
}
