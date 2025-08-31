package tools

import (
	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

// ToolInfo represents the structure of a tool's metadata
type ToolInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Required    []string          `json:"required,omitempty"`
	callback    func(toolsConfig *config.ToolsConfig, toolCallID string, args map[string]interface{}) api.ToolResult
}

// GetAllTools contains the metadata for all available tools
func GetAllTools() []ToolInfo {
	return []ToolInfo{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			Parameters: map[string]string{
				"path": "The file path to read",
			},
			Required: []string{"path"},
			callback: ReadFile,
		},
		{
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters: map[string]string{
				"path":    "The file path to write",
				"content": "The content to write",
			},
			Required: []string{"path", "content"},
			callback: WriteFile,
		},
		{
			Name:        "execute_command",
			Description: "Execute a shell command",
			Parameters: map[string]string{
				"command":     "The command to execute",
				"working_dir": "Working directory for the command (optional)",
			},
			Required: []string{"command"},
			callback: ExecuteCommand,
		},
		{
			Name:        "list_directory",
			Description: "List files in a directory",
			Parameters: map[string]string{
				"path": "The directory path to list",
			},
			Required: []string{"path"},
			callback: ListDirectory,
		},
		{
			Name:        "list_tools",
			Description: "List all available tools and their descriptions",
			Parameters:  map[string]string{},
			Required:    []string{},
			callback:    ListTools,
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
			callback: GitGrep,
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
			callback: GitLsFiles,
		},
	}
}
