package tools

import (
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func GitGrep(config *config.ToolsConfig, toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	// Get the pattern (required)
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		result.Error = fmt.Errorf("missing or invalid 'pattern' argument")
		result.Output = "Error: Missing or invalid 'pattern' argument"
		return result
	}

	// Build git grep command
	cmdArgs := []string{"grep"}

	// Add optional flags
	if caseInsensitive, ok := args["case_insensitive"].(bool); ok && caseInsensitive {
		cmdArgs = append(cmdArgs, "-i")
	}

	if lineNumber, ok := args["line_number"].(bool); ok && lineNumber {
		cmdArgs = append(cmdArgs, "-n")
	}

	if count, ok := args["count"].(bool); ok && count {
		cmdArgs = append(cmdArgs, "-c")
	}

	// Add the pattern
	cmdArgs = append(cmdArgs, pattern)

	// Add optional path
	if path, ok := args["path"].(string); ok && path != "" {
		cmdArgs = append(cmdArgs, "--", path)
	}

	// Execute git grep
	slog.Info("executing git grep",
		slog.String("pattern", pattern),
		slog.Any("args", cmdArgs))

	cmd := exec.Command("git", cmdArgs...)

	// Set timeout
	timer := time.NewTimer(config.CommandTimeout)
	defer timer.Stop()

	// Run command
	output := make(chan []byte, 1)
	errChan := make(chan error, 1)

	go func() {
		out, err := cmd.CombinedOutput()
		if err != nil {
			// git grep returns exit code 1 when no matches found - this is not an error
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				output <- []byte("No matches found")
			} else {
				errChan <- err
			}
		} else {
			output <- out
		}
	}()

	// Wait for completion or timeout
	select {
	case out := <-output:
		// Limit output size
		if len(out) > config.MaxOutputSize {
			out = out[:config.MaxOutputSize]
			out = append(out, []byte(fmt.Sprintf("\n\n[Output truncated at %d bytes]", config.MaxOutputSize))...)
		}
		result.Output = string(out)
		slog.Info("git grep completed", "outputLength", len(out))

	case err := <-errChan:
		result.Error = err
		result.Output = fmt.Sprintf("Git grep failed: %v", err)
		slog.Error("git grep failed", "error", err)

	case <-timer.C:
		// Kill the process on timeout
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				slog.Error("failed to kill git grep process after timeout",
					"error", err)
			}
		}
		result.Error = fmt.Errorf("git grep timed out after %v", config.CommandTimeout)
		result.Output = fmt.Sprintf("Error: Git grep timed out after %v", config.CommandTimeout)
		slog.Error("git grep timed out", "timeout", config.CommandTimeout)
	}

	return result
}
