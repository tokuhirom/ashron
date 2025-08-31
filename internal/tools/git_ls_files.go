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

type GitLsFilesArgs struct {
	Cached          bool   `json:"cached,omitempty"`           // Show cached files
	Deleted         bool   `json:"deleted,omitempty"`          // Show deleted files
	Modified        bool   `json:"modified,omitempty"`         // Show modified files
	Others          bool   `json:"others,omitempty"`           // Show untracked files
	Ignored         bool   `json:"ignored,omitempty"`          // Show ignored files
	Stage           bool   `json:"stage,omitempty"`            // Show staged files
	Unmerged        bool   `json:"unmerged,omitempty"`         // Show unmerged files
	Killed          bool   `json:"killed,omitempty"`           // Show files that git checkout would overwrite
	ExcludeStandard bool   `json:"exclude_standard,omitempty"` // Use standard git exclusions
	FullName        bool   `json:"full_name,omitempty"`        // Show full path from repository root
	Path            string `json:"path,omitempty"`             // Limit to specific path or file pattern
}

func GitLsFiles(config *config.ToolsConfig, toolCallID string, argsJson string) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	var args GitLsFilesArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		slog.Error("Failed to parse tool arguments",
			slog.Any("error", err),
			slog.Any("arguments", argsJson))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	// Build git ls-files command
	cmdArgs := []string{"ls-files"}

	// Add optional flags
	if args.Cached {
		cmdArgs = append(cmdArgs, "--cached")
	}
	if args.Deleted {
		cmdArgs = append(cmdArgs, "--deleted")
	}
	if args.Modified {
		cmdArgs = append(cmdArgs, "--modified")
	}
	if args.Others {
		cmdArgs = append(cmdArgs, "--others")
	}
	if args.Ignored {
		cmdArgs = append(cmdArgs, "--ignored")
	}
	if args.Stage {
		cmdArgs = append(cmdArgs, "--stage")
	}
	if args.Unmerged {
		cmdArgs = append(cmdArgs, "--unmerged")
	}
	if args.Killed {
		cmdArgs = append(cmdArgs, "--killed")
	}
	if args.ExcludeStandard {
		cmdArgs = append(cmdArgs, "--exclude-standard")
	}
	if args.FullName {
		cmdArgs = append(cmdArgs, "--full-name")
	}

	// Add optional path
	if args.Path != "" {
		cmdArgs = append(cmdArgs, "--", args.Path)
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
