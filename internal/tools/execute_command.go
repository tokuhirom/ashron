package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type ExecuteCommandArgs struct {
	Command     string `json:"command"`
	WorkingDir  string `json:"working_dir,omitempty"`
	SandboxMode string `json:"sandbox_mode,omitempty"`
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

	ctx, cancel := context.WithTimeout(context.Background(), config.CommandTimeout)
	defer cancel()

	cmd, backend, err := buildShellCommand(ctx, config, args, command, workingDir)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: %v", err)
		slog.Error("failed to build command", slog.Any("error", err))
		return result
	}

	slog.Info("executing command",
		slog.String("command", command),
		slog.String("workingDir", cmd.Dir),
		slog.String("sandboxBackend", backend))

	out, err := cmd.CombinedOutput()
	if len(out) > config.MaxOutputSize {
		out = out[:config.MaxOutputSize]
		out = append(out, []byte(fmt.Sprintf("\n\n[Output truncated at %d bytes]", config.MaxOutputSize))...)
	}
	result.Output = string(out)

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Errorf("command timed out after %v", config.CommandTimeout)
		if result.Output == "" {
			result.Output = fmt.Sprintf("Error: Command timed out after %v", config.CommandTimeout)
		}
		slog.Error("Command execution timed out",
			slog.String("command", command),
			slog.Duration("timeout", config.CommandTimeout))
		return result
	}

	if err != nil {
		result.Error = err
		if result.Output == "" {
			result.Output = fmt.Sprintf("Command failed: %v", err)
		}
		slog.Error("Command execution failed",
			slog.String("command", command),
			slog.String("error", err.Error()),
			slog.String("output", result.Output))
		return result
	}

	slog.Info("Command execution completed",
		slog.String("command", command),
		slog.String("output", result.Output))

	return result
}

func buildShellCommand(
	ctx context.Context,
	cfg *config.ToolsConfig,
	args ExecuteCommandArgs,
	command string,
	workingDir string,
) (*exec.Cmd, string, error) {
	sandboxMode := EffectiveSandboxMode(cfg, args)

	absWorkingDir, err := resolveWorkingDir(workingDir)
	if err != nil {
		return nil, "", err
	}

	if sandboxMode == "off" {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = absWorkingDir
		return cmd, "none", nil
	}

	switch runtime.GOOS {
	case "darwin":
		return buildDarwinSandboxCommand(ctx, command, absWorkingDir)
	case "linux":
		return buildLinuxSandboxCommand(ctx, command, absWorkingDir)
	default:
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = absWorkingDir
		return cmd, "none", nil
	}
}

func EffectiveSandboxMode(cfg *config.ToolsConfig, args ExecuteCommandArgs) string {
	if cfg.Yolo {
		return "off"
	}

	mode := strings.TrimSpace(strings.ToLower(cfg.SandboxMode))
	if mode == "" {
		mode = "auto"
	}

	override := strings.TrimSpace(strings.ToLower(args.SandboxMode))
	switch override {
	case "":
		return mode
	case "auto", "off":
		return override
	default:
		return mode
	}
}

func resolveWorkingDir(workingDir string) (string, error) {
	if workingDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to determine working directory: %w", err)
		}
		return wd, nil
	}
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}
	return absWorkingDir, nil
}

func buildDarwinSandboxCommand(ctx context.Context, command string, absWorkingDir string) (*exec.Cmd, string, error) {
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		return nil, "", fmt.Errorf("sandbox-exec is required on macOS but was not found in PATH")
	}

	profile := buildDarwinSandboxProfile(absWorkingDir)
	cmd := exec.CommandContext(ctx, "sandbox-exec", "-p", profile, "sh", "-c", command)
	cmd.Dir = absWorkingDir
	return cmd, "sandbox-exec", nil
}

func buildDarwinSandboxProfile(absWorkingDir string) string {
	escapedWorkingDir := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(absWorkingDir)
	return fmt.Sprintf(`(version 1)
(deny default)
(import "system.sb")
(allow process*)
(allow signal (target self))
(allow sysctl-read)
(allow network*)
(allow file-read*)
(allow file-write*
    (subpath "%s")
    (subpath "/tmp")
    (subpath "/private/tmp")
    (subpath "/var/tmp"))`, escapedWorkingDir)
}

func buildLinuxSandboxCommand(ctx context.Context, command string, absWorkingDir string) (*exec.Cmd, string, error) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		return nil, "", fmt.Errorf("bwrap is required on Linux but was not found in PATH")
	}

	args := []string{
		"--die-with-parent",
		"--new-session",
		"--proc", "/proc",
		"--dev", "/dev",
		"--ro-bind", "/", "/",
		"--bind", absWorkingDir, absWorkingDir,
		"--chdir", absWorkingDir,
		"--tmpfs", "/tmp",
		"--tmpfs", "/var/tmp",
		"--setenv", "HOME", absWorkingDir,
		"--setenv", "TMPDIR", "/tmp",
		"sh", "-c", command,
	}
	cmd := exec.CommandContext(ctx, "bwrap", args...)
	cmd.Dir = absWorkingDir
	return cmd, "bwrap", nil
}
