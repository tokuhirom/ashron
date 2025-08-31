package tools

import (
	"encoding/json"

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

// BuiltinTools contains the definitions of built-in tools
// Internal tool definitions for file and command operations
var BuiltinTools = []api.Tool{
	{
		Type: "function",
		Function: api.FunctionDef{
			Name:        "read_file",
			Description: "Read the contents of a file",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "The file path to read"
					}
				},
				"required": ["path"]
			}`),
		},
	},
	{
		Type: "function",
		Function: api.FunctionDef{
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "The file path to write"
					},
					"content": {
						"type": "string",
						"description": "The content to write"
					}
				},
				"required": ["path", "content"]
			}`),
		},
	},
	{
		Type: "function",
		Function: api.FunctionDef{
			Name:        "execute_command",
			Description: "Execute a shell command",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {
						"type": "string",
						"description": "The command to execute"
					},
					"working_dir": {
						"type": "string",
						"description": "Working directory for the command"
					}
				},
				"required": ["command"]
			}`),
		},
	},
	{
		Type: "function",
		Function: api.FunctionDef{
			Name:        "list_directory",
			Description: "List files in a directory",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "The directory path to list"
					}
				},
				"required": ["path"]
			}`),
		},
	},
	{
		Type: "function",
		Function: api.FunctionDef{
			Name:        "list_tools",
			Description: "List all available tools and their descriptions",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"format": {
						"type": "string",
						"description": "Output format: 'text' (default) or 'json'"
					}
				},
				"required": []
			}`),
		},
	},
	{
		Type: "function",
		Function: api.FunctionDef{
			Name:        "git_grep",
			Description: "Search for a pattern in git repository files",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {
						"type": "string",
						"description": "The pattern to search for"
					},
					"path": {
						"type": "string",
						"description": "Limit search to specific path or file pattern"
					},
					"case_insensitive": {
						"type": "boolean",
						"description": "Perform case-insensitive search"
					},
					"line_number": {
						"type": "boolean",
						"description": "Show line numbers in output"
					},
					"count": {
						"type": "boolean",
						"description": "Show only count of matching lines"
					}
				},
				"required": ["pattern"]
			}`),
		},
	},
	{
		Type: "function",
		Function: api.FunctionDef{
			Name:        "git_ls_files",
			Description: "List files in git repository",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"cached": {
						"type": "boolean",
						"description": "Show cached files"
					},
					"deleted": {
						"type": "boolean",
						"description": "Show deleted files"
					},
					"modified": {
						"type": "boolean",
						"description": "Show modified files"
					},
					"others": {
						"type": "boolean",
						"description": "Show other (untracked) files"
					},
					"ignored": {
						"type": "boolean",
						"description": "Show ignored files"
					},
					"stage": {
						"type": "boolean",
						"description": "Show staged contents' object name"
					},
					"unmerged": {
						"type": "boolean",
						"description": "Show unmerged files"
					},
					"killed": {
						"type": "boolean",
						"description": "Show files that git checkout would overwrite"
					},
					"exclude_standard": {
						"type": "boolean",
						"description": "Use standard git exclusions"
					},
					"full_name": {
						"type": "boolean",
						"description": "Show full path from repository root"
					},
					"path": {
						"type": "string",
						"description": "Limit to specific path or file pattern"
					}
				},
				"required": []
			}`),
		},
	},
}
