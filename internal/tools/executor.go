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
	ResultStore    *ResultStore
}

// NewExecutor creates a new tool executor
func NewExecutor(cfg *config.ToolsConfig, store *ResultStore) *Executor {
	toolInfoByName := make(map[string]ToolInfo)
	for _, tool := range GetAllTools() {
		toolInfoByName[tool.Name] = tool
	}

	return &Executor{
		config:         cfg,
		toolInfoByName: toolInfoByName,
		ResultStore:    store,
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

	// get_tool_result is handled here directly because it needs access to ResultStore.
	if toolCall.Function.Name == "get_tool_result" {
		return e.executeGetToolResult(toolCall)
	}

	// Execute based on function name
	if tool, ok := e.toolInfoByName[toolCall.Function.Name]; ok {
		slog.Debug("Found tool info",
			slog.String("tool", tool.Name),
			slog.Any("args", toolCall.Function.Arguments))
		result = tool.callback(e.config, toolCall.ID, toolCall.Function.Arguments)
	} else {
		slog.Warn("Tool not found in tool info list",
			slog.String("tool", toolCall.Function.Name),
			slog.Any("args", toolCall.Function.Arguments))
		result.Error = fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
		result.Output = fmt.Sprintf("Error: Unknown tool '%s'", toolCall.Function.Name)
	}

	return result
}

func (e *Executor) executeGetToolResult(toolCall api.ToolCall) api.ToolResult {
	var args struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil || args.ID == "" {
		return api.ToolResult{
			ToolCallID: toolCall.ID,
			Output:     "Error: missing or invalid 'id' argument",
			Error:      fmt.Errorf("invalid arguments"),
		}
	}
	if e.ResultStore == nil {
		return api.ToolResult{
			ToolCallID: toolCall.ID,
			Output:     "Error: result store not available",
			Error:      fmt.Errorf("result store not available"),
		}
	}
	content, ok := e.ResultStore.Get(args.ID)
	if !ok {
		return api.ToolResult{
			ToolCallID: toolCall.ID,
			Output:     fmt.Sprintf("No result stored for id %q", args.ID),
			Error:      fmt.Errorf("result not found"),
		}
	}
	return api.ToolResult{
		ToolCallID: toolCall.ID,
		Output:     content,
	}
}
