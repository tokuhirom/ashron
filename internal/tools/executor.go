package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

// Executor handles tool execution
type Executor struct {
	config *config.ToolsConfig
}

// NewExecutor creates a new tool executor
func NewExecutor(cfg *config.ToolsConfig) *Executor {
	return &Executor{
		config: cfg,
	}
}

// Execute runs a tool call and returns the result
func (e *Executor) Execute(toolCall api.ToolCall) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCall.ID,
	}

	slog.Info("Executing tool", "tool", toolCall.Function.Name, "id", toolCall.ID)

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		slog.Error("Failed to parse tool arguments",
			slog.Any("error", err),
			slog.Any("arguments", toolCall.Function.Arguments))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	slog.Debug("Tool arguments parsed", "tool", toolCall.Function.Name, "args", args)

	// Execute based on function name
	switch toolCall.Function.Name {
	case "read_file":
		result = e.readFile(toolCall.ID, args)
	case "write_file":
		result = e.writeFile(toolCall.ID, args)
	case "execute_command":
		result = e.executeCommand(toolCall.ID, args)
	case "list_directory":
		result = e.listDirectory(toolCall.ID, args)
	case "init":
		result = e.generateAgentsMD(toolCall.ID, args)
	case "list_tools":
		result = e.listTools(toolCall.ID, args)
	default:
		slog.Error("Unknown tool requested", "tool", toolCall.Function.Name)
		result.Error = fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
		result.Output = fmt.Sprintf("Error: Unknown tool '%s'", toolCall.Function.Name)
	}

	if result.Error != nil {
		slog.Error("Tool execution failed", "tool", toolCall.Function.Name, "error", result.Error)
	} else {
		slog.Info("Tool execution completed", "tool", toolCall.Function.Name, "outputLength", len(result.Output))
	}

	return result
}

// readFile reads the contents of a file
func (e *Executor) readFile(toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	if !e.config.EnableFileOps {
		result.Error = fmt.Errorf("file operations are disabled")
		result.Output = "Error: File operations are disabled in configuration"
		return result
	}

	path, ok := args["path"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'path' argument")
		result.Output = "Error: Missing or invalid 'path' argument"
		return result
	}

	// Clean and validate path
	path = filepath.Clean(path)

	// Read file
	file, err := os.Open(path)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading file: %v", err)
		return result
	}
	defer file.Close()

	// Limit file size
	limited := &io.LimitedReader{
		R: file,
		N: int64(e.config.MaxOutputSize),
	}

	content, err := io.ReadAll(limited)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading file content: %v", err)
		return result
	}

	result.Output = string(content)
	if limited.N == 0 {
		result.Output += fmt.Sprintf("\n\n[File truncated at %d bytes]", e.config.MaxOutputSize)
	}

	return result
}

// writeFile writes content to a file
func (e *Executor) writeFile(toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	if !e.config.EnableFileOps {
		result.Error = fmt.Errorf("file operations are disabled")
		result.Output = "Error: File operations are disabled in configuration"
		return result
	}

	path, ok := args["path"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'path' argument")
		result.Output = "Error: Missing or invalid 'path' argument"
		return result
	}

	content, ok := args["content"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'content' argument")
		result.Output = "Error: Missing or invalid 'content' argument"
		return result
	}

	// Clean and validate path
	path = filepath.Clean(path)

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error creating directory: %v", err)
		return result
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error writing file: %v", err)
		return result
	}

	result.Output = fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)
	return result
}

// executeCommand runs a shell command
func (e *Executor) executeCommand(toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	if !e.config.EnableCommandExec {
		result.Error = fmt.Errorf("command execution is disabled")
		result.Output = "Error: Command execution is disabled in configuration"
		return result
	}

	command, ok := args["command"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'command' argument")
		result.Output = "Error: Missing or invalid 'command' argument"
		return result
	}

	// Get working directory if specified
	workingDir := ""
	if wd, ok := args["working_dir"].(string); ok {
		workingDir = wd
	}

	// Create command
	slog.Info("executing command by 'sh -c'",
		slog.String("command", command),
		slog.String("workingDir", workingDir))
	cmd := exec.Command("sh", "-c", command)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Set timeout
	timeout := time.Duration(e.config.CommandTimeout) * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Run command
	output := make(chan []byte, 1)
	errChan := make(chan error, 1)

	go func() {
		out, err := cmd.CombinedOutput()
		if err != nil {
			errChan <- err
		}
		output <- out
	}()

	// Wait for completion or timeout
	select {
	case out := <-output:
		// Limit output size
		if len(out) > e.config.MaxOutputSize {
			out = out[:e.config.MaxOutputSize]
			out = append(out, []byte(fmt.Sprintf("\n\n[Output truncated at %d bytes]", e.config.MaxOutputSize))...)
		}
		result.Output = string(out)
		
		// Log and display the command output
		slog.Info("Command execution completed",
			slog.String("command", command),
			slog.String("output", result.Output))
		fmt.Fprintf(os.Stderr, "Command output:\n%s\n", result.Output)

	case err := <-errChan:
		result.Error = err
		result.Output = fmt.Sprintf("Command failed: %v", err)
		
		// Log and display the error
		slog.Error("Command execution failed",
			slog.String("command", command),
			slog.String("error", err.Error()))
		fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)

	case <-timer.C:
		// Kill the process on timeout
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		result.Error = fmt.Errorf("command timed out after %v", timeout)
		result.Output = fmt.Sprintf("Error: Command timed out after %v", timeout)
		
		// Log and display the timeout
		slog.Error("Command execution timed out",
			slog.String("command", command),
			slog.Duration("timeout", timeout))
		fmt.Fprintf(os.Stderr, "Command timed out after %v\n", timeout)
	}

	return result
}

// listDirectory lists files in a directory
func (e *Executor) listDirectory(toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	if !e.config.EnableFileOps {
		result.Error = fmt.Errorf("file operations are disabled")
		result.Output = "Error: File operations are disabled in configuration"
		return result
	}

	path, ok := args["path"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'path' argument")
		result.Output = "Error: Missing or invalid 'path' argument"
		return result
	}

	// Clean and validate path
	path = filepath.Clean(path)

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading directory: %v", err)
		return result
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Contents of %s:\n", path))

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Format entry
		typeStr := "file"
		if entry.IsDir() {
			typeStr = "dir "
		}

		output.WriteString(fmt.Sprintf("  [%s] %s (%d bytes) %s\n",
			typeStr,
			entry.Name(),
			info.Size(),
			info.ModTime().Format("2006-01-02 15:04:05"),
		))
	}

	result.Output = output.String()
	return result
}

// generateAgentsMD generates an AGENTS.md file for the project
func (e *Executor) generateAgentsMD(toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	// Get current directory as root path
	rootPath := "."

	// Generate AGENTS.md
	content, err := GenerateAgentsMD(rootPath)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error generating AGENTS.md: %v", err)
		return result
	}

	result.Output = fmt.Sprintf("Successfully generated AGENTS.md\n\n%s", content)
	return result
}

// listTools lists all available tools
func (e *Executor) listTools(toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	// Check if JSON format is requested
	format := ""
	if f, ok := args["format"].(string); ok {
		format = f
	}

	var output string
	var err error

	if format == "json" {
		output, err = ListToolsJSON()
	} else {
		output, err = ListTools()
	}

	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error listing tools: %v", err)
		return result
	}

	result.Output = output
	return result
}

// IsAutoApproved checks if a tool is auto-approved
func (e *Executor) IsAutoApproved(toolName string) bool {
	// This is now handled by the config, but kept for compatibility
	return false
}