package tools

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func GitLsFiles(config *config.ToolsConfig, toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	// Build git ls-files command
	cmdArgs := []string{"ls-files"}

	// Add optional flags
	if cached, ok := args["cached"].(bool); ok && cached {
		cmdArgs = append(cmdArgs, "--cached")
	}

	if deleted, ok := args["deleted"].(bool); ok && deleted {
		cmdArgs = append(cmdArgs, "--deleted")
	}

	if modified, ok := args["modified"].(bool); ok && modified {
		cmdArgs = append(cmdArgs, "--modified")
	}

	if others, ok := args["others"].(bool); ok && others {
		cmdArgs = append(cmdArgs, "--others")
	}

	if ignored, ok := args["ignored"].(bool); ok && ignored {
		cmdArgs = append(cmdArgs, "--ignored")
	}

	if stage, ok := args["stage"].(bool); ok && stage {
		cmdArgs = append(cmdArgs, "--stage")
	}

	if unmerged, ok := args["unmerged"].(bool); ok && unmerged {
		cmdArgs = append(cmdArgs, "--unmerged")
	}

	if killed, ok := args["killed"].(bool); ok && killed {
		cmdArgs = append(cmdArgs, "--killed")
	}

	if excludeStandard, ok := args["exclude_standard"].(bool); ok && excludeStandard {
		cmdArgs = append(cmdArgs, "--exclude-standard")
	}

	if fullName, ok := args["full_name"].(bool); ok && fullName {
		cmdArgs = append(cmdArgs, "--full-name")
	}

	// Add optional path
	if path, ok := args["path"].(string); ok && path != "" {
		cmdArgs = append(cmdArgs, "--", path)
	}

	// Execute git ls-files
	slog.Info("executing git ls-files",
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
			// git ls-files returns exit code 128 for non-git directories
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 128 {
				errChan <- fmt.Errorf("not in a git repository")
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
		if result.Output == "" {
			result.Output = "No files found"
		}
		slog.Info("git ls-files completed", "outputLength", len(out))

	case err := <-errChan:
		result.Error = err
		result.Output = fmt.Sprintf("Git ls-files failed: %v", err)
		slog.Error("git ls-files failed", "error", err)

	case <-timer.C:
		// Kill the process on timeout
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				slog.Error("failed to kill git ls-files process after timeout",
					"error", err)
			}
		}
		result.Error = fmt.Errorf("git ls-files timed out after %v", config.CommandTimeout)
		result.Output = fmt.Sprintf("Error: Git ls-files timed out after %v", config.CommandTimeout)
		slog.Error("git ls-files timed out", "timeout", config.CommandTimeout)
	}

	return result
}
