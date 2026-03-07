package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

// bgTask represents a background command execution.
type bgTask struct {
	ID        string
	Command   string
	StartedAt time.Time
	done      chan struct{}
	mu        sync.Mutex
	output    bytes.Buffer
	exitCode  int
	err       error
	finished  bool
}

var (
	bgTasksMu sync.Mutex
	bgTasks   = make(map[string]*bgTask)
	bgSeq     atomic.Int64
)

func nextBGID() string {
	return fmt.Sprintf("bg_%d", bgSeq.Add(1))
}

// ExecuteBackgroundCommand starts a command in the background and returns immediately.
func ExecuteBackgroundCommand(cfg *config.ToolsConfig, toolCallID string, argsJson string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args ExecuteCommandArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.CommandTimeout)

	cmd, backend, err := buildShellCommand(ctx, cfg, args, args.Command, args.WorkingDir)
	if err != nil {
		cancel()
		result.Error = err
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}

	task := &bgTask{
		ID:        nextBGID(),
		Command:   args.Command,
		StartedAt: time.Now(),
		done:      make(chan struct{}),
	}

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		cancel()
		result.Error = fmt.Errorf("failed to start command: %w", err)
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}

	slog.Info("background command started",
		slog.String("id", task.ID),
		slog.String("command", args.Command),
		slog.String("sandboxBackend", backend))

	bgTasksMu.Lock()
	bgTasks[task.ID] = task
	bgTasksMu.Unlock()

	// Read output in background.
	go func() {
		defer cancel()
		defer close(task.done)

		// Drain pipe into buffer.
		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			buf := make([]byte, 4096)
			for {
				n, rerr := pr.Read(buf)
				if n > 0 {
					task.mu.Lock()
					task.output.Write(buf[:n])
					// Cap buffer size to prevent unbounded growth.
					if task.output.Len() > cfg.MaxOutputSize*2 {
						// Keep last MaxOutputSize bytes.
						b := task.output.Bytes()
						keep := b[len(b)-cfg.MaxOutputSize:]
						task.output.Reset()
						task.output.WriteString("[earlier output truncated]\n")
						task.output.Write(keep)
					}
					task.mu.Unlock()
				}
				if rerr != nil {
					break
				}
			}
		}()

		waitErr := cmd.Wait()
		_ = pw.Close()
		<-readDone

		task.mu.Lock()
		task.finished = true
		task.err = waitErr
		if cmd.ProcessState != nil {
			task.exitCode = cmd.ProcessState.ExitCode()
		}
		task.mu.Unlock()

		slog.Info("background command finished",
			slog.String("id", task.ID),
			slog.String("command", args.Command),
			slog.Int("exitCode", task.exitCode))
	}()

	result.Output = fmt.Sprintf("Background command started.\nID: %s\nCommand: %s\nUse get_background_output with this ID to check output.", task.ID, args.Command)
	return result
}

type GetBackgroundOutputArgs struct {
	ID string `json:"id"`
}

// GetBackgroundOutput returns the current output of a background command.
func GetBackgroundOutput(_ *config.ToolsConfig, toolCallID string, argsJson string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args GetBackgroundOutputArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: %v", err)
		return result
	}

	bgTasksMu.Lock()
	task, ok := bgTasks[args.ID]
	bgTasksMu.Unlock()

	if !ok {
		result.Error = fmt.Errorf("background task %q not found", args.ID)
		result.Output = fmt.Sprintf("Error: background task %q not found", args.ID)
		return result
	}

	task.mu.Lock()
	output := task.output.String()
	finished := task.finished
	exitCode := task.exitCode
	taskErr := task.err
	task.mu.Unlock()

	var sb strings.Builder
	fmt.Fprintf(&sb, "ID: %s\n", task.ID)
	fmt.Fprintf(&sb, "Command: %s\n", task.Command)
	if finished {
		fmt.Fprintf(&sb, "Status: finished (exit code %d)\n", exitCode)
		if taskErr != nil {
			fmt.Fprintf(&sb, "Error: %v\n", taskErr)
		}
	} else {
		fmt.Fprintf(&sb, "Status: running (elapsed %s)\n", time.Since(task.StartedAt).Truncate(time.Second))
	}
	fmt.Fprintf(&sb, "---\n%s", output)

	result.Output = sb.String()
	return result
}

// ListBackgroundCommands lists all background commands and their status.
func ListBackgroundCommands(_ *config.ToolsConfig, toolCallID string, _ string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	bgTasksMu.Lock()
	tasks := make([]*bgTask, 0, len(bgTasks))
	for _, t := range bgTasks {
		tasks = append(tasks, t)
	}
	bgTasksMu.Unlock()

	if len(tasks) == 0 {
		result.Output = "No background commands."
		return result
	}

	var sb strings.Builder
	for _, t := range tasks {
		t.mu.Lock()
		finished := t.finished
		exitCode := t.exitCode
		outputLen := t.output.Len()
		t.mu.Unlock()

		if finished {
			fmt.Fprintf(&sb, "- %s [finished, exit %d] %s (%d bytes output)\n", t.ID, exitCode, t.Command, outputLen)
		} else {
			fmt.Fprintf(&sb, "- %s [running, %s] %s (%d bytes output)\n", t.ID, time.Since(t.StartedAt).Truncate(time.Second), t.Command, outputLen)
		}
	}

	result.Output = sb.String()
	return result
}
