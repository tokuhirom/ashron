package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

// Executor handles tool execution
type Executor struct {
	config         *config.ToolsConfig
	toolInfoByName map[string]ToolInfo
}

// NewExecutor creates a new tool executor
func NewExecutor(cfg *config.ToolsConfig) *Executor {
	toolInfoByName := make(map[string]ToolInfo)
	for _, tool := range GetAllTools() {
		toolInfoByName[tool.Name] = tool
	}

	return &Executor{
		config:         cfg,
		toolInfoByName: toolInfoByName,
	}
}

// Execute runs a tool call and returns the result
func (e *Executor) Execute(toolCall api.ToolCall) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCall.ID,
	}

	slog.Info("Executing tool",
		slog.String("tool", toolCall.Function.Name),
		slog.String("id", toolCall.ID))

	// Parse arguments
	// TODO: don't parse here. parse it in the each tool function
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		slog.Error("Failed to parse tool arguments",
			slog.Any("error", err),
			slog.Any("arguments", toolCall.Function.Arguments))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	// Execute based on function name
	if tool, ok := e.toolInfoByName[toolCall.Function.Name]; ok {
		slog.Debug("Found tool info",
			slog.String("tool", tool.Name),
			slog.Any("args", args))
		result = tool.callback(e.config, toolCall.ID, args)
	} else {
		slog.Warn("Tool not found in tool info list",
			slog.String("tool", toolCall.Function.Name),
			slog.Any("args", args))
		result.Error = fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
		result.Output = fmt.Sprintf("Error: Unknown tool '%s'", toolCall.Function.Name)
	}

	if result.Error != nil {
		slog.Error("Tool execution failed",
			slog.String("tool", toolCall.Function.Name),
			slog.Any("error", result.Error))
	}

	return result
}

// IsAutoApproved checks if a tool is auto-approved
func (e *Executor) IsAutoApproved(toolName string) bool {
	// This is now handled by the config, but kept for compatibility
	return false
}
