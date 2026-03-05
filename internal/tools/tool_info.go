package tools

import (
	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

// ToolInfo represents the structure of a tool's metadata
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  api.FunctionParameters `json:"parameters"`
	callback    func(toolsConfig *config.ToolsConfig, toolCallID string, args string) api.ToolResult
}

// GetAllTools contains the metadata for all available tools
func GetAllTools() []ToolInfo {
	return []ToolInfo{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"path": {
						Type:        "string",
						Description: "The file path to read",
					},
				},
				Required: []string{"path"},
			},
			callback: ReadFile,
		},
		{
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"path": {
						Type:        "string",
						Description: "The file path to write",
					},
					"content": {
						Type:        "string",
						Description: "The content to write",
					},
				},
				Required: []string{"path", "content"},
			},
			callback: WriteFile,
		},
		{
			Name:        "execute_command",
			Description: "Execute a shell command",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"command": {
						Type:        "string",
						Description: "The command to execute",
					},
					"working_dir": {
						Type:        "string",
						Description: "Working directory for the command (optional)",
					},
					"sandbox_mode": {
						Type:        "string",
						Description: "Sandbox mode override for this command: 'auto' (default) or 'off'",
					},
				},
				Required: []string{"command"},
			},
			callback: ExecuteCommand,
		},
		{
			Name:        "list_directory",
			Description: "List files in a directory",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"path": {
						Type:        "string",
						Description: "The directory path to list",
					},
				},
				Required: []string{"path"},
			},
			callback: ListDirectory,
		},
		{
			Name:        "list_tools",
			Description: "List all available tools and their descriptions",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"format": {
						Type:        "string",
						Description: "Output format: 'text' (default) or 'json'",
					},
				},
				Required: []string{},
			},
			callback: ListTools,
		},
		{
			Name:        "spawn_subagent",
			Description: "Spawn a subagent with an initial prompt",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"prompt": {
						Type:        "string",
						Description: "Initial prompt for the subagent",
					},
				},
				Required: []string{"prompt"},
			},
			callback: SpawnSubagent,
		},
		{
			Name:        "send_subagent_input",
			Description: "Send additional input to an existing subagent",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"id": {
						Type:        "string",
						Description: "Subagent ID",
					},
					"input": {
						Type:        "string",
						Description: "Input text to send",
					},
				},
				Required: []string{"id", "input"},
			},
			callback: SendSubagentInput,
		},
		{
			Name:        "wait_subagent",
			Description: "Wait for a subagent to finish and return its latest status/output",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"id": {
						Type:        "string",
						Description: "Subagent ID",
					},
					"timeout_seconds": {
						Type:        "integer",
						Description: "How many seconds to wait (default: 30)",
					},
				},
				Required: []string{"id"},
			},
			callback: WaitSubagent,
		},
		{
			Name:        "list_subagents",
			Description: "List currently known subagents and their status",
			Parameters: api.FunctionParameters{
				Type:       "object",
				Properties: map[string]api.FunctionProperty{},
				Required:   []string{},
			},
			callback: ListSubagents,
		},
		{
			Name:        "close_subagent",
			Description: "Close a subagent and release its resources",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"id": {
						Type:        "string",
						Description: "Subagent ID",
					},
				},
				Required: []string{"id"},
			},
			callback: CloseSubagent,
		},
		{
			Name:        "git_grep",
			Description: "Search for a pattern in git repository files",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"pattern": {
						Type:        "string",
						Description: "The pattern to search for",
					},
					"path": {
						Type:        "string",
						Description: "Limit search to specific path or file pattern",
					},
					"case_insensitive": {
						Type:        "boolean",
						Description: "Perform case-insensitive search",
					},
					"line_number": {
						Type:        "boolean",
						Description: "Show line numbers in output",
					},
					"count": {
						Type:        "boolean",
						Description: "Show only count of matching lines",
					},
				},
				Required: []string{"pattern"},
			},
			callback: GitGrep,
		},
		{
			Name:        "git_ls_files",
			Description: "List files in git repository",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"cached": {
						Type:        "boolean",
						Description: "Show cached files",
					},
					"deleted": {
						Type:        "boolean",
						Description: "Show deleted files",
					},
					"modified": {
						Type:        "boolean",
						Description: "Show modified files",
					},
					"others": {
						Type:        "boolean",
						Description: "Show other (untracked) files",
					},
					"ignored": {
						Type:        "boolean",
						Description: "Show ignored files",
					},
					"stage": {
						Type:        "boolean",
						Description: "Show staged contents' object name",
					},
					"unmerged": {
						Type:        "boolean",
						Description: "Show unmerged files",
					},
					"killed": {
						Type:        "boolean",
						Description: "Show files that git checkout would overwrite",
					},
					"exclude_standard": {
						Type:        "boolean",
						Description: "Use standard git exclusions",
					},
					"full_name": {
						Type:        "boolean",
						Description: "Show full path from repository root",
					},
					"path": {
						Type:        "string",
						Description: "Limit to specific path or file pattern",
					},
				},
				Required: []string{},
			},
			callback: GitLsFiles,
		},
	}
}

func GetBuiltinTools() []api.Tool {
	srcTools := GetAllTools()
	tools := make([]api.Tool, 0, len(srcTools))
	for _, t := range srcTools {
		tools = append(tools, api.Tool{
			Type: "function",
			Function: api.FunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return tools
}
