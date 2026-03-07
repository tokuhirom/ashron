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
			Name:        "get_tool_result",
			Description: "Retrieve the full output of a previous tool call by its ID. Tool results from earlier turns are stored by ID to save tokens; use this when you need the complete content.",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"id": {
						Type:        "string",
						Description: "The tool call ID whose result to retrieve",
					},
				},
				Required: []string{"id"},
			},
			callback: nil, // handled directly in Executor.Execute
		},
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
			Name:        "search_and_replace",
			Description: "Replace all occurrences of a string in a file",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"path": {
						Type:        "string",
						Description: "The file path to edit",
					},
					"search": {
						Type:        "string",
						Description: "The literal string to search for",
					},
					"replace": {
						Type:        "string",
						Description: "The replacement string",
					},
				},
				Required: []string{"path", "search", "replace"},
			},
			callback: SearchAndReplace,
		},
		{
			Name:        "replace_range",
			Description: "Replace a 1-based inclusive line range in a file",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"path": {
						Type:        "string",
						Description: "The file path to edit",
					},
					"start_line": {
						Type:        "integer",
						Description: "Start line number (1-based, inclusive)",
					},
					"end_line": {
						Type:        "integer",
						Description: "End line number (1-based, inclusive)",
					},
					"content": {
						Type:        "string",
						Description: "Replacement content for that line range",
					},
				},
				Required: []string{"path", "start_line", "end_line", "content"},
			},
			callback: ReplaceRange,
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
			Name:        "execute_background_command",
			Description: "Start a shell command in the background. Returns immediately with a task ID. Use get_background_output to check output later, or list_background_commands to see all tasks.",
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
			callback: ExecuteBackgroundCommand,
		},
		{
			Name:        "get_background_output",
			Description: "Get the current output of a background command. Works while the command is still running.",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"id": {
						Type:        "string",
						Description: "Background task ID returned by execute_background_command",
					},
				},
				Required: []string{"id"},
			},
			callback: GetBackgroundOutput,
		},
		{
			Name:        "list_background_commands",
			Description: "List all background commands and their status",
			Parameters: api.FunctionParameters{
				Type:       "object",
				Properties: map[string]api.FunctionProperty{},
				Required:   []string{},
			},
			callback: ListBackgroundCommands,
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
			Name:        "scratchpad_write",
			Description: "Write a note to the session scratchpad. Scratchpad entries survive context compaction but are discarded when the session ends. Use this to track progress, decisions, file changes, and other notes during long tasks.",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"key": {
						Type:        "string",
						Description: "A short label for this note (e.g. \"progress\", \"decisions\", \"changed_files\")",
					},
					"content": {
						Type:        "string",
						Description: "The note content (markdown). Replaces any existing content for this key.",
					},
				},
				Required: []string{"key", "content"},
			},
			callback: ScratchpadWrite,
		},
		{
			Name:        "scratchpad_read",
			Description: "Read scratchpad entries. If key is omitted, returns all entries. Use this to recall progress notes after context compaction.",
			Parameters: api.FunctionParameters{
				Type: "object",
				Properties: map[string]api.FunctionProperty{
					"key": {
						Type:        "string",
						Description: "Key to read (optional — omit to list all entries)",
					},
				},
				Required: []string{},
			},
			callback: ScratchpadRead,
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
