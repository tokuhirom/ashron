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
			Name:        "mcp_call",
			Description: "Call a tool on a configured external MCP server",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"server": {
						Type:        "string",
						Description: "MCP server name from config mcp_servers",
					},
					"tool": {
						Type:        "string",
						Description: "Tool name exposed by the target MCP server",
					},
					"arguments": {
						Type:        "object",
						Description: "Tool arguments object passed to MCP tools/call",
					},
				},
				Required: []string{"server", "tool"},
			},
			callback: MCPCall,
		},
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
			Name:        "read_skill",
			Description: "Read the full SKILL.md content for an installed skill by name",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"name": {
						Type:        "string",
						Description: "Skill name from discovered skills list",
					},
				},
				Required: []string{"name"},
			},
			callback: ReadSkill,
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
			Name:        "apply_patch",
			Description: "Safely apply minimal patch hunks to a file with backup and failure hints",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"path": {
						Type:        "string",
						Description: "The file path to patch",
					},
					"patch": {
						Type:        "string",
						Description: "Unified diff hunks (lines starting with @@ ... @@ and +/ -/ context lines)",
					},
				},
				Required: []string{"path", "patch"},
			},
			callback: ApplyPatch,
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
			Name:        "get_subagent_log",
			Description: "Get the current (possibly partial) output log of a subagent. Works while the subagent is still running.",
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
			callback: GetSubagentLogTool,
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
			Name:        "fetch_url",
			Description: "Fetch the content of a URL. HTML pages are automatically converted to plain text.",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"url": {
						Type:        "string",
						Description: "The URL to fetch",
					},
					"raw": {
						Type:        "boolean",
						Description: "Return raw content without stripping HTML tags (default: false)",
					},
					"timeout_seconds": {
						Type:        "integer",
						Description: "Request timeout in seconds (default: 30)",
					},
				},
				Required: []string{"url"},
			},
			callback: FetchURL,
		},
		{
			Name:        "memory_write",
			Description: "Save or update persistent memory that survives across sessions. Use scope=\"global\" for notes that apply to all projects, or scope=\"project\" (default) for notes specific to the current project. The content completely replaces the existing memory for that scope.",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"content": {
						Type:        "string",
						Description: "Markdown content to store. Completely replaces the current memory for the scope.",
					},
					"scope": {
						Type:        "string",
						Description: "\"global\" (all projects) or \"project\" (this project only, default)",
					},
				},
				Required: []string{"content"},
			},
			callback: MemoryWrite,
		},
		{
			Name:        "memory_list",
			Description: "Show the current contents of both global and project memory files, along with their file paths.",
			Parameters: api.FunctionParameters{
				Type:       "object",
				Properties: map[string]api.FunctionProperty{},
				Required:   []string{},
			},
			callback: MemoryList,
		},
		{
			Name:        "get_diagnostics",
			Description: "Get language server diagnostics (errors, warnings) for a source file. Requires the appropriate language server to be installed (gopls for Go, pyright for Python, typescript-language-server for TS/JS, rust-analyzer for Rust, clangd for C/C++).",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"path": {
						Type:        "string",
						Description: "Path to the source file to check",
					},
				},
				Required: []string{"path"},
			},
			callback: GetDiagnostics,
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
