package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type ExecuteCommandArgs struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir,omitempty"`
}

func ExecuteCommand(config *config.ToolsConfig, toolCallID string, argsJson string) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	var args ExecuteCommandArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		slog.Error("Failed to parse tool arguments",
			slog.Any("error", err),
			slog.Any("arguments", args))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	command := args.Command
	workingDir := args.WorkingDir

	// Create command
	slog.Info("executing command by 'sh -c'",
		slog.String("command", command),
		slog.String("workingDir", workingDir))
	cmd := exec.Command("sh", "-c", command)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Set timeout
	timer := time.NewTimer(config.CommandTimeout)
	defer timer.Stop()

	// Run command
	output := make(chan []byte, 1)
	errChan := make(chan error, 1)

	go func() {
		out, err := cmd.CombinedOutput()
		output <- out
		if err != nil {
			errChan <- err
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

		// Log the command output
		slog.Info("Command execution completed",
			slog.String("command", command),
			slog.String("output", result.Output))

		return result

	case err := <-errChan:
		// Also get the output even when command fails
		select {
		case out := <-output:
			if len(out) > config.MaxOutputSize {
				out = out[:config.MaxOutputSize]
				out = append(out, []byte(fmt.Sprintf("\n\n[Output truncated at %d bytes]", config.MaxOutputSize))...)
			}
			result.Output = string(out)
			if result.Output == "" {
				result.Output = fmt.Sprintf("Command failed: %v", err)
			}
		default:
			result.Output = fmt.Sprintf("Command failed: %v", err)
		}
		result.Error = err

		// Log the error with output
		slog.Error("Command execution failed",
			slog.String("command", command),
			slog.String("error", err.Error()),
			slog.String("output", result.Output))

		return result

	case <-timer.C:
		// Kill the process on timeout
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				slog.Error("failed to kill command process after timeout",
					slog.String("command", command),
					slog.Any("error", err))
			}
		}
		result.Error = fmt.Errorf("command timed out after %v", config.CommandTimeout)
		result.Output = fmt.Sprintf("Error: Command timed out after %v", config.CommandTimeout)

		// Log the timeout
		slog.Error("Command execution timed out",
			slog.String("command", command),
			slog.Duration("timeout", config.CommandTimeout))

		return result
	}
}
