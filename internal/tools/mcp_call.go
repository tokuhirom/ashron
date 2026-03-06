package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
	"github.com/tokuhirom/ashron/internal/mcp"
)

type MCPCallArgs struct {
	Server    string          `json:"server"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

func MCPCall(cfg *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args MCPCallArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}
	if args.Server == "" || args.Tool == "" {
		result.Error = fmt.Errorf("server and tool are required")
		result.Output = "Error: server and tool are required"
		return result
	}

	if cfg == nil {
		result.Error = fmt.Errorf("tools config is missing")
		result.Output = "Error: tools config is missing"
		return result
	}
	serverCfg, ok := cfg.MCPServers[args.Server]
	if !ok {
		result.Error = fmt.Errorf("unknown mcp server: %s", args.Server)
		result.Output = fmt.Sprintf("Error: unknown mcp server '%s'", args.Server)
		return result
	}

	output, err := mcp.CallTool(context.Background(), serverCfg, args.Tool, args.Arguments)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error calling MCP tool: %v", err)
		return result
	}
	result.Output = output
	return result
}
