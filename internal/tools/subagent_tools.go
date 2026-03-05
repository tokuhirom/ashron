package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type SpawnSubagentArgs struct {
	Prompt string `json:"prompt"`
}

type SubagentIDArgs struct {
	ID string `json:"id"`
}

type SendSubagentInputArgs struct {
	ID    string `json:"id"`
	Input string `json:"input"`
}

type WaitSubagentArgs struct {
	ID             string `json:"id"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func SpawnSubagent(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}
	mgr := getSubagentManager()
	if mgr == nil {
		result.Error = fmt.Errorf("subagent runtime is not configured")
		result.Output = "Error: subagent runtime is not configured"
		return result
	}

	var args SpawnSubagentArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: invalid arguments - %v", err)
		return result
	}

	id, err := mgr.Spawn(args.Prompt)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}
	result.Output = fmt.Sprintf(`{"id":"%s","status":"running"}`, id)
	return result
}

func SendSubagentInput(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}
	mgr := getSubagentManager()
	if mgr == nil {
		result.Error = fmt.Errorf("subagent runtime is not configured")
		result.Output = "Error: subagent runtime is not configured"
		return result
	}

	var args SendSubagentInputArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: invalid arguments - %v", err)
		return result
	}

	if err := mgr.SendInput(args.ID, args.Input); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}
	result.Output = fmt.Sprintf(`{"id":"%s","status":"running"}`, args.ID)
	return result
}

func WaitSubagent(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}
	mgr := getSubagentManager()
	if mgr == nil {
		result.Error = fmt.Errorf("subagent runtime is not configured")
		result.Output = "Error: subagent runtime is not configured"
		return result
	}

	var args WaitSubagentArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: invalid arguments - %v", err)
		return result
	}
	timeout := time.Duration(args.TimeoutSeconds) * time.Second
	snap, timedOut, err := mgr.Wait(args.ID, timeout)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}
	payload := map[string]any{
		"id":         snap.ID,
		"status":     snap.Status,
		"timed_out":  timedOut,
		"output":     snap.LastOutput,
		"error":      snap.LastError,
		"created_at": snap.CreatedAt,
		"updated_at": snap.UpdatedAt,
	}
	b, _ := json.Marshal(payload)
	result.Output = string(b)
	return result
}

func ListSubagents(_ *config.ToolsConfig, toolCallID string, _ string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}
	mgr := getSubagentManager()
	if mgr == nil {
		result.Error = fmt.Errorf("subagent runtime is not configured")
		result.Output = "Error: subagent runtime is not configured"
		return result
	}
	b, err := json.Marshal(mgr.List())
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}
	result.Output = string(b)
	return result
}

func CloseSubagent(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}
	mgr := getSubagentManager()
	if mgr == nil {
		result.Error = fmt.Errorf("subagent runtime is not configured")
		result.Output = "Error: subagent runtime is not configured"
		return result
	}

	var args SubagentIDArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: invalid arguments - %v", err)
		return result
	}

	if err := mgr.Close(args.ID); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}
	result.Output = fmt.Sprintf(`{"id":"%s","closed":true}`, args.ID)
	return result
}
