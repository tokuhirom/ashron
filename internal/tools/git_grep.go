package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type GitGrepArgs struct {
	Pattern         string `json:"pattern"`                    // The search pattern (required)
	CaseInsensitive bool   `json:"case_insensitive,omitempty"` // Perform case-insensitive matching
	LineNumber      bool   `json:"line_number,omitempty"`      // Show line numbers in output
	Count           bool   `json:"count,omitempty"`            // Show only count of matching lines
	Path            string `json:"path,omitempty"`             // Limit search to specific path or file pattern
}

func GitGrep(config *config.ToolsConfig, toolCallID string, argsJson string) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	var args GitGrepArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		slog.Error("Failed to parse tool arguments",
			slog.Any("error", err),
			slog.Any("arguments", argsJson))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	pattern := args.Pattern

	// Build git grep command
	cmdArgs := []string{"grep"}
	if args.CaseInsensitive {
		cmdArgs = append(cmdArgs, "-i")
	}
	if args.LineNumber {
		cmdArgs = append(cmdArgs, "-n")
	}
	if args.Count {
		cmdArgs = append(cmdArgs, "-c")
	}

	// Add the pattern
	cmdArgs = append(cmdArgs, pattern)

	// Add an optional path
	if args.Path != "" {
		cmdArgs = append(cmdArgs, "--", args.Path)
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
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				output <- []byte("No matches found")
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
