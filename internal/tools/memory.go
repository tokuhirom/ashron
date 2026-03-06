package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
	"github.com/tokuhirom/ashron/internal/memory"
)

type MemoryWriteArgs struct {
	Content string `json:"content"`
	Scope   string `json:"scope"` // "global" or "project" (default: "project")
}

// MemoryWrite overwrites the requested memory scope with the given content.
func MemoryWrite(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args MemoryWriteArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = err
		result.Output = "Error: " + err.Error()
		return result
	}

	scope := strings.ToLower(strings.TrimSpace(args.Scope))
	if scope == "" {
		scope = "project"
	}

	switch scope {
	case "global":
		if err := memory.WriteGlobal(args.Content); err != nil {
			result.Error = err
			result.Output = fmt.Sprintf("Error writing global memory: %v", err)
			return result
		}
		result.Output = fmt.Sprintf("Global memory updated (%s).", memory.GlobalPath())

	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			result.Error = err
			result.Output = "Error getting current directory: " + err.Error()
			return result
		}
		if err := memory.WriteProject(cwd, args.Content); err != nil {
			result.Error = err
			result.Output = fmt.Sprintf("Error writing project memory: %v", err)
			return result
		}
		result.Output = fmt.Sprintf("Project memory updated (%s).", memory.ProjectPath(cwd))

	default:
		result.Error = fmt.Errorf("unknown scope %q: must be \"global\" or \"project\"", scope)
		result.Output = result.Error.Error()
	}

	return result
}

// MemoryList returns the current contents of both memory scopes.
func MemoryList(_ *config.ToolsConfig, toolCallID string, _ string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	cwd, _ := os.Getwd()

	globalPath := memory.GlobalPath()
	projectPath := memory.ProjectPath(cwd)
	globalContent := memory.ReadGlobal()
	projectContent := memory.ReadProject(cwd)

	var sb strings.Builder

	fmt.Fprintf(&sb, "Global memory (%s):\n", globalPath)
	if globalContent == "" {
		sb.WriteString("  (empty)\n")
	} else {
		sb.WriteString(globalContent)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	fmt.Fprintf(&sb, "Project memory (%s):\n", projectPath)
	if projectContent == "" {
		sb.WriteString("  (empty)\n")
	} else {
		sb.WriteString(projectContent)
		sb.WriteString("\n")
	}

	result.Output = sb.String()
	return result
}
